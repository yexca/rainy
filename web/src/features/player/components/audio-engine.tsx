import { useEffect } from 'react'

import { PlayerEngine } from '../engine/engine'
import { PwaPrompt } from './pwa-prompt'

/**
 * Headless audio engine host (mounted once by the app shell). The engine itself lives outside
 * React (`engine/engine.ts`) and reacts to the player store directly, so playback never waits
 * for a render. Also hosts the PWA update / offline-ready toasts.
 */
export function AudioEngine() {
  useEffect(() => {
    const engine = new PlayerEngine()
    engine.start()
    return () => engine.dispose()
  }, [])

  return <PwaPrompt />
}
