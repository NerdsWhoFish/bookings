import type { Slot } from './types'

export const availabilityPageDays = 14

export function resolvedTimeZone(fallback: string, detected = Intl.DateTimeFormat().resolvedOptions().timeZone) {
  return detected || fallback
}

export function formatInTimeZone(value: string | Date, timeZone: string, options: Intl.DateTimeFormatOptions) {
  return new Intl.DateTimeFormat(undefined, { timeZone, ...options }).format(new Date(value))
}

export function groupSlotsByDay(slots: Slot[], timeZone: string): [string, Slot[]][] {
  const grouped = new Map<string, Slot[]>()
  for (const slot of slots) {
    const key = formatInTimeZone(slot.start, timeZone, { year: 'numeric', month: '2-digit', day: '2-digit' })
    grouped.set(key, [...(grouped.get(key) ?? []), slot])
  }
  return Array.from(grouped.entries())
}

export function addDays(value: Date, days: number) {
  const result = new Date(value)
  result.setUTCDate(result.getUTCDate() + days)
  return result
}

export function hasLaterAvailability(rangeStart: Date, bookingWindowStart: Date, bookingWindowDays: number) {
  return addDays(rangeStart, availabilityPageDays).getTime() < addDays(bookingWindowStart, bookingWindowDays).getTime()
}
