import type { CalendarConnection, CalendarInfo, CalendarInvitation, Confirmation, CreatedCalendarInvitation, MeetingType, PublicConfig, Session, Slot } from './types'
import { captureError } from './telemetry-client'

class HTTPError extends Error {
  constructor(message: string, readonly status: number) {
    super(message)
  }
}

async function request<T>(input: RequestInfo, init?: RequestInit, expectedStatus?: number): Promise<T> {
  try {
    const response = await fetch(input, {
      ...init,
      headers: { 'Content-Type': 'application/json', ...init?.headers },
    })
    if (!response.ok) {
      const problem = await response.json().catch(() => ({ title: 'Something went wrong' }))
      throw new HTTPError(problem.title ?? `Request failed with ${response.status}`, response.status)
    }
    if (response.status === 204) return undefined as T
    return await response.json() as T
  } catch (error) {
    if (!(error instanceof HTTPError && error.status === expectedStatus)) captureError(error, 'api')
    throw error
  }
}

async function requestList<T>(input: RequestInfo, init?: RequestInit): Promise<T[]> {
  const value = await request<T[] | null>(input, init)
  if (value === null) return []
  if (!Array.isArray(value)) {
    const error = new Error('Expected a list response')
    captureError(error, 'api')
    throw error
  }
  return value
}

export const api = {
  config: () => request<PublicConfig>('/api/public/config'),
  meetingTypes: () => requestList<MeetingType>('/api/public/meeting-types'),
  meetingType: (slug: string) => request<MeetingType>(`/api/public/meeting-types/${encodeURIComponent(slug)}`),
  availability: (slug: string, from = new Date()) =>
    requestList<Slot>(`/api/public/meeting-types/${encodeURIComponent(slug)}/availability?from=${encodeURIComponent(from.toISOString())}`),
  book: (body: {
    meetingTypeSlug: string
    start: string
    guestName: string
    guestEmail: string
    guestNotes: string
    turnstileToken: string
  }) => request<Confirmation>('/api/public/bookings', { method: 'POST', body: JSON.stringify(body) }),
  adminSession: async (): Promise<Session | null> => {
    try {
      const session = await request<Session>('/api/admin/session', undefined, 401)
      if (!session || typeof session.email !== 'string' || !session.email || !Number.isFinite(session.expiresAt)) {
        const error = new Error('Invalid session response')
        captureError(error, 'api')
        throw error
      }
      return session
    } catch (error) {
      if (error instanceof HTTPError && error.status === 401) return null
      throw error
    }
  },
  adminMeetingTypes: () => requestList<MeetingType>('/api/admin/meeting-types'),
  connections: () => requestList<CalendarConnection>('/api/admin/connections'),
  calendarInvitations: () => requestList<CalendarInvitation>('/api/admin/calendar-invitations'),
  createCalendarInvitation: (email: string) => request<CreatedCalendarInvitation>('/api/admin/calendar-invitations', { method: 'POST', body: JSON.stringify({ email }) }),
  revokeCalendarInvitation: (id: string) => request<void>(`/api/admin/calendar-invitations/${encodeURIComponent(id)}`, { method: 'DELETE' }),
  startCalendarInvitation: (token: string) => request<{ url: string }>('/api/public/calendar-invitations/start', { method: 'POST', body: JSON.stringify({ token }) }),
  calendars: (connectionID: string) => requestList<CalendarInfo>(`/api/admin/connections/${encodeURIComponent(connectionID)}/calendars`),
  saveConnection: (connectionID: string, busyCalendarIds: string[]) => request<CalendarConnection>(`/api/admin/connections/${encodeURIComponent(connectionID)}`, { method: 'PUT', body: JSON.stringify({ busyCalendarIds }) }),
  saveMeetingType: (meeting: MeetingType) => request<MeetingType>(`/api/admin/meeting-types/${encodeURIComponent(meeting.id)}`, { method: 'PUT', body: JSON.stringify(meeting) }),
  deleteMeetingType: (id: string) => request<void>(`/api/admin/meeting-types/${encodeURIComponent(id)}`, { method: 'DELETE' }),
}
