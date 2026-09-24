/**
 * Media Session integration: lock screen / notification / hardware key controls.
 */
import i18n from '@/lib/i18n'
import { coverUrlForPixels } from '@/lib/cover'

import type { PlayableTrack } from '../types'

const ARTWORK_SIZES = [128, 256, 512] as const

export interface MediaSessionHandlers {
  play(): void
  pause(): void
  stop(): void
  previous(): void
  next(): void
  seekTo(time: number): void
  seekBy(delta: number): void
}

function session(): MediaSession | null {
  return typeof navigator !== 'undefined' && 'mediaSession' in navigator ? navigator.mediaSession : null
}

function setHandler(ms: MediaSession, action: MediaSessionAction, handler: MediaSessionActionHandler | null): void {
  try {
    ms.setActionHandler(action, handler)
  } catch {
    // Action not supported by this browser.
  }
}

/** Register (or, with `null`, remove) every action handler. `live` disables seeking. */
export function setMediaSessionHandlers(handlers: MediaSessionHandlers | null, live = false): void {
  const ms = session()
  if (!ms) return
  const h = handlers
  setHandler(ms, 'play', h ? () => h.play() : null)
  setHandler(ms, 'pause', h ? () => h.pause() : null)
  setHandler(ms, 'stop', h ? () => h.stop() : null)
  setHandler(ms, 'previoustrack', h ? () => h.previous() : null)
  setHandler(ms, 'nexttrack', h ? () => h.next() : null)
  const seekable = h && !live
  setHandler(
    ms,
    'seekto',
    seekable
      ? (details) => {
          if (typeof details.seekTime === 'number') h.seekTo(details.seekTime)
        }
      : null,
  )
  setHandler(ms, 'seekbackward', seekable ? (details) => h.seekBy(-(details.seekOffset ?? 10)) : null)
  setHandler(ms, 'seekforward', seekable ? (details) => h.seekBy(details.seekOffset ?? 10) : null)
}

export function setMediaSessionMetadata(track: PlayableTrack | undefined): void {
  const ms = session()
  if (!ms) return
  if (!track || typeof MediaMetadata === 'undefined') {
    ms.metadata = null
    return
  }
  const artwork = track.coverArt
    ? ARTWORK_SIZES.map((size) => ({
        src: new URL(coverUrlForPixels(track.coverArt, size), window.location.href).href,
        sizes: `${size}x${size}`,
        type: 'image/jpeg',
      }))
    : []
  ms.metadata = new MediaMetadata({
    title: track.title,
    artist: track.isRadio ? i18n.t('player:live.radio') : track.artist,
    album: track.isRadio ? '' : track.album,
    artwork,
  })
}

export function setMediaSessionPlaybackState(state: MediaSessionPlaybackState): void {
  const ms = session()
  if (ms) ms.playbackState = state
}

/** Position for the lock screen scrubber; cleared for live streams / unknown duration. */
export function setMediaSessionPosition(position: number, duration: number, rate = 1): void {
  const ms = session()
  if (!ms || typeof ms.setPositionState !== 'function') return
  try {
    if (!(duration > 0) || !Number.isFinite(duration)) {
      ms.setPositionState()
      return
    }
    ms.setPositionState({
      duration,
      position: Math.min(Math.max(0, position), duration),
      playbackRate: rate > 0 ? rate : 1,
    })
  } catch {
    // Invalid state (e.g. duration not yet known): ignore.
  }
}
