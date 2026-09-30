/**
 * Pure helpers of the listening report (no imports, unit-tested in web/tests): report periods
 * in the viewer's local calendar, comparisons, and chart scales.
 */

/** Rolling periods, calendar years (`y2025`) and all time. */
export type ListeningRange = '7d' | '30d' | '90d' | '12m' | 'all' | `y${number}`

export const ROLLING_RANGES = ['7d', '30d', '90d', '12m', 'all'] as const

export interface RangeBounds {
  /** unix ms; 0 = since the first play. */
  from: number
  /** unix ms (exclusive); 0 = now. */
  to: number
}

/** Local midnight `days` days before `now`'s day. */
function startOfDay(now: Date, daysBack: number): Date {
  return new Date(now.getFullYear(), now.getMonth(), now.getDate() - daysBack)
}

/**
 * Bounds of a range in local time. Rolling ranges end now and start at a local midnight so
 * the first bar of the chart is a whole day (7 days = today and the 6 before it).
 */
export function rangeBounds(range: ListeningRange, now: Date = new Date()): RangeBounds {
  switch (range) {
    case '7d':
      return { from: startOfDay(now, 6).getTime(), to: 0 }
    case '30d':
      return { from: startOfDay(now, 29).getTime(), to: 0 }
    case '90d':
      return { from: startOfDay(now, 89).getTime(), to: 0 }
    case '12m':
      return { from: new Date(now.getFullYear(), now.getMonth() - 11, 1).getTime(), to: 0 }
    case 'all':
      return { from: 0, to: 0 }
  }
  const year = yearOf(range)
  if (year === null) return { from: 0, to: 0 }
  return { from: new Date(year, 0, 1).getTime(), to: new Date(year + 1, 0, 1).getTime() }
}

/** The year of a `y2025` range, `null` for other ranges. */
export function yearOf(range: string): number | null {
  const m = /^y(\d{4})$/.exec(range)
  return m ? Number(m[1]) : null
}

/** Whether `value` is a range (for URL parameters). */
export function isListeningRange(value: string | null | undefined): value is ListeningRange {
  if (!value) return false
  return (ROLLING_RANGES as readonly string[]).includes(value) || yearOf(value) !== null
}

/** Calendar years with plays, newest first (from the first play's year to `now`'s). */
export function yearsSince(firstPlayAt: number, now: Date = new Date()): number[] {
  if (!firstPlayAt) return []
  const first = new Date(firstPlayAt).getFullYear()
  const out: number[] = []
  for (let y = now.getFullYear(); y >= first && out.length < 50; y--) out.push(y)
  return out
}

/** Relative change in percent, rounded; `null` when there is nothing to compare with. */
export function percentChange(current: number, previous: number | null | undefined): number | null {
  if (previous === null || previous === undefined || previous <= 0) return null
  return Math.round(((current - previous) / previous) * 100)
}

/** A round axis maximum ≥ `max` (1, 2 or 5 × 10ⁿ, at least 1) so gridlines land on readable counts. */
export function niceMax(max: number): number {
  if (!(max > 1)) return 1
  const base = 10 ** Math.floor(Math.log10(max))
  for (const step of [1, 2, 5]) {
    if (step * base >= max) return step * base
  }
  return 10 * base
}

/** Heatmap shade 0 (no plays) … 4 (the busiest hour) on a square-root scale. */
export function heatLevel(value: number, max: number): 0 | 1 | 2 | 3 | 4 {
  if (!(value > 0) || !(max > 0)) return 0
  const level = Math.ceil(Math.sqrt(value / max) * 4)
  return Math.min(4, Math.max(1, level)) as 1 | 2 | 3 | 4
}

/** Local day key (`2026-09-30`) of a timestamp, for grouping the history. */
export function dayKey(ms: number): string {
  const d = new Date(ms)
  const mm = String(d.getMonth() + 1).padStart(2, '0')
  const dd = String(d.getDate()).padStart(2, '0')
  return `${d.getFullYear()}-${mm}-${dd}`
}

/** `today` / `yesterday` / `null` (an older day) for a day key relative to `now`. */
export function relativeDay(key: string, now: Date = new Date()): 'today' | 'yesterday' | null {
  if (key === dayKey(now.getTime())) return 'today'
  if (key === dayKey(startOfDay(now, 1).getTime())) return 'yesterday'
  return null
}

/** Groups items (newest first) by local day, keeping the order. */
export function groupByDay<T>(items: readonly T[], at: (item: T) => number): { key: string; items: T[] }[] {
  const out: { key: string; items: T[] }[] = []
  for (const item of items) {
    const key = dayKey(at(item))
    const last = out[out.length - 1]
    if (last && last.key === key) last.items.push(item)
    else out.push({ key, items: [item] })
  }
  return out
}

/**
 * Reorders the server's Monday-first clock rows for a locale whose week starts on
 * `firstDay` (0 = Sunday … 6 = Saturday, as `Intl.Locale#weekInfo`), returning ISO weekday
 * indexes (0 = Monday).
 */
export function weekdayOrder(firstDay: number): number[] {
  const startIso = (((firstDay + 6) % 7) + 7) % 7
  return Array.from({ length: 7 }, (_, i) => (startIso + i) % 7)
}
