/**
 * Local queue persistence (localStorage, per browser).
 *
 * - `rainy.queue`: the queue itself (full track objects so a reload restores instantly, even
 *   offline). Falls back to track ids when the queue is too large for the storage quota.
 * - `rainy.queue.position`: the playback position, written far more often (tiny payload).
 */
import type { QueueSnapshot } from '../store'
import type { PlayableTrack } from '../types'

const QUEUE_KEY = 'rainy.queue'
const POSITION_KEY = 'rainy.queue.position'
const VERSION = 1

interface StoredFull {
  v: typeof VERSION
  kind: 'full'
  /** Owner (the signed-in user when saved). */
  userId: string
  queue: PlayableTrack[]
  index: number
  shuffle: boolean
  /** Positions in `queue` in unshuffled order (while shuffled). */
  original: number[] | null
}

interface StoredIds {
  v: typeof VERSION
  kind: 'ids'
  userId: string
  ids: string[]
  index: number
}

type Stored = StoredFull | StoredIds

interface StoredPosition {
  /** Id of the track the position belongs to. */
  id: string
  index: number
  /** Seconds. */
  time: number
}

export type LoadedQueue =
  | { kind: 'full'; userId: string; snapshot: QueueSnapshot; position: number }
  | { kind: 'ids'; userId: string; ids: string[]; index: number; position: number }
  | { kind: 'empty' }

function read(key: string): unknown {
  try {
    const raw = localStorage.getItem(key)
    return raw ? (JSON.parse(raw) as unknown) : null
  } catch {
    return null
  }
}

function write(key: string, value: unknown): boolean {
  try {
    localStorage.setItem(key, JSON.stringify(value))
    return true
  } catch {
    return false
  }
}

function remove(key: string): void {
  try {
    localStorage.removeItem(key)
  } catch {
    // storage unavailable
  }
}

function isTrackLike(value: unknown): value is PlayableTrack {
  if (typeof value !== 'object' || value === null) return false
  const t = value as Record<string, unknown>
  return typeof t.id === 'string' && typeof t.title === 'string' && typeof t.duration === 'number'
}

/** Save the queue (call whenever queue / index / shuffle change). */
export function saveQueue(snapshot: QueueSnapshot, userId: string): void {
  const { queue, index, shuffle, originalQueue } = snapshot
  if (queue.length === 0) {
    remove(QUEUE_KEY)
    remove(POSITION_KEY)
    return
  }
  let original: number[] | null = null
  if (shuffle && originalQueue) {
    const at = new Map(queue.map((entry, i) => [entry, i]))
    original = originalQueue.map((entry) => at.get(entry) ?? -1).filter((i) => i >= 0)
  }
  const full: StoredFull = { v: VERSION, kind: 'full', userId, queue, index, shuffle, original }
  if (write(QUEUE_KEY, full)) return
  // Quota exceeded: keep the (non-radio) ids and re-fetch the tracks on restore.
  const current = queue[index]
  const kept = queue.filter((t) => !t.isRadio)
  const ids: StoredIds = {
    v: VERSION,
    kind: 'ids',
    userId,
    ids: kept.map((t) => t.id),
    index: current ? Math.max(0, kept.indexOf(current)) : 0,
  }
  if (!write(QUEUE_KEY, ids)) remove(QUEUE_KEY)
}

/** Save the position of the current entry (cheap; call often). */
export function savePosition(track: PlayableTrack | undefined, index: number, time: number): void {
  if (!track || track.isRadio) {
    remove(POSITION_KEY)
    return
  }
  const position: StoredPosition = { id: track.id, index, time: Math.max(0, Math.round(time * 10) / 10) }
  write(POSITION_KEY, position)
}

function readPosition(id: string | undefined, index: number): number {
  const value = read(POSITION_KEY) as Partial<StoredPosition> | null
  if (!value || !id || value.id !== id || value.index !== index) return 0
  return typeof value.time === 'number' && Number.isFinite(value.time) && value.time > 0 ? value.time : 0
}

/** Read the saved queue (validated). */
export function loadQueue(): LoadedQueue {
  const value = read(QUEUE_KEY) as Partial<Stored> | null
  if (!value || value.v !== VERSION) return { kind: 'empty' }

  if (value.kind === 'ids' && Array.isArray(value.ids)) {
    const ids = value.ids.filter((id): id is string => typeof id === 'string')
    if (ids.length === 0) return { kind: 'empty' }
    const index = typeof value.index === 'number' ? Math.min(Math.max(0, value.index), ids.length - 1) : 0
    const userId = typeof value.userId === 'string' ? value.userId : ''
    return { kind: 'ids', userId, ids, index, position: readPosition(ids[index], index) }
  }

  if (value.kind === 'full' && Array.isArray(value.queue)) {
    const queue = value.queue.filter(isTrackLike).map((t) => ({ ...t, genres: Array.isArray(t.genres) ? t.genres : [] }))
    if (queue.length === 0 || queue.length !== value.queue.length) return { kind: 'empty' }
    const index = typeof value.index === 'number' ? Math.min(Math.max(0, value.index), queue.length - 1) : 0
    const shuffle = value.shuffle === true
    let originalQueue: PlayableTrack[] | null = null
    if (shuffle && Array.isArray(value.original) && value.original.length === queue.length) {
      const order = value.original.filter((i): i is number => Number.isInteger(i) && i >= 0 && i < queue.length)
      if (new Set(order).size === queue.length) originalQueue = order.map((i) => queue[i])
    }
    return {
      kind: 'full',
      userId: typeof value.userId === 'string' ? value.userId : '',
      snapshot: { queue, index, shuffle: shuffle && originalQueue !== null, originalQueue },
      position: readPosition(queue[index]?.id, index),
    }
  }
  return { kind: 'empty' }
}

/** Forget the saved queue (logout). */
export function clearSavedQueue(): void {
  remove(QUEUE_KEY)
  remove(POSITION_KEY)
}
