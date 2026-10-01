/**
 * Infinite mode helpers (docs/architecture/contract.md §9.2): when the queue runs low, songs
 * picked by `/api/recommend/mix` are appended as `autoAdded` entries. Pure functions without
 * imports so they can be unit-tested (web/tests/infinite.test.ts).
 */

/** The fields of a queue entry these helpers read. */
export interface QueueItem {
  id: string
  isRadio?: boolean
  autoAdded?: boolean
}

/** Ask for more songs when this few (or fewer) are left after the current one. */
export const REFILL_THRESHOLD = 2
/** Songs per request. */
export const REFILL_SIZE = 10
/** Most recent songs sent as seeds (server limit 10). */
export const MAX_SEEDS = 5
/** Queue songs sent as "don't pick these" (server limit 1000). */
export const MAX_EXCLUDE = 500

export interface InfiniteState {
  infinite: boolean
  repeat: 'off' | 'all' | 'one'
  queue: readonly QueueItem[]
  index: number
}

/** True when infinite mode should fetch more songs now. */
export function needsRefill({ infinite, repeat, queue, index }: InfiniteState): boolean {
  if (!infinite || repeat !== 'off' || index < 0 || queue.length === 0) return false
  if (queue[index]?.isRadio) return false
  return queue.length - 1 - index <= REFILL_THRESHOLD
}

/**
 * Seeds: the current song and the ones just before it (newest last), live radio left out.
 * Upcoming songs the user queued themselves count too, so a mix follows what they chose.
 */
export function mixSeeds(queue: readonly QueueItem[], index: number): string[] {
  const upcoming = queue.slice(index + 1).filter((t) => !t.autoAdded)
  const recent = [...queue.slice(0, index + 1), ...upcoming].filter((t) => !t.isRadio)
  return dedupe(recent.map((t) => t.id).reverse())
    .slice(0, MAX_SEEDS)
    .reverse()
}

/** Ids already in the queue, the most recent ones first when the queue is long. */
export function mixExclude(queue: readonly QueueItem[]): string[] {
  return dedupe(
    queue
      .filter((t) => !t.isRadio)
      .map((t) => t.id)
      .reverse(),
  ).slice(0, MAX_EXCLUDE)
}

/** A key for the request a queue state would make, so the same request isn't repeated. */
export function refillKey(queue: readonly QueueItem[], index: number): string {
  return `${queue.length}:${index}:${mixSeeds(queue, index).join(',')}`
}

/**
 * Where manually queued songs go: before the first song infinite mode added after the
 * current one (so "Add to queue" doesn't wait behind suggestions), else at the end.
 */
export function manualInsertAt(queue: readonly QueueItem[], index: number): number {
  for (let i = Math.max(index + 1, 0); i < queue.length; i++) {
    if (queue[i].autoAdded) return i
  }
  return queue.length
}

/** Entries for songs infinite mode adds; songs already in the queue are dropped. */
export function autoEntries<T extends QueueItem>(tracks: readonly T[], queue: readonly QueueItem[]): T[] {
  const have = new Set(queue.map((t) => t.id))
  const out: T[] = []
  for (const t of tracks) {
    if (have.has(t.id)) continue
    have.add(t.id)
    out.push({ ...t, autoAdded: true })
  }
  return out
}

function dedupe(ids: string[]): string[] {
  return [...new Set(ids)]
}
