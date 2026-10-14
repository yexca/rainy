import { PanelLeft } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'

import { Logo } from '@/components/logo'
import { Button } from '@/components/ui/button'
import { useSidebar } from '@/components/ui/sidebar'
import { useIsMobile } from '@/hooks/use-media-query'
import { cn } from '@/lib/utils'
import { useUI } from '@/stores/ui'

import { HeaderActions } from './header-actions'
import { QuickActionsField } from './quick-actions'

/** The sidebar toggle (tablets and desktops), before the logo. */
export function SidebarToggle() {
  const { t } = useTranslation()
  const { toggleSidebar } = useSidebar()
  const label = t('nav.toggleSidebar')

  return (
    <Button
      variant="ghost"
      size="icon"
      onClick={toggleSidebar}
      aria-label={label}
      title={label}
      className="size-9 shrink-0 text-sidebar-foreground/70 hover:bg-sidebar-accent"
    >
      <PanelLeft strokeWidth={1.75} />
    </Button>
  )
}

/** The logo and name, linking home. */
export function Brand({ className }: { className?: string }) {
  return (
    <Link
      to="/"
      className={cn(
        'flex min-w-0 items-center gap-2 rounded-lg px-1.5 py-1 outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring',
        className,
      )}
    >
      <Logo size={26} />
      <span className="truncate text-[17px] font-semibold tracking-tight">Rainy</span>
    </Link>
  )
}

/**
 * App header: a full-width bar across the top of every layout. Tablets and desktops paint it in
 * the sidebar's colours so the two read as one frame (`.chrome-bar`: glass over the backdrop in
 * immersive mode), with the sidebar toggle before the logo;
 * phones (no sidebar) get a glass bar matching the tab bar, with just the logo. The quick-control
 * tray and the account menu sit on the right. Its height is `--app-header-h` (safe area
 * included); page top bars stick right below it. A page without one (`PageHeader bar={false}`)
 * has its title after the logo once its large title has scrolled away (`useUI.headerTitle`).
 *
 * `hidden` (styles with the `sidebar` chrome, tablets and desktops): the bar slides up out of
 * view and the sidebar shows the same controls; only a strip over the safe area stays.
 * `field` (styles whose Quick actions entry is a field, tablets and desktops): Quick actions is a
 * search-like field in the middle of the bar instead of the lightning icon in the tray.
 */
export function AppHeader({ hidden = false, field = false, className }: { hidden?: boolean; field?: boolean; className?: string }) {
  const isMobile = useIsMobile()
  const headerTitle = useUI((s) => s.headerTitle)

  return (
    <header
      data-chrome-motion
      inert={hidden}
      aria-hidden={hidden || undefined}
      className={cn(
        'ui-chrome fixed inset-x-0 top-0 z-40 h-[calc(3rem+var(--safe-top))] pt-safe transition-transform duration-300 ease-out',
        isMobile ? 'glass hairline-b' : 'chrome-bar border-b border-sidebar-border text-sidebar-foreground',
        hidden && '-translate-y-12',
        className,
      )}
    >
      <div
        data-chrome-motion
        className={cn(
          'flex h-full items-center gap-1 pr-[calc(0.75rem+var(--safe-right))] transition-[visibility] duration-300',
          isMobile ? 'pl-[calc(0.75rem+var(--safe-left))]' : 'pl-[calc(0.5rem+var(--safe-left))]',
          hidden && 'invisible',
        )}
      >
        {isMobile ? null : <SidebarToggle />}
        <Brand className="-ml-1.5 shrink-0 md:ml-0" />
        {headerTitle ? (
          <div
            aria-hidden={!headerTitle.shown}
            className={cn(
              'flex min-w-0 items-center gap-2 transition-opacity duration-200 md:gap-3 md:pl-1.5',
              headerTitle.shown ? 'opacity-100' : 'opacity-0',
            )}
          >
            <span aria-hidden className="h-5 w-px shrink-0 bg-border md:bg-sidebar-border" />
            <span className="truncate text-sm font-semibold">{headerTitle.title}</span>
          </div>
        ) : null}
        {field ? (
          <div className="flex min-w-0 flex-1 justify-center px-4">
            <QuickActionsField className="w-full max-w-md" />
          </div>
        ) : null}
        <HeaderActions active={!hidden} field={field} />
      </div>
    </header>
  )
}
