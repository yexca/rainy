import { ChevronDown, Wrench } from 'lucide-react'
import { Fragment } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useAuth } from '@/hooks/use-auth'
import { ADMIN_NAV, MANAGE_NAV } from '@/layouts/nav'
import { cn } from '@/lib/utils'

/**
 * Phones have no sidebar: the Metadata tab keeps the rarer library tools and the admin pages
 * folded behind one "Library tools" menu, like the folded sidebar sections on larger screens.
 */
export function ManageSections({ className }: { className?: string }) {
  const { t } = useTranslation()
  const { isAdmin } = useAuth()
  const groups = [
    { label: t('nav.libraryTools'), items: MANAGE_NAV },
    ...(isAdmin ? [{ label: t('nav.admin'), items: ADMIN_NAV }] : []),
  ]
  return (
    <div className={cn('flex', className)}>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            className="flex h-11 items-center gap-2 rounded-xl bg-secondary px-3.5 text-[15px] font-medium text-secondary-foreground transition-transform outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 active:scale-[0.97]"
          >
            <Wrench className="size-[18px] text-primary" strokeWidth={1.75} aria-hidden />
            {t('nav.libraryTools')}
            <ChevronDown className="size-4 text-muted-foreground" aria-hidden />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="min-w-60 rounded-xl">
          {groups.map((group, i) => (
            <Fragment key={group.label}>
              {i > 0 ? <DropdownMenuSeparator /> : null}
              <DropdownMenuLabel className="text-xs text-muted-foreground">{group.label}</DropdownMenuLabel>
              {group.items.map((item) => (
                <DropdownMenuItem key={item.to} asChild className="h-11 text-[15px]">
                  <Link to={item.to}>
                    <item.icon className="size-[18px] text-primary" strokeWidth={1.75} aria-hidden />
                    {t(item.labelKey)}
                  </Link>
                </DropdownMenuItem>
              ))}
            </Fragment>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  )
}
