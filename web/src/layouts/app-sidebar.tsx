import { useQuery } from '@tanstack/react-query'
import { ChevronRight, ListMusic, PanelLeft, Plus, ShieldCheck, Wrench, type LucideIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, NavLink, useLocation } from 'react-router'

import { Logo } from '@/components/logo'
import { Button } from '@/components/ui/button'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
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
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
  SidebarRail,
  useSidebar,
} from '@/components/ui/sidebar'
import { useAuth } from '@/hooks/use-auth'
import { api } from '@/lib/api/endpoints'
import { queryKeys } from '@/lib/query-keys'
import { cn } from '@/lib/utils'

import { ADMIN_NAV, LIBRARY_NAV, MANAGE_NAV, METADATA_NAV, pathMatches, type NavItem } from './nav'
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

const SUB_ITEM_CLASS =
  'h-8 gap-3 rounded-lg text-[13px] data-[active=true]:bg-transparent data-[active=true]:font-medium data-[active=true]:text-sidebar-primary [&>svg]:size-4 [&>svg]:text-sidebar-foreground/70 data-[active=true]:[&>svg]:text-sidebar-primary'

/**
 * A folded group of rarely used pages (library tools, administration). It starts collapsed on
 * every visit and opens by itself while one of its pages is showing. On the icon rail it is a
 * single icon with a flyout menu.
 */
function FoldedSection({ label, icon: Icon, items }: { label: string; icon: LucideIcon; items: readonly NavItem[] }) {
  const { t } = useTranslation()
  const { pathname } = useLocation()
  const { state, isMobile } = useSidebar()
  const active = items.some((item) => pathMatches(pathname, item.to, item.end))
  const [open, setOpen] = useState(active)
  // Open when navigation lands on one of the pages (without closing it again on the way out).
  const [wasActive, setWasActive] = useState(active)
  if (active !== wasActive) {
    setWasActive(active)
    if (active) setOpen(true)
  }

  if (state === 'collapsed' && !isMobile) {
    return (
      <SidebarMenuItem>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <SidebarMenuButton isActive={active} aria-label={label} title={label} className={ITEM_CLASS}>
              <Icon strokeWidth={1.75} />
              <span>{label}</span>
            </SidebarMenuButton>
          </DropdownMenuTrigger>
          <DropdownMenuContent side="right" align="start" className="min-w-48 rounded-xl">
            <DropdownMenuLabel className="text-xs text-muted-foreground">{label}</DropdownMenuLabel>
            {items.map((item) => (
              <DropdownMenuItem key={item.to} asChild>
                <Link to={item.to}>
                  <item.icon strokeWidth={1.75} />
                  {t(item.labelKey)}
                </Link>
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
      </SidebarMenuItem>
    )
  }

  return (
    <Collapsible asChild open={open} onOpenChange={setOpen} className="group/folded">
      <SidebarMenuItem>
        <CollapsibleTrigger asChild>
          <SidebarMenuButton isActive={!open && active} className={cn(ITEM_CLASS, 'text-sidebar-foreground/80')}>
            <Icon strokeWidth={1.75} />
            <span>{label}</span>
            <ChevronRight
              aria-hidden
              className="ml-auto size-4! transition-transform duration-200 group-data-[state=open]/folded:rotate-90"
            />
          </SidebarMenuButton>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <SidebarMenuSub className="mr-0 pr-0">
            {items.map((item) => (
              <SidebarMenuSubItem key={item.to}>
                <SidebarMenuSubButton asChild isActive={pathMatches(pathname, item.to, item.end)} className={SUB_ITEM_CLASS}>
                  <NavLink to={item.to} end={item.end}>
                    <item.icon strokeWidth={1.75} />
                    <span>{t(item.labelKey)}</span>
                  </NavLink>
                </SidebarMenuSubButton>
              </SidebarMenuSubItem>
            ))}
          </SidebarMenuSub>
        </CollapsibleContent>
      </SidebarMenuItem>
    </Collapsible>
  )
}

/** Managers: metadata editing up front, the other tools and administration folded below it. */
function ManageGroup({ isAdmin }: { isAdmin: boolean }) {
  const { t } = useTranslation()
  const { pathname } = useLocation()
  const label = t(METADATA_NAV.labelKey)
  return (
    <SidebarGroup>
      <SidebarGroupLabel>{t('nav.manage')}</SidebarGroupLabel>
      <SidebarGroupContent>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton
              asChild
              isActive={pathMatches(pathname, METADATA_NAV.to, METADATA_NAV.end)}
              tooltip={label}
              className={ITEM_CLASS}
            >
              <NavLink to={METADATA_NAV.to} end={METADATA_NAV.end}>
                <METADATA_NAV.icon strokeWidth={1.75} />
                <span>{label}</span>
              </NavLink>
            </SidebarMenuButton>
          </SidebarMenuItem>
          <FoldedSection label={t('nav.libraryTools')} icon={Wrench} items={MANAGE_NAV} />
          {isAdmin ? <FoldedSection label={t('nav.admin')} icon={ShieldCheck} items={ADMIN_NAV} /> : null}
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
        {isManager ? <ManageGroup isAdmin={isAdmin} /> : null}
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
