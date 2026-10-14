import { currentLocale } from '@/lib/i18n'

import { weekDays } from './mix-period'

/** "Sep 28 – Oct 4" for a weekly mix week (`2026-09-28`, the Monday). */
export function weekRangeLabel(monday: string): string {
  const days = weekDays(monday)
  if (!days) return monday
  return new Intl.DateTimeFormat(currentLocale(), { month: 'short', day: 'numeric' }).formatRange(days[0], days[1])
}

/** "Wednesday, October 1" for a daily mix day (`2026-10-01`). */
export function dailyDateLabel(day: string): string {
  const [y, m, d] = day.split('-').map(Number)
  if (!y || !m || !d) return day
  return new Intl.DateTimeFormat(currentLocale(), { weekday: 'long', month: 'long', day: 'numeric' }).format(new Date(y, m - 1, d))
}
