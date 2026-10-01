import { currentLocale } from '@/lib/i18n'

/** "Wednesday, October 1" for a daily mix day (`2026-10-01`). */
export function dailyDateLabel(day: string): string {
  const [y, m, d] = day.split('-').map(Number)
  if (!y || !m || !d) return day
  return new Intl.DateTimeFormat(currentLocale(), { weekday: 'long', month: 'long', day: 'numeric' }).format(new Date(y, m - 1, d))
}
