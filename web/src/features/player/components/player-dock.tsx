import { ChevronDown, ChevronUp } from 'lucide-react'
import { AnimatePresence } from 'motion/react'
import { useEffect, useRef, useState, type FocusEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import { usePlayerDock } from '../dock'
import { dockAttribute } from '../lib/dock'
import { useCurrentTrack, usePlayer } from '../store'
import { CompactPlayer } from './compact-player'
import { PlayerBar } from './player-bar'
import { PlayerWindow } from './player-window'

/** How long the auto-hiding bar stays up after the pointer / focus leaves it (ms). */
const HIDE_DELAY = 500

/**
 * Tablet / desktop player chrome (docs/architecture/contract.md §9.4), mounted once by the shell:
 * the bottom bar (optionally auto-hiding), the floating window or the floating mini bar, as
 * chosen in the layout menu (`usePlayerDock`). Full-screen Now Playing covers all of them.
 *
 * Publishes `<html data-player-dock>` so `index.css` can size the space the chrome reserves
 * (`--player-reserve`, `--player-clearance`, toast offsets) for the sidebar, `.page-pad` & co.
 */
export function PlayerDock() {
  const track = useCurrentTrack()
  const mode = usePlayerDock((s) => s.mode)
  const barAutoHide = usePlayerDock((s) => s.barAutoHide)
  const nowPlayingOpen = usePlayer((s) => s.nowPlayingOpen)
  const attribute = dockAttribute({ mode, barAutoHide }, !!track)

  useEffect(() => {
    const root = document.documentElement
    root.dataset.playerDock = attribute
    return () => {
      delete root.dataset.playerDock
    }
  }, [attribute])

  if (mode === 'bar') return <BarSlot autoHide={barAutoHide} />

  return (
    <AnimatePresence>
      {track && !nowPlayingOpen ? (
        mode === 'window' ? (
          <PlayerWindow key="window" track={track} />
        ) : (
          <CompactPlayer key="compact" track={track} />
        )
      ) : null}
    </AnimatePresence>
  )
}

/**
 * Bottom bar slot. With auto-hide on (NetEase style) the bar slides below the viewport and only
 * a handle stays; hovering the handle or the bottom edge, or tabbing into the bar, peeks it up
 * until the pointer / focus leaves. The handle pins it open (and, on a pinned bar, turns
 * auto-hide back on).
 */
function BarSlot({ autoHide }: { autoHide: boolean }) {
  const { t } = useTranslation('player')
  const setBarAutoHide = usePlayerDock((s) => s.setBarAutoHide)
  const slotRef = useRef<HTMLDivElement>(null)
  const menuOpen = useRef(false)
  const hideTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const [peek, setPeek] = useState(false)
  const hidden = autoHide && !peek

  useEffect(() => () => clearTimeout(hideTimer.current), [])

  const show = () => {
    clearTimeout(hideTimer.current)
    setPeek(true)
  }

  const scheduleHide = () => {
    clearTimeout(hideTimer.current)
    hideTimer.current = setTimeout(() => {
      const slot = slotRef.current
      // Still in use: the next leave / blur / menu close schedules again.
      if (slot && (menuOpen.current || slot.matches(':hover') || slot.querySelector(':focus-visible'))) return
      setPeek(false)
    }, HIDE_DELAY)
  }

  const onFocus = (event: FocusEvent) => {
    // Keyboard focus reveals the bar; a mouse click inside it does not keep it up.
    if (event.target instanceof Element && event.target.matches(':focus-visible')) show()
  }

  const onMenuOpenChange = (open: boolean) => {
    menuOpen.current = open
    if (open) show()
    else scheduleHide()
  }

  const toggleAutoHide = () => {
    clearTimeout(hideTimer.current)
    setPeek(false)
    setBarAutoHide(!autoHide)
  }

  const handleLabel = autoHide ? t('dock.pinBar') : t('dock.hideBar')

  return (
    <div
      ref={slotRef}
      data-slot="player-bar"
      className="pointer-events-none fixed inset-x-0 bottom-0 z-40"
      onPointerEnter={autoHide ? show : undefined}
      onPointerLeave={autoHide ? scheduleHide : undefined}
      onFocus={autoHide ? onFocus : undefined}
      onBlur={autoHide ? scheduleHide : undefined}
    >
      {/* Bottom-edge hot zone that peeks the hidden bar. */}
      {autoHide ? <div aria-hidden className="pointer-events-auto absolute inset-x-0 bottom-0 h-1.5" /> : null}
      <div
        className={cn(
          'group/dock pointer-events-auto relative transition-transform duration-300 ease-out motion-reduce:transition-none',
          hidden && 'translate-y-full',
        )}
      >
        <button
          type="button"
          onClick={toggleAutoHide}
          aria-label={handleLabel}
          title={handleLabel}
          aria-pressed={!autoHide}
          className={cn(
            'glass absolute bottom-full left-1/2 grid h-5 w-14 -translate-x-1/2 place-items-center rounded-t-lg border border-b-0 border-border/60 text-muted-foreground transition-[opacity,color] outline-none',
            // Larger hit area than the visible tab.
            'before:absolute before:-inset-x-3 before:-top-3 before:bottom-0 before:content-[""]',
            'hover:text-foreground focus-visible:text-foreground focus-visible:opacity-100 focus-visible:ring-2 focus-visible:ring-ring/50',
            autoHide ? 'opacity-100' : 'opacity-0 group-hover/dock:opacity-100',
          )}
        >
          {autoHide ? <ChevronUp className="size-4" strokeWidth={2} /> : <ChevronDown className="size-4" strokeWidth={2} />}
        </button>
        <PlayerBar onMenuOpenChange={onMenuOpenChange} />
      </div>
    </div>
  )
}
