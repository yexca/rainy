/**
 * Display formatting helpers (docs/architecture/contract.md §9.2). Locale-aware helpers follow the active
 * i18next language.
 */
import i18n, { currentLocale } from '@/lib/i18n'

const UNKNOWN = '—'

function pad2(n: number): string {
  return n < 10 ? `0${n}` : String(n)
}

/** `3:07`, `1:02:03` — track times (seconds, floored). Invalid/negative → `0:00`. */
export function formatDuration(seconds: number): string {
  const total = Number.isFinite(seconds) && seconds > 0 ? Math.floor(seconds) : 0
  const h = Math.floor(total / 3600)
  const m = Math.floor((total % 3600) / 60)
  const s = total % 60
  return h > 0 ? `${h}:${pad2(m)}:${pad2(s)}` : `${m}:${pad2(s)}`
}

/** `1 hr 23 min`, `47 min`, `35 sec` — album / playlist totals. */
export function formatDurationLong(seconds: number): string {
  const total = Number.isFinite(seconds) && seconds > 0 ? Math.round(seconds) : 0
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  if (hours > 0 && minutes > 0) return i18n.t('common:duration.hoursMinutes', { hours, minutes })
  if (hours > 0) return i18n.t('common:duration.hours', { hours })
  if (minutes > 0) return i18n.t('common:duration.minutes', { minutes })
  return i18n.t('common:duration.seconds', { seconds: total })
}

/** Locale-formatted number (`12,345`). */
export function formatNumber(n: number): string {
  return new Intl.NumberFormat(currentLocale()).format(Number.isFinite(n) ? n : 0)
}

const BYTE_UNITS = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'] as const

/** `0 B`, `812 KB`, `4.2 MB`, `1.35 GB` (binary multiples). */
export function formatBytes(bytes: number, fractionDigits?: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  const exp = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), BYTE_UNITS.length - 1)
  const value = bytes / 1024 ** exp
  const digits = fractionDigits ?? (exp === 0 || value >= 100 ? 0 : value >= 10 ? 1 : 2)
  const formatted = new Intl.NumberFormat(currentLocale(), { maximumFractionDigits: digits }).format(value)
  return `${formatted} ${BYTE_UNITS[exp]}`
}

/** `320 kbps` (`''` when unknown). */
export function formatBitrate(kbps: number): string {
  return kbps > 0 ? `${Math.round(kbps)} kbps` : ''
}

/** `44.1 kHz` (`''` when unknown). */
export function formatSampleRate(hz: number): string {
  if (!(hz > 0)) return ''
  const khz = new Intl.NumberFormat(currentLocale(), { maximumFractionDigits: 1 }).format(hz / 1000)
  return `${khz} kHz`
}

/** `Sep 23, 2026` / `2026年9月23日`. `0` → `—`. */
export function formatDate(ms: number): string {
  if (!ms) return UNKNOWN
  return new Intl.DateTimeFormat(currentLocale(), { dateStyle: 'medium' }).format(new Date(ms))
}

/** Date and time (`Sep 23, 2026, 2:05 PM`). `0` → `—`. */
export function formatDateTime(ms: number): string {
  if (!ms) return UNKNOWN
  return new Intl.DateTimeFormat(currentLocale(), { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(ms))
}

const RELATIVE_STEPS: readonly (readonly [Intl.RelativeTimeFormatUnit, number])[] = [
  ['second', 60],
  ['minute', 60],
  ['hour', 24],
  ['day', 7],
  ['week', 4.34524],
  ['month', 12],
  ['year', Number.POSITIVE_INFINITY],
]

/** `5 minutes ago`, `yesterday`, `in 2 days`. `0` → `Never`. */
export function formatRelative(ms: number, now: number = Date.now()): string {
  if (!ms) return i18n.t('common:time.never')
  let value = (ms - now) / 1000
  const rtf = new Intl.RelativeTimeFormat(currentLocale(), { numeric: 'auto' })
  for (const [unit, step] of RELATIVE_STEPS) {
    if (Math.abs(value) < step) return rtf.format(Math.round(value), unit)
    value /= step
  }
  return formatDate(ms)
}
