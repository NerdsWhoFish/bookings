package booking

import (
	"context"
	"errors"
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

func testMeeting() domain.MeetingType {
	return domain.MeetingType{
		ID: "deep-dive", Slug: "deep-dive", Name: "Deep dive", DurationMinutes: 45,
		BufferBeforeMinutes: 10, BufferAfterMinutes: 15, BookingWindowDays: 14,
		SlotIntervalMinutes: 15, TimeZone: "America/New_York", Active: true,
		Availability: []domain.WeekdayHours{{Weekday: int(time.Wednesday), Start: "09:00", End: "17:00"}},
	}
}
