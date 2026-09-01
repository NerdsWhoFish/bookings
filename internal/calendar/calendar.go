package calendar

import (
	"context"
	"fmt"
	"time"

	"github.com/NerdsWhoFish/bookings/internal/domain"
)

type Provider interface {
	Busy(context.Context, []domain.CalendarConnection, time.Time, time.Time) ([]domain.BusyPeriod, error)
	Calendars(context.Context, domain.CalendarConnection) ([]domain.CalendarInfo, error)
	CreateEvent(context.Context, domain.CalendarConnection, string, domain.Booking, domain.MeetingType) (string, error)
	CreateBlockEvent(context.Context, domain.CalendarConnection, string, domain.Booking, domain.MeetingType, string, int) (string, error)
	DeleteEvent(context.Context, domain.CalendarConnection, string, string) error
}

type Mock struct {
	Periods []domain.BusyPeriod
}

func (m Mock) Busy(context.Context, []domain.CalendarConnection, time.Time, time.Time) ([]domain.BusyPeriod, error) {
	return append([]domain.BusyPeriod(nil), m.Periods...), nil
}

func (Mock) Calendars(context.Context, domain.CalendarConnection) ([]domain.CalendarInfo, error) {
	return []domain.CalendarInfo{{ID: "primary", Name: "Primary", Primary: true}}, nil
}

func (Mock) CreateEvent(_ context.Context, _ domain.CalendarConnection, _ string, booking domain.Booking, _ domain.MeetingType) (string, error) {
	return "dev-" + booking.ID, nil
}

func (Mock) CreateBlockEvent(_ context.Context, _ domain.CalendarConnection, _ string, booking domain.Booking, _ domain.MeetingType, _ string, sequence int) (string, error) {
	return fmt.Sprintf("dev-%s-block-%d", booking.ID, sequence), nil
}

func (Mock) DeleteEvent(context.Context, domain.CalendarConnection, string, string) error {
	return nil
}
