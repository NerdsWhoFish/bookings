package booking

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/NerdsWhoFish/bookings/internal/availability"
	calendarprovider "github.com/NerdsWhoFish/bookings/internal/calendar"
	"github.com/NerdsWhoFish/bookings/internal/domain"
	"github.com/NerdsWhoFish/bookings/internal/securetoken"
	"github.com/NerdsWhoFish/bookings/internal/store"
)

type Service struct {
	store    store.Store
	calendar calendarprovider.Provider
	now      func() time.Time
}

type Request struct {
	MeetingTypeSlug string    `json:"meetingTypeSlug"`
	Start           time.Time `json:"start"`
	GuestName       string    `json:"guestName"`
	GuestEmail      string    `json:"guestEmail"`
	GuestNotes      string    `json:"guestNotes"`
}

type Confirmation struct {
	Booking     domain.Booking `json:"booking"`
	CancelToken string         `json:"cancelToken"`
}

func NewService(data store.Store, provider calendarprovider.Provider) *Service {
	return &Service{store: data, calendar: provider, now: time.Now}
}

func (s *Service) Availability(ctx context.Context, meeting domain.MeetingType, from time.Time) ([]domain.Slot, error) {
	connections, err := s.store.ListConnections(ctx)
	if err != nil {
		return nil, err
	}
	if meeting.DestinationConnectionID != "" {
		attendees := make(map[string]bool, len(meeting.AttendeeEmails))
		for _, email := range meeting.AttendeeEmails {
			attendees[strings.ToLower(email)] = true
		}
		relevant := make([]domain.CalendarConnection, 0, len(connections))
		for _, connection := range connections {
			if connection.ID == meeting.DestinationConnectionID || attendees[strings.ToLower(connection.Email)] {
				relevant = append(relevant, connection)
			}
		}
		connections = relevant
	}
	busy, err := s.calendar.Busy(ctx, connections, from, from.AddDate(0, 0, 14))
	if err != nil {
		return nil, err
	}
	return availability.Slots(meeting, from, s.now(), busy)
}

func (s *Service) Create(ctx context.Context, request Request) (Confirmation, error) {
	meeting, err := s.store.GetMeetingType(ctx, request.MeetingTypeSlug)
	if err != nil {
		return Confirmation{}, err
	}
	if request.GuestName == "" || len(request.GuestName) > 120 || len(request.GuestNotes) > 2000 {
		return Confirmation{}, fmt.Errorf("invalid guest details")
	}
	address, err := mail.ParseAddress(request.GuestEmail)
	if err != nil || address.Address != request.GuestEmail {
		return Confirmation{}, fmt.Errorf("invalid guest email")
	}

	slots, err := s.Availability(ctx, meeting, request.Start)
	if err != nil {
		return Confirmation{}, err
	}
	end := request.Start.Add(time.Duration(meeting.DurationMinutes) * time.Minute)
	if !contains(slots, request.Start, end) {
		return Confirmation{}, store.ErrConflict
	}

	id, err := securetoken.RandomHex(16)
	if err != nil {
		return Confirmation{}, err
	}
	cancelToken, err := securetoken.RandomHex(32)
	if err != nil {
		return Confirmation{}, err
	}
	now := s.now().UTC()
	booking := domain.Booking{
		ID:              id,
		MeetingTypeID:   meeting.ID,
		Start:           request.Start,
		End:             end,
		GuestName:       request.GuestName,
		GuestEmail:      request.GuestEmail,
		GuestNotes:      request.GuestNotes,
		Status:          "pending",
		CancelTokenHash: securetoken.Hash(cancelToken),
		LockIDs:         lockIDs(request.Start, end, meeting),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.store.ClaimBooking(ctx, booking); err != nil {
		return Confirmation{}, err
	}

	connection, err := s.store.GetConnection(ctx, meeting.DestinationConnectionID)
	if err != nil {
		if meeting.DestinationConnectionID == "" {
			connection = domain.CalendarConnection{ID: "dev"}
		} else {
			_ = s.store.FailBooking(ctx, booking.ID)
			return Confirmation{}, err
		}
	}
	eventID, err := s.calendar.CreateEvent(ctx, connection, meeting.DestinationCalendarID, booking, meeting)
	if err != nil {
		_ = s.store.FailBooking(ctx, booking.ID)
		return Confirmation{}, err
	}
	if err := s.store.ConfirmBooking(ctx, booking.ID, eventID); err != nil {
		return Confirmation{}, err
	}
	booking.Status = "confirmed"
	return Confirmation{Booking: booking, CancelToken: cancelToken}, nil
}

func (s *Service) Cancel(ctx context.Context, id, token string) error {
	booking, err := s.store.GetBooking(ctx, id)
	if err != nil {
		return err
	}
	if !securetoken.EqualHash(token, booking.CancelTokenHash) {
		return errors.New("invalid cancellation token")
	}
	meeting, err := s.store.GetMeetingTypeRecord(ctx, booking.MeetingTypeID)
	if err != nil {
		return err
	}
	connection, err := s.store.GetConnection(ctx, meeting.DestinationConnectionID)
	if err == nil && booking.EventID != "" {
		if err := s.calendar.DeleteEvent(ctx, connection, meeting.DestinationCalendarID, booking.EventID); err != nil {
			return err
		}
	}
	_, err = s.store.CancelBooking(ctx, id)
	return err
}

func contains(slots []domain.Slot, start, end time.Time) bool {
	for _, slot := range slots {
		if slot.Start.Equal(start) && slot.End.Equal(end) {
			return true
		}
	}
	return false
}

func lockIDs(start, end time.Time, meeting domain.MeetingType) []string {
	claimStart := start.Add(-time.Duration(meeting.BufferBeforeMinutes) * time.Minute).Truncate(5 * time.Minute)
	claimEnd := end.Add(time.Duration(meeting.BufferAfterMinutes) * time.Minute)
	var result []string
	for bucket := claimStart; bucket.Before(claimEnd); bucket = bucket.Add(5 * time.Minute) {
		result = append(result, fmt.Sprintf("slot-%d", bucket.Unix()))
	}
	return result
}
