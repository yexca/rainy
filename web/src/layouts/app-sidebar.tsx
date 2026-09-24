import { useQuery } from '@tanstack/react-query'
import { ListMusic, PanelLeft, Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link, NavLink, useLocation } from 'react-router'

import { Logo } from '@/components/logo'
import { Button } from '@/components/ui/button'
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupAction,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSkeleton,
  SidebarRail,
  useSidebar,
} from '@/components/ui/sidebar'
import { useAuth } from '@/hooks/use-auth'
import { api } from '@/lib/api/endpoints'
import { queryKeys } from '@/lib/query-keys'
import { cn } from '@/lib/utils'

import { ADMIN_NAV, LIBRARY_NAV, MANAGE_NAV, pathMatches, type NavItem } from './nav'
import { UserMenu } from './user-menu'

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

/** Desktop / tablet navigation (shadcn Sidebar, collapsible to an icon rail). */
export function AppSidebar({ className }: { className?: string }) {
  const { t } = useTranslation()
  const { isManager, isAdmin } = useAuth()
  const { toggleSidebar } = useSidebar()
  const toggleLabel = t('nav.toggleSidebar')

  return (
    <Sidebar collapsible="icon" className={cn('ui-chrome', className)}>
      <SidebarHeader className="pt-3">
        <div className="flex items-center gap-1 group-data-[collapsible=icon]:flex-col group-data-[collapsible=icon]:gap-2">
          <Link
            to="/"
            className="flex min-w-0 flex-1 items-center gap-2 rounded-lg px-1.5 py-1 outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring group-data-[collapsible=icon]:flex-none group-data-[collapsible=icon]:px-0"
          >
            <Logo size={28} />
            <span className="truncate text-[17px] font-semibold tracking-tight group-data-[collapsible=icon]:hidden">
              Rainy
            </span>
          </Link>
          <Button
            variant="ghost"
            size="icon"
            onClick={toggleSidebar}
            aria-label={toggleLabel}
            title={toggleLabel}
            className="size-8 text-sidebar-foreground/70"
          >
            <PanelLeft strokeWidth={1.75} />
          </Button>
        </div>
      </SidebarHeader>

      <SidebarContent className="scrollbar-thin">
        <NavGroup label={t('nav.library')} items={LIBRARY_NAV} />
        <PlaylistsGroup />
        {isManager ? <NavGroup label={t('nav.manage')} items={MANAGE_NAV} /> : null}
        {isAdmin ? <NavGroup label={t('nav.admin')} items={ADMIN_NAV} /> : null}
      </SidebarContent>

      <SidebarFooter>
        <SidebarMenu>
          <SidebarMenuItem>
            <UserMenu variant="sidebar" />
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>
      <SidebarRail aria-label={toggleLabel} title={toggleLabel} />
    </Sidebar>
  )
}
