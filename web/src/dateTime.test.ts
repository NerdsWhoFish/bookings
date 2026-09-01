import { describe, expect, it } from 'vitest'
import { addDays, groupSlotsByDay, hasLaterAvailability, resolvedTimeZone } from './dateTime'

describe('date and time presentation', () => {
  it('uses the visitor time zone and falls back to the meeting time zone', () => {
    expect(resolvedTimeZone('America/New_York', 'Europe/London')).toBe('Europe/London')
    expect(resolvedTimeZone('America/New_York', '')).toBe('America/New_York')
  })

  it('groups an absolute slot by its date in the visitor time zone', () => {
    const slots = [{ start: '2026-09-02T01:00:00Z', end: '2026-09-02T01:20:00Z' }]
    expect(groupSlotsByDay(slots, 'America/New_York')[0][0]).toBe('09/01/2026')
    expect(groupSlotsByDay(slots, 'Europe/London')[0][0]).toBe('09/02/2026')
  })

  it('paginates until the configured booking window ends', () => {
    const start = new Date('2026-09-01T12:00:00Z')
    expect(hasLaterAvailability(start, start, 30)).toBe(true)
    expect(hasLaterAvailability(addDays(start, 14), start, 30)).toBe(true)
    expect(hasLaterAvailability(addDays(start, 28), start, 30)).toBe(false)
  })
})
