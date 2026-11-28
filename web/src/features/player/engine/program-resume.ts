/**
 * Program resume (docs/architecture/contract.md §9.4): for program episodes (`Track.program`)
 * the engine asks here where to start, and reports progress so the position is kept as a
 * bookmark (`/api/bookmarks`, shared with Subsonic clients): every 15 s while playing, on
 * pause, on seek, when the entry changes and when the page is hidden. A finished episode, or
 * one stopped in its first seconds, has no bookmark. Other entries are ignored.
 */
import { api } from '@/lib/api/endpoints'
import { queryClient } from '@/lib/query-client'
import { queryKeys } from '@/lib/query-keys'

import { bookmarkAction, resumeAt, sameAction, saveDue, type BookmarkAction } from '../lib/program-resume'
import type { PlayableTrack } from '../types'

/** Give up waiting for the bookmark after this long and start from the beginning (ms). */
const LOOKUP_TIMEOUT_MS = 4000

export class ProgramResume {
  private entry: PlayableTrack | undefined
  /** The entry's bookmark was read (or could not be): saving may replace it. */
  private ready = false
  private seq = 0
  /**
   * Last position seen while playing, paused or seeking; null until then, so an entry restored
   * at startup (or still waiting for its bookmark) never overwrites the bookmark with 0.
   */
  private time: number | null = null
  private lastSaveAt = 0
  private last: BookmarkAction | null = null
  private lookup: AbortController | null = null

  /**
   * A new entry became current: keeps where the previous episode stopped, then reads the new
   * one's bookmark. Resolves to the position to start from in seconds (0 = the start, and
   * always 0 for entries that are not program episodes).
   */
  async enter(entry: PlayableTrack | undefined): Promise<number> {
    this.flush()
    this.lookup?.abort()
    const seq = ++this.seq
    this.entry = entry?.program && !entry.isRadio ? entry : undefined
    this.ready = false
    this.time = null
    this.last = null
    this.lastSaveAt = Date.now()
    const target = this.entry
    if (!target) return 0
    const ctrl = new AbortController()
    this.lookup = ctrl
    const timer = setTimeout(() => ctrl.abort(), LOOKUP_TIMEOUT_MS)
    try {
      const bookmarks = await api.bookmarks.list([target.id], { signal: ctrl.signal })
      const saved = bookmarks.find((b) => b.trackId === target.id)
      if (seq !== this.seq) return 0
      this.last = saved ? { kind: 'save', positionMs: saved.positionMs } : { kind: 'clear' }
      return resumeAt(saved?.positionMs, target.duration)
    } catch {
      return 0
    } finally {
      clearTimeout(timer)
      if (seq === this.seq) this.ready = true
    }
  }

  /** Progress tick (seconds into the entry); only ticks while playing count. */
  tick(time: number, playing: boolean): void {
    if (!this.entry || !playing) return
    this.time = time
    if (saveDue(Date.now(), this.lastSaveAt)) this.save()
  }

  /** Paused or moved to `time`: save now. */
  stopped(time: number): void {
    if (!this.entry) return
    this.time = time
    this.save()
  }

  /** Played to the end: the episode is done. */
  finished(): void {
    if (this.entry) this.stopped(Math.max(this.time ?? 0, this.entry.duration))
  }

  /** Save the last known position now; `keepalive` lets the request outlive the page. */
  flush(keepalive = false): void {
    if (this.entry) this.save(keepalive)
  }

  private save(keepalive = false): void {
    const entry = this.entry
    if (!entry || !this.ready || this.time === null) return
    this.lastSaveAt = Date.now()
    const action = bookmarkAction(this.time, entry.duration)
    if (sameAction(this.last, action)) return
    this.last = action
    const request =
      action.kind === 'save'
        ? api.bookmarks.save(entry.id, action.positionMs, { keepalive })
        : api.bookmarks.delete(entry.id, { keepalive })
    request.then(
      () => {
        // Album pages and the Programs tab show the new progress.
        if (keepalive) return
        void queryClient.invalidateQueries({ queryKey: queryKeys.bookmarks })
        void queryClient.invalidateQueries({ queryKey: queryKeys.programs })
        // A finished episode shows as played once its play count is fresh.
        if (action.kind === 'clear') void queryClient.invalidateQueries({ queryKey: queryKeys.album(entry.albumId) })
      },
      () => {
        // Offline: try again with the next save.
        if (this.entry === entry && this.last === action) this.last = null
      },
    )
  }
}
