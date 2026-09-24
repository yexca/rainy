import { ChevronRight } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'

import { cn } from '@/lib/utils'

export interface SectionHeaderProps {
  title: ReactNode
  /** "See all" target. */
  to?: string
  /** Custom label for the link (default "See all"). */
  linkLabel?: string
  /** Extra controls on the right (before the link). */
  actions?: ReactNode
  id?: string
  className?: string
}

/** Section title row: `text-xl` semibold with an optional accent "See all" link. */
export function SectionHeader({ title, to, linkLabel, actions, id, className }: SectionHeaderProps) {
  const { t } = useTranslation()
  return (
    <div className={cn('mb-3 flex min-h-9 items-center justify-between gap-3', className)}>
      <h2 id={id} className="min-w-0 truncate text-[22px] leading-7 font-bold tracking-tight md:text-xl md:font-semibold">
        {to ? (
          <Link to={to} className="group/title inline-flex items-center gap-0.5 hover:opacity-80 md:pointer-events-auto">
            {title}
            <ChevronRight
              className="size-5 text-muted-foreground transition-transform group-hover/title:translate-x-0.5 md:hidden"
              strokeWidth={2}
              aria-hidden
            />
          </Link>
        ) : (
          title
        )}
      </h2>
      <div className="flex shrink-0 items-center gap-1">
        {actions}
        {to ? (
          <Link
            to={to}
            className="hidden rounded-md px-2 py-1 text-sm font-medium text-primary outline-none hover:bg-primary/10 focus-visible:ring-2 focus-visible:ring-ring/50 md:inline-flex"
          >
            {linkLabel ?? t('library:actions.seeAll')}
          </Link>
        ) : null}
      </div>
    </div>
  )
}
