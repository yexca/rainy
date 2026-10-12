import { useEffect } from 'react'

import { PlayerEngine } from '../engine/engine'
import { InfiniteRefill } from '../engine/infinite'
import { StopTimerEngine } from '../engine/stop-timer'
import { PipLyricsHost } from '../pip/pip-lyrics-window'
import { PwaPrompt } from './pwa-prompt'

/**
 * Headless audio engine host (mounted once by the app shell). The engine itself lives outside
 * React (`engine/engine.ts`) and reacts to the player store directly, so playback never waits
 * for a render. Infinite mode's refill (`engine/infinite.ts`) and the auto-stop timer
 * (`engine/stop-timer.ts`) run beside it. Also hosts the PWA update / offline-ready toasts and the
 * picture-in-picture lyrics window (`pip/`).
 */
export function AudioEngine() {
  useEffect(() => {
    const engine = new PlayerEngine()
    const infinite = new InfiniteRefill()
    const stopTimer = new StopTimerEngine()
    engine.start()
    infinite.start()
    stopTimer.start()
    return () => {
      stopTimer.dispose()
      infinite.dispose()
      engine.dispose()
    }
  }, [])

  return (
    <>
      <PwaPrompt />
      <PipLyricsHost />
    </>
  )
}
