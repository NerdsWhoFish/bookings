import { describe, expect, it } from 'vitest'
import { normalizeBusyCalendarIDs } from './calendarSelection'

const calendars = [
  { id: 'family@example.com', name: 'Family', primary: false },
  { id: 'joey@example.com', name: 'Joey', primary: true },
]

describe('normalizeBusyCalendarIDs', () => {
  it('replaces the Google primary alias with the listed calendar ID', () => {
    expect(normalizeBusyCalendarIDs(['primary'], calendars)).toEqual(['joey@example.com'])
  })

  it('deduplicates the primary calendar after normalization', () => {
    expect(normalizeBusyCalendarIDs(['primary', 'joey@example.com'], calendars)).toEqual(['joey@example.com'])
  })
})
