export type MeetingType = {
  id: string
  slug: string
  name: string
  description: string
  durationMinutes: number
  bufferBeforeMinutes: number
  bufferAfterMinutes: number
  minimumNoticeMinutes: number
  bookingWindowDays: number
  slotIntervalMinutes: number
  timeZone: string
  location: string
  destinationConnectionId: string
  destinationCalendarId: string
  attendeeEmails: string[]
  blockerEmails: string[]
  availability: { weekday: number; start: string; end: string }[]
  active: boolean
  hidden: boolean
}

export type Slot = {
  start: string
  end: string
}

export type PublicConfig = {
  theme: string
  turnstileSiteKey: string
  faroURL: string
  faroAppName: string
}

export type Confirmation = {
  booking: {
    id: string
    start: string
    end: string
    guestName: string
    guestEmail: string
  }
  cancelToken: string
}

export type Session = { email: string; expiresAt: number }

export type CalendarConnection = {
  id: string
  email: string
  busyCalendarIds: string[]
  createdAt: string
  updatedAt: string
}

export type CalendarInfo = { id: string; name: string; primary: boolean }

export type CalendarInvitation = {
  id: string
  email: string
  expiresAt: string
  usedAt: string | null
  createdAt: string
}

export type CreatedCalendarInvitation = {
  invitation: CalendarInvitation
  url: string
}
