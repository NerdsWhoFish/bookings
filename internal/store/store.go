package store

import (
	"context"
	"errors"
	"time"

	"github.com/NerdsWhoFish/bookings/internal/domain"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrConflict      = errors.New("slot is no longer available")
	ErrExpired       = errors.New("invitation expired")
	ErrAlreadyUsed   = errors.New("invitation already used")
	ErrEmailMismatch = errors.New("invitation email does not match Google account")
)

type Store interface {
	ListMeetingTypes(context.Context) ([]domain.MeetingType, error)
	ListAllMeetingTypes(context.Context) ([]domain.MeetingType, error)
	GetMeetingType(context.Context, string) (domain.MeetingType, error)
	GetMeetingTypeRecord(context.Context, string) (domain.MeetingType, error)
	PutMeetingType(context.Context, domain.MeetingType) error
	DeleteMeetingType(context.Context, string) error
	ListConnections(context.Context) ([]domain.CalendarConnection, error)
	GetConnection(context.Context, string) (domain.CalendarConnection, error)
	PutConnection(context.Context, domain.CalendarConnection) error
	ListCalendarInvitations(context.Context) ([]domain.CalendarInvitation, error)
	GetCalendarInvitation(context.Context, string) (domain.CalendarInvitation, error)
	PutCalendarInvitation(context.Context, domain.CalendarInvitation) error
	DeleteCalendarInvitation(context.Context, string) error
	UseCalendarInvitation(context.Context, string, string, time.Time) error
	ClaimBooking(context.Context, domain.Booking) error
	ConfirmBooking(context.Context, string, string) error
	FailBooking(context.Context, string) error
	GetBooking(context.Context, string) (domain.Booking, error)
	CancelBooking(context.Context, string) (domain.Booking, error)
}
