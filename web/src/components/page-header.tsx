import { ChevronLeft } from 'lucide-react'
import { useEffect, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'

import { Button } from '@/components/ui/button'
import { useIsMobile } from '@/hooks/use-media-query'
import { cn } from '@/lib/utils'

export interface PageHeaderProps {
  /** Page title (also used for the collapsed bar and `document.title`). */
  title: string
  subtitle?: ReactNode
  /** Primary actions: right of the title on desktop, below it on mobile. */
  actions?: ReactNode
  /** Compact icon buttons in the top bar (iOS nav bar on mobile, sticky bar on desktop). */
  navActions?: ReactNode
  /** Back button: `true` = history back (falls back to Home), a string = link target. */
  back?: boolean | string
  /** Extra content below the title block (tabs, filters, search field…). */
  children?: ReactNode
  /** Set `document.title` to "<title> · Rainy" (default true). */
  documentTitle?: boolean
  className?: string
}

/**
 * Page title block.
 *
 * - Mobile: iOS large title (34/41 bold). Once it scrolls under the top bar, the bar turns into
 *   a glass nav bar with a small centred title (17px semibold).
 * - Desktop: `text-3xl` title with subtitle + actions; a slim glass bar with the title appears
 *   on scroll.
 *
 * Must be rendered inside a `.page-x` container (`<Page>`): the sticky bar bleeds to the edges.
 * Collapsing uses an IntersectionObserver on the large title (no scroll listeners).
 */
export function PageHeader({
  title,
  subtitle,
  actions,
  navActions,
  back,
  children,
  documentTitle = true,
  className,
}: PageHeaderProps) {
  const { t } = useTranslation()
  const isMobile = useIsMobile()
  const navigate = useNavigate()
  const barRef = useRef<HTMLDivElement>(null)
  const titleRef = useRef<HTMLHeadingElement>(null)
  const [collapsed, setCollapsed] = useState(false)

  useEffect(() => {
    const bar = barRef.current
    const heading = titleRef.current
    if (!bar || !heading || typeof IntersectionObserver === 'undefined') return
    // The bar's bottom edge: it sticks below the app header (`--app-header-h`, 0 on phones).
    const edge = (Number.parseFloat(getComputedStyle(bar).top) || 0) + bar.offsetHeight
    const observer = new IntersectionObserver(
      ([entry]) => {
        // Collapsed once the large title is fully hidden under the bar (and not below the fold).
        setCollapsed(!entry.isIntersecting && entry.boundingClientRect.top < edge)
      },
      { rootMargin: `-${edge}px 0px 0px 0px`, threshold: 0 },
    )
    observer.observe(heading)
    return () => observer.disconnect()
  }, [isMobile])

  useEffect(() => {
    if (!documentTitle) return
    const previous = document.title
    document.title = title ? `${title} · Rainy` : 'Rainy'
    return () => {
      document.title = previous
    }
  }, [title, documentTitle])

  const goBack = () => {
    if (typeof back === 'string') {
      navigate(back)
      return
    }
    const idx = (window.history.state as { idx?: number } | null)?.idx ?? 0
    if (idx > 0) navigate(-1)
    else navigate('/')
  }

  return (
    <header className={cn('relative', className)}>
      <div
        ref={barRef}
        className={cn(
          'ui-chrome bleed-x page-x hairline-b sticky top-(--app-header-h) z-30 pt-safe md:pt-0 transition-[background-color,border-color,backdrop-filter] duration-200',
          collapsed ? 'glass' : '[--hairline-color:transparent]',
        )}
      >
        <div className="relative flex h-11 items-center gap-1 md:h-12">
          <div className="flex min-w-0 flex-1 items-center gap-1">
            {back ? (
              <Button
                variant="ghost"
                size="icon"
                onClick={goBack}
                aria-label={t('actions.back')}
                className="-ml-2 size-11 text-primary hover:bg-transparent hover:text-primary/80 md:-ml-1 md:size-8 md:text-foreground md:hover:bg-accent"
              >
                <ChevronLeft className="size-7 md:size-5" strokeWidth={isMobile ? 2 : 1.75} />
              </Button>
            ) : null}
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
            <div className="pointer-events-none absolute inset-x-14 flex justify-center">
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
          {navActions ? <div className="flex shrink-0 items-center gap-1">{navActions}</div> : null}
        </div>
      </div>

      <div className="pt-1 pb-4 md:flex md:items-end md:justify-between md:gap-6 md:pt-2 md:pb-6">
        <div className="min-w-0">
          <h1
            ref={titleRef}
            className="text-[34px] leading-[41px] font-bold tracking-tight text-balance break-words md:text-3xl"
          >
            {title}
          </h1>
          {subtitle ? <div className="mt-1 text-[15px] text-muted-foreground md:text-sm">{subtitle}</div> : null}
        </div>
        {actions ? <div className="mt-4 flex flex-wrap items-center gap-2 md:mt-0 md:shrink-0">{actions}</div> : null}
      </div>

      {children}
    </header>
  )
}
