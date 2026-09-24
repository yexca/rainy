import { useCallback, useState } from 'react'

/**
 * Distance from the top of the document to an element — the `scrollMargin` a window
 * virtualizer needs for a list that starts below other content. Re-measured whenever the page
 * layout changes size.
 */
export function useScrollMargin<T extends HTMLElement>(): [(el: T | null) => (() => void) | undefined, number] {
  const [margin, setMargin] = useState(0)
  const ref = useCallback((el: T | null) => {
    if (!el) return undefined
    const update = () => setMargin(Math.round(el.getBoundingClientRect().top + window.scrollY))
    update()
    const observer = new ResizeObserver(update)
    observer.observe(document.body)
    if (el.parentElement) observer.observe(el.parentElement)
    return () => observer.disconnect()
  }, [])
  return [ref, margin]
}
