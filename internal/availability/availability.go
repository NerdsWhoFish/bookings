package availability

import (
	"fmt"
	"sort"
	"time"

	"github.com/NerdsWhoFish/bookings/internal/domain"
)

func Slots(meeting domain.MeetingType, from, now time.Time, busy []domain.BusyPeriod) ([]domain.Slot, error) {
	location, err := time.LoadLocation(meeting.TimeZone)
	if err != nil {
		return nil, fmt.Errorf("load meeting time zone: %w", err)
	}
	if meeting.DurationMinutes <= 0 || meeting.SlotIntervalMinutes <= 0 {
		return nil, fmt.Errorf("duration and slot interval must be positive")
	}

	windowStart := from.In(location)
	if earliest := now.Add(time.Duration(meeting.MinimumNoticeMinutes) * time.Minute); earliest.After(windowStart) {
		windowStart = earliest.In(location)
	}
	windowEnd := now.In(location).AddDate(0, 0, meeting.BookingWindowDays)
	if limit := from.In(location).AddDate(0, 0, 14); limit.Before(windowEnd) {
		windowEnd = limit
	}

	sort.Slice(busy, func(i, j int) bool { return busy[i].Start.Before(busy[j].Start) })
	var slots []domain.Slot
	for day := startOfDay(windowStart); day.Before(windowEnd); day = day.AddDate(0, 0, 1) {
		for _, hours := range meeting.Availability {
			if int(day.Weekday()) != hours.Weekday {
				continue
			}
			start, err := clockOn(day, hours.Start)
			if err != nil {
				return nil, err
			}
			end, err := clockOn(day, hours.End)
			if err != nil {
				return nil, err
			}
			for candidate := ceil(start, time.Duration(meeting.SlotIntervalMinutes)*time.Minute); ; candidate = candidate.Add(time.Duration(meeting.SlotIntervalMinutes) * time.Minute) {
				slot := domain.Slot{Start: candidate, End: candidate.Add(time.Duration(meeting.DurationMinutes) * time.Minute)}
				if slot.End.After(end) {
					break
				}
				if slot.Start.Before(windowStart) || slot.End.After(windowEnd) || overlaps(slot, meeting, busy) {
					continue
				}
				slots = append(slots, slot)
			}
		}
	}
	return slots, nil
}

func overlaps(slot domain.Slot, meeting domain.MeetingType, busy []domain.BusyPeriod) bool {
	claimStart := slot.Start.Add(-time.Duration(meeting.BufferBeforeMinutes) * time.Minute)
	claimEnd := slot.End.Add(time.Duration(meeting.BufferAfterMinutes) * time.Minute)
	for _, period := range busy {
		if claimStart.Before(period.End) && claimEnd.After(period.Start) {
			return true
		}
	}
	return false
}

func startOfDay(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, value.Location())
}

func clockOn(day time.Time, clock string) (time.Time, error) {
	value, err := time.Parse("15:04", clock)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse availability clock %q: %w", clock, err)
	}
	return time.Date(day.Year(), day.Month(), day.Day(), value.Hour(), value.Minute(), 0, 0, day.Location()), nil
}

func ceil(value time.Time, interval time.Duration) time.Time {
	remainder := time.Duration(value.UnixNano() % int64(interval))
	if remainder == 0 {
		return value
	}
	return value.Add(interval - remainder)
}
