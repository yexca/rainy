import type { KeyboardEvent } from 'react'

import { usePlayback } from '../store'

const KEY_STEP = 5
const PAGE_STEP = 30

/**
 * Slider keys for a playback position control: arrows ±5 s, Page Up / Down ±30 s, Home / End.
 * Seeks and swallows the key when it is one of them (the global ←/→ shortcuts stay out).
 */
export function seekByKey(event: KeyboardEvent<HTMLElement>) {
  const s = usePlayback.getState()
  let target: number | null = null
  if (event.key === 'ArrowRight' || event.key === 'ArrowUp') target = s.currentTime + KEY_STEP
  else if (event.key === 'ArrowLeft' || event.key === 'ArrowDown') target = s.currentTime - KEY_STEP
  else if (event.key === 'PageUp') target = s.currentTime + PAGE_STEP
  else if (event.key === 'PageDown') target = s.currentTime - PAGE_STEP
  else if (event.key === 'Home') target = 0
  else if (event.key === 'End') target = Math.max(0, s.duration - 1)
  if (target === null) return
  event.preventDefault()
  event.stopPropagation()
  s.seek(target)
}
