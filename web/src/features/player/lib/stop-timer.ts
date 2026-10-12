/**
 * Auto-stop timer (docs/architecture/contract.md §9.4). Pure helpers so they can be unit tested
 * without a DOM; the store lives in `../stop-timer.ts` and `engine/stop-timer.ts` runs it.
 *
 * - `sleep`: playback stops once, the picked time after the timer was set.
 * - `idle`: playback stops when the app saw no activity for the picked time. Every activity
 *   restarts the count, and the mode stays on (and is remembered) until it is turned off.
 */

export const TIMER_MODES = ['sleep', 'idle'] as const
export type TimerMode = (typeof TIMER_MODES)[number]

/** Times offered in the picker (minutes). */
export const TIMER_MINUTES = [15, 30, 45, 60, 90, 120] as const

/** Longest custom time (minutes): a day. */
export const TIMER_MAX_MINUTES = 1440

/** Minutes typed into the custom field: whole, 1 to {@link TIMER_MAX_MINUTES}; `null` otherwise. */
export function parseMinutes(text: string): number | null {
  const trimmed = text.trim()
  if (!/^\d+$/.test(trimmed)) return null
  const minutes = Number(trimmed)
  return minutes >= 1 && minutes <= TIMER_MAX_MINUTES ? minutes : null
}

/** Toast shown when the timer stopped playback (sonner id). */
export const STOP_TOAST_ID = 'stop-timer'

/** Activity closer together than this keeps the deadline (ms): pointer moves arrive in bursts. */
const TOUCH_INTERVAL = 5000

export interface StopTimer {
  mode: TimerMode | null
  minutes: number
  /** When playback stops (epoch ms). `null`: off, or `idle` waiting for the next activity. */
  endsAt: number | null
}

export const TIMER_OFF: StopTimer = { mode: null, minutes: 30, endsAt: null }

export function startTimer(mode: TimerMode, minutes: number, now: number): StopTimer {
  return { mode, minutes, endsAt: now + minutes * 60_000 }
}

/** Activity at `now`: `idle` counts again from here. Returns `timer` itself when nothing changes. */
export function touchTimer<T extends StopTimer>(timer: T, now: number): T {
  if (timer.mode !== 'idle') return timer
  const endsAt = now + timer.minutes * 60_000
  if (timer.endsAt !== null && Math.abs(endsAt - timer.endsAt) < TOUCH_INTERVAL) return timer
  return { ...timer, endsAt }
}

/** The deadline passed: `sleep` is done, `idle` waits for the next activity. */
export function expireTimer(timer: StopTimer): StopTimer {
  return timer.mode === 'idle' ? { ...TIMER_OFF, mode: 'idle', minutes: timer.minutes } : TIMER_OFF
}

/** Only `idle` survives a reload: playback restores paused, so a sleep countdown is over by then. */
export function sanitizeTimer(input: Partial<StopTimer> | null | undefined): StopTimer {
  const p = input ?? {}
  const minutes = typeof p.minutes === 'number' ? parseMinutes(String(p.minutes)) : null
  if (p.mode !== 'idle' || minutes === null) return TIMER_OFF
  return { mode: 'idle', minutes, endsAt: null }
}

/** Whole seconds left until `endsAt`, rounded up. */
export function remainingSeconds(endsAt: number, now: number): number {
  return Math.max(0, Math.ceil((endsAt - now) / 1000))
}
