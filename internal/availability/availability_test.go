package availability

import (
	"testing"
	"time"

	"github.com/NerdsWhoFish/bookings/internal/domain"
)

func TestSlotsRespectsDurationBuffersAndBusyPeriods(t *testing.T) {
	meeting := domain.MeetingType{
		DurationMinutes:     30,
		BufferBeforeMinutes: 10,
		BufferAfterMinutes:  10,
		BookingWindowDays:   2,
		SlotIntervalMinutes: 15,
		TimeZone:            "America/New_York",
		Availability:        []domain.WeekdayHours{{Weekday: int(time.Monday), Start: "09:00", End: "11:00"}},
	}
	now := time.Date(2026, 9, 6, 8, 0, 0, 0, time.UTC)
	from := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	busy := []domain.BusyPeriod{{
		Start: time.Date(2026, 9, 7, 9, 45, 0, 0, time.FixedZone("EDT", -4*60*60)),
		End:   time.Date(2026, 9, 7, 10, 15, 0, 0, time.FixedZone("EDT", -4*60*60)),
	}}

	slots, err := Slots(meeting, from, now, busy)
	if err != nil {
		t.Fatal(err)
	}
	if len(slots) != 2 {
		t.Fatalf("expected 2 slots, got %d: %#v", len(slots), slots)
	}
	if got := slots[0].Start.In(time.FixedZone("EDT", -4*60*60)).Format("15:04"); got != "09:00" {
		t.Fatalf("expected first slot at 09:00, got %s", got)
	}
}
