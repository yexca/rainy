import { PictureInPicture, Settings2, Timer } from 'lucide-react'
import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Popover, PopoverAnchor, PopoverTrigger } from '@/components/ui/popover'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

import { STOP_TOAST_ID } from '../lib/stop-timer'
import { pipLyricsSupport, togglePipLyrics, usePipLyrics } from '../pip/pip-lyrics'
import { useStopTimer } from '../stop-timer'
import { useCurrentTrack } from '../store'
import { DockModeItems } from './dock-mode-menu'
import { TimerPanelContent } from './stop-timer-button'
import { ModeToggle } from './transport'

/** After a menu or panel closes, ignore the tooltip opening from the focus it returns (ms). */
const TOOLTIP_QUIET = 400

/** Tooltip state that stays shut for a moment after `hush()`, so returned focus does not open it. */
function useQuietTooltip() {
  const [open, setOpen] = useState(false)
  const quietUntil = useRef(0)
  return {
    open,
    onOpenChange: (next: boolean) => setOpen(next && Date.now() >= quietUntil.current),
    hush: () => {
      setOpen(false)
      quietUntil.current = Date.now() + TOOLTIP_QUIET
    },
  }
}

/**
 * Right end of the bottom bar: the "Player options" menu (layout, picture-in-picture lyrics,
 * auto-stop timer) and, while picture-in-picture lyrics or a timer is on, a tinted button for
 * each beside it, so a running mode stays in view and one press away. The timer panel opens
 * above this group, from the menu or from the timer button. `onMenuOpenChange` keeps the
 * auto-hiding bar up while the menu or the panel is open.
 */
export function BarOptions({ onMenuOpenChange }: { onMenuOpenChange?: (open: boolean) => void }) {
  const { t } = useTranslation('player')
  const [pipSupported] = useState(() => pipLyricsSupport() !== null)
  const pipOpen = usePipLyrics((s) => s.mode !== null)
  const timerOn = useStopTimer((s) => s.mode !== null)
  const hasTrack = !!useCurrentTrack()
  const [panelOpen, setPanelOpen] = useState(false)
  const menuButton = useRef<HTMLButtonElement>(null)
  /** The timer item was picked: the panel opens once the menu has closed. */
  const panelPending = useRef(false)
  /** The panel was opened from the menu, so focus goes back to the menu button. */
  const panelFromMenu = useRef(false)
  const interactedOutside = useRef(false)
  const menuTip = useQuietTooltip()
  const timerTip = useQuietTooltip()
  const pipTip = useQuietTooltip()
  const label = t('options')
  const timerLabel = t('timer.title')

  const focusMenuButton = () => {
    menuTip.hush()
    menuButton.current?.focus()
  }

  const onMenuChange = (open: boolean) => {
    menuTip.hush()
    // The timer panel takes over from the menu, so the bar stays up in between.
    if (!open && panelPending.current) return
    onMenuOpenChange?.(open)
  }

  const openPanel = (fromMenu: boolean) => {
    panelFromMenu.current = fromMenu
    // The "stopped playback" message sits where the panel opens, and the panel says more.
    toast.dismiss(STOP_TOAST_ID)
    setPanelOpen(true)
  }

  const onPanelChange = (open: boolean) => {
    timerTip.hush()
    if (open) openPanel(false)
    else setPanelOpen(false)
    onMenuOpenChange?.(open)
  }

  return (
    <Popover open={panelOpen} onOpenChange={onPanelChange}>
      <PopoverAnchor asChild>
        <div className="flex items-center gap-0.5">
          {pipOpen ? (
            <Tooltip open={pipTip.open} onOpenChange={pipTip.onOpenChange}>
              <TooltipTrigger asChild>
                <ModeToggle
                  tone="bar"
                  active
                  aria-label={t('pip.close')}
                  className="size-8 rounded-md"
                  onClick={() => {
                    togglePipLyrics()
                    // This button goes away with the window.
                    focusMenuButton()
                  }}
                >
                  <PictureInPicture className="size-[18px]" strokeWidth={1.9} />
                </ModeToggle>
              </TooltipTrigger>
              <TooltipContent side="top">{t('pip.close')}</TooltipContent>
            </Tooltip>
          ) : null}

          {timerOn ? (
            <Tooltip open={timerTip.open && !panelOpen} onOpenChange={timerTip.onOpenChange}>
              <TooltipTrigger asChild>
                <PopoverTrigger asChild>
                  <ModeToggle tone="bar" active aria-label={timerLabel} className="size-8 rounded-md">
                    <Timer className="size-[18px]" strokeWidth={1.9} />
                  </ModeToggle>
                </PopoverTrigger>
              </TooltipTrigger>
              <TooltipContent side="top">{timerLabel}</TooltipContent>
            </Tooltip>
          ) : null}

          <DropdownMenu onOpenChange={onMenuChange}>
            <Tooltip open={menuTip.open} onOpenChange={menuTip.onOpenChange}>
              <TooltipTrigger asChild>
                <DropdownMenuTrigger asChild>
                  <ModeToggle ref={menuButton} tone="bar" active={false} aria-label={label} className="size-8 rounded-md">
                    <Settings2 className="size-[18px]" strokeWidth={1.9} />
                  </ModeToggle>
                </DropdownMenuTrigger>
              </TooltipTrigger>
              <TooltipContent side="top">{label}</TooltipContent>
            </Tooltip>
            <DropdownMenuContent
              side="top"
              align="end"
              className="min-w-56"
              onCloseAutoFocus={(event) => {
                if (!panelPending.current) return
                // Open the panel only now: the menu returning focus to its button would close it.
                event.preventDefault()
                panelPending.current = false
                openPanel(true)
              }}
            >
              <DockModeItems />
              <DropdownMenuSeparator />
              {pipSupported ? (
                <DropdownMenuCheckboxItem
                  checked={pipOpen}
                  disabled={!hasTrack && !pipOpen}
                  onCheckedChange={() => togglePipLyrics()}
                >
                  <PictureInPicture className="text-muted-foreground" strokeWidth={1.75} />
                  {t('pip.open')}
                </DropdownMenuCheckboxItem>
              ) : null}
              <DropdownMenuItem
                inset
                aria-haspopup="dialog"
                onSelect={() => {
                  panelPending.current = true
                }}
              >
                <Timer strokeWidth={1.75} />
                {timerLabel}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </PopoverAnchor>
      <TimerPanelContent
        side="top"
        onInteractOutside={() => {
          interactedOutside.current = true
        }}
        onCloseAutoFocus={(event) => {
          const outside = interactedOutside.current
          interactedOutside.current = false
          // Radix returns focus to the timer button; with none to return to (opened from the
          // menu, or the timer was just turned off) the menu button takes it, unless the user
          // clicked elsewhere.
          if (!panelFromMenu.current && timerOn) return
          event.preventDefault()
          if (!outside) focusMenuButton()
        }}
      />
    </Popover>
  )
}
