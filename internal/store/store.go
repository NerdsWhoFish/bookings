package store

import (
	"context"
	"errors"

	"github.com/NerdsWhoFish/bookings/internal/domain"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("slot is no longer available")
)

type Store interface {
	ListMeetingTypes(context.Context) ([]domain.MeetingType, error)
	GetMeetingType(context.Context, string) (domain.MeetingType, error)
	PutMeetingType(context.Context, domain.MeetingType) error
	ListConnections(context.Context) ([]domain.CalendarConnection, error)
	GetConnection(context.Context, string) (domain.CalendarConnection, error)
	PutConnection(context.Context, domain.CalendarConnection) error
	ClaimBooking(context.Context, domain.Booking) error
	ConfirmBooking(context.Context, string, string) error
	FailBooking(context.Context, string) error
	GetBooking(context.Context, string) (domain.Booking, error)
	CancelBooking(context.Context, string) (domain.Booking, error)
}
