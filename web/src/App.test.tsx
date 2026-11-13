import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import App from './App'
import type { MeetingType, Slot } from './types'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

it('selects an available day immediately and resets selection when the range changes', async () => {
  const meeting: MeetingType = {
    id: 'test', slug: 'test', name: 'Test meeting', description: 'Synthetic appointment',
    durationMinutes: 30, bufferBeforeMinutes: 0, bufferAfterMinutes: 0, minimumNoticeMinutes: 0,
    bookingWindowDays: 90, slotIntervalMinutes: 30, timeZone: 'UTC', location: 'Online',
    destinationConnectionId: '', destinationCalendarId: '', attendeeEmails: [], blockerEmails: [],
    availability: [], active: true, hidden: false,
  }
  const slot = (days: number): Slot => {
    const start = new Date()
    start.setUTCDate(start.getUTCDate() + days)
    start.setUTCHours(14, 0, 0, 0)
    return { start: start.toISOString(), end: new Date(start.getTime() + 1800000).toISOString() }
  }
  let availabilityRequests = 0
  vi.stubGlobal('fetch', vi.fn().mockImplementation((input: string) => {
    if (input === '/api/public/config') return Promise.resolve(Response.json({ theme: 'nerdswhofish', turnstileSiteKey: '', faroURL: '', faroAppName: 'bookings' }))
    if (input === '/api/public/meeting-types') return Promise.resolve(Response.json([meeting]))
    availabilityRequests++
    return Promise.resolve(Response.json(availabilityRequests === 1 ? [slot(2), slot(3)] : [slot(9)]))
  }))
  render(<App />)
  fireEvent.click(await screen.findByRole('button', { name: /Test meeting/ }))
  const days = await screen.findAllByRole('tab')
  expect(days[0]).toHaveAttribute('aria-selected', 'true')
  fireEvent.click(days[1])
  expect(days[1]).toHaveAttribute('aria-selected', 'true')
  fireEvent.click(screen.getByRole('button', { name: /Later/ }))
  await waitFor(() => expect(screen.getAllByRole('tab')).toHaveLength(1))
  expect(screen.getByRole('tab')).toHaveAttribute('aria-selected', 'true')
  expect(availabilityRequests).toBe(2)
})
