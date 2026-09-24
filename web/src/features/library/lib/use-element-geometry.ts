import { useCallback, useEffect, useState } from 'react'

export interface ElementGeometry {
  /** Distance from the top of the document (for `useWindowVirtualizer`'s `scrollMargin`). */
  top: number
  /** Content width in CSS px (0 until measured). */
  width: number
}

const INITIAL: ElementGeometry = { top: 0, width: 0 }

function measure(node: HTMLElement): ElementGeometry {
  const rect = node.getBoundingClientRect()
  return { top: Math.round(rect.top + window.scrollY), width: Math.round(node.clientWidth) }
}

/**
 * Track an element's document offset and width. Returns a callback ref and the geometry.
 * Re-measures when the element or the page resizes (content above it growing moves it down).
 */
export function useElementGeometry<T extends HTMLElement>(): [(node: T | null) => void, ElementGeometry] {
  const [node, setNode] = useState<T | null>(null)
  const [geometry, setGeometry] = useState<ElementGeometry>(INITIAL)

  useEffect(() => {
    if (!node) return
    let frame = 0
    const schedule = () => {
      cancelAnimationFrame(frame)
      frame = requestAnimationFrame(() => {
        const next = measure(node)
        setGeometry((prev) => (prev.top === next.top && prev.width === next.width ? prev : next))
      })
    }
    // ResizeObserver reports once right after observing, which does the initial measurement.
    const observer = new ResizeObserver(schedule)
    observer.observe(node)
    observer.observe(document.body)
    window.addEventListener('resize', schedule)
    return () => {
      cancelAnimationFrame(frame)
      observer.disconnect()
      window.removeEventListener('resize', schedule)
    }
  }, [node])

  // Measure synchronously on attach (commit phase) so the first paint already has the real
  // width — no flash of a wrongly sized grid.
  const ref = useCallback((el: T | null) => {
    setNode(el)
    if (el) setGeometry(measure(el))
  }, [])

  return [ref, geometry]
}
