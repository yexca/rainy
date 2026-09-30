import { useTranslation } from 'react-i18next'
import { Link, useLocation } from 'react-router'

import { useIsMobile } from '@/hooks/use-media-query'
import { TRACKS_TABS, pathMatches } from '@/layouts/nav'
import { cn } from '@/lib/utils'

import { ManageSections } from './manage-sections'

/**
 * Tabs of the Tracks entry (Metadata / Upload / Online), rendered in each page's header. Each tab
 * is its own route, so links such as "upload into this folder" keep working. Phones also get the
 * folded library tools here, since they have no sidebar.
 */
export function TracksTabs({ className }: { className?: string }) {
  const { t } = useTranslation()
  const { pathname } = useLocation()
  const isMobile = useIsMobile()
  return (
    <div className={cn('flex flex-wrap items-center gap-3 pb-4', className)}>
      <nav
        aria-label={t('nav.tracks')}
        className="inline-flex h-11 items-center rounded-xl bg-muted p-1 text-muted-foreground max-sm:w-full sm:h-9 sm:rounded-lg sm:p-[3px]"
      >
        {TRACKS_TABS.map((tab) => {
          const active = pathMatches(pathname, tab.to, tab.end)
          return (
            <Link
              key={tab.to}
              to={tab.to}
              aria-current={active ? 'page' : undefined}
              className={cn(
                'inline-flex h-full flex-1 items-center justify-center gap-1.5 rounded-lg px-3 text-sm font-medium whitespace-nowrap transition-all outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 sm:rounded-md sm:px-5',
                active ? 'bg-background text-foreground shadow-sm dark:bg-input/30' : 'text-foreground/60 hover:text-foreground',
              )}
            >
              <tab.icon className="size-4 shrink-0" strokeWidth={1.75} aria-hidden />
              {t(tab.labelKey)}
            </Link>
          )
        })}
      </nav>
      {isMobile ? <ManageSections /> : null}
    </div>
  )
}
