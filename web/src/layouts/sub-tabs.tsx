import { useTranslation } from 'react-i18next'
import { Link, useLocation } from 'react-router'

import { cn } from '@/lib/utils'

import { navItemActive, type NavItem } from './nav'

/**
 * The pages grouped under one section tab (`NavItem.items`), as a pill switch at the top of each
 * of them: Tracks → Add (Upload, Links, Online), Admin → Settings (General, yt-dlp, …). Phones
 * show labels only; the switch scrolls sideways if it still does not fit.
 */
export function SubTabs({ items, label, className }: { items: readonly NavItem[]; label: string; className?: string }) {
  const { t } = useTranslation()
  const { pathname } = useLocation()
  return (
    <nav
      aria-label={label}
      className={cn('scrollbar-none inline-flex max-w-full items-center gap-0.5 overflow-x-auto rounded-full border p-0.5 max-sm:w-full', className)}
    >
      {items.map((item) => {
        const active = navItemActive(item, pathname)
        return (
          <Link
            key={item.to}
            to={item.to}
            aria-current={active ? 'page' : undefined}
            className={cn(
              'inline-flex h-7 shrink-0 items-center justify-center gap-1.5 rounded-full px-3 text-xs font-medium whitespace-nowrap outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 max-sm:h-10 max-sm:flex-1',
              active ? 'bg-foreground text-background' : 'text-foreground/65 hover:text-foreground',
            )}
          >
            <item.icon className="size-3.5 max-sm:hidden" strokeWidth={2} aria-hidden />
            {t(item.labelKey)}
          </Link>
        )
      })}
    </nav>
  )
}
