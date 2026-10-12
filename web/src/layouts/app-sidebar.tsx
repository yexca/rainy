import { useQuery } from '@tanstack/react-query'
import { ListMusic, Plus } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, NavLink, useLocation } from 'react-router'

import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupAction,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSkeleton,
  SidebarRail,
} from '@/components/ui/sidebar'
import { useAuth } from '@/hooks/use-auth'
import { api } from '@/lib/api/endpoints'
import { queryKeys } from '@/lib/query-keys'
import { cn } from '@/lib/utils'

import { Brand, SidebarToggle } from './app-header'
import { SidebarActions } from './header-actions'
import {
  ADMIN_SECTION,
  FOOTER_NAV,
  LIBRARY_NAV,
  TOOLS_SECTION,
  TRACKS_SECTION,
  isSectionPath,
  pathMatches,
  type NavItem,
} from './nav'
import { QuickActionsField } from './quick-actions'

/** Active items use the accent colour (Apple Music style) on a soft pill. */
const ITEM_CLASS =
  'h-9 gap-3 rounded-lg text-[13px] data-[active=true]:font-medium data-[active=true]:text-sidebar-primary [&>svg]:size-[18px] [&>svg]:text-sidebar-foreground/70 data-[active=true]:[&>svg]:text-sidebar-primary'

/**
 * On top of `ITEM_CLASS` for a rail with labels (`STYLE_RAIL`, Material): collapsed, an item is
 * its icon on a pill (filled while active or hovered) with the label on a line below.
 */
const LABEL_RAIL_ITEM_CLASS = [
  'group-data-[collapsible=icon]:size-auto! group-data-[collapsible=icon]:flex-col group-data-[collapsible=icon]:gap-0.5 group-data-[collapsible=icon]:p-0!',
  'group-data-[collapsible=icon]:text-center group-data-[collapsible=icon]:text-[11px] group-data-[collapsible=icon]:leading-4',
  'group-data-[collapsible=icon]:hover:bg-transparent group-data-[collapsible=icon]:active:bg-transparent group-data-[collapsible=icon]:data-[active=true]:bg-transparent',
  'group-data-[collapsible=icon]:[&>span]:max-w-full',
  'group-data-[collapsible=icon]:[&>svg]:h-8 group-data-[collapsible=icon]:[&>svg]:w-12 group-data-[collapsible=icon]:[&>svg]:rounded-full group-data-[collapsible=icon]:[&>svg]:px-[15px] group-data-[collapsible=icon]:[&>svg]:py-[7px]',
  'group-data-[collapsible=icon]:hover:[&>svg]:bg-sidebar-accent group-data-[collapsible=icon]:data-[active=true]:[&>svg]:bg-sidebar-accent',
].join(' ')

function NavGroup({ label, items, itemClass }: { label: string; items: readonly NavItem[]; itemClass: string }) {
  const { t } = useTranslation()
  const { pathname } = useLocation()
  return (
    <SidebarGroup>
      <SidebarGroupLabel>{label}</SidebarGroupLabel>
      <SidebarGroupContent>
        <SidebarMenu>
          {items.map((item) => {
            const label = t(item.labelKey)
            return (
              <SidebarMenuItem key={item.to}>
                <SidebarMenuButton
                  asChild
                  isActive={pathMatches(pathname, item.to, item.end)}
                  tooltip={label}
                  className={itemClass}
                >
                  <NavLink to={item.to} end={item.end}>
                    <item.icon strokeWidth={1.75} />
                    <span>{label}</span>
                  </NavLink>
                </SidebarMenuButton>
              </SidebarMenuItem>
            )
          })}
        </SidebarMenu>
      </SidebarGroupContent>
    </SidebarGroup>
  )
}

/**
 * Managers: one entry per section (Tracks, Library tools, Admin). Each links to its first page and
 * stays active on all of them; the section's pages are horizontal tabs in the page header.
 */
function ManageGroup({ isAdmin, itemClass }: { isAdmin: boolean; itemClass: string }) {
  const { t } = useTranslation()
  const { pathname } = useLocation()
  const sections = isAdmin ? [TRACKS_SECTION, TOOLS_SECTION, ADMIN_SECTION] : [TRACKS_SECTION, TOOLS_SECTION]
  return (
    <SidebarGroup>
      <SidebarGroupLabel>{t('nav.manage')}</SidebarGroupLabel>
      <SidebarGroupContent>
        <SidebarMenu>
          {sections.map((section) => {
            const label = t(section.labelKey)
            const first = section.tabs[0]
            return (
              <SidebarMenuItem key={section.labelKey}>
                <SidebarMenuButton
                  asChild
                  isActive={isSectionPath(section, pathname)}
                  tooltip={label}
                  className={itemClass}
                >
                  <NavLink to={first.to} end={first.end}>
                    <section.icon strokeWidth={1.75} />
                    <span>{label}</span>
                  </NavLink>
                </SidebarMenuButton>
              </SidebarMenuItem>
            )
          })}
        </SidebarMenu>
      </SidebarGroupContent>
    </SidebarGroup>
  )
}

function PlaylistsGroup({ itemClass }: { itemClass: string }) {
  const { t } = useTranslation()
  const { pathname } = useLocation()
  const { data: playlists, isPending, isError } = useQuery({
    queryKey: queryKeys.playlists,
    queryFn: ({ signal }) => api.playlists.list({ signal }),
  })
  const allLabel = t('nav.allPlaylists')

  return (
    <SidebarGroup>
      <SidebarGroupLabel asChild>
        <Link to="/playlists" className="hover:text-sidebar-foreground">
          {t('nav.playlists')}
        </Link>
      </SidebarGroupLabel>
      <SidebarGroupAction asChild title={t('nav.newPlaylist')}>
        <Link to="/playlists?new=1">
          <Plus />
          <span className="sr-only">{t('nav.newPlaylist')}</span>
        </Link>
      </SidebarGroupAction>
      <SidebarGroupContent>
        <SidebarMenu>
          {/* Icon rail: a single entry instead of one icon per playlist. */}
          <SidebarMenuItem className="hidden group-data-[collapsible=icon]:block">
            <SidebarMenuButton
              asChild
              isActive={pathMatches(pathname, '/playlists')}
              tooltip={allLabel}
              className={itemClass}
            >
              <NavLink to="/playlists">
                <ListMusic strokeWidth={1.75} />
                <span>{allLabel}</span>
              </NavLink>
            </SidebarMenuButton>
          </SidebarMenuItem>

          {isPending ? (
            Array.from({ length: 3 }, (_, i) => (
              <SidebarMenuItem key={i} className="group-data-[collapsible=icon]:hidden">
                <SidebarMenuSkeleton showIcon />
              </SidebarMenuItem>
            ))
          ) : isError || !playlists || playlists.length === 0 ? (
            <SidebarMenuItem className="group-data-[collapsible=icon]:hidden">
              <p className="px-2 py-1.5 text-xs text-muted-foreground">{t('nav.noPlaylists')}</p>
            </SidebarMenuItem>
          ) : (
            playlists.map((playlist) => (
              <SidebarMenuItem key={playlist.id} className="group-data-[collapsible=icon]:hidden">
                <SidebarMenuButton
                  asChild
                  isActive={pathname === `/playlists/${playlist.id}`}
                  className={itemClass}
                >
                  <NavLink to={`/playlists/${playlist.id}`}>
                    <ListMusic strokeWidth={1.75} />
                    <span>{playlist.name}</span>
                  </NavLink>
                </SidebarMenuButton>
              </SidebarMenuItem>
            ))
          )}
        </SidebarMenu>
      </SidebarGroupContent>
    </SidebarGroup>
  )
}

/** Entries pinned below the scrolling groups (Settings, About) while the app header holds the account menu. */
function FooterNav({ itemClass }: { itemClass: string }) {
  const { t } = useTranslation()
  const { pathname } = useLocation()
  return (
    <SidebarFooter className="border-t border-sidebar-border">
      <SidebarMenu>
        {FOOTER_NAV.map((item) => {
          const label = t(item.labelKey)
          return (
            <SidebarMenuItem key={item.to}>
              <SidebarMenuButton asChild isActive={pathMatches(pathname, item.to, item.end)} tooltip={label} className={itemClass}>
                <NavLink to={item.to} end={item.end}>
                  <item.icon strokeWidth={1.75} />
                  <span>{label}</span>
                </NavLink>
              </SidebarMenuButton>
            </SidebarMenuItem>
          )
        })}
      </SidebarMenu>
    </SidebarFooter>
  )
}

/**
 * A sidebar block that folds away (its height animates) when the chrome layout has no use for it;
 * while folded it is out of the tab order and the accessibility tree.
 */
function Fold({ open, children }: { open: boolean; children: ReactNode }) {
  return (
    <div
      data-chrome-motion
      inert={!open}
      aria-hidden={!open || undefined}
      className={cn(
        'grid shrink-0 transition-[grid-template-rows,visibility] duration-300 ease-out',
        open ? 'grid-rows-[1fr]' : 'invisible grid-rows-[0fr]',
      )}
    >
      <div className="min-h-0 overflow-hidden">{children}</div>
    </div>
  )
}

/**
 * Desktop / tablet navigation (shadcn Sidebar, collapsible to an icon rail). It sits below the
 * app header, which holds the logo, the sidebar toggle, quick controls and the account menu.
 *
 * `standalone` (styles with the `sidebar` chrome): there is no app header, so the sidebar reaches
 * the top with the toggle and logo above the groups, and the account row with the quick controls
 * replaces Settings and About at the bottom (both are in the account menu). The blocks fold in
 * and out when the style changes.
 * `field`: Quick actions is a search-like field; a standalone sidebar shows it under the logo
 * (the header has it otherwise).
 * `labelRail`: collapsed, the sidebar is a wider rail with a label under each icon; it scrolls
 * when the items do not fit.
 */
export function AppSidebar({
  standalone = false,
  field = false,
  labelRail = false,
  className,
}: {
  standalone?: boolean
  field?: boolean
  labelRail?: boolean
  className?: string
}) {
  const { t } = useTranslation()
  const { isManager, isAdmin } = useAuth()
  const toggleLabel = t('nav.toggleSidebar')
  const itemClass = labelRail ? cn(ITEM_CLASS, LABEL_RAIL_ITEM_CLASS) : ITEM_CLASS

  return (
    <Sidebar collapsible="icon" data-chrome-motion className={cn('ui-chrome', className)}>
      <Fold open={standalone}>
        <div className="flex h-12 items-center gap-1 px-2">
          <SidebarToggle />
          <Brand className="group-data-[collapsible=icon]:hidden" />
        </div>
      </Fold>
      <Fold open={standalone && field}>
        <div className="px-2 pt-1 pb-1.5">
          <QuickActionsField className="w-full" />
        </div>
      </Fold>
      <SidebarContent className={cn('scrollbar-thin pt-1', labelRail && 'group-data-[collapsible=icon]:overflow-auto')}>
        <NavGroup label={t('nav.library')} items={LIBRARY_NAV} itemClass={itemClass} />
        <PlaylistsGroup itemClass={itemClass} />
        {isManager ? <ManageGroup isAdmin={isAdmin} itemClass={itemClass} /> : null}
      </SidebarContent>
      <Fold open={!standalone}>
        <FooterNav itemClass={itemClass} />
      </Fold>
      <Fold open={standalone}>
        <SidebarFooter className="border-t border-sidebar-border">
          <SidebarActions active={standalone} field={field} />
        </SidebarFooter>
      </Fold>
      <SidebarRail aria-label={toggleLabel} title={toggleLabel} />
    </Sidebar>
  )
}
