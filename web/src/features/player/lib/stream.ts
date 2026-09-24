/**
 * Stream planning: which URL to load for a queue entry given the user's quality settings.
 *
 * Mirrors the server's `transcode.Decide` (docs/architecture/contract.md §5.10) so the client knows whether a
 * stream is the original file (byte-range seekable) or a live transcode (seek by reloading
 * with `offset`). We only request a format when the server will actually transcode:
 * - the browser can't play the original codec → transcode (320 kbps in "original" mode);
 * - a bit-rate cap is set and the file is lossless, of unknown bit rate, or above the cap.
 */
import { api } from '@/lib/api/endpoints'

import type { PlaybackPrefs } from '../prefs'
import type { PlayableTrack, TranscodeFormat } from '../types'
import { canPlayNatively, playableFormat } from './audio-support'

export interface StreamPlan {
  url: string
  /** The server transcodes (no Range support: seeking reloads with `offset`). */
  transcoded: boolean
  /** Live radio (no duration, no seeking). */
  live: boolean
  /** Seconds into the track at which this stream starts (transcoded streams only). */
  offset: number
  format?: TranscodeFormat
  bitrate?: number
}

const LOSSLESS_SUFFIXES = new Set(['flac', 'wav', 'aif', 'aiff', 'ape', 'wv', 'alac', 'dsf', 'dff', 'tta'])
const LOSSLESS_CODECS = ['flac', 'alac', 'pcm', 'wav', 'aiff', 'ape', 'monkey', 'wavpack', 'tta', 'dsd']
/** TagLib reports a bit depth for some lossy codecs (e.g. AAC in m4a), so these are checked before bitDepth. */
const LOSSY_CODECS = ['aac', 'mp3', 'mpeg', 'vorbis', 'opus', 'speex', 'musepack', 'mpc']

/** Same heuristics as the server's `transcode.IsLossless`. */
export function isLossless(track: Pick<PlayableTrack, 'suffix' | 'codec' | 'bitDepth'>): boolean {
  if (LOSSLESS_SUFFIXES.has(track.suffix.toLowerCase())) return true
  const codec = track.codec.toLowerCase()
  if (LOSSLESS_CODECS.some((c) => codec.includes(c))) return true
  if (LOSSY_CODECS.some((c) => codec.includes(c))) return false
  return track.bitDepth > 0
}

/** Transcode fallback bit rate when the original codec is unplayable in "original" mode. */
const FALLBACK_BITRATE = 320

export function planStream(track: PlayableTrack, prefs: Pick<PlaybackPrefs, 'quality' | 'format'>, startAt = 0): StreamPlan {
  if (track.isRadio) {
    return { url: track.streamUrlOverride ?? '', transcoded: false, live: true, offset: 0 }
  }
  const cap = prefs.quality === 'original' ? 0 : Number(prefs.quality)
  let bitrate = 0
  if (!canPlayNatively(track)) {
    bitrate = cap || FALLBACK_BITRATE
  } else if (cap > 0 && (isLossless(track) || track.bitrate <= 0 || track.bitrate > cap)) {
    bitrate = cap
  }
  if (bitrate === 0) {
    return { url: api.streamUrl(track.id, { format: 'raw' }), transcoded: false, live: false, offset: 0 }
  }
  const format = playableFormat(prefs.format)
  const offset = startAt > 0.5 ? startAt : 0
  return {
    url: api.streamUrl(track.id, { format, bitrate, offset }),
    transcoded: true,
    live: false,
    offset,
    format,
    bitrate,
  }
}
