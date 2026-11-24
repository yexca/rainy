import { useEffect, useRef, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useLocation } from 'react-router'

import { cn } from '@/lib/utils'

import { navItemActive, type NavSection } from './nav'

/**
 * Horizontal route tabs of a manager section (Tracks, Library tools, Admin), rendered in each of
 * its pages' header. Each tab is its own route, so deep links such as "upload into this folder"
 * keep working. Phones show labels only, like an iOS segmented control; if they still do not fit,
 * the bar scrolls sideways with the active tab kept in view. `children` sit beside it.
 * `badges` puts a count on a tab (by its `to`); a tab in `hidden` is left out unless it is open.
 */
export function SectionTabs({
  section,
  badges,
  hidden,
  children,
  className,
}: {
  section: NavSection
  badges?: Readonly<Record<string, number>>
  hidden?: readonly string[]
  children?: ReactNode
  className?: string
}) {
  const { t } = useTranslation()
  const { pathname } = useLocation()
  const navRef = useRef<HTMLElement>(null)

  useEffect(() => {
    const nav = navRef.current
    const active = nav?.querySelector<HTMLElement>('[aria-current="page"]')
    if (!nav || !active || nav.scrollWidth <= nav.clientWidth) return
    nav.scrollLeft = active.offsetLeft - (nav.clientWidth - active.offsetWidth) / 2
  }, [pathname])

  return (
    <div className={cn('flex flex-wrap items-center gap-3 pb-4', className)}>
      <nav
        ref={navRef}
        aria-label={t(section.labelKey)}
        className="scrollbar-none relative inline-flex h-11 max-w-full items-center overflow-x-auto rounded-xl bg-muted p-1 text-muted-foreground max-sm:w-full sm:h-9 sm:rounded-lg sm:p-[3px]"
      >
        {section.tabs.map((tab) => {
          const active = navItemActive(tab, pathname)
          if (!active && hidden?.includes(tab.to)) return null
          const badge = badges?.[tab.to] ?? 0
          return (
            <Link
              key={tab.to}
              to={tab.to}
              aria-current={active ? 'page' : undefined}
              className={cn(
                'inline-flex h-full flex-1 shrink-0 items-center justify-center gap-1.5 rounded-lg px-3 text-sm font-medium whitespace-nowrap transition-all outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 sm:rounded-md sm:px-5',
                active ? 'bg-background text-foreground shadow-sm dark:bg-input/30' : 'text-foreground/60 hover:text-foreground',
              )}
            >
              <tab.icon className="size-4 shrink-0 max-sm:hidden" strokeWidth={1.75} aria-hidden />
              {t(tab.labelKey)}
              {badge > 0 ? (
                <span className="tnum grid h-[18px] min-w-[18px] place-items-center rounded-full bg-primary px-1 text-[11px] leading-none font-semibold text-primary-foreground">
                  {badge}
                </span>
              ) : null}
            </Link>
          )
        })}
      </nav>
      {children}
    </div>
  )
}
