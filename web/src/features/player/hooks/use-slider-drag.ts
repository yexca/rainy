import { useCallback, useRef, type PointerEvent } from 'react'

export interface SliderDragOptions {
  /** Current value (0..1), used as the starting point of relative (touch) drags. */
  getValue: () => number
  onStart?: (ratio: number) => void
  onMove: (ratio: number) => void
  /** `committed` is false when the drag was cancelled (or a touch tap without movement). */
  onEnd: (ratio: number, committed: boolean) => void
  disabled?: boolean
  /**
   * Touch drags move relative to where the finger went down (the iOS scrubber behaviour:
   * grabbing the bar doesn't jump). Mouse / pen jump to the pointer.
   */
  relativeTouch?: boolean
}

function clamp01(n: number): number {
  return Math.min(1, Math.max(0, n))
}

/** Pointer handlers for a horizontal slider track (pointer capture, relative touch drags). */
export function useSliderDrag({ getValue, onStart, onMove, onEnd, disabled, relativeTouch = true }: SliderDragOptions) {
  const state = useRef<{
    id: number
    left: number
    width: number
    startX: number
    startValue: number
    relative: boolean
    moved: boolean
    last: number
  } | null>(null)

  const ratioFor = useCallback((clientX: number): number => {
    const s = state.current
    if (!s || s.width <= 0) return 0
    if (s.relative) return clamp01(s.startValue + (clientX - s.startX) / s.width)
    return clamp01((clientX - s.left) / s.width)
  }, [])

  const onPointerDown = useCallback(
    (event: PointerEvent<HTMLElement>) => {
      if (disabled || (event.pointerType === 'mouse' && event.button !== 0)) return
      const rect = event.currentTarget.getBoundingClientRect()
      const relative = relativeTouch && event.pointerType === 'touch'
      event.currentTarget.setPointerCapture(event.pointerId)
      state.current = {
        id: event.pointerId,
        left: rect.left,
        width: rect.width,
        startX: event.clientX,
        startValue: getValue(),
        relative,
        moved: false,
        last: 0,
      }
      const ratio = ratioFor(event.clientX)
      state.current.last = ratio
      onStart?.(ratio)
    },
    [disabled, getValue, onStart, ratioFor, relativeTouch],
  )

  const onPointerMove = useCallback(
    (event: PointerEvent<HTMLElement>) => {
      const s = state.current
      if (!s || s.id !== event.pointerId) return
      if (Math.abs(event.clientX - s.startX) > 2) s.moved = true
      s.last = ratioFor(event.clientX)
      onMove(s.last)
    },
    [onMove, ratioFor],
  )

  const finish = useCallback(
    (event: PointerEvent<HTMLElement>, cancelled: boolean) => {
      const s = state.current
      if (!s || s.id !== event.pointerId) return
      state.current = null
      if (event.currentTarget.hasPointerCapture(event.pointerId)) {
        event.currentTarget.releasePointerCapture(event.pointerId)
      }
      // A touch tap on a relative slider does nothing (no accidental seeks).
      const committed = !cancelled && (!s.relative || s.moved)
      onEnd(s.last, committed)
    },
    [onEnd],
  )

  return {
    onPointerDown,
    onPointerMove,
    onPointerUp: (event: PointerEvent<HTMLElement>) => finish(event, false),
    onPointerCancel: (event: PointerEvent<HTMLElement>) => finish(event, true),
    onLostPointerCapture: (event: PointerEvent<HTMLElement>) => finish(event, false),
  }
}
