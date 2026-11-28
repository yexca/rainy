/**
 * Program resume (docs/architecture/contract.md §9.4): for program episodes (`Track.program`)
 * the engine asks here where to start, and reports progress so the position is kept as a
 * bookmark (`/api/bookmarks`, shared with Subsonic clients): every 15 s while playing, on
 * pause, on seek, when the entry changes and when the page is hidden. A finished episode, or
 * one stopped in its first seconds, has no bookmark. Other entries are ignored.
 */
import { api } from '@/lib/api/endpoints'
import type { Bookmark } from '@/lib/api/types'
import { queryClient } from '@/lib/query-client'
import { queryKeys } from '@/lib/query-keys'

import { bookmarkAction, resumeAt, sameAction, saveDue, type BookmarkAction } from '../lib/program-resume'
import type { PlayableTrack } from '../types'

/** Give up waiting for the bookmark after this long and start from the beginning (ms). */
const LOOKUP_TIMEOUT_MS = 4000

function isEpisode(entry: PlayableTrack | undefined): entry is PlayableTrack {
  return !!entry?.program && !entry.isRadio
}

/**
 * The bookmark of a track as the freshest cached `['bookmarks', trackIds]` list that asked for
 * it holds it (ms, 0 = none), or undefined when no cached list asked for it.
 */
function cachedBookmarkMs(trackId: string): number | undefined {
  let found: number | undefined
  let updatedAt = -1
  for (const query of queryClient.getQueryCache().findAll({ queryKey: queryKeys.bookmarks })) {
    const ids = query.queryKey[1]
    const data = query.state.data as Bookmark[] | undefined
    if (!data || !Array.isArray(ids) || !ids.includes(trackId) || query.state.dataUpdatedAt <= updatedAt) continue
    updatedAt = query.state.dataUpdatedAt
    found = data.find((b) => b.trackId === trackId)?.positionMs ?? 0
  }
  return found
}

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
  /** Bookmarks this tab last read or wrote, by track id (ms, 0 = none). */
  private readonly known = new Map<string, number>()
  /** Bookmark writes still on their way, by track id; a lookup waits for them. */
  private readonly writes = new Map<string, Promise<unknown>>()

  /**
   * Where a program episode starts when that is known without asking the server (seconds, 0 =
   * the start): from what this tab last saved or read, else from a cached bookmark list such as
   * the album page's. Undefined for other entries and for episodes whose bookmark is unknown. The
   * engine starts the stream there inside the user's gesture; `enter` still reads the bookmark.
   */
  startHint(entry: PlayableTrack | undefined): number | undefined {
    if (!isEpisode(entry)) return undefined
    const ms = this.known.get(entry.id) ?? cachedBookmarkMs(entry.id)
    return ms === undefined ? undefined : resumeAt(ms, entry.duration)
  }

  /**
   * A new entry became current: keeps where the previous episode stopped, then reads the new
   * one's bookmark once any save of it still on its way has landed. Resolves to the position to
   * start from in seconds (0 = the start, and always 0 for entries that are not program
   * episodes).
   */
  async enter(entry: PlayableTrack | undefined): Promise<number> {
    this.flush()
    this.lookup?.abort()
    const seq = ++this.seq
    this.entry = isEpisode(entry) ? entry : undefined
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
      // Re-entering an episode right after leaving it: read the position just saved, not the one before.
      await this.writes.get(target.id)?.catch(() => undefined)
      if (seq !== this.seq) return 0
      const bookmarks = await api.bookmarks.list([target.id], { signal: ctrl.signal })
      const saved = bookmarks.find((b) => b.trackId === target.id)
      if (seq !== this.seq) return 0
      this.known.set(target.id, saved?.positionMs ?? 0)
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
    this.known.set(entry.id, action.kind === 'save' ? action.positionMs : 0)
    const request =
      action.kind === 'save'
        ? api.bookmarks.save(entry.id, action.positionMs, { keepalive })
        : api.bookmarks.delete(entry.id, { keepalive })
    const write = request.then(
      () => {
        // Album pages and the Programs tab show the new progress.
        if (keepalive) return
        void queryClient.invalidateQueries({ queryKey: queryKeys.bookmarks })
        void queryClient.invalidateQueries({ queryKey: queryKeys.programs })
        // A finished episode shows as played once its play count is fresh.
        if (action.kind === 'clear') void queryClient.invalidateQueries({ queryKey: queryKeys.album(entry.albumId) })
      },
      () => {
        // Offline: try again with the next save, and ask the server where the episode is.
        this.known.delete(entry.id)
        if (this.entry === entry && this.last === action) this.last = null
      },
    )
    this.writes.set(entry.id, write)
    void write.finally(() => {
      if (this.writes.get(entry.id) === write) this.writes.delete(entry.id)
    })
  }
}
