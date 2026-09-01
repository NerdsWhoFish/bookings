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
	payload, err := json.Marshal(eventForBooking(booking, meeting))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(payload, []byte(`"reminders":{"useDefault":false}`)) {
		t.Fatalf("event payload does not explicitly disable reminders: %s", payload)
	}
}
