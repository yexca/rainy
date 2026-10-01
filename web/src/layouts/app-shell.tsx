import { useState, type CSSProperties } from 'react'
import { useTranslation } from 'react-i18next'
import { Outlet } from 'react-router'

import { AppToaster } from '@/components/app-toaster'
import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar'
import { AddToPlaylistHost } from '@/features/library/components/add-to-playlist-host'
import { TagEditorHost } from '@/features/manage/tag-editor-host'
import { AudioEngine } from '@/features/player/components/audio-engine'
import { MascotCompanion } from '@/features/player/components/mascot-companion'
import { MiniPlayer } from '@/features/player/components/mini-player'
import { NowPlayingSheet } from '@/features/player/components/now-playing-sheet'
import { PlayerDock } from '@/features/player/components/player-dock'
import { useIsDesktop, useIsMobile } from '@/hooks/use-media-query'
import { useServerEvents } from '@/hooks/use-server-events'
import { useUI } from '@/stores/ui'

import { AppHeader } from './app-header'
import { AppSidebar } from './app-sidebar'
import { MobileTabBar } from './mobile-tab-bar'
import { NavigationProgress } from './navigation-progress'

const SIDEBAR_STYLE = { '--sidebar-width': '15rem', '--sidebar-width-icon': '3.25rem' } as CSSProperties

/**
 * Signed-in layout (docs/architecture/contract.md §9.3).
 *
 * - ≥ 768px: `<AppHeader/>` across the top (sidebar toggle, logo, account menu; `--app-header-h`),
 *   with the sidebar and the content below it.
 * - ≥ 1024px: expandable sidebar (state persisted) + content + bottom player bar.
 * - 768–1023px: icon sidebar (expandable for the session) + player bar.
 * - < 768px: no sidebar; bottom glass tab bar with a floating mini player above it.
 *
 * The document is the scroll container (native iOS behaviour; use `useWindowVirtualizer`).
 * Player chrome:
 * - `<PlayerDock/>` (≥ 768px) positions itself: the bottom bar (`--playerbar-h` + safe area,
 *   optionally auto-hiding), a floating window or a floating mini bar in the bottom-right corner.
 *   It publishes `<html data-player-dock>`; the sidebar, `.page-pad` and toasts read the space it
 *   reserves from `--player-reserve` / `--player-clearance` (index.css).
 * - `<MascotCompanion/>` (≥ 768px, Settings › Mascot) sits on top of that chrome in the
 *   bottom-right corner (`--player-clearance`, `--mascot-right`).
 * - `<MiniPlayer/>` renders inside a `fixed` slot 8px above the tab bar with 8px side margins
 *   and owns its height (`h-(--miniplayer-h)`); it may render nothing when idle.
 * Slots have no transform/filter, so `position: fixed` descendants stay viewport-relative.
 */
export function AppShell() {
  useServerEvents()
  const { t } = useTranslation()
  const isMobile = useIsMobile()
  const isDesktop = useIsDesktop()
  const sidebarOpen = useUI((s) => s.sidebarOpen)
  const setSidebarOpen = useUI((s) => s.setSidebarOpen)
  const mascotCompanion = useUI((s) => s.mascotCompanion)
  const [tabletOpen, setTabletOpen] = useState(false)

  return (
    <>
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:fixed focus:top-3 focus:left-3 focus:z-[70] focus:rounded-lg focus:bg-background focus:px-3 focus:py-2 focus:text-sm focus:shadow-lg"
      >
        {t('a11y.skipToContent')}
      </a>
      <NavigationProgress />

      <SidebarProvider
        open={isDesktop ? sidebarOpen : tabletOpen}
        onOpenChange={isDesktop ? setSidebarOpen : setTabletOpen}
        style={SIDEBAR_STYLE}
      >
        {isMobile ? null : (
          <>
            <AppHeader />
            <AppSidebar className="top-(--app-header-h) bottom-(--player-reserve) h-auto transition-[bottom] duration-300 ease-out" />
          </>
        )}
        <SidebarInset
          id="main"
          tabIndex={-1}
          className="min-w-0 pt-(--app-header-h) outline-none transition-[padding] duration-200 xl:pr-(--player-panel-w,0px)"
        >
          <Outlet />
        </SidebarInset>
      </SidebarProvider>

      {isMobile ? (
        <>
          <div data-slot="mini-player" className="fixed inset-x-2 bottom-[calc(var(--tabbar-h)+var(--safe-bottom)+8px)] z-40">
            <MiniPlayer />
          </div>
          <MobileTabBar />
        </>
      ) : (
        <>
          <PlayerDock />
          {mascotCompanion ? <MascotCompanion /> : null}
        </>
      )}

      <AudioEngine />
      <NowPlayingSheet />
      <TagEditorHost />
      <AddToPlaylistHost />
      <AppToaster />
    </>
  )
}
