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

func TestSlotsNormalizesLegacyIntervalsToLocalHalfHours(t *testing.T) {
	location, err := time.LoadLocation("Asia/Kathmandu")
	if err != nil {
		t.Fatal(err)
	}
	meeting := domain.MeetingType{
		DurationMinutes:     20,
		BookingWindowDays:   2,
		SlotIntervalMinutes: 10,
		TimeZone:            "Asia/Kathmandu",
		Availability:        []domain.WeekdayHours{{Weekday: int(time.Monday), Start: "09:10", End: "11:00"}},
	}
	now := time.Date(2026, 9, 6, 8, 0, 0, 0, location)
	from := time.Date(2026, 9, 7, 0, 0, 0, 0, location)

	slots, err := Slots(meeting, from, now, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"09:30", "10:00", "10:30"}
	if len(slots) != len(want) {
		t.Fatalf("expected %d slots, got %d: %#v", len(want), len(slots), slots)
	}
	for index := range want {
		if got := slots[index].Start.In(location).Format("15:04"); got != want[index] {
			t.Fatalf("slot %d: expected %s, got %s", index, want[index], got)
		}
	}
}

func TestSlotsReturnsAnEmptyListWhenNothingIsAvailable(t *testing.T) {
	meeting := domain.MeetingType{
		DurationMinutes:     20,
		BookingWindowDays:   1,
		SlotIntervalMinutes: 30,
		TimeZone:            "America/New_York",
		Availability:        []domain.WeekdayHours{{Weekday: int(time.Tuesday), Start: "09:00", End: "10:00"}},
	}
	now := time.Date(2026, 9, 6, 8, 0, 0, 0, time.UTC)
	from := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)

	slots, err := Slots(meeting, from, now, nil)
	if err != nil {
		t.Fatal(err)
	}
	if slots == nil {
		t.Fatal("expected an empty list, got nil")
	}
	if len(slots) != 0 {
		t.Fatalf("expected no slots, got %#v", slots)
	}
}
