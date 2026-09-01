package domain

import "time"

const DefaultSlotIntervalMinutes = 30

func NormalizeSlotIntervalMinutes(minutes int) int {
	if minutes < DefaultSlotIntervalMinutes {
		return DefaultSlotIntervalMinutes
	}
	return ((minutes + DefaultSlotIntervalMinutes - 1) / DefaultSlotIntervalMinutes) * DefaultSlotIntervalMinutes
}

type WeekdayHours struct {
	Weekday int    `json:"weekday" firestore:"weekday"`
	Start   string `json:"start" firestore:"start"`
	End     string `json:"end" firestore:"end"`
}

type MeetingType struct {
	ID                      string         `json:"id" firestore:"id"`
	Slug                    string         `json:"slug" firestore:"slug"`
	Name                    string         `json:"name" firestore:"name"`
	Description             string         `json:"description" firestore:"description"`
	DurationMinutes         int            `json:"durationMinutes" firestore:"duration_minutes"`
	BufferBeforeMinutes     int            `json:"bufferBeforeMinutes" firestore:"buffer_before_minutes"`
	BufferAfterMinutes      int            `json:"bufferAfterMinutes" firestore:"buffer_after_minutes"`
	MinimumNoticeMinutes    int            `json:"minimumNoticeMinutes" firestore:"minimum_notice_minutes"`
	BookingWindowDays       int            `json:"bookingWindowDays" firestore:"booking_window_days"`
	SlotIntervalMinutes     int            `json:"slotIntervalMinutes" firestore:"slot_interval_minutes"`
	TimeZone                string         `json:"timeZone" firestore:"time_zone"`
	Location                string         `json:"location" firestore:"location"`
	DestinationConnectionID string         `json:"destinationConnectionId,omitempty" firestore:"destination_connection_id"`
	DestinationCalendarID   string         `json:"destinationCalendarId,omitempty" firestore:"destination_calendar_id"`
	AttendeeEmails          []string       `json:"attendeeEmails" firestore:"attendee_emails"`
	BlockerEmails           []string       `json:"blockerEmails" firestore:"blocker_emails"`
	Availability            []WeekdayHours `json:"availability" firestore:"availability"`
	Active                  bool           `json:"active" firestore:"active"`
	Hidden                  bool           `json:"hidden" firestore:"hidden"`
	Deleted                 bool           `json:"-" firestore:"deleted"`
}

type CalendarConnection struct {
	ID              string    `json:"id" firestore:"id"`
	Email           string    `json:"email" firestore:"email"`
	EncryptedToken  []byte    `json:"-" firestore:"encrypted_token"`
	BusyCalendarIDs []string  `json:"busyCalendarIds" firestore:"busy_calendar_ids"`
	CreatedAt       time.Time `json:"createdAt" firestore:"created_at"`
	UpdatedAt       time.Time `json:"updatedAt" firestore:"updated_at"`
}

type CalendarInvitation struct {
	ID        string     `json:"id" firestore:"id"`
	Email     string     `json:"email" firestore:"email"`
	TokenHash []byte     `json:"-" firestore:"token_hash"`
	ExpiresAt time.Time  `json:"expiresAt" firestore:"expires_at"`
	UsedAt    *time.Time `json:"usedAt" firestore:"used_at"`
	CreatedAt time.Time  `json:"createdAt" firestore:"created_at"`
}

type BusyPeriod struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type CalendarInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Primary bool   `json:"primary"`
}

type Slot struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type Booking struct {
	ID              string    `json:"id" firestore:"id"`
	MeetingTypeID   string    `json:"meetingTypeId" firestore:"meeting_type_id"`
	Start           time.Time `json:"start" firestore:"start"`
	End             time.Time `json:"end" firestore:"end"`
	GuestName       string    `json:"guestName" firestore:"guest_name"`
	GuestEmail      string    `json:"guestEmail" firestore:"guest_email"`
	GuestNotes      string    `json:"guestNotes,omitempty" firestore:"guest_notes"`
	Status          string    `json:"status" firestore:"status"`
	EventID         string    `json:"-" firestore:"event_id"`
	ShadowEventIDs  []string  `json:"-" firestore:"shadow_event_ids"`
	CancelTokenHash []byte    `json:"-" firestore:"cancel_token_hash"`
	LockIDs         []string  `json:"-" firestore:"lock_ids"`
	CreatedAt       time.Time `json:"createdAt" firestore:"created_at"`
	UpdatedAt       time.Time `json:"updatedAt" firestore:"updated_at"`
}

type ExternalBlock struct {
	ID        string    `json:"id" firestore:"id"`
	Start     time.Time `json:"start" firestore:"start"`
	End       time.Time `json:"end" firestore:"end"`
	CreatedAt time.Time `json:"createdAt" firestore:"created_at"`
	UpdatedAt time.Time `json:"updatedAt" firestore:"updated_at"`
}
