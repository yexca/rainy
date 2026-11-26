import { AppWindow, Maximize2, PanelBottom, PictureInPicture2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

import { usePlayerDock, type DockMode } from '../dock'
import { DOCK_MODES } from '../lib/dock'
import { useCurrentTrack, usePlayer } from '../store'
import { ModeToggle } from './transport'

const MODE_ICONS: Record<DockMode, typeof PanelBottom> = {
  window: AppWindow,
  bar: PanelBottom,
}

/** Menu value for full-screen Now Playing, which is an overlay rather than a dock mode. */
const FULL_SCREEN = 'fullscreen'

/**
 * "Player layout" menu items (tablet / desktop), three layouts: full screen, floating window or
 * bottom bar. Full screen opens over the current layout and returns to it on close. Shared by
 * the floating window's layout menu and the bottom bar's options menu.
 */
export function DockModeItems() {
  const { t } = useTranslation('player')
  const mode = usePlayerDock((s) => s.mode)
  const setMode = usePlayerDock((s) => s.setMode)
  const nowPlayingOpen = usePlayer((s) => s.nowPlayingOpen)
  const setNowPlayingOpen = usePlayer((s) => s.setNowPlayingOpen)
  const hasTrack = !!useCurrentTrack()

  const onLayoutChange = (value: string) => {
    if (value === FULL_SCREEN) {
      setNowPlayingOpen(true)
      return
    }
    setMode(value as DockMode)
    setNowPlayingOpen(false)
  }

  return (
    <>
      <DropdownMenuLabel className="text-xs font-medium text-muted-foreground">{t('layout.title')}</DropdownMenuLabel>
      <DropdownMenuRadioGroup value={nowPlayingOpen ? FULL_SCREEN : mode} onValueChange={onLayoutChange}>
        <DropdownMenuRadioItem value={FULL_SCREEN} disabled={!hasTrack}>
          <Maximize2 className="text-muted-foreground" strokeWidth={1.75} />
          {t('expand')}
        </DropdownMenuRadioItem>
        {DOCK_MODES.map((m) => {
          const Icon = MODE_ICONS[m]
          return (
            <DropdownMenuRadioItem key={m} value={m}>
              <Icon className="text-muted-foreground" strokeWidth={1.75} />
              {t(`layout.${m}`)}
            </DropdownMenuRadioItem>
          )
        })}
      </DropdownMenuRadioGroup>
    </>
  )
}

/** The floating window's "Player layout" button with `<DockModeItems/>`. */
export function DockModeMenu() {
  const { t } = useTranslation('player')
  const label = t('layout.title')

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <ModeToggle tone="sheet" active={false} aria-label={label} title={label} className="size-9 rounded-full">
          <PictureInPicture2 className="size-[18px]" strokeWidth={1.9} />
        </ModeToggle>
      </DropdownMenuTrigger>
      <DropdownMenuContent side="top" align="end" className="min-w-52">
        <DockModeItems />
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
