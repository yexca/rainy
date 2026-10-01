import { useQuery } from '@tanstack/react-query'
import { ListMusic, Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link, NavLink, useLocation } from 'react-router'

import {
  Sidebar,
  SidebarContent,
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

import {
  ADMIN_SECTION,
  LIBRARY_NAV,
  TOOLS_SECTION,
  TRACKS_SECTION,
  isSectionPath,
  pathMatches,
  type NavItem,
} from './nav'

/** Active items use the accent colour (Apple Music style) on a soft pill. */
const ITEM_CLASS =
  'h-9 gap-3 rounded-lg text-[13px] data-[active=true]:font-medium data-[active=true]:text-sidebar-primary [&>svg]:size-[18px] [&>svg]:text-sidebar-foreground/70 data-[active=true]:[&>svg]:text-sidebar-primary'

function NavGroup({ label, items }: { label: string; items: readonly NavItem[] }) {
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
                  className={ITEM_CLASS}
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
function ManageGroup({ isAdmin }: { isAdmin: boolean }) {
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
                  className={ITEM_CLASS}
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

function PlaylistsGroup() {
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
              className={ITEM_CLASS}
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
                  className={ITEM_CLASS}
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

/**
 * Desktop / tablet navigation (shadcn Sidebar, collapsible to an icon rail). It sits below the
 * app header, which holds the logo, the sidebar toggle and the account menu.
 */
export function AppSidebar({ className }: { className?: string }) {
  const { t } = useTranslation()
  const { isManager, isAdmin } = useAuth()
  const toggleLabel = t('nav.toggleSidebar')

  return (
    <Sidebar collapsible="icon" className={cn('ui-chrome', className)}>
      <SidebarContent className="scrollbar-thin pt-1">
        <NavGroup label={t('nav.library')} items={LIBRARY_NAV} />
        <PlaylistsGroup />
        {isManager ? <ManageGroup isAdmin={isAdmin} /> : null}
      </SidebarContent>
      <SidebarRail aria-label={toggleLabel} title={toggleLabel} />
    </Sidebar>
  )
}
