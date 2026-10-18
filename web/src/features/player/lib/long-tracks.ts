/**
 * Long-song filter (docs/architecture/contract.md §9.2): with the playback pref `skipLongTracks`
 * on, songs of `longTrackMinutes` or more are left out when songs are queued. Pure functions
 * without imports so they can be unit-tested (web/tests/long-tracks.test.ts).
 */

/** The fields of a queue entry these helpers read. */
export interface LongTrackItem {
  /** Seconds; 0 = unknown. */
  duration: number
  isRadio?: boolean
}

/** Default limit in minutes. */
export const DEFAULT_LONG_TRACK_MINUTES = 10
/** Range of the limit in minutes. */
export const MIN_LONG_TRACK_MINUTES = 1
export const MAX_LONG_TRACK_MINUTES = 600

/** True for a song at least `limitSeconds` long (live radio and unknown durations never are). */
export function isLongTrack(track: LongTrackItem, limitSeconds: number): boolean {
  return !track.isRadio && limitSeconds > 0 && track.duration >= limitSeconds
}

export interface LongTrackFilter<T> {
  tracks: T[]
  /** Where `keep` ended up in `tracks` (-1 without `keep`). */
  index: number
  /** How many songs were left out. */
  skipped: number
}

/**
 * Leaves out long songs. The song at `keep` (the one the user picked) always stays, and when
 * every song is long nothing is left out, so asking for long songs alone still queues them.
 */
export function withoutLongTracks<T extends LongTrackItem>(
  tracks: readonly T[],
  limitSeconds: number,
  keep?: number,
): LongTrackFilter<T> {
  const picked = keep !== undefined && keep >= 0 && keep < tracks.length ? keep : -1
  if (tracks.every((t) => isLongTrack(t, limitSeconds))) return { tracks: tracks.slice(), index: picked, skipped: 0 }
  const out: T[] = []
  let index = -1
  for (let i = 0; i < tracks.length; i++) {
    if (i === picked) index = out.length
    else if (isLongTrack(tracks[i], limitSeconds)) continue
    out.push(tracks[i])
  }
  return { tracks: out, index, skipped: tracks.length - out.length }
}

/** Clamps a limit in minutes to the allowed whole-minute range (`fallback` when not a number). */
export function clampLongTrackMinutes(value: unknown, fallback = DEFAULT_LONG_TRACK_MINUTES): number {
  if (typeof value !== 'number' || !Number.isFinite(value)) return fallback
  return Math.min(MAX_LONG_TRACK_MINUTES, Math.max(MIN_LONG_TRACK_MINUTES, Math.round(value)))
}
