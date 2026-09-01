package booking

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	calendarprovider "github.com/NerdsWhoFish/bookings/internal/calendar"
	"github.com/NerdsWhoFish/bookings/internal/domain"
	"github.com/NerdsWhoFish/bookings/internal/store"
)

func TestCreateClaimsSlotAndRejectsDuplicate(t *testing.T) {
	meeting := testMeeting()
	data := store.NewMemory([]domain.MeetingType{meeting})
	service := NewService(data, calendarprovider.Mock{})
	service.now = func() time.Time { return time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC) }
	start := time.Date(2026, 9, 2, 9, 0, 0, 0, time.FixedZone("EDT", -4*60*60))
	request := Request{MeetingTypeSlug: meeting.Slug, Start: start, GuestName: "Ada Angler", GuestEmail: "ada@example.com"}

	confirmation, err := service.Create(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if confirmation.Booking.Status != "confirmed" || confirmation.CancelToken == "" {
		t.Fatalf("unexpected confirmation: %#v", confirmation)
	}
	if _, err := service.Create(context.Background(), request); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	if err := service.Cancel(context.Background(), confirmation.Booking.ID, confirmation.CancelToken); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(context.Background(), request); err != nil {
		t.Fatalf("expected released slot to be bookable, got %v", err)
	}
}

func TestCreateRejectsMalformedEmail(t *testing.T) {
	meeting := testMeeting()
	service := NewService(store.NewMemory([]domain.MeetingType{meeting}), calendarprovider.Mock{})
	service.now = func() time.Time { return time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC) }
	_, err := service.Create(context.Background(), Request{
		MeetingTypeSlug: meeting.Slug,
		Start:           time.Date(2026, 9, 2, 9, 0, 0, 0, time.FixedZone("EDT", -4*60*60)),
		GuestName:       "Ada",
		GuestEmail:      "Ada <ada@example.com>",
	})
	if err == nil {
		t.Fatal("expected malformed email to fail")
	}
}

func TestAvailabilityChecksOnlyOrganizerAndSelectedAttendeeAccounts(t *testing.T) {
	meeting := testMeeting()
	meeting.DestinationConnectionID = "organizer"
	meeting.AttendeeEmails = []string{"teammate@example.com"}
	data := store.NewMemory([]domain.MeetingType{meeting})
	for _, connection := range []domain.CalendarConnection{
		{ID: "organizer", Email: "owner@example.com"},
		{ID: "teammate", Email: "teammate@example.com"},
		{ID: "unrelated", Email: "other@example.com"},
	} {
		if err := data.PutConnection(context.Background(), connection); err != nil {
			t.Fatal(err)
		}
	}
	provider := &capturingProvider{}
	service := NewService(data, provider)
	service.now = func() time.Time { return time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC) }
	if _, err := service.Availability(context.Background(), meeting, service.now()); err != nil {
		t.Fatal(err)
	}
	selected := make(map[string]bool, len(provider.connections))
	for _, connection := range provider.connections {
		selected[connection.ID] = true
	}
	if len(provider.connections) != 2 || !selected["organizer"] || !selected["teammate"] {
		t.Fatalf("unexpected relevant connections: %#v", provider.connections)
	}
}

func TestAvailabilityIncludesExternallyPushedBlocks(t *testing.T) {
	meeting := testMeeting()
	data := store.NewMemory([]domain.MeetingType{meeting})
	blockedStart := time.Date(2026, 9, 2, 9, 0, 0, 0, time.FixedZone("EDT", -4*60*60))
	if err := data.PutExternalBlock(context.Background(), domain.ExternalBlock{ID: "work:event", Start: blockedStart, End: blockedStart.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	service := NewService(data, calendarprovider.Mock{})
	service.now = func() time.Time { return time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC) }
	slots, err := service.Availability(context.Background(), meeting, service.now())
	if err != nil {
		t.Fatal(err)
	}
	for _, slot := range slots {
		if slot.Start.Equal(blockedStart) {
			t.Fatalf("external block did not remove slot: %#v", slot)
		}
	}
}

func TestCreateAndCancelManageSeparatePrivateBlockEvents(t *testing.T) {
	meeting := testMeeting()
	meeting.BlockerEmails = []string{"work@example.com", "other-work@example.com"}
	data := store.NewMemory([]domain.MeetingType{meeting})
	provider := &capturingProvider{failBlockAt: -1}
	service := NewService(data, provider)
	service.now = func() time.Time { return time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC) }
	start := time.Date(2026, 9, 2, 9, 0, 0, 0, time.FixedZone("EDT", -4*60*60))
	confirmation, err := service.Create(context.Background(), Request{MeetingTypeSlug: meeting.Slug, Start: start, GuestName: "Ada", GuestEmail: "ada@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	stored, err := data.GetBooking(context.Background(), confirmation.Booking.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.ShadowEventIDs) != 2 || len(provider.blockEmails) != 2 {
		t.Fatalf("private block events were not recorded: %#v, %#v", stored.ShadowEventIDs, provider.blockEmails)
	}
	if err := service.Cancel(context.Background(), confirmation.Booking.ID, confirmation.CancelToken); err != nil {
		t.Fatal(err)
	}
	if len(provider.deleted) != 3 {
		t.Fatalf("expected guest and private events to be deleted, got %#v", provider.deleted)
	}
}

func TestPrivateBlockFailureCompensatesGuestEventAndSlot(t *testing.T) {
	meeting := testMeeting()
	meeting.BlockerEmails = []string{"work@example.com"}
	data := store.NewMemory([]domain.MeetingType{meeting})
	provider := &capturingProvider{failBlockAt: 0}
	service := NewService(data, provider)
	service.now = func() time.Time { return time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC) }
	request := Request{MeetingTypeSlug: meeting.Slug, Start: time.Date(2026, 9, 2, 9, 0, 0, 0, time.FixedZone("EDT", -4*60*60)), GuestName: "Ada", GuestEmail: "ada@example.com"}
	if _, err := service.Create(context.Background(), request); err == nil {
		t.Fatal("expected private block failure")
	}
	if len(provider.deleted) != 1 || provider.deleted[0] != "main" {
		t.Fatalf("guest event was not compensated: %#v", provider.deleted)
	}
	provider.failBlockAt = -1
	if _, err := service.Create(context.Background(), request); err != nil {
		t.Fatalf("slot was not released after compensation: %v", err)
	}
}

type capturingProvider struct {
	connections []domain.CalendarConnection
	blockEmails []string
	deleted     []string
	failBlockAt int
}

func (p *capturingProvider) Busy(_ context.Context, connections []domain.CalendarConnection, _, _ time.Time) ([]domain.BusyPeriod, error) {
	p.connections = append([]domain.CalendarConnection(nil), connections...)
	return []domain.BusyPeriod{}, nil
}

func (*capturingProvider) Calendars(context.Context, domain.CalendarConnection) ([]domain.CalendarInfo, error) {
	return []domain.CalendarInfo{}, nil
}

func (*capturingProvider) CreateEvent(context.Context, domain.CalendarConnection, string, domain.Booking, domain.MeetingType) (string, error) {
	return "main", nil
}

func (p *capturingProvider) CreateBlockEvent(_ context.Context, _ domain.CalendarConnection, _ string, _ domain.Booking, _ domain.MeetingType, email string, sequence int) (string, error) {
	if sequence == p.failBlockAt {
		return "", errors.New("private block failed")
	}
	p.blockEmails = append(p.blockEmails, email)
	return fmt.Sprintf("block-%d", sequence), nil
}

func (p *capturingProvider) DeleteEvent(_ context.Context, _ domain.CalendarConnection, _ string, eventID string) error {
	p.deleted = append(p.deleted, eventID)
	return nil
}

func testMeeting() domain.MeetingType {
	return domain.MeetingType{
		ID: "deep-dive", Slug: "deep-dive", Name: "Deep dive", DurationMinutes: 45,
		BufferBeforeMinutes: 10, BufferAfterMinutes: 15, BookingWindowDays: 14,
		SlotIntervalMinutes: 15, TimeZone: "America/New_York", Active: true,
		Availability: []domain.WeekdayHours{{Weekday: int(time.Wednesday), Start: "09:00", End: "17:00"}},
	}
}
