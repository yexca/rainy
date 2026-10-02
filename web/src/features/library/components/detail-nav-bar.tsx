import { ChevronLeft } from 'lucide-react'
import { useEffect, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'

import { Button } from '@/components/ui/button'
import { useIsMobile } from '@/hooks/use-media-query'
import { cn } from '@/lib/utils'

export interface DetailNavBarProps {
  /** Shown centred (phones) / left (desktop) once `watch` scrolls under the bar. */
  title: string
  /** The hero title element; the bar turns into glass with a small title once it is hidden. */
  watch: HTMLElement | null
  /** Compact controls on the right. */
  actions?: ReactNode
  /** Back target; default: history back (or Home). */
  backTo?: string
  /** Transparent until collapsed (for pages with an artwork backdrop). */
  className?: string
}

/**
 * Sticky top bar for detail pages (album, artist, playlist, genre): back button, a title that
 * fades in when the hero title scrolls away, and actions. Mirrors `PageHeader`'s bar, for pages
 * whose title lives in a custom hero instead of a large title. Must be a direct child of `<Page>`
 * (or of a wrapper spanning the whole page) so it stays sticky.
 */
export function DetailNavBar({ title, watch, actions, backTo, className }: DetailNavBarProps) {
  const { t } = useTranslation()
  const isMobile = useIsMobile()
  const navigate = useNavigate()
  const barRef = useRef<HTMLDivElement>(null)
  const [collapsed, setCollapsed] = useState(false)

  useEffect(() => {
    const bar = barRef.current
    if (!bar || !watch || typeof IntersectionObserver === 'undefined') return
    // The bar's bottom edge: it sticks below the app header (`--app-header-h`, 0 on phones).
    const edge = (Number.parseFloat(getComputedStyle(bar).top) || 0) + bar.offsetHeight
    const observer = new IntersectionObserver(
      ([entry]) => setCollapsed(!entry.isIntersecting && entry.boundingClientRect.top < edge),
      { rootMargin: `-${edge}px 0px 0px 0px`, threshold: 0 },
    )
    observer.observe(watch)
    return () => observer.disconnect()
  }, [watch, isMobile])

  useEffect(() => {
    if (!title) return
    const previous = document.title
    document.title = `${title} · Rainy`
    return () => {
      document.title = previous
    }
  }, [title])

  const goBack = () => {
    if (backTo) {
      navigate(backTo)
      return
    }
    const idx = (window.history.state as { idx?: number } | null)?.idx ?? 0
    if (idx > 0) navigate(-1)
    else navigate('/')
  }

  return (
    <div
      ref={barRef}
      className={cn(
        'ui-chrome bleed-x page-x hairline-b sticky top-(--app-header-h) z-30 pt-safe md:pt-0 transition-[background-color,border-color] duration-200',
        collapsed ? 'glass' : '[--hairline-color:transparent]',
        className,
      )}
    >
      <div className="relative flex h-11 items-center gap-1 md:h-12">
        <div className="flex min-w-0 flex-1 items-center gap-1">
          <Button
            variant="ghost"
            size="icon"
            onClick={goBack}
            aria-label={t('common:actions.back')}
            className="-ml-2 size-11 text-primary hover:bg-transparent hover:text-primary/80 md:-ml-1 md:size-8 md:text-foreground md:hover:bg-accent"
          >
            <ChevronLeft className="size-7 md:size-5" strokeWidth={isMobile ? 2 : 1.75} />
          </Button>
          {!isMobile ? (
            <span
              aria-hidden={!collapsed}
              className={cn(
                'truncate text-sm font-semibold transition-opacity duration-200',
                collapsed ? 'opacity-100' : 'opacity-0',
              )}
            >
              {title}
            </span>
          ) : null}
        </div>
        {isMobile ? (
          <div className="pointer-events-none absolute inset-x-24 flex justify-center">
            <span
              aria-hidden={!collapsed}
              className={cn(
                'truncate text-[17px] font-semibold transition-opacity duration-200',
                collapsed ? 'opacity-100' : 'opacity-0',
              )}
            >
              {title}
            </span>
          </div>
        ) : null}
        {actions ? <div className="flex shrink-0 items-center gap-0.5">{actions}</div> : null}
      </div>
    </div>
  )
}
