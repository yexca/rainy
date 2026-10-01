/**
 * Player state (docs/architecture/contract.md §9.2) — queue, transport, shuffle/repeat/infinite, volume and the
 * now-playing UI flags. Playback position lives in the separate high-frequency
 * {@link usePlayback} store so track lists don't re-render on every `timeupdate`.
 *
 * Owner: player feature. The audio engine (`components/audio-engine.tsx` → `engine/engine.ts`)
 * subscribes to this store; it also persists the queue (localStorage + `/api/queue`) and
 * restores it on startup (paused).
 *
 * Queue entries are shallow copies of the given tracks, so every entry has its own identity
 * even when the same track is queued twice (shuffle/unshuffle, removals and drag-and-drop rely
 * on it; see {@link entryKey}).
 */
import { create, type StoreApi, type UseBoundStore } from 'zustand'
import { createJSONStorage, persist } from 'zustand/middleware'

import type { Track } from '@/lib/api/types'

import { autoEntries, manualInsertAt } from './lib/infinite'
import type { PlayableTrack, PlayerPanel, RepeatMode } from './types'

export type { PlayableTrack, PlayerPanel, RepeatMode } from './types'

/** Queue contents as restored from storage / the server. */
export interface QueueSnapshot {
  queue: PlayableTrack[]
  index: number
  shuffle: boolean
  /** Unshuffled order (same entry objects as `queue`) while shuffled. */
  originalQueue: PlayableTrack[] | null
}

export interface PlayerState {
  queue: PlayableTrack[]
  /** Index of the current track in `queue`; -1 = nothing loaded. */
  index: number
  isPlaying: boolean
  shuffle: boolean
  repeat: RepeatMode
  /** Infinite mode: with repeat off, songs like the ones playing are added when the queue runs low. */
  infinite: boolean
  /** The queue ended in infinite mode; the next added songs start playing. */
  awaitingMore: boolean
  /** 0..1 (slider position; the engine applies a perceptual curve). */
  volume: number
  muted: boolean
  nowPlayingOpen: boolean
  panel: PlayerPanel
  /** Unshuffled order while `shuffle` is on (same entry objects as `queue`), else null. */
  originalQueue: PlayableTrack[] | null

  /** Replace the queue and start playing. `opts.shuffle` overrides the current shuffle mode. */
  playTracks(tracks: readonly PlayableTrack[], startIndex?: number, opts?: { shuffle?: boolean }): void
  /** Insert right after the current track (starts playback when nothing is loaded). */
  playNext(tracks: readonly PlayableTrack[]): void
  /** Append to the queue, before songs infinite mode added (starts playback when nothing is loaded). */
  addToQueue(tracks: readonly PlayableTrack[]): void
  /** Append songs picked by infinite mode (marked `autoAdded`; continues playback when the queue had ended). */
  appendAuto(tracks: readonly PlayableTrack[]): void
  /** Remove the upcoming songs infinite mode added. */
  clearAuto(): void
  play(): void
  pause(): void
  togglePlay(): void
  /** Skip forward (ignores repeat-one). At the end: wraps with repeat-all, otherwise stops (infinite mode then waits for more). */
  next(): void
  /** Restart the track when > 3 s in, otherwise go to the previous one (wraps with repeat-all). */
  prev(): void
  jumpTo(index: number): void
  removeAt(index: number): void
  move(from: number, to: number): void
  /** Empty the queue and stop. */
  clearQueue(): void
  /** Remove everything after the current track. */
  clearUpcoming(): void
  /** Called by the audio engine when a track finishes (handles repeat-one). */
  onTrackEnded(): void
  setVolume(v: number): void
  toggleMute(): void
  toggleShuffle(): void
  cycleRepeat(): void
  toggleInfinite(): void
  setNowPlayingOpen(open: boolean): void
  setPanel(p: PlayerPanel): void
  /** Replace the queue without starting playback (startup restore). */
  restoreQueue(snapshot: QueueSnapshot): void
  /** Update fields of every queue entry of a track (e.g. after starring it). */
  patchTrack(trackId: string, patch: Partial<Track>): void
}

/** Seconds into a track after which "previous" restarts it instead. */
const RESTART_THRESHOLD = 3

function entries(tracks: readonly PlayableTrack[]): PlayableTrack[] {
  return tracks.map((t) => ({ ...t }))
}

/** Fisher–Yates on a copy. */
function shuffled<T>(items: readonly T[]): T[] {
  const out = items.slice()
  for (let i = out.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1))
    ;[out[i], out[j]] = [out[j], out[i]]
  }
  return out
}

/** Current entry first, the rest shuffled. */
function shuffleAround(queue: readonly PlayableTrack[], current: PlayableTrack | undefined): PlayableTrack[] {
  if (!current) return shuffled(queue)
  return [current, ...shuffled(queue.filter((t) => t !== current))]
}

function clampIndex(index: number, length: number): number {
  if (length === 0) return -1
  return Math.min(Math.max(index, 0), length - 1)
}

export const usePlayer: UseBoundStore<StoreApi<PlayerState>> = create<PlayerState>()(
  persist(
    (set, get) => ({
      queue: [],
      index: -1,
      isPlaying: false,
      shuffle: false,
      repeat: 'off',
      infinite: false,
      awaitingMore: false,
      volume: 1,
      muted: false,
      nowPlayingOpen: false,
      panel: 'none',
      originalQueue: null,

      playTracks: (tracks, startIndex, opts) => {
        if (tracks.length === 0) return
        const list = entries(tracks)
        const doShuffle = opts?.shuffle ?? get().shuffle
        if (doShuffle) {
          const start =
            startIndex === undefined ? Math.floor(Math.random() * list.length) : clampIndex(startIndex, list.length)
          set({
            queue: shuffleAround(list, list[start]),
            originalQueue: list,
            index: 0,
            shuffle: true,
            isPlaying: true,
            awaitingMore: false,
          })
          return
        }
        set({
          queue: list,
          originalQueue: null,
          index: clampIndex(startIndex ?? 0, list.length),
          shuffle: false,
          isPlaying: true,
          awaitingMore: false,
        })
      },

      playNext: (tracks) => {
        if (tracks.length === 0) return
        const { queue, index, originalQueue } = get()
        if (index < 0 || queue.length === 0) {
          set({ queue: entries(tracks), originalQueue: null, index: 0, isPlaying: true })
          return
        }
        const added = entries(tracks)
        const nextQueue = [...queue.slice(0, index + 1), ...added, ...queue.slice(index + 1)]
        let nextOriginal = originalQueue
        if (originalQueue) {
          const at = originalQueue.indexOf(queue[index])
          nextOriginal = [...originalQueue.slice(0, at + 1), ...added, ...originalQueue.slice(at + 1)]
        }
        set({ queue: nextQueue, originalQueue: nextOriginal })
      },

      addToQueue: (tracks) => {
        if (tracks.length === 0) return
        const { queue, index, originalQueue } = get()
        const added = entries(tracks)
        if (index < 0 || queue.length === 0) {
          set({ queue: added, originalQueue: null, index: 0, isPlaying: true })
          return
        }
        const at = manualInsertAt(queue, index)
        let nextOriginal = originalQueue
        if (originalQueue) {
          // Before the same auto entry in the unshuffled order, else at its end.
          const auto = queue[at]
          const atOriginal = auto ? originalQueue.indexOf(auto) : -1
          nextOriginal =
            atOriginal >= 0
              ? [...originalQueue.slice(0, atOriginal), ...added, ...originalQueue.slice(atOriginal)]
              : [...originalQueue, ...added]
        }
        set({ queue: [...queue.slice(0, at), ...added, ...queue.slice(at)], originalQueue: nextOriginal })
      },

      appendAuto: (tracks) => {
        const { queue, index, originalQueue, awaitingMore, infinite, isPlaying } = get()
        if (!infinite || queue.length === 0) return
        const added = autoEntries(tracks, queue)
        if (added.length === 0) {
          if (awaitingMore) set({ awaitingMore: false })
          return
        }
        // Continue only while still stopped at the end (not after the user pressed play again).
        const resume = awaitingMore && !isPlaying && index === queue.length - 1
        set({
          queue: [...queue, ...added],
          originalQueue: originalQueue ? [...originalQueue, ...added] : null,
          awaitingMore: false,
          ...(resume ? { index: index + 1, isPlaying: true } : {}),
        })
      },

      clearAuto: () => {
        const { queue, index, originalQueue } = get()
        const drop = new Set(queue.filter((t, i) => i > index && t.autoAdded))
        if (drop.size === 0) return
        set({
          queue: queue.filter((t) => !drop.has(t)),
          originalQueue: originalQueue ? originalQueue.filter((t) => !drop.has(t)) : null,
        })
      },

      play: () => {
        const { queue, index } = get()
        if (queue.length === 0) return
        set({ isPlaying: true, index: index < 0 ? 0 : index })
      },
      pause: () => set({ isPlaying: false }),
      togglePlay: () => (get().isPlaying ? get().pause() : get().play()),

      next: () => {
        const { queue, index, repeat } = get()
        if (queue.length === 0) return
        if (index < queue.length - 1) {
          set({ index: index + 1, isPlaying: true })
        } else if (repeat === 'all') {
          set({ index: 0, isPlaying: true })
        } else {
          // End of the queue: stop on the last track, rewound. Infinite mode continues as soon
          // as more songs arrive.
          set({ isPlaying: false, awaitingMore: get().infinite && !queue[index]?.isRadio })
          usePlayback.getState().seek(0)
        }
      },

      prev: () => {
        const { queue, index, repeat } = get()
        if (queue.length === 0) return
        const playback = usePlayback.getState()
        if (playback.currentTime > RESTART_THRESHOLD) {
          playback.seek(0)
          return
        }
        if (index > 0) set({ index: index - 1, isPlaying: true })
        else if (repeat === 'all') set({ index: queue.length - 1, isPlaying: true })
        else playback.seek(0)
      },

      jumpTo: (target) => {
        const { queue } = get()
        if (target < 0 || target >= queue.length) return
        set({ index: target, isPlaying: true, awaitingMore: false })
      },

      removeAt: (at) => {
        const { queue, index, originalQueue } = get()
        if (at < 0 || at >= queue.length) return
        const entry = queue[at]
        const nextQueue = queue.filter((_, i) => i !== at)
        const nextOriginal = originalQueue ? originalQueue.filter((t) => t !== entry) : null
        if (nextQueue.length === 0) {
          set({ queue: [], originalQueue: null, index: -1, isPlaying: false })
          return
        }
        let nextIndex = index
        if (at < index) nextIndex = index - 1
        else if (at === index) nextIndex = Math.min(index, nextQueue.length - 1)
        set({ queue: nextQueue, originalQueue: nextOriginal, index: nextIndex })
      },

      move: (from, to) => {
        const { queue, index } = get()
        if (from === to || from < 0 || to < 0 || from >= queue.length || to >= queue.length) return
        const nextQueue = queue.slice()
        const [entry] = nextQueue.splice(from, 1)
        nextQueue.splice(to, 0, entry)
        let nextIndex = index
        if (from === index) nextIndex = to
        else if (from < index && to >= index) nextIndex = index - 1
        else if (from > index && to <= index) nextIndex = index + 1
        set({ queue: nextQueue, index: nextIndex })
      },

      clearQueue: () => set({ queue: [], originalQueue: null, index: -1, isPlaying: false, awaitingMore: false }),

      clearUpcoming: () => {
        const { queue, index, originalQueue } = get()
        if (index < 0) return
        const kept = queue.slice(0, index + 1)
        set({
          queue: kept,
          originalQueue: originalQueue ? originalQueue.filter((t) => kept.includes(t)) : null,
        })
      },

      onTrackEnded: () => {
        if (get().repeat === 'one') {
          usePlayback.getState().seek(0)
          set({ isPlaying: true })
          return
        }
        get().next()
      },

      setVolume: (v) => {
        const volume = Math.min(1, Math.max(0, Number.isFinite(v) ? v : 1))
        set({ volume, muted: volume === 0 ? get().muted : false })
      },
      toggleMute: () => set({ muted: !get().muted }),

      toggleShuffle: () => {
        const { shuffle, queue, index, originalQueue } = get()
        const current = index >= 0 ? queue[index] : undefined
        if (shuffle) {
          const restored = originalQueue ?? queue
          const restoredIndex = current ? restored.indexOf(current) : -1
          set({
            shuffle: false,
            queue: restored,
            originalQueue: null,
            index: restoredIndex >= 0 ? restoredIndex : clampIndex(index, restored.length),
          })
          return
        }
        if (queue.length === 0) {
          set({ shuffle: true })
          return
        }
        set({
          shuffle: true,
          originalQueue: queue,
          queue: shuffleAround(queue, current),
          index: current ? 0 : -1,
        })
      },

      cycleRepeat: () => {
        const order: RepeatMode[] = ['off', 'all', 'one']
        const nextMode = order[(order.indexOf(get().repeat) + 1) % order.length]
        set({ repeat: nextMode })
      },

      toggleInfinite: () => {
        const infinite = !get().infinite
        // Turning it off drops the suggestions that haven't played yet.
        if (!infinite) get().clearAuto()
        set({ infinite, awaitingMore: false })
      },

      setNowPlayingOpen: (nowPlayingOpen) => set({ nowPlayingOpen }),
      setPanel: (panel) => set({ panel }),

      restoreQueue: ({ queue, index, shuffle, originalQueue }) => {
        if (queue.length === 0) {
          set({ queue: [], originalQueue: null, index: -1, isPlaying: false })
          return
        }
        set({
          queue,
          index: clampIndex(index, queue.length),
          shuffle,
          originalQueue: shuffle ? originalQueue : null,
          isPlaying: false,
          awaitingMore: false,
        })
      },

      patchTrack: (trackId, patch) => {
        const { queue, originalQueue } = get()
        if (!queue.some((t) => t.id === trackId)) return
        // New entry objects (so selectors re-render) that inherit the old entries' keys (so the
        // engine and drag-and-drop still see the same entries); shared by both orders.
        const replaced = new Map<PlayableTrack, PlayableTrack>()
        const swap = (entry: PlayableTrack): PlayableTrack => {
          if (entry.id !== trackId) return entry
          let next = replaced.get(entry)
          if (!next) {
            next = { ...entry, ...patch }
            inheritKey(entry, next)
            replaced.set(entry, next)
          }
          return next
        }
        set({ queue: queue.map(swap), originalQueue: originalQueue ? originalQueue.map(swap) : null })
      },
    }),
    {
      name: 'rainy.player',
      version: 1,
      storage: createJSONStorage(() => localStorage),
      // Preferences only; the queue is persisted by the engine (`lib/queue-storage.ts`).
      partialize: (s) => ({ volume: s.volume, muted: s.muted, repeat: s.repeat, infinite: s.infinite }),
    },
  ),
)

/** The track at `index`, or `undefined` when nothing is loaded. */
export const useCurrentTrack = (): PlayableTrack | undefined =>
  usePlayer((s) => (s.index >= 0 ? s.queue[s.index] : undefined))

// ---------------------------------------------------------------------------------------------
// Entry keys
// ---------------------------------------------------------------------------------------------

const keys = new WeakMap<object, string>()
let keySeq = 0

/**
 * Stable, unique key for a queue entry (React keys, drag-and-drop ids, "is this still the same
 * entry" checks in the engine). Survives {@link PlayerState.patchTrack}.
 */
export function entryKey(entry: PlayableTrack): string {
  let key = keys.get(entry)
  if (!key) {
    keySeq += 1
    key = `q${keySeq}`
    keys.set(entry, key)
  }
  return key
}

function inheritKey(from: PlayableTrack, to: PlayableTrack): void {
  keys.set(to, entryKey(from))
}

// ---------------------------------------------------------------------------------------------
// High-frequency playback position
// ---------------------------------------------------------------------------------------------

export type SeekHandler = (time: number) => void

export interface PlaybackState {
  /** Seconds. */
  currentTime: number
  /** Seconds (0 while unknown, and for live streams). */
  duration: number
  /** Seconds buffered ahead from the start (end of the buffered range containing currentTime). */
  buffered: number
  /** Waiting for data while playback was requested. */
  buffering: boolean
  /** Seek the audio engine (no-op until an engine registered its handler). */
  seek(time: number): void
  /** Used by the audio engine to publish progress. */
  update(patch: Partial<Pick<PlaybackState, 'currentTime' | 'duration' | 'buffered' | 'buffering'>>): void
  /** The audio engine registers how to seek; returns an unregister function. */
  registerSeekHandler(handler: SeekHandler): () => void
}

let seekHandler: SeekHandler | null = null

export const usePlayback: UseBoundStore<StoreApi<PlaybackState>> = create<PlaybackState>()((set, get) => ({
  currentTime: 0,
  duration: 0,
  buffered: 0,
  buffering: false,
  seek: (time) => {
    if (!Number.isFinite(time)) return
    const { duration } = get()
    const target = Math.max(0, duration > 0 ? Math.min(time, duration) : time)
    set({ currentTime: target })
    seekHandler?.(target)
  },
  update: (patch) => set(patch),
  registerSeekHandler: (handler) => {
    seekHandler = handler
    return () => {
      if (seekHandler === handler) seekHandler = null
    }
  },
}))
