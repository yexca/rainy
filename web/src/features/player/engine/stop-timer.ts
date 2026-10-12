/**
 * Auto-stop timer (docs/architecture/contract.md §9.4): pauses playback when the timer in
 * `useStopTimer` runs out. In `idle` mode it also reports activity, which restarts the count:
 * pointer, keyboard, wheel and touch input in the app and in the picture-in-picture lyrics
 * window, and playback starting. (Media Session actions report theirs from the audio engine.)
 */
import { toast } from 'sonner'

import i18n from '@/lib/i18n'

import { STOP_TOAST_ID } from '../lib/stop-timer'
import { usePipLyrics } from '../pip/pip-lyrics'
import { usePlayback, usePlayer } from '../store'
import { useStopTimer } from '../stop-timer'

const ACTIVITY_EVENTS = ['pointerdown', 'pointermove', 'keydown', 'wheel', 'touchstart'] as const

function touch(): void {
  useStopTimer.getState().touch()
}

export class StopTimerEngine {
  private timeout: number | undefined
  private unwatch: (() => void) | null = null
  private readonly cleanups: (() => void)[] = []

  start(): void {
    this.cleanups.push(
      useStopTimer.subscribe((state, prev) => {
        if (state.endsAt !== prev.endsAt) this.arm()
        if (state.mode !== prev.mode) this.watch()
      }),
      usePipLyrics.subscribe((state, prev) => {
        if (state.window !== prev.window) this.watch()
      }),
      usePlayer.subscribe((state, prev) => {
        if (!state.isPlaying || prev.isPlaying) return
        toast.dismiss(STOP_TOAST_ID)
        touch()
      }),
      // Browsers hold timers back in background tabs; playback progress keeps arriving.
      usePlayback.subscribe(() => this.check()),
    )
    this.arm()
    this.watch()
  }

  dispose(): void {
    for (const fn of this.cleanups.splice(0)) fn()
    window.clearTimeout(this.timeout)
    this.unwatch?.()
    this.unwatch = null
  }

  private arm(): void {
    window.clearTimeout(this.timeout)
    const { endsAt } = useStopTimer.getState()
    if (endsAt === null) return
    this.timeout = window.setTimeout(() => {
      // Not due after all (the clock changed): wait for the rest.
      if (!this.check()) this.arm()
    }, Math.max(0, endsAt - Date.now()))
  }

  /** Stops playback when the deadline has passed; says whether it had. */
  private check(): boolean {
    const { mode, minutes, endsAt, expire } = useStopTimer.getState()
    if (endsAt === null || Date.now() < endsAt) return false
    expire()
    const player = usePlayer.getState()
    if (!player.isPlaying) return true
    player.pause()
    const time = i18n.t('common:duration.minutes', { minutes })
    // Stays up until closed or playback resumes: whoever comes back sees why the music stopped.
    toast(mode === 'idle' ? i18n.t('player:timer.stoppedIdle', { time }) : i18n.t('player:timer.stoppedSleep'), {
      id: STOP_TOAST_ID,
      duration: Infinity,
      closeButton: true,
    })
    return true
  }

  /** Listens for activity while `idle` mode is on. */
  private watch(): void {
    this.unwatch?.()
    this.unwatch = null
    if (useStopTimer.getState().mode !== 'idle') return
    const abort = new AbortController()
    for (const target of [window, usePipLyrics.getState().window]) {
      for (const type of ACTIVITY_EVENTS) {
        target?.addEventListener(type, touch, { capture: true, passive: true, signal: abort.signal })
      }
    }
    this.unwatch = () => abort.abort()
  }
}
