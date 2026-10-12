/**
 * Auto-stop timer state (docs/architecture/contract.md §9.4): set from `<StopTimerButton/>`, run
 * by `engine/stop-timer.ts`. Only the `idle` mode is persisted (per browser).
 *
 *   const mode = useStopTimer((s) => s.mode)   // 'sleep' | 'idle' | null
 */
import { create } from 'zustand'
import { createJSONStorage, persist } from 'zustand/middleware'

import { expireTimer, sanitizeTimer, startTimer, TIMER_OFF, touchTimer, type StopTimer, type TimerMode } from './lib/stop-timer'

export interface StopTimerState extends StopTimer {
  start(mode: TimerMode, minutes: number): void
  cancel(): void
  /** The user did something: `idle` counts again from now. */
  touch(): void
  /** The deadline passed (called by the engine). */
  expire(): void
}

export const useStopTimer = create<StopTimerState>()(
  persist(
    (set) => ({
      ...TIMER_OFF,
      start: (mode, minutes) => set(startTimer(mode, minutes, Date.now())),
      cancel: () => set(TIMER_OFF),
      touch: () => set((s) => touchTimer(s, Date.now())),
      expire: () => set((s) => expireTimer(s)),
    }),
    {
      name: 'rainy.stop-timer',
      version: 1,
      storage: createJSONStorage(() => localStorage),
      partialize: (s): Pick<StopTimer, 'mode' | 'minutes'> => ({ mode: s.mode, minutes: s.minutes }),
      merge: (persisted, current) => ({ ...current, ...sanitizeTimer(persisted as Partial<StopTimer>) }),
    },
  ),
)
