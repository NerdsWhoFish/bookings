package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/NerdsWhoFish/bookings/internal/domain"
	"github.com/NerdsWhoFish/bookings/internal/tokencrypto"
	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type Google struct {
	oauth  *oauth2.Config
	cipher tokencrypto.Cipher
}

func NewGoogle(oauth *oauth2.Config, cipher tokencrypto.Cipher) *Google {
	return &Google{oauth: oauth, cipher: cipher}
}

func (g *Google) Busy(ctx context.Context, connections []domain.CalendarConnection, start, end time.Time) ([]domain.BusyPeriod, error) {
	var result []domain.BusyPeriod
	for _, connection := range connections {
		service, err := g.service(ctx, connection)
		if err != nil {
			return nil, err
		}
		for offset := 0; offset < len(connection.BusyCalendarIDs); offset += 50 {
			limit := min(offset+50, len(connection.BusyCalendarIDs))
			items := make([]*calendar.FreeBusyRequestItem, 0, limit-offset)
			for _, id := range connection.BusyCalendarIDs[offset:limit] {
				items = append(items, &calendar.FreeBusyRequestItem{Id: id})
			}
			response, err := service.Freebusy.Query(&calendar.FreeBusyRequest{
				TimeMin: start.Format(time.RFC3339),
				TimeMax: end.Format(time.RFC3339),
				Items:   items,
			}).Context(ctx).Do()
			if err != nil {
				return nil, fmt.Errorf("query Google freebusy for %s: %w", connection.Email, err)
			}
			for id, calendarResult := range response.Calendars {
				if len(calendarResult.Errors) > 0 {
					return nil, fmt.Errorf("query Google freebusy calendar %s: %s", id, calendarResult.Errors[0].Reason)
				}
				for _, period := range calendarResult.Busy {
					periodStart, err := time.Parse(time.RFC3339, period.Start)
					if err != nil {
						return nil, fmt.Errorf("parse Google busy start: %w", err)
					}
					periodEnd, err := time.Parse(time.RFC3339, period.End)
					if err != nil {
						return nil, fmt.Errorf("parse Google busy end: %w", err)
					}
					result = append(result, domain.BusyPeriod{Start: periodStart, End: periodEnd})
				}
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Start.Before(result[j].Start) })
	return result, nil
}

func (g *Google) Calendars(ctx context.Context, connection domain.CalendarConnection) ([]domain.CalendarInfo, error) {
	service, err := g.service(ctx, connection)
	if err != nil {
		return nil, err
	}
	var result []domain.CalendarInfo
	pageToken := ""
	for {
		call := service.CalendarList.List().ShowHidden(false).MinAccessRole("reader").Context(ctx)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		page, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("list Google calendars: %w", err)
		}
		for _, item := range page.Items {
			result = append(result, domain.CalendarInfo{ID: item.Id, Name: item.Summary, Primary: item.Primary})
		}
		if page.NextPageToken == "" {
			break
		}
		pageToken = page.NextPageToken
	}
	return result, nil
}

func (g *Google) CreateEvent(ctx context.Context, connection domain.CalendarConnection, calendarID string, booking domain.Booking, meeting domain.MeetingType) (string, error) {
	service, err := g.service(ctx, connection)
	if err != nil {
		return "", err
	}
	event := eventForBooking(booking, meeting)
	insert := service.Events.Insert(calendarID, event).SendUpdates("all").Context(ctx)
	if meeting.Location == "Google Meet" {
		event.ConferenceData = &calendar.ConferenceData{CreateRequest: &calendar.CreateConferenceRequest{
			RequestId:             booking.ID,
			ConferenceSolutionKey: &calendar.ConferenceSolutionKey{Type: "hangoutsMeet"},
		}}
		insert = insert.ConferenceDataVersion(1)
	}
	created, err := insert.Do()
	if err != nil {
		existing, getErr := service.Events.Get(calendarID, event.Id).Context(ctx).Do()
		if getErr == nil {
			return existing.Id, nil
		}
		return "", fmt.Errorf("create Google Calendar event: %w", err)
	}
	return created.Id, nil
}

func eventForBooking(booking domain.Booking, meeting domain.MeetingType) *calendar.Event {
	return &calendar.Event{
		Id:          deterministicEventID(booking.ID),
		Summary:     meeting.Name + " with " + booking.GuestName,
		Description: booking.GuestNotes,
		Location:    meeting.Location,
		Start:       &calendar.EventDateTime{DateTime: booking.Start.Format(time.RFC3339), TimeZone: meeting.TimeZone},
		End:         &calendar.EventDateTime{DateTime: booking.End.Format(time.RFC3339), TimeZone: meeting.TimeZone},
		Attendees:   []*calendar.EventAttendee{{Email: booking.GuestEmail, DisplayName: booking.GuestName}},
		Reminders:   &calendar.EventReminders{UseDefault: false, ForceSendFields: []string{"UseDefault"}},
	}
}

func (g *Google) DeleteEvent(ctx context.Context, connection domain.CalendarConnection, calendarID, eventID string) error {
	service, err := g.service(ctx, connection)
	if err != nil {
		return err
	}
	if err := service.Events.Delete(calendarID, eventID).SendUpdates("all").Context(ctx).Do(); err != nil {
		return fmt.Errorf("delete Google Calendar event: %w", err)
	}
	return nil
}

func (g *Google) service(ctx context.Context, connection domain.CalendarConnection) (*calendar.Service, error) {
	encoded, err := g.cipher.Decrypt(ctx, connection.ID, connection.EncryptedToken)
	if err != nil {
		return nil, err
	}
	var token oauth2.Token
	if err := json.Unmarshal(encoded, &token); err != nil {
		return nil, fmt.Errorf("decode OAuth token: %w", err)
	}
	client := g.oauth.Client(ctx, &token)
	service, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("create Google Calendar client: %w", err)
	}
	return service, nil
}

func deterministicEventID(bookingID string) string {
	return "booking" + bookingID
}
