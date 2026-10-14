/**
 * Local periods of the stored mixes (no imports, unit-tested): the daily mix belongs to a local
 * day, the weekly mix to a local week that starts on Monday.
 */

/** Local day (`2026-10-01`), so the daily mix query changes at midnight. */
export function localDay(now: Date = new Date()): string {
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`
}

/** Local date of this week's Monday (`2026-09-28`), so the weekly mix query changes on Mondays. */
export function localMonday(now: Date = new Date()): string {
  return localDay(new Date(now.getFullYear(), now.getMonth(), now.getDate() - ((now.getDay() + 6) % 7)))
}

/** Monday and Sunday of a weekly mix week (`2026-09-28`, the Monday); `null` for a malformed date. */
export function weekDays(monday: string): [Date, Date] | null {
  const [y, m, d] = monday.split('-').map(Number)
  if (!y || !m || !d) return null
  return [new Date(y, m - 1, d), new Date(y, m - 1, d + 6)]
}
