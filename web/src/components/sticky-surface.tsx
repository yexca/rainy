import { Slot } from 'radix-ui'
import { useEffect, useState, type ComponentProps } from 'react'

import { useIsMobile } from '@/hooks/use-media-query'
import { useTheme } from '@/hooks/use-theme'
import { cn } from '@/lib/utils'

/**
 * A sticky bar inside a page (selection bars, section headings): `sticky-surface` with
 * `data-stuck` while it is pinned at its `top`. Resting in the flow it is see-through, like the
 * page header above it; once pinned it turns frosted (glass in immersive mode) with its hairline.
 * The caller sets `sticky`, `top-*` and usually `hairline-b`.
 * `asChild` renders the child element (a heading, for example) instead of a `div`.
 */
export function StickySurface({ asChild = false, className, ...props }: ComponentProps<'div'> & { asChild?: boolean }) {
  const [element, setElement] = useState<HTMLElement | null>(null)
  const stuck = useStuck(element)
  const Comp = asChild ? Slot.Root : 'div'
  return <Comp ref={setElement} data-stuck={stuck || undefined} className={cn('sticky-surface', className)} {...props} />
}

/** Whether a `position: sticky` element is pinned at its `top` (an IntersectionObserver, no scroll listeners). */
function useStuck(element: HTMLElement | null): boolean {
  const [stuck, setStuck] = useState(false)
  // Both move the `top` the element sticks at (`--app-header-h`, the phone page bar).
  const isMobile = useIsMobile()
  const { style } = useTheme()

  useEffect(() => {
    if (!element || typeof IntersectionObserver === 'undefined') return
    const edge = Number.parseFloat(getComputedStyle(element).top) || 0
    // The root's top sits 1px below the sticking line: a pinned element pokes above it.
    const observer = new IntersectionObserver(
      ([entry]) => setStuck(entry.intersectionRatio < 1 && entry.boundingClientRect.top <= edge + 0.5),
      { rootMargin: `-${edge + 1}px 0px 0px 0px`, threshold: 1 },
    )
    observer.observe(element)
    return () => observer.disconnect()
  }, [element, isMobile, style])

  return stuck
}
