import { ChevronLeft } from 'lucide-react'
import { useEffect, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'

import { Button } from '@/components/ui/button'
import { useIsMobile } from '@/hooks/use-media-query'
import { cn } from '@/lib/utils'
import { useUI } from '@/stores/ui'

export interface PageHeaderProps {
  /** Page title (also used for the collapsed bar and `document.title`). */
  title: string
  subtitle?: ReactNode
  /**
   * Subtitle on the title's baseline, to its right, instead of below it: the header keeps its
   * height whether or not there is one (management pages, whose subtitle is a count that loads later).
   */
  subtitleInline?: boolean
  /** Primary actions: right of the title on desktop, below it on mobile. */
  actions?: ReactNode
  /** Compact control aligned inline to the right of the heading on mobile. */
  titleAction?: ReactNode
  /** Compact icon buttons in the top bar (iOS nav bar on mobile, sticky bar on desktop). */
  navActions?: ReactNode
  /** Back button: `true` = history back (falls back to Home), a string = link target. */
  back?: boolean | string
  /**
   * `false`: no top bar of its own. The collapsed title shows in the app header instead (styles
   * without an app header show none), so `back` and `navActions` have nowhere to go.
   */
  bar?: boolean
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
 * - `bar={false}`: the bar is left out and the app header shows the title on scroll.
 *
 * Must be rendered directly inside a `.page-x` container (`<Page>`): the sticky bar bleeds to the
 * edges and, rendered beside the `<header>`, sticks for as long as that container scrolls.
 * Collapsing uses an IntersectionObserver on the large title (no scroll listeners).
 */
export function PageHeader({
  title,
  subtitle,
  subtitleInline = false,
  actions,
  titleAction,
  navActions,
  back,
  bar = true,
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
  const setHeaderTitle = useUI((s) => s.setHeaderTitle)

  useEffect(() => {
    const bar = barRef.current
    const heading = titleRef.current
    if (!bar || !heading || typeof IntersectionObserver === 'undefined') return
    // The bar's bottom edge: it sticks below the app header (`--app-header-h`).
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
  }, [isMobile, bar])

  useEffect(() => {
    if (bar) return
    setHeaderTitle({ title, shown: collapsed })
    return () => setHeaderTitle(null)
  }, [bar, title, collapsed, setHeaderTitle])

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

  // The bar is the header's sibling, not its child: a sticky element only sticks inside its
  // parent, so inside the short <header> it would scroll away with the title. Here its parent is
  // the <Page>, so it stays pinned for the whole page.
  return (
    <>
      {bar ? (
        <div
          ref={barRef}
          className={cn(
            'ui-chrome bleed-x page-x hairline-b sticky top-(--app-header-h) z-30 transition-[background-color,border-color] duration-200',
            collapsed ? 'glass-bar' : '[--hairline-color:transparent]',
          )}
        >
          <div className="relative flex h-(--page-bar-h) items-center gap-1">
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
      ) : (
        // Where a bar would stick: the title is collapsed once it has scrolled above this line.
        <div ref={barRef} aria-hidden className="sticky top-(--app-header-h) h-0" />
      )}

      <header className={className}>
        <div
          className={cn(
            'pb-4 md:flex md:items-end md:justify-between md:gap-6 md:pb-6',
            // Without a bar the title keeps the place it has below one.
            bar ? 'pt-1 md:pt-2' : 'pt-12 md:pt-14',
          )}
        >
          <div className={cn('min-w-0', titleAction && 'flex items-center justify-between gap-3 md:block')}>
            <div className={cn('min-w-0', subtitleInline && 'flex flex-wrap items-baseline gap-x-3')}>
              <h1
                ref={titleRef}
                className="text-[34px] leading-[41px] font-bold tracking-tight text-balance break-words md:text-3xl"
              >
                {title}
              </h1>
              {subtitle ? (
                <div className={cn('text-[15px] text-muted-foreground md:text-sm', subtitleInline ? 'tnum whitespace-nowrap' : 'mt-1')}>{subtitle}</div>
              ) : null}
            </div>
            {titleAction ? <div className="shrink-0 md:hidden">{titleAction}</div> : null}
          </div>
          {actions ? <div className="mt-4 flex flex-wrap items-center gap-2 md:mt-0 md:shrink-0">{actions}</div> : null}
        </div>

        {children}
      </header>
    </>
  )
}
