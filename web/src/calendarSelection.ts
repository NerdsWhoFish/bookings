import type { CalendarInfo } from './types'

export function normalizeBusyCalendarIDs(ids: string[], calendars: CalendarInfo[]) {
  const primaryID = calendars.find((calendar) => calendar.primary)?.id
  return [...new Set(ids.map((id) => id === 'primary' && primaryID ? primaryID : id))]
}
