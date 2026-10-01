import { ChevronRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'

import { useAuth } from '@/hooks/use-auth'
import { ADMIN_SECTION, TOOLS_SECTION } from '@/layouts/nav'
import { cn } from '@/lib/utils'

/**
 * Phones have no sidebar: the Tracks tabs link to the other manager sections (library tools,
 * admin), whose pages carry their own tabs, like the sidebar entries on larger screens.
 */
export function ManageSections({ className }: { className?: string }) {
  const { t } = useTranslation()
  const { isAdmin } = useAuth()
  const sections = isAdmin ? [TOOLS_SECTION, ADMIN_SECTION] : [TOOLS_SECTION]
  return (
    <div className={cn('flex flex-wrap gap-2', className)}>
      {sections.map((section) => (
        <Link
          key={section.labelKey}
          to={section.tabs[0].to}
          className="flex h-11 items-center gap-2 rounded-xl bg-secondary px-3.5 text-[15px] font-medium text-secondary-foreground transition-transform outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 active:scale-[0.97]"
        >
          <section.icon className="size-[18px] text-primary" strokeWidth={1.75} aria-hidden />
          {t(section.labelKey)}
          <ChevronRight className="size-4 text-muted-foreground" aria-hidden />
        </Link>
      ))}
    </div>
  )
}
