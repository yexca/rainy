/**
 * Scrobbling (docs/architecture/contract.md §9.4): "now playing" once playback of an entry starts, a
 * submission after the user actually listened to 50 % of the track or 4 minutes (whichever
 * comes first). Only real listening time counts — seeking forward does not.
 */
import { api } from '@/lib/api/endpoints'

import type { PlayableTrack } from '../types'

/** Longest gap between two progress ticks still counted as continuous listening (s). */
const MAX_TICK = 2
const MAX_THRESHOLD = 240

export class Scrobbler {
  private track: PlayableTrack | undefined
  private startedAt = 0
  private listened = 0
  private lastTime = -1
  private nowPlayingSent = false
  private submitted = false

  /** Start tracking a new play of `track` (a new entry, or a repeat of the same one). */
  reset(track: PlayableTrack | undefined): void {
    this.track = track
    this.startedAt = 0
    this.listened = 0
    this.lastTime = -1
    this.nowPlayingSent = false
    this.submitted = false
  }

  /** Playback actually started (the `playing` event). */
  onPlaying(time: number): void {
    const track = this.track
    if (!track || track.isRadio) return
    this.lastTime = time
    if (!this.startedAt) this.startedAt = Date.now()
    if (!this.nowPlayingSent) {
      this.nowPlayingSent = true
      send(track.id, false)
    }
  }

  /** Progress tick (`timeupdate`), `time` in seconds into the track. */
  onTime(time: number, playing: boolean): void {
    const track = this.track
    if (!track || track.isRadio || this.submitted) return
    if (playing && this.lastTime >= 0) {
      const delta = time - this.lastTime
      if (delta > 0 && delta <= MAX_TICK) this.listened += delta
    }
    this.lastTime = time
    const threshold = track.duration > 0 ? Math.min(track.duration / 2, MAX_THRESHOLD) : MAX_THRESHOLD
    if (this.listened >= threshold) {
      this.submitted = true
      send(track.id, true, this.startedAt || Date.now())
    }
  }

  /** Position jumped (seek): don't count the jump as listening time. */
  onSeek(time: number): void {
    this.lastTime = time
  }
}

function send(trackId: string, submission: boolean, time?: number): void {
  api.scrobble({ trackId, submission, time }).catch(() => {
    // Best effort: scrobbles are not worth bothering the listener about.
  })
}
