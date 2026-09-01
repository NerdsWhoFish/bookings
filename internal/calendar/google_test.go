package calendar

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/NerdsWhoFish/bookings/internal/domain"
)

func TestEventForBookingDisablesReminders(t *testing.T) {
	booking := domain.Booking{
		ID:    "test",
		Start: time.Date(2026, time.September, 2, 9, 0, 0, 0, time.UTC),
		End:   time.Date(2026, time.September, 2, 9, 20, 0, 0, time.UTC),
	}
	meeting := domain.MeetingType{TimeZone: "America/New_York"}
	payload, err := json.Marshal(eventForBooking(booking, meeting, "owner@example.com"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(payload, []byte(`"reminders":{"useDefault":false}`)) {
		t.Fatalf("event payload does not explicitly disable reminders: %s", payload)
	}
}

func TestEventForBookingAddsSelectedConnectedAccountsWithoutDuplicates(t *testing.T) {
	booking := domain.Booking{
		GuestName:  "Ada Angler",
		GuestEmail: "guest@example.com",
		Start:      time.Date(2026, time.September, 2, 9, 0, 0, 0, time.UTC),
		End:        time.Date(2026, time.September, 2, 9, 20, 0, 0, time.UTC),
	}
	meeting := domain.MeetingType{
		TimeZone:       "America/New_York",
		AttendeeEmails: []string{"teammate@example.com", "GUEST@example.com", "owner@example.com"},
	}
	event := eventForBooking(booking, meeting, "OWNER@example.com")
	if len(event.Attendees) != 2 {
		t.Fatalf("got %d attendees, want guest and teammate", len(event.Attendees))
	}
	if event.Attendees[0].Email != "guest@example.com" || event.Attendees[1].Email != "teammate@example.com" {
		t.Fatalf("unexpected attendees: %#v", event.Attendees)
	}
}
