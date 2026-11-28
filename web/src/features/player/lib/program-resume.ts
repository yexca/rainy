/**
 * Program resume (docs/architecture/contract.md §9.4): program episodes (`Track.program`) keep a
 * bookmark of where the listener stopped, shared with Subsonic bookmarks. Pure functions without
 * imports so they can be unit-tested (web/tests/program-resume.test.ts).
 */

/** A bookmark this close to the start is not worth resuming (seconds). */
export const RESUME_MIN_SECONDS = 5
/** Within this much of the end an episode counts as finished: its bookmark is removed (seconds). */
export const FINISHED_MARGIN_SECONDS = 30
/** While playing, the position is saved at most this often (ms). */
export const SAVE_INTERVAL_MS = 15_000

/** True when the episode is finished at `position` seconds of `duration` (0 = unknown). */
export function isFinished(position: number, duration: number): boolean {
  return duration > 0 && position >= duration - FINISHED_MARGIN_SECONDS
}

/**
 * Where to start an episode from its bookmark: the saved position in seconds when it lies past
 * the first seconds and before the last ones, else 0 (from the start).
 */
export function resumeAt(bookmarkMs: number | undefined, duration: number): number {
  if (bookmarkMs === undefined || !Number.isFinite(bookmarkMs)) return 0
  const position = bookmarkMs / 1000
  if (position <= RESUME_MIN_SECONDS || isFinished(position, duration)) return 0
  return position
}

/** What to store for a position: a bookmark, or none (at the start, or finished). */
export type BookmarkAction = { kind: 'save'; positionMs: number } | { kind: 'clear' }

export function bookmarkAction(position: number, duration: number): BookmarkAction {
  if (!Number.isFinite(position) || position <= RESUME_MIN_SECONDS || isFinished(position, duration)) {
    return { kind: 'clear' }
  }
  return { kind: 'save', positionMs: Math.round(position * 1000) }
}

/**
 * True when `next` would store the same thing as `last` (the same kind, positions within a
 * second), so it need not be sent again.
 */
export function sameAction(last: BookmarkAction | null, next: BookmarkAction): boolean {
  if (!last || last.kind !== next.kind) return false
  return next.kind === 'clear' || Math.abs((last as { positionMs: number }).positionMs - next.positionMs) < 1000
}

/** True when a periodic save is due while playing. */
export function saveDue(now: number, lastSaveAt: number): boolean {
  return now - lastSaveAt >= SAVE_INTERVAL_MS
}
