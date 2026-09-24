/**
 * Server-side queue sync (`PUT /api/queue`, debounced) so Subsonic clients (and other browsers)
 * can resume where the web player left off. Radio entries are never synced, and nothing is
 * sent while a radio station is the current entry.
 */
import { api, type SaveQueueInput } from '@/lib/api/endpoints'

import { usePlayback, usePlayer } from '../store'

const DEBOUNCE_MS = 10_000
/** While playing, re-save the position at most this often. */
const PLAYING_INTERVAL_MS = 30_000

export class QueueSync {
  private timer: ReturnType<typeof setTimeout> | undefined
  private lastSent = ''
  private dirty = false
  private enabled = false
  private lastSaveAt = 0

  /** Start syncing (after the startup restore finished, so we never overwrite it). */
  enable(): void {
    this.enabled = true
    // Remember what the server already has, so an untouched restore isn't sent back.
    this.lastSent = this.serialize(this.payload())
  }

  /** Something changed: save within the debounce window. */
  schedule(delay = DEBOUNCE_MS): void {
    if (!this.enabled) return
    this.dirty = true
    clearTimeout(this.timer)
    this.timer = setTimeout(() => void this.flush(), delay)
  }

  /** Progress tick while playing: keep the server position reasonably fresh. */
  tick(): void {
    if (!this.enabled || this.timer !== undefined) return
    if (Date.now() - this.lastSaveAt >= PLAYING_INTERVAL_MS) this.schedule(0)
  }

  /** Save now if needed. `keepalive` lets the request outlive the page (pagehide). */
  async flush(keepalive = false): Promise<void> {
    clearTimeout(this.timer)
    this.timer = undefined
    if (!this.enabled || !this.dirty) return
    const body = this.payload()
    if (!body) return
    const serialized = this.serialize(body)
    this.dirty = false
    this.lastSaveAt = Date.now()
    if (serialized === this.lastSent) return
    try {
      await api.queue.save(body, { keepalive: keepalive && serialized.length < 60_000 })
      this.lastSent = serialized
    } catch {
      // Offline / not supported: retry with the next change.
      this.dirty = true
    }
  }

  /** Stop without flushing (logout / unmount): pending changes are dropped. */
  dispose(): void {
    clearTimeout(this.timer)
    this.timer = undefined
    this.enabled = false
  }

  private payload(): SaveQueueInput | null {
    const { queue, index } = usePlayer.getState()
    const current = index >= 0 ? queue[index] : undefined
    if (current?.isRadio) return null
    const tracks = queue.filter((t) => !t.isRadio)
    return {
      trackIds: tracks.map((t) => t.id),
      currentId: current?.id ?? '',
      currentIndex: current ? tracks.indexOf(current) : undefined,
      positionMs: current ? Math.max(0, Math.round(usePlayback.getState().currentTime * 1000)) : 0,
    }
  }

  private serialize(body: SaveQueueInput | null): string {
    if (!body) return ''
    // Position is rounded to 5 s so tiny drifts alone don't cause a request.
    return JSON.stringify({ ...body, positionMs: Math.round(body.positionMs / 5000) })
  }
}
