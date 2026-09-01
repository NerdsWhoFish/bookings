import { describe, expect, it } from 'vitest'
import { formatMinutes } from './timeUnits'

describe('formatMinutes', () => {
  it('makes minute configuration values understandable', () => {
    expect(formatMinutes(0)).toBe('No minimum notice')
    expect(formatMinutes(60)).toBe('1 hour')
    expect(formatMinutes(240)).toBe('4 hours')
    expect(formatMinutes(1440)).toBe('1 day')
    expect(formatMinutes(90)).toBe('90 minutes')
  })
})
