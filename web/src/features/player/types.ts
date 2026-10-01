/**
 * Types shared by the player and the features that feed it (library, radio).
 *
 * Radio contract (library ↔ player): a radio station is queued as a `PlayableTrack` with
 * `isRadio: true` and `streamUrlOverride` set to the station's stream URL. For such items the
 * engine plays `streamUrlOverride` as-is, disables seeking, scrobbling, preloading and the
 * server-side queue sync, and the UI shows a "Live" badge instead of a scrubber.
 */
import type { Track } from '@/lib/api/types'

export type PlayableTrack = Track & {
  /** Play this URL instead of `/api/stream/{id}` (internet radio). */
  streamUrlOverride?: string
  /** Live stream: no duration, no seeking, no scrobbling. */
  isRadio?: boolean
  /** Added by infinite mode (shown as a suggestion; "Add to queue" goes before these). */
  autoAdded?: boolean
}

export type RepeatMode = 'off' | 'all' | 'one'
export type PlayerPanel = 'none' | 'queue' | 'lyrics'

/** Streaming quality: `original` = the file as stored (transcoded only if the browser can't play it). */
export const STREAM_QUALITIES = ['original', '320', '192', '128'] as const
export type StreamQuality = (typeof STREAM_QUALITIES)[number]

export const TRANSCODE_FORMATS = ['mp3', 'opus', 'aac'] as const
export type TranscodeFormat = (typeof TRANSCODE_FORMATS)[number]

export const REPLAY_GAIN_MODES = ['off', 'track', 'album'] as const
export type ReplayGainMode = (typeof REPLAY_GAIN_MODES)[number]

/** True for live radio entries. */
export function isLive(track: PlayableTrack | undefined | null): boolean {
  return !!track?.isRadio
}
