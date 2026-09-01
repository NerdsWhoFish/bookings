import type { CalendarConnection, CalendarInfo, Confirmation, MeetingType, PublicConfig, Session, Slot } from './types'

async function request<T>(input: RequestInfo, init?: RequestInit): Promise<T> {
  const response = await fetch(input, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...init?.headers },
  })
  if (!response.ok) {
    const problem = await response.json().catch(() => ({ title: 'Something went wrong' }))
    throw new Error(problem.title ?? `Request failed with ${response.status}`)
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

export const api = {
  config: () => request<PublicConfig>('/api/public/config'),
  meetingTypes: () => request<MeetingType[]>('/api/public/meeting-types'),
  availability: (slug: string) =>
    request<Slot[]>(`/api/public/meeting-types/${encodeURIComponent(slug)}/availability?from=${encodeURIComponent(new Date().toISOString())}`),
  book: (body: {
    meetingTypeSlug: string
    start: string
    guestName: string
    guestEmail: string
    guestNotes: string
    turnstileToken: string
  }) => request<Confirmation>('/api/public/bookings', { method: 'POST', body: JSON.stringify(body) }),
  adminSession: () => request<Session>('/api/admin/session'),
  adminMeetingTypes: () => request<MeetingType[]>('/api/admin/meeting-types'),
  connections: () => request<CalendarConnection[]>('/api/admin/connections'),
  calendars: (connectionID: string) => request<CalendarInfo[]>(`/api/admin/connections/${encodeURIComponent(connectionID)}/calendars`),
  saveConnection: (connectionID: string, busyCalendarIds: string[]) => request<CalendarConnection>(`/api/admin/connections/${encodeURIComponent(connectionID)}`, { method: 'PUT', body: JSON.stringify({ busyCalendarIds }) }),
  saveMeetingType: (meeting: MeetingType) => request<MeetingType>(`/api/admin/meeting-types/${encodeURIComponent(meeting.id)}`, { method: 'PUT', body: JSON.stringify(meeting) }),
}
