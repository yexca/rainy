import { useTranslation } from 'react-i18next'
import { Link, useLocation } from 'react-router'

import { useAuth } from '@/hooks/use-auth'
import { cn } from '@/lib/utils'

import { TABS, isTabActive } from './nav'

/** iOS-style bottom tab bar (phones): glass, hairline on top, safe-area aware. */
export function MobileTabBar() {
  const { t } = useTranslation()
  const { pathname } = useLocation()
  const { isManager } = useAuth()
  const tabs = TABS.filter((tab) => !tab.managersOnly || isManager)

  return (
    <nav
      aria-label={t('nav.main')}
      className="ui-chrome glass hairline-t fixed inset-x-0 bottom-0 z-40 pb-safe pl-safe pr-safe"
    >
      <ul className="mx-auto flex h-(--tabbar-h) max-w-lg items-stretch">
        {tabs.map((tab) => {
          const active = isTabActive(tab, pathname)
          return (
            <li key={tab.to} className="flex flex-1">
              <Link
                to={tab.to}
                aria-current={active ? 'page' : undefined}
                className={cn(
                  'flex flex-1 flex-col items-center justify-center gap-0.5 pt-1 text-[10px] font-medium tracking-wide transition-[color,transform] duration-150 active:scale-[0.94]',
                  'outline-none focus-visible:text-foreground',
                  active ? 'text-primary' : 'text-muted-foreground',
                )}
              >
                <tab.icon className="size-6" strokeWidth={active ? 2.1 : 1.75} aria-hidden />
                <span>{t(tab.labelKey)}</span>
              </Link>
            </li>
          )
        })}
      </ul>
    </nav>
  )
}
