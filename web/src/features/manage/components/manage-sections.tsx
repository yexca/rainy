import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'

import { useAuth } from '@/hooks/use-auth'
import { ADMIN_NAV, MANAGE_NAV } from '@/layouts/nav'
import { cn } from '@/lib/utils'

/**
 * Phones have no sidebar: a horizontally scrolling row of shortcuts to the other manage and
 * admin pages (shown on the library manager, the Manage tab's landing page).
 */
export function ManageSections({ className }: { className?: string }) {
  const { t } = useTranslation()
  const { isAdmin } = useAuth()
  const links = [...MANAGE_NAV.filter((item) => item.to !== '/manage'), ...(isAdmin ? ADMIN_NAV : [])]
  return (
    <nav aria-label={t('nav.manage')} className={cn('bleed-x', className)}>
      <ul className="page-x scrollbar-none flex gap-2 overflow-x-auto pb-1">
        {links.map((item) => (
          <li key={item.to} className="shrink-0">
            <Link
              to={item.to}
              className="flex h-11 items-center gap-2 rounded-xl bg-secondary px-3.5 text-[15px] font-medium text-secondary-foreground transition-transform active:scale-[0.97]"
            >
              <item.icon className="size-[18px] text-primary" strokeWidth={1.75} aria-hidden />
              {t(item.labelKey)}
            </Link>
          </li>
        ))}
      </ul>
    </nav>
  )
}
