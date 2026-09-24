/**
 * Browser audio capability detection (evaluated lazily, cached).
 */
import { isIOS } from '@/lib/platform'

import type { PlayableTrack, TranscodeFormat } from '../types'

let probe: HTMLAudioElement | null = null
const cache = new Map<string, boolean>()

function probeElement(): HTMLAudioElement | null {
  if (typeof document === 'undefined') return null
  probe ??= document.createElement('audio')
  return probe
}

/** `canPlayType` as a boolean ("maybe" counts as yes), cached per MIME string. */
export function canPlayMime(mime: string): boolean {
  if (!mime) return false
  const cached = cache.get(mime)
  if (cached !== undefined) return cached
  const el = probeElement()
  const result = !!el && el.canPlayType(mime) !== ''
  cache.set(mime, result)
  return result
}

/** The most specific MIME type (with codecs) we can derive for a track. */
export function playbackMime(track: Pick<PlayableTrack, 'suffix' | 'codec' | 'contentType'>): string {
  const suffix = track.suffix.toLowerCase()
  const codec = track.codec.toLowerCase()
  switch (suffix) {
    case 'mp3':
      return 'audio/mpeg'
    case 'opus':
      return 'audio/ogg; codecs="opus"'
    case 'ogg':
    case 'oga':
      if (codec.includes('opus')) return 'audio/ogg; codecs="opus"'
      if (codec.includes('flac')) return 'audio/ogg; codecs="flac"'
      return 'audio/ogg; codecs="vorbis"'
    case 'm4a':
    case 'm4b':
    case 'mp4':
    case 'alac':
      if (codec.includes('alac') || suffix === 'alac') return 'audio/mp4; codecs="alac"'
      if (codec.includes('flac')) return 'audio/mp4; codecs="flac"'
      return 'audio/mp4; codecs="mp4a.40.2"'
    case 'aac':
      return 'audio/aac'
    case 'flac':
      return 'audio/flac'
    case 'wav':
      return 'audio/wav'
    case 'aif':
    case 'aiff':
      return 'audio/aiff'
    default:
      return track.contentType || ''
  }
}

/** Whether the browser can play the original file without transcoding. */
export function canPlayNatively(track: Pick<PlayableTrack, 'suffix' | 'codec' | 'contentType'>): boolean {
  return canPlayMime(playbackMime(track))
}

const FORMAT_MIME: Record<TranscodeFormat, string> = {
  mp3: 'audio/mpeg',
  opus: 'audio/ogg; codecs="opus"',
  aac: 'audio/aac',
}

/** Whether the browser can play a transcoded stream in `format`. */
export function canPlayFormat(format: TranscodeFormat): boolean {
  if (format === 'mp3') return true
  if (format === 'aac') return canPlayMime(FORMAT_MIME.aac) || canPlayMime('audio/mp4; codecs="mp4a.40.2"')
  return canPlayMime(FORMAT_MIME[format])
}

/** The preferred transcode format if playable here, else AAC (Apple) / MP3. */
export function playableFormat(preferred: TranscodeFormat): TranscodeFormat {
  if (canPlayFormat(preferred)) return preferred
  if (isIOS && canPlayFormat('aac')) return 'aac'
  return 'mp3'
}

let volumeControllable: boolean | undefined

/**
 * Whether `audio.volume` can be changed. iOS ignores it (hardware volume only), so the volume
 * slider and ReplayGain are disabled there.
 */
export function canControlVolume(): boolean {
  if (volumeControllable !== undefined) return volumeControllable
  if (isIOS) {
    volumeControllable = false
    return false
  }
  const el = probeElement()
  if (!el) return true
  try {
    const before = el.volume
    el.volume = 0.5
    volumeControllable = Math.abs(el.volume - 0.5) < 0.01
    el.volume = before
  } catch {
    volumeControllable = false
  }
  return volumeControllable
}

/** Safari exposes AirPlay target picking on media elements. */
export function supportsAirPlay(): boolean {
  return typeof window !== 'undefined' && 'WebKitPlaybackTargetAvailabilityEvent' in window
}
