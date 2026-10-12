import { ChevronDown, ChevronUp, Pin } from 'lucide-react'
import { AnimatePresence } from 'motion/react'
import { useEffect, useRef, useState, type FocusEvent, type PointerEvent as ReactPointerEvent } from 'react'
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

const HANDLE_BUTTON =
  'relative grid h-full w-9 place-items-center transition-colors outline-none hover:text-foreground focus-visible:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50 before:absolute before:-top-3 before:bottom-0 before:content-[""]'

/**
 * Tablet / desktop player chrome (docs/architecture/contract.md §9.4), mounted once by the shell:
 * the bottom bar (pinned or auto-hiding), the floating window or the floating mini bar, as
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
 * Bottom bar slot. Its handle (a tab above the bar) holds a collapse / expand chevron and a pin.
 * Unpinned (auto-hide, NetEase style) the bar slides below the viewport and only the handle
 * stays; hovering the handle or the bottom edge, or tabbing into the bar, peeks it up until the
 * pointer / focus leaves. The chevron hides or shows the bar right away (hiding a pinned bar
 * unpins it); the pin switches between staying open and auto-hiding.
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

  // Touch has no hover to end the peek: pressing anywhere else hides the bar again.
  useEffect(() => {
    if (!autoHide || !peek) return
    const onPointerDown = (event: PointerEvent) => {
      if (menuOpen.current || (event.target instanceof Node && slotRef.current?.contains(event.target))) return
      setPeek(false)
    }
    document.addEventListener('pointerdown', onPointerDown, true)
    return () => document.removeEventListener('pointerdown', onPointerDown, true)
  }, [autoHide, peek])

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

  // A touch is not a hover: it would reveal the bar under the finger and hide it on release.
  const onPointerEnter = (event: ReactPointerEvent) => {
    if (event.pointerType !== 'touch') show()
  }

  const onPointerLeave = (event: ReactPointerEvent) => {
    if (event.pointerType !== 'touch') scheduleHide()
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

  const toggleBar = () => {
    if (hidden) {
      show()
      return
    }
    clearTimeout(hideTimer.current)
    setPeek(false)
    if (!autoHide) setBarAutoHide(true)
  }

  const togglePin = () => {
    clearTimeout(hideTimer.current)
    // A bar that was just unpinned stays up until the pointer or focus leaves it.
    setPeek(!autoHide)
    setBarAutoHide(!autoHide)
  }

  const barLabel = hidden ? t('dock.showBar') : t('dock.hideBar')
  const pinLabel = autoHide ? t('dock.pinBar') : t('dock.unpinBar')

  return (
    <div
      ref={slotRef}
      data-slot="player-bar"
      className="pointer-events-none fixed inset-x-0 bottom-0 z-40"
      onPointerEnter={autoHide ? onPointerEnter : undefined}
      onPointerLeave={autoHide ? onPointerLeave : undefined}
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
        <div
          className={cn(
            'glass absolute bottom-full left-1/2 flex h-6 -translate-x-1/2 rounded-t-lg border border-b-0 border-border/60 text-muted-foreground transition-opacity',
            // A pinned bar shows its handle on hover / focus; without hover it is always there.
            autoHide
              ? 'opacity-100'
              : 'opacity-0 group-hover/dock:opacity-100 has-[:focus-visible]:opacity-100 [@media(hover:none)]:opacity-100',
          )}
        >
          {/* Both buttons reach past the visible tab for a larger hit area. */}
          <button
            type="button"
            onClick={toggleBar}
            aria-label={barLabel}
            title={barLabel}
            className={cn(HANDLE_BUTTON, 'rounded-tl-lg before:right-0 before:-left-3')}
          >
            {hidden ? <ChevronUp className="size-4" strokeWidth={2} /> : <ChevronDown className="size-4" strokeWidth={2} />}
          </button>
          <button
            type="button"
            onClick={togglePin}
            aria-label={pinLabel}
            title={pinLabel}
            className={cn(HANDLE_BUTTON, 'rounded-tr-lg before:-right-3 before:left-0', !autoHide && 'text-foreground')}
          >
            <Pin className={cn('size-3.5 transition-transform', autoHide ? 'rotate-45' : 'fill-current')} strokeWidth={2} />
          </button>
        </div>
        <PlayerBar onMenuOpenChange={onMenuOpenChange} />
      </div>
    </div>
  )
}
