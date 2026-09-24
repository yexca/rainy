import { ChevronLeft, ChevronRight } from 'lucide-react'
import { Children, useEffect, useId, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import { SectionHeader } from './section-header'

const ITEM_WIDTH = {
  /** Album / playlist cards. */
  md: 'w-[calc((100%-1rem)/2.3)] sm:w-[calc((100%-2rem)/3.3)] md:w-44 lg:w-48',
  /** Artist circles. */
  sm: 'w-[calc((100%-2rem)/3.2)] sm:w-32 md:w-36 lg:w-40',
  /** Wide tiles (genres). */
  lg: 'w-[calc((100%-1rem)/1.6)] sm:w-64 md:w-72',
} as const

export interface HorizontalShelfProps {
  title: ReactNode
  /** "See all" target. */
  to?: string
  /** Extra header controls. */
  actions?: ReactNode
  itemSize?: keyof typeof ITEM_WIDTH
  children: ReactNode
  className?: string
}

/**
 * A titled, horizontally scrolling row with snap points (Apple Music shelves). It bleeds to the
 * screen edges on phones; on desktop, hover reveals previous / next page buttons.
 */
export function HorizontalShelf({ title, to, actions, itemSize = 'md', children, className }: HorizontalShelfProps) {
  const { t } = useTranslation()
  const headingId = useId()
  const [scroller, setScroller] = useState<HTMLDivElement | null>(null)
  const [edges, setEdges] = useState({ start: true, end: true })

  useEffect(() => {
    if (!scroller) return
    const update = () => {
      const start = scroller.scrollLeft <= 2
      const end = scroller.scrollLeft + scroller.clientWidth >= scroller.scrollWidth - 2
      setEdges((prev) => (prev.start === start && prev.end === end ? prev : { start, end }))
    }
    const observer = new ResizeObserver(update)
    observer.observe(scroller)
    scroller.addEventListener('scroll', update, { passive: true })
    return () => {
      observer.disconnect()
      scroller.removeEventListener('scroll', update)
    }
  }, [scroller])

  const page = (direction: 1 | -1) => {
    if (!scroller) return
    scroller.scrollBy({ left: direction * scroller.clientWidth * 0.85, behavior: 'smooth' })
  }

  const arrow =
    'absolute top-[calc(50%-2.25rem)] z-10 hidden size-9 -translate-y-1/2 place-items-center rounded-full border border-border/60 bg-background/90 text-foreground shadow-md backdrop-blur transition-opacity hover:bg-background md:grid'

  return (
    <section aria-labelledby={headingId} className={cn('mt-8 first:mt-2 md:mt-10', className)}>
      <SectionHeader id={headingId} title={title} to={to} actions={actions} />
      <div className="group/shelf relative">
        <div
          ref={setScroller}
          className="bleed-x page-x scrollbar-none flex snap-x snap-mandatory gap-4 overflow-x-auto overscroll-x-contain scroll-px-[calc(var(--page-px)+var(--safe-left))] pb-2 md:gap-5"
        >
          {Children.map(children, (child) =>
            child ? <div className={cn('shrink-0 snap-start', ITEM_WIDTH[itemSize])}>{child}</div> : null,
          )}
        </div>
        <button
          type="button"
          aria-label={t('library:actions.scrollBack')}
          onClick={() => page(-1)}
          className={cn(arrow, '-left-3 opacity-0 group-hover/shelf:opacity-100', edges.start && 'pointer-events-none opacity-0!')}
        >
          <ChevronLeft className="size-5" strokeWidth={2} />
        </button>
        <button
          type="button"
          aria-label={t('library:actions.scrollForward')}
          onClick={() => page(1)}
          className={cn(arrow, '-right-3 opacity-0 group-hover/shelf:opacity-100', edges.end && 'pointer-events-none opacity-0!')}
        >
          <ChevronRight className="size-5" strokeWidth={2} />
        </button>
      </div>
    </section>
  )
}
