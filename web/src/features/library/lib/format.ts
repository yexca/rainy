/** Display helpers for library pages. */
import type { Track } from '@/lib/api/types'
import { formatSampleRate } from '@/lib/format'

const LOSSLESS = new Set(['flac', 'alac', 'wav', 'aif', 'aiff', 'ape', 'wv', 'tta', 'dsf', 'dff'])

function mostCommon<T>(values: readonly T[]): T | undefined {
  const counts = new Map<T, number>()
  let best: T | undefined
  let bestCount = 0
  for (const value of values) {
    const n = (counts.get(value) ?? 0) + 1
    counts.set(value, n)
    if (n > bestCount) {
      best = value
      bestCount = n
    }
  }
  return best
}

/** Display name of a format: MP4 containers are named after their codec (`AAC`, `ALAC`). */
export function formatLabel(suffix: string, codec: string): string {
  const lower = suffix.toLowerCase()
  if (['m4a', 'm4b', 'mp4'].includes(lower)) {
    const c = codec.toLowerCase()
    if (c.includes('alac')) return 'ALAC'
    if (c.includes('aac') || !c) return 'AAC'
    return codec.toUpperCase()
  }
  return lower.toUpperCase()
}

/**
 * Short audio-quality summary of a set of tracks, e.g. `FLAC · 24-bit / 96 kHz` or
 * `MP3 · 320 kbps` (from the most common format; `''` when unknown).
 */
export function formatQuality(tracks: readonly Track[]): string {
  if (tracks.length === 0) return ''
  const suffix = mostCommon(tracks.map((t) => t.suffix.toLowerCase()).filter(Boolean))
  if (!suffix) return ''
  const same = tracks.filter((t) => t.suffix.toLowerCase() === suffix)
  const label = formatLabel(suffix, same[0]?.codec ?? '')
  if (LOSSLESS.has(suffix) || label === 'ALAC') {
    const depth = mostCommon(same.map((t) => t.bitDepth).filter((n) => n > 0))
    const rate = mostCommon(same.map((t) => t.sampleRate).filter((n) => n > 0))
    const parts = [depth ? `${depth}-bit` : '', rate ? formatSampleRate(rate) : ''].filter(Boolean)
    return parts.length > 0 ? `${label} · ${parts.join(' / ')}` : label
  }
  const bitrates = same.map((t) => t.bitrate).filter((n) => n > 0)
  if (bitrates.length === 0) return label
  const avg = Math.round(bitrates.reduce((a, b) => a + b, 0) / bitrates.length)
  return `${label} · ${avg} kbps`
}

/** Sum of track durations in seconds. */
export function totalDuration(tracks: readonly Track[]): number {
  return tracks.reduce((sum, t) => sum + (Number.isFinite(t.duration) ? t.duration : 0), 0)
}

/** Earliest non-empty release date of the tracks (`2019-05-17`, or `''`). */
export function releaseDate(tracks: readonly Track[]): string {
  const dates = tracks.map((t) => t.date).filter((d) => /^\d{4}-\d{2}(-\d{2})?$/.test(d))
  dates.sort()
  return dates[0] ?? ''
}

/** `2019-05-17` → `May 17, 2019` / `2019年5月17日`; `2019-05` → `May 2019`. */
export function formatReleaseDate(date: string, locale: string): string {
  const match = /^(\d{4})-(\d{2})(?:-(\d{2}))?$/.exec(date)
  if (!match) return date
  const [, y, m, d] = match
  const value = new Date(Date.UTC(Number(y), Number(m) - 1, d ? Number(d) : 1))
  const options: Intl.DateTimeFormatOptions = d
    ? { year: 'numeric', month: 'long', day: 'numeric', timeZone: 'UTC' }
    : { year: 'numeric', month: 'long', timeZone: 'UTC' }
  return new Intl.DateTimeFormat(locale, options).format(value)
}

/** Stored multi-genre strings (`"Pop; Rock"`) → `Pop, Rock`. */
export function formatGenres(genre: string): string {
  return genre
    .split(/\s*;\s*/)
    .filter(Boolean)
    .join(', ')
}
