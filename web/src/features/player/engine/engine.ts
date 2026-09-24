/**
 * The audio engine (docs/architecture/contract.md §9.4): drives two `HTMLAudioElement`s from the player store.
 *
 * - The *primary* element plays the current entry. ~20 s before it ends, the *spare* element
 *   buffers the next entry; on `ended` the two are swapped for near-gapless playback. (On iOS
 *   only one element is used: WebKit requires a user gesture per element.)
 * - Loading is lazy: while paused, changing tracks only updates the UI; the stream is loaded
 *   when playback starts (so restoring a queue or skipping while paused costs nothing).
 * - Everything reacts synchronously to store changes (zustand `subscribe`), so `play()` runs
 *   inside the user's click — required by iOS / autoplay policies.
 * - Original files are seeked with byte ranges; transcoded streams are re-requested with
 *   `offset` when seeking outside what is buffered.
 * - Also: Media Session, scrobbling, ReplayGain, error handling (toast + skip, stop after 3
 *   consecutive failures), queue persistence (localStorage + debounced `/api/queue`) and the
 *   startup restore (paused), keyboard shortcuts.
 */
import { toast } from 'sonner'
import { create } from 'zustand'

import { api } from '@/lib/api/endpoints'
import type { AuthStatus, Track } from '@/lib/api/types'
import i18n from '@/lib/i18n'
import { isIOS } from '@/lib/platform'
import { authQueryKey, queryClient } from '@/lib/query-client'

import { canControlVolume, supportsAirPlay } from '../lib/audio-support'
import { loadQueue, savePosition, saveQueue } from '../lib/queue-storage'
import { replayGainFactor, sliderToGain } from '../lib/replay-gain'
import { planStream, type StreamPlan } from '../lib/stream'
import { usePlaybackPrefs, type PlaybackPrefsState } from '../prefs'
import { entryKey, usePlayback, usePlayer, type PlayerState } from '../store'
import type { PlayableTrack } from '../types'
import { installKeyboardShortcuts } from './keyboard'
import {
  setMediaSessionHandlers,
  setMediaSessionMetadata,
  setMediaSessionPlaybackState,
  setMediaSessionPosition,
  type MediaSessionHandlers,
} from './media-session'
import { QueueSync } from './queue-sync'
import { Scrobbler } from './scrobbler'

/** Start buffering the next track this many seconds before the current one ends. */
const PRELOAD_BEFORE_END = 20
/** Stop skipping after this many consecutive failures. */
const MAX_FAILURES = 3
/** Minimum interval between local position saves while playing (ms). */
const POSITION_SAVE_INTERVAL = 1000
/** Chunk size when re-fetching a queue by ids. */
const ID_CHUNK = 200

interface Loaded {
  entry: PlayableTrack
  key: string
  plan: StreamPlan
  /** A network error already triggered one reload of this entry. */
  retried: boolean
}

interface WebKitAudioElement extends HTMLAudioElement {
  webkitShowPlaybackTargetPicker?: () => void
}

/** AirPlay availability (Safari). */
export const useAirPlay = create<{ available: boolean }>()(() => ({ available: false }))

let activePicker: (() => void) | null = null

/** Open the AirPlay device picker (Safari only). */
export function showAirPlayPicker(): void {
  activePicker?.()
}

function currentOf(state: PlayerState): PlayableTrack | undefined {
  return state.index >= 0 ? state.queue[state.index] : undefined
}

function inRanges(ranges: TimeRanges, t: number): boolean {
  for (let i = 0; i < ranges.length; i++) {
    if (ranges.start(i) <= t && t <= ranges.end(i)) return true
  }
  return false
}

function bufferedEnd(el: HTMLAudioElement): number {
  const { buffered, currentTime } = el
  for (let i = 0; i < buffered.length; i++) {
    if (buffered.start(i) <= currentTime && currentTime <= buffered.end(i)) return buffered.end(i)
  }
  return buffered.length > 0 ? buffered.end(buffered.length - 1) : 0
}

function currentUserId(): string {
  return queryClient.getQueryData<AuthStatus>(authQueryKey)?.user?.id ?? ''
}

export class PlayerEngine {
  private readonly els: [HTMLAudioElement, HTMLAudioElement]
  private primary = 0
  private loaded: Loaded | null = null
  private preloaded: { key: string; plan: StreamPlan } | null = null
  /** Element we swapped away from (fallback if the new one may not autoplay). */
  private swappedFrom: HTMLAudioElement | null = null
  private readonly dual = !isIOS
  private currentKey: string | null = null
  private failures = 0
  /** Element-relative position to apply once metadata is available. */
  private pendingSeek: number | null = null
  private lastPositionSave = 0
  private saveQueued = false
  private disposed = false
  private readonly scrobbler = new Scrobbler()
  private readonly sync = new QueueSync()
  private readonly abort = new AbortController()
  private readonly cleanups: (() => void)[] = []

  private readonly handlers: MediaSessionHandlers = {
    play: () => usePlayer.getState().play(),
    pause: () => usePlayer.getState().pause(),
    stop: () => {
      usePlayer.getState().pause()
      usePlayback.getState().seek(0)
    },
    previous: () => usePlayer.getState().prev(),
    next: () => usePlayer.getState().next(),
    seekTo: (time) => usePlayback.getState().seek(time),
    seekBy: (delta) => {
      const playback = usePlayback.getState()
      playback.seek(playback.currentTime + delta)
    },
  }

  constructor() {
    this.els = [this.createElement(), this.createElement()]
  }

  private get el(): HTMLAudioElement {
    return this.els[this.primary]
  }

  private get spare(): HTMLAudioElement {
    return this.els[1 - this.primary]
  }

  // -------------------------------------------------------------------------------------------
  // Lifecycle
  // -------------------------------------------------------------------------------------------

  start(): void {
    activePicker = this.showPlaybackTargetPicker
    for (const el of this.els) document.body.appendChild(el)

    // Store truth at startup: nothing plays until the user asks.
    if (usePlayer.getState().isPlaying) usePlayer.setState({ isPlaying: false })

    this.cleanups.push(
      usePlayer.subscribe((state, prev) => this.onPlayerChange(state, prev)),
      usePlaybackPrefs.subscribe((prefs, prev) => this.onPrefsChange(prefs, prev)),
      usePlayback.getState().registerSeekHandler((time) => this.onSeek(time)),
      installKeyboardShortcuts(),
    )

    const onPageHide = () => {
      this.savePositionNow()
      void this.sync.flush(true)
    }
    const onVisibility = () => {
      if (document.visibilityState === 'hidden') onPageHide()
    }
    window.addEventListener('pagehide', onPageHide)
    document.addEventListener('visibilitychange', onVisibility)
    this.cleanups.push(() => {
      window.removeEventListener('pagehide', onPageHide)
      document.removeEventListener('visibilitychange', onVisibility)
    })

    setMediaSessionHandlers(this.handlers)
    useAirPlay.setState({ available: supportsAirPlay() })

    // Pick up whatever the store already holds (e.g. a remount), then restore.
    const state = usePlayer.getState()
    const current = currentOf(state)
    this.currentKey = current ? entryKey(current) : null
    if (current) this.announce(current)
    this.applyVolume()
    void this.restore()
  }

  dispose(): void {
    if (this.disposed) return
    this.disposed = true
    this.abort.abort()
    this.savePositionNow()
    for (const fn of this.cleanups.splice(0)) fn()
    this.sync.dispose()
    for (const el of this.els) {
      this.unload(el)
      el.remove()
    }
    this.loaded = null
    this.preloaded = null
    if (usePlayer.getState().isPlaying) usePlayer.setState({ isPlaying: false })
    usePlayback.getState().update({ buffering: false })
    setMediaSessionHandlers(null)
    setMediaSessionMetadata(undefined)
    setMediaSessionPlaybackState('none')
    if (activePicker === this.showPlaybackTargetPicker) activePicker = null
  }

  readonly showPlaybackTargetPicker = (): void => {
    ;(this.el as WebKitAudioElement).webkitShowPlaybackTargetPicker?.()
  }

  // -------------------------------------------------------------------------------------------
  // Restore
  // -------------------------------------------------------------------------------------------

  private async restore(): Promise<void> {
    try {
      if (usePlayer.getState().queue.length > 0) return
      const saved = loadQueue()
      if (saved.kind === 'full') {
        if (saved.userId && saved.userId !== currentUserId()) return this.restoreFromServer()
        this.applyRestore(saved.snapshot.queue, saved.snapshot.index, saved.position, saved.snapshot)
        return
      }
      if (saved.kind === 'ids') {
        if (saved.userId && saved.userId !== currentUserId()) return this.restoreFromServer()
        const tracks = await this.fetchTracks(saved.ids)
        if (this.disposed || usePlayer.getState().queue.length > 0) return
        const currentId = saved.ids[saved.index]
        const index = Math.max(0, tracks.findIndex((t) => t.id === currentId))
        this.applyRestore(tracks, index, tracks[index]?.id === currentId ? saved.position : 0)
        return
      }
      await this.restoreFromServer()
    } catch {
      // Nothing to restore (offline, endpoint unavailable, …): start empty.
    } finally {
      if (!this.disposed) this.sync.enable()
    }
  }

  private async restoreFromServer(): Promise<void> {
    const remote = await api.queue.get({ signal: this.abort.signal })
    if (this.disposed || usePlayer.getState().queue.length > 0) return
    const tracks = (remote.tracks ?? []).filter((t) => !t.missing)
    if (tracks.length === 0) return
    const found = tracks.findIndex((t) => t.id === remote.currentId)
    this.applyRestore(tracks, Math.max(0, found), found >= 0 ? remote.positionMs / 1000 : 0)
  }

  private async fetchTracks(ids: string[]): Promise<Track[]> {
    const byId = new Map<string, Track>()
    for (let i = 0; i < ids.length; i += ID_CHUNK) {
      const chunk = ids.slice(i, i + ID_CHUNK)
      const page = await api.tracks.list({ ids: chunk, limit: chunk.length }, { signal: this.abort.signal })
      for (const t of page.items) byId.set(t.id, t)
    }
    return ids.map((id) => byId.get(id)).filter((t): t is Track => t !== undefined)
  }

  private applyRestore(
    queue: PlayableTrack[],
    index: number,
    position: number,
    snapshot?: { shuffle: boolean; originalQueue: PlayableTrack[] | null },
  ): void {
    if (queue.length === 0) return
    usePlayer.getState().restoreQueue({
      queue,
      index,
      shuffle: snapshot?.shuffle ?? false,
      originalQueue: snapshot?.originalQueue ?? null,
    })
    const current = currentOf(usePlayer.getState())
    if (!current || current.isRadio) return
    const time = position > 0 && (current.duration <= 0 || position < current.duration - 1) ? position : 0
    usePlayback.getState().update({ currentTime: time })
    setMediaSessionPosition(time, current.duration)
  }

  // -------------------------------------------------------------------------------------------
  // Store → engine
  // -------------------------------------------------------------------------------------------

  private onPlayerChange(state: PlayerState, prev: PlayerState): void {
    if (
      state.queue !== prev.queue ||
      state.index !== prev.index ||
      state.shuffle !== prev.shuffle ||
      state.originalQueue !== prev.originalQueue
    ) {
      this.scheduleLocalSave()
      this.sync.schedule()
    }

    const current = currentOf(state)
    const key = current ? entryKey(current) : null
    if (key !== this.currentKey) {
      this.currentKey = key
      this.onEntryChange(current, state.isPlaying)
    } else {
      if (current && this.loaded && this.loaded.key === key && this.loaded.entry !== current) {
        // Same entry, updated fields (e.g. starred).
        this.loaded.entry = current
        this.announce(current)
      }
      if (state.isPlaying !== prev.isPlaying) {
        if (state.isPlaying) this.resume()
        else this.suspend()
      }
    }

    if (state.isPlaying !== prev.isPlaying) this.sync.schedule()
    if (state.volume !== prev.volume || state.muted !== prev.muted) this.applyVolume()
  }

  private onPrefsChange(prefs: PlaybackPrefsState, prev: PlaybackPrefsState): void {
    if (prefs.replayGain !== prev.replayGain) this.applyVolume()
    if (!prefs.preloadNext && prev.preloadNext) this.dropPreload()
    // Quality / format changes apply from the next load; a stale preload is re-planned then.
  }

  private onEntryChange(current: PlayableTrack | undefined, playing: boolean): void {
    this.pendingSeek = null
    this.scrobbler.reset(current)
    if (!current) {
      for (const el of this.els) this.unload(el)
      this.loaded = null
      this.preloaded = null
      usePlayback.getState().update({ currentTime: 0, duration: 0, buffered: 0, buffering: false })
      setMediaSessionMetadata(undefined)
      setMediaSessionPlaybackState('none')
      return
    }
    usePlayback
      .getState()
      .update({ currentTime: 0, duration: current.isRadio ? 0 : current.duration, buffered: 0, buffering: false })
    this.announce(current)
    if (playing) {
      this.load(current, 0, true)
    } else {
      if (this.loaded) {
        this.unload(this.el)
        this.loaded = null
      }
      setMediaSessionPlaybackState('paused')
      setMediaSessionPosition(0, current.isRadio ? 0 : current.duration)
    }
  }

  /** Media Session metadata + action set for an entry. */
  private announce(entry: PlayableTrack): void {
    setMediaSessionMetadata(entry)
    setMediaSessionHandlers(this.handlers, !!entry.isRadio)
  }

  private resume(): void {
    const state = usePlayer.getState()
    const current = currentOf(state)
    if (!current) return
    const loaded = this.loaded
    // Live streams restart at the live edge; everything else continues where it was.
    if (loaded && loaded.key === this.currentKey && !loaded.plan.live) {
      if (this.el.ended) usePlayback.getState().seek(0)
      this.playElement()
      return
    }
    this.load(current, current.isRadio ? 0 : usePlayback.getState().currentTime, true)
  }

  private suspend(): void {
    const loaded = this.loaded
    if (!loaded) return
    if (loaded.plan.live) {
      // Don't keep downloading a radio stream while paused.
      this.unload(this.el)
      this.loaded = null
    } else {
      this.el.pause()
    }
    usePlayback.getState().update({ buffering: false })
    this.savePositionNow()
    setMediaSessionPlaybackState('paused')
  }

  // -------------------------------------------------------------------------------------------
  // Loading & playing
  // -------------------------------------------------------------------------------------------

  private load(entry: PlayableTrack, startAt: number, autoplay: boolean): void {
    const plan = planStream(entry, usePlaybackPrefs.getState(), startAt)
    const key = entryKey(entry)
    if (!plan.url) {
      this.loaded = { entry, key, plan, retried: false }
      this.fail()
      return
    }

    const pre = this.preloaded
    this.preloaded = null
    this.swappedFrom = null
    this.pendingSeek = null
    if (this.dual && pre && pre.key === key && pre.plan.url === plan.url && startAt < 0.5) {
      // Near-gapless: the spare element already buffered this entry.
      const old = this.el
      this.primary = 1 - this.primary
      this.unload(old)
      this.swappedFrom = old
    } else {
      if (pre) this.unload(this.spare)
      const el = this.el
      el.preload = 'auto'
      el.src = plan.url
      if (!plan.transcoded && !plan.live && startAt > 0.5) this.pendingSeek = startAt
    }

    this.loaded = { entry, key, plan, retried: false }
    this.applyVolume()
    this.publishTimes()
    usePlayback.getState().update({ buffering: autoplay })
    if (autoplay) this.playElement()
  }

  private playElement(): void {
    const el = this.el
    const key = this.loaded?.key
    const promise = el.play()
    promise.catch((error: unknown) => {
      if (this.disposed || el !== this.el || this.loaded?.key !== key) return // superseded
      const name = error instanceof DOMException ? error.name : ''
      if (name === 'AbortError' || name === 'NotSupportedError') return // src changed / reported via `error`
      if (name === 'NotAllowedError') {
        const fallback = this.swappedFrom
        if (fallback && this.loaded) {
          // The swapped-in element may not start without a gesture: reuse the one that could.
          this.swappedFrom = null
          const { plan } = this.loaded
          this.unload(el)
          this.primary = this.els.indexOf(fallback)
          fallback.src = plan.url
          this.applyVolume()
          this.playElement()
          return
        }
        // Autoplay blocked (no user gesture yet): reflect the paused state.
        usePlayer.getState().pause()
        usePlayback.getState().update({ buffering: false })
        return
      }
      this.fail()
    })
  }

  private unload(el: HTMLAudioElement): void {
    el.pause()
    if (el.hasAttribute('src')) {
      el.removeAttribute('src')
      el.load()
    }
  }

  private dropPreload(): void {
    if (!this.preloaded) return
    this.preloaded = null
    this.unload(this.spare)
  }

  private nextEntry(): PlayableTrack | undefined {
    const { queue, index, repeat } = usePlayer.getState()
    if (index < 0 || repeat === 'one') return undefined
    if (index + 1 < queue.length) return queue[index + 1]
    if (repeat === 'all' && queue.length > 1) return queue[0]
    return undefined
  }

  private maybePreload(time: number): void {
    const prefs = usePlaybackPrefs.getState()
    const loaded = this.loaded
    if (!this.dual || !prefs.preloadNext || !loaded || loaded.plan.live) return
    const { duration } = usePlayback.getState()
    if (!(duration > 0) || duration - time > PRELOAD_BEFORE_END) return
    const next = this.nextEntry()
    if (!next || next.isRadio) return
    const key = entryKey(next)
    if (key === loaded.key) return
    const plan = planStream(next, prefs, 0)
    if (!plan.url || (this.preloaded?.key === key && this.preloaded.plan.url === plan.url)) return
    const spare = this.spare
    spare.preload = 'auto'
    spare.src = plan.url
    spare.load()
    this.preloaded = { key, plan }
  }

  private applyVolume(): void {
    const { volume, muted } = usePlayer.getState()
    const entry = this.loaded?.entry ?? currentOf(usePlayer.getState())
    for (const el of this.els) el.muted = muted
    if (!canControlVolume()) return
    const gain = sliderToGain(volume) * replayGainFactor(entry, usePlaybackPrefs.getState().replayGain)
    this.el.volume = Math.min(1, Math.max(0, gain))
  }

  // -------------------------------------------------------------------------------------------
  // Seeking
  // -------------------------------------------------------------------------------------------

  private onSeek(time: number): void {
    this.scrobbler.onSeek(time)
    const loaded = this.loaded
    const duration = usePlayback.getState().duration
    if (!loaded || loaded.key !== this.currentKey) {
      // Not loaded yet: playback will start at usePlayback.currentTime.
      setMediaSessionPosition(time, duration)
      return
    }
    if (loaded.plan.live) return
    const el = this.el
    if (!loaded.plan.transcoded) {
      if (el.readyState >= HTMLMediaElement.HAVE_METADATA) el.currentTime = time
      else this.pendingSeek = time
    } else {
      // Only what is already buffered: browsers report chunked streams as seekable to
      // Infinity and would re-request them with a Range header the transcoder ignores.
      const rel = time - loaded.plan.offset
      if (rel >= 0 && el.readyState >= HTMLMediaElement.HAVE_METADATA && inRanges(el.buffered, rel)) {
        el.currentTime = rel
      } else if (usePlayer.getState().isPlaying) {
        this.load(loaded.entry, time, true)
      } else {
        // Reload lazily at the new position when playback resumes.
        this.unload(el)
        this.loaded = null
      }
    }
    setMediaSessionPosition(time, duration)
  }

  // -------------------------------------------------------------------------------------------
  // Element events
  // -------------------------------------------------------------------------------------------

  private createElement(): HTMLAudioElement {
    const el = document.createElement('audio')
    el.preload = 'auto'
    el.hidden = true
    el.setAttribute('playsinline', '')
    el.setAttribute('webkit-playsinline', '')
    el.setAttribute('x-webkit-airplay', 'allow')
    el.crossOrigin = null

    const on = <K extends keyof HTMLMediaElementEventMap>(type: K, handler: (event: HTMLMediaElementEventMap[K]) => void) => {
      el.addEventListener(type, handler as EventListener)
    }
    const primary = () => !this.disposed && el === this.el && this.loaded !== null

    on('timeupdate', () => primary() && this.onTimeUpdate())
    on('durationchange', () => primary() && this.publishTimes())
    on('loadedmetadata', () => {
      if (!primary()) return
      if (this.pendingSeek !== null) {
        el.currentTime = this.pendingSeek
        this.pendingSeek = null
      }
      this.publishTimes()
    })
    on('progress', () => {
      if (primary()) usePlayback.getState().update({ buffered: (this.loaded?.plan.offset ?? 0) + bufferedEnd(el) })
    })
    on('waiting', () => {
      if (primary() && usePlayer.getState().isPlaying) usePlayback.getState().update({ buffering: true })
    })
    on('canplay', () => {
      if (primary()) usePlayback.getState().update({ buffering: false })
    })
    on('playing', () => primary() && this.onPlaying())
    on('pause', () => primary() && this.onPause())
    on('seeked', () => {
      if (!primary()) return
      const time = this.position()
      usePlayback.getState().update({ currentTime: time })
      setMediaSessionPosition(time, usePlayback.getState().duration)
    })
    on('ended', () => primary() && this.onEnded())
    on('error', () => {
      if (this.disposed) return
      if (el !== this.el) {
        // Preloading failed: load normally when the time comes.
        if (el === this.spare) this.preloaded = null
        return
      }
      this.onError(el)
    })
    el.addEventListener('webkitplaybacktargetavailabilitychanged', (event) => {
      const availability = (event as Event & { availability?: string }).availability
      useAirPlay.setState({ available: availability === 'available' })
    })
    return el
  }

  /** Position in the track (seconds), accounting for transcode offsets. */
  private position(): number {
    return (this.loaded?.plan.offset ?? 0) + this.el.currentTime
  }

  private publishTimes(): void {
    const loaded = this.loaded
    if (!loaded) return
    const el = this.el
    let duration: number
    if (loaded.plan.live) duration = 0
    else if (loaded.plan.transcoded) {
      duration = loaded.entry.duration > 0 ? loaded.entry.duration : loaded.plan.offset + (Number.isFinite(el.duration) ? el.duration : 0)
    } else {
      duration = Number.isFinite(el.duration) && el.duration > 0 ? el.duration : loaded.entry.duration
    }
    usePlayback.getState().update({ duration, buffered: loaded.plan.offset + bufferedEnd(el) })
    if (!el.paused) setMediaSessionPosition(this.position(), duration)
  }

  private onTimeUpdate(): void {
    const time = this.position()
    const playing = !this.el.paused
    usePlayback.getState().update({ currentTime: time })
    this.scrobbler.onTime(time, playing)
    if (!playing) return
    this.maybePreload(time)
    this.sync.tick()
    const now = Date.now()
    if (now - this.lastPositionSave >= POSITION_SAVE_INTERVAL) {
      this.lastPositionSave = now
      this.savePositionNow()
    }
  }

  private onPlaying(): void {
    if (!usePlayer.getState().isPlaying) {
      // The store is the source of truth (e.g. paused while play() was pending).
      this.el.pause()
      return
    }
    this.failures = 0
    this.swappedFrom = null
    const time = this.position()
    usePlayback.getState().update({ buffering: false })
    this.scrobbler.onPlaying(time)
    setMediaSessionPlaybackState('playing')
    setMediaSessionPosition(time, usePlayback.getState().duration)
  }

  private onPause(): void {
    const el = this.el
    // Ignore the pause that precedes `ended`, stale events after play() and src changes.
    if (!el.paused || el.ended || el.readyState === HTMLMediaElement.HAVE_NOTHING) return
    setMediaSessionPlaybackState('paused')
    this.savePositionNow()
    // Paused from outside (headphones unplugged, OS interruption, AirPlay…): follow it.
    if (usePlayer.getState().isPlaying) usePlayer.getState().pause()
  }

  private onEnded(): void {
    const before = this.currentKey
    usePlayer.getState().onTrackEnded()
    if (this.disposed || this.currentKey !== before) return
    // Same entry again (repeat one, or repeat all with a single entry): replay it.
    const loaded = this.loaded
    if (!loaded || !usePlayer.getState().isPlaying) return
    this.scrobbler.reset(loaded.entry)
    if (this.el.ended) usePlayback.getState().seek(0)
    if (this.el.paused && this.loaded) this.playElement()
  }

  private onError(el: HTMLAudioElement): void {
    const loaded = this.loaded
    if (!loaded || !el.hasAttribute('src')) return
    const code = el.error?.code
    if (code === MediaError.MEDIA_ERR_ABORTED) return
    // A network hiccup mid-track: retry once from where we were.
    if (code === MediaError.MEDIA_ERR_NETWORK && !loaded.retried && !loaded.plan.live) {
      const time = usePlayback.getState().currentTime
      if (time > 0) {
        const playing = usePlayer.getState().isPlaying
        this.load(loaded.entry, time, playing)
        if (this.loaded) this.loaded.retried = true
        return
      }
    }
    this.fail()
  }

  /** The current entry can't be played: toast, then skip (or stop after repeated failures). */
  private fail(): void {
    const entry = this.loaded?.entry ?? currentOf(usePlayer.getState())
    this.unload(this.el)
    this.loaded = null
    usePlayback.getState().update({ buffering: false })
    this.failures += 1
    const player = usePlayer.getState()
    if (this.failures >= MAX_FAILURES) {
      this.failures = 0
      player.pause()
      toast.error(i18n.t('player:errors.stopped'), { id: 'player-error', description: undefined })
      return
    }
    const hasNext = player.index < player.queue.length - 1 || (player.repeat === 'all' && player.queue.length > 1)
    const skip = player.isPlaying && hasNext
    toast.error(i18n.t('player:errors.playback', { title: entry?.title ?? '' }), {
      id: 'player-error',
      description: skip ? i18n.t('player:errors.skipping') : undefined,
    })
    if (skip) player.next()
    else player.pause()
  }

  // -------------------------------------------------------------------------------------------
  // Persistence
  // -------------------------------------------------------------------------------------------

  private scheduleLocalSave(): void {
    if (this.saveQueued) return
    this.saveQueued = true
    queueMicrotask(() => {
      this.saveQueued = false
      if (this.disposed) return
      const { queue, index, shuffle, originalQueue } = usePlayer.getState()
      saveQueue({ queue, index, shuffle, originalQueue }, currentUserId())
    })
  }

  private savePositionNow(): void {
    const { queue, index } = usePlayer.getState()
    savePosition(index >= 0 ? queue[index] : undefined, index, usePlayback.getState().currentTime)
  }
}
