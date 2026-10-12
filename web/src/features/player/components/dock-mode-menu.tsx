import { AppWindow, Maximize2, PanelBottom, PictureInPicture2, RectangleHorizontal } from 'lucide-react'
import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

import { usePlayerDock, type DockMode } from '../dock'
import { DOCK_MODES } from '../lib/dock'
import { useCurrentTrack, usePlayer } from '../store'
import type { PlayerTone } from './scrubber'
import { ModeToggle } from './transport'

/** After the menu closes, ignore the tooltip opening from the focus Radix returns (ms). */
const TOOLTIP_QUIET = 400

const MODE_ICONS: Record<DockMode, typeof PanelBottom> = {
  window: AppWindow,
  compact: RectangleHorizontal,
  bar: PanelBottom,
}

/** Menu value for full-screen Now Playing, which is an overlay rather than a dock mode. */
const FULL_SCREEN = 'fullscreen'

/**
 * "Player layout" menu (tablet / desktop): full screen, floating window, floating mini bar or
 * bottom bar. Full screen opens over the current layout and returns to it on close.
 */
export function DockModeMenu({
  tone,
  className,
  onOpenChange,
}: {
  tone: PlayerTone
  className?: string
  onOpenChange?: (open: boolean) => void
}) {
  const { t } = useTranslation('player')
  const mode = usePlayerDock((s) => s.mode)
  const setMode = usePlayerDock((s) => s.setMode)
  const nowPlayingOpen = usePlayer((s) => s.nowPlayingOpen)
  const setNowPlayingOpen = usePlayer((s) => s.setNowPlayingOpen)
  const hasTrack = !!useCurrentTrack()
  const label = t('layout.title')
  const [tooltip, setTooltip] = useState(false)
  const quietUntil = useRef(0)

  const onMenuOpenChange = (open: boolean) => {
    setTooltip(false)
    if (!open) quietUntil.current = Date.now() + TOOLTIP_QUIET
    onOpenChange?.(open)
  }

  const onLayoutChange = (value: string) => {
    if (value === FULL_SCREEN) {
      setNowPlayingOpen(true)
      return
    }
    setMode(value as DockMode)
    setNowPlayingOpen(false)
  }

  const trigger = (
    <DropdownMenuTrigger asChild>
      <ModeToggle
        tone={tone}
        active={false}
        aria-label={label}
        title={tone === 'sheet' ? label : undefined}
        className={cn(tone === 'sheet' ? 'size-9 rounded-full' : 'size-8 rounded-md', className)}
      >
        <PictureInPicture2 className="size-[18px]" strokeWidth={1.9} />
      </ModeToggle>
    </DropdownMenuTrigger>
  )

  return (
    <DropdownMenu onOpenChange={onMenuOpenChange}>
      {tone === 'bar' ? (
        <Tooltip open={tooltip} onOpenChange={(open) => setTooltip(open && Date.now() >= quietUntil.current)}>
          <TooltipTrigger asChild>{trigger}</TooltipTrigger>
          <TooltipContent side="top">{label}</TooltipContent>
        </Tooltip>
      ) : (
        trigger
      )}
      <DropdownMenuContent side="top" align="end" className="min-w-52">
        <DropdownMenuLabel className="text-xs font-medium text-muted-foreground">{label}</DropdownMenuLabel>
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
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
