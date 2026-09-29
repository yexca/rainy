import { AppWindow, Maximize2, PanelBottom, PictureInPicture2, RectangleHorizontal } from 'lucide-react'
import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
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
  bar: PanelBottom,
  window: AppWindow,
  compact: RectangleHorizontal,
}

/**
 * "Player layout" menu (tablet / desktop): bottom bar, floating window or floating mini bar,
 * the bottom bar's auto-hide switch, and full screen.
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
  const barAutoHide = usePlayerDock((s) => s.barAutoHide)
  const setMode = usePlayerDock((s) => s.setMode)
  const setBarAutoHide = usePlayerDock((s) => s.setBarAutoHide)
  const hasTrack = !!useCurrentTrack()
  const label = t('layout.title')
  const [tooltip, setTooltip] = useState(false)
  const quietUntil = useRef(0)

  const onMenuOpenChange = (open: boolean) => {
    setTooltip(false)
    if (!open) quietUntil.current = Date.now() + TOOLTIP_QUIET
    onOpenChange?.(open)
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
        <DropdownMenuRadioGroup value={mode} onValueChange={(value) => setMode(value as DockMode)}>
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
        <DropdownMenuSeparator />
        <DropdownMenuCheckboxItem
          checked={barAutoHide}
          disabled={mode !== 'bar'}
          onCheckedChange={(checked) => setBarAutoHide(checked === true)}
        >
          {t('layout.autoHide')}
        </DropdownMenuCheckboxItem>
        <DropdownMenuItem disabled={!hasTrack} onSelect={() => usePlayer.getState().setNowPlayingOpen(true)}>
          <Maximize2 strokeWidth={1.75} />
          {t('expand')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
