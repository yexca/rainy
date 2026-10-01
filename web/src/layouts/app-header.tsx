import { PanelLeft } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'

import { Logo } from '@/components/logo'
import { Button } from '@/components/ui/button'
import { useSidebar } from '@/components/ui/sidebar'
import { cn } from '@/lib/utils'

import { UserMenu } from './user-menu'

/**
 * App header (tablet / desktop): a full-width bar above the sidebar and the content, in the
 * sidebar's colours so the two read as one frame. The sidebar toggle and the logo sit on the left,
 * the account menu on the right. Its height is `--app-header-h` (0 on phones, which keep their
 * own iOS nav bars); page top bars stick right below it.
 */
export function AppHeader({ className }: { className?: string }) {
  const { t } = useTranslation()
  const { toggleSidebar } = useSidebar()
  const toggleLabel = t('nav.toggleSidebar')

  return (
    <header
      className={cn(
        'ui-chrome fixed inset-x-0 top-0 z-40 flex h-(--app-header-h) items-center gap-1 border-b border-sidebar-border bg-sidebar pt-safe pr-[calc(0.75rem+var(--safe-right))] pl-[calc(0.5rem+var(--safe-left))] text-sidebar-foreground',
        className,
      )}
    >
      <Button
        variant="ghost"
        size="icon"
        onClick={toggleSidebar}
        aria-label={toggleLabel}
        title={toggleLabel}
        className="size-9 text-sidebar-foreground/70 hover:bg-sidebar-accent"
      >
        <PanelLeft strokeWidth={1.75} />
      </Button>
      <Link
        to="/"
        className="flex min-w-0 items-center gap-2 rounded-lg px-1.5 py-1 outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring"
      >
        <Logo size={26} />
        <span className="truncate text-[17px] font-semibold tracking-tight">Rainy</span>
      </Link>
      <div className="ml-auto flex items-center gap-1">
        <UserMenu variant="header" />
      </div>
    </header>
  )
}
