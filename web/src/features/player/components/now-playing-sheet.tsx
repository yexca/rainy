import { ChevronDown, ListMusic, MessageSquareQuote, X } from 'lucide-react'
import { AnimatePresence, motion, useDragControls, useMotionValue, useTransform, type PanInfo } from 'motion/react'
import { useEffect, useRef, type PointerEvent as ReactPointerEvent, type ReactNode, type RefObject } from 'react'
import { useTranslation } from 'react-i18next'
import { useLocation } from 'react-router'

import { useIsMobile, useMediaQuery } from '@/hooks/use-media-query'
import { cn } from '@/lib/utils'

import { usePlayerDock } from '../dock'
import { useCoverColor } from '../hooks/use-cover-color'
import { useCurrentTrack, usePlayer } from '../store'
import type { PlayableTrack, PlayerPanel } from '../types'
import { LiveProgress } from './live-badge'
import { LyricsView } from './lyrics-view'
import { NowPlayingBackground } from './now-playing-background'
import { AirPlayButton, NowPlayingActions, NowPlayingArtwork, NowPlayingTitle } from './now-playing-parts'
import { QueueView } from './queue-view'
import { Scrubber } from './scrubber'
import { TranslationToggle } from './translation-toggle'
import { NextButton, PlayPauseButton, PrevButton, RepeatButton, ShuffleButton } from './transport'
import { SheetVolume } from './volume-control'

const SHEET_SPRING = { type: 'spring', stiffness: 380, damping: 40, mass: 0.9 } as const
/** Drag distance / velocity that dismisses the sheet. */
const DISMISS_OFFSET = 120
const DISMISS_VELOCITY = 500
const LANDSCAPE_PHONE = '(orientation: landscape) and (max-height: 540px)'
/** Desktop queue / lyrics side panel width (docs/architecture/contract.md §9.4; keep in sync with `w-[360px]`). */
const SIDE_PANEL_WIDTH = 360

/**
 * Now Playing (docs/architecture/contract.md §9.4): full-screen sheet on phones (spring slide-up, drag down to
 * dismiss), full-window "stage" on larger screens, plus the desktop side panel for queue /
 * lyrics. Mounted once by the shell; open state = `usePlayer().nowPlayingOpen`.
 */
export function NowPlayingSheet() {
  const isMobile = useIsMobile()
  // Phones in landscape are wider than the mobile breakpoint but still get the sheet.
  const landscapePhone = useMediaQuery(LANDSCAPE_PHONE)
  const sheet = isMobile || landscapePhone
  const open = usePlayer((s) => s.nowPlayingOpen)
  const track = useCurrentTrack()
  const setNowPlayingOpen = usePlayer((s) => s.setNowPlayingOpen)
  const visible = open && !!track

  useCloseOnNavigate()

  // Nothing to show any more (queue cleared): reset the flag.
  useEffect(() => {
    if (open && !track) setNowPlayingOpen(false)
  }, [open, track, setNowPlayingOpen])

  return (
    <>
      <AnimatePresence>
        {visible && track ? (
          sheet ? (
            <MobileNowPlaying key="sheet" track={track} />
          ) : (
            <StageNowPlaying key="stage" track={track} />
          )
        ) : null}
      </AnimatePresence>
      {!isMobile ? <SidePanel hidden={visible} /> : null}
    </>
  )
}

function close() {
  usePlayer.getState().setNowPlayingOpen(false)
}

/** Close Now Playing whenever the route changes (e.g. "Go to album" from the menu). */
function useCloseOnNavigate() {
  const { pathname } = useLocation()
  const last = useRef(pathname)
  useEffect(() => {
    if (pathname === last.current) return
    last.current = pathname
    close()
  }, [pathname])
}

/** A Radix menu / dialog is open on top: let it handle Escape. */
function overlayOnTop(): boolean {
  return document.querySelector('[role="menu"], [role="alertdialog"], [role="dialog"][data-state="open"]') !== null
}

/**
 * Modal behaviour for the full-screen surfaces: lock page scrolling, Escape closes, focus moves
 * in (and back out on close), browser chrome takes the backdrop colour.
 */
function useModalSurface(ref: RefObject<HTMLElement | null>, chromeColor: string) {
  useEffect(() => {
    const root = document.documentElement
    const previousOverflow = root.style.overflow
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
    root.style.overflow = 'hidden'
    ref.current?.focus({ preventScroll: true })
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && !event.defaultPrevented && !overlayOnTop()) close()
    }
    window.addEventListener('keydown', onKey)
    return () => {
      root.style.overflow = previousOverflow
      window.removeEventListener('keydown', onKey)
      previousFocus?.focus({ preventScroll: true })
    }
  }, [ref])

  // Tint the browser / status bar like the backdrop while open.
  useEffect(() => {
    const metas = Array.from(document.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]'))
    const previous = metas.map((m) => m.content)
    for (const meta of metas) meta.content = chromeColor
    return () => metas.forEach((meta, i) => (meta.content = previous[i]))
  }, [chromeColor])
}

// ---------------------------------------------------------------------------------------------
// Phone: full-screen sheet
// ---------------------------------------------------------------------------------------------

function MobileNowPlaying({ track }: { track: PlayableTrack }) {
  const { t } = useTranslation('player')
  const ref = useRef<HTMLDivElement>(null)
  const controls = useDragControls()
  const y = useMotionValue(0)
  const radius = useTransform(y, [0, 80], [0, 32])
  const panel = usePlayer((s) => s.panel)
  const isPlaying = usePlayer((s) => s.isPlaying)
  const landscape = useMediaQuery(LANDSCAPE_PHONE)
  const palette = useCoverColor(track.coverArt)
  useModalSurface(ref, palette.top)
  const offscreen = typeof window === 'undefined' ? 1000 : window.innerHeight + 40

  const startDrag = (event: ReactPointerEvent) => {
    if (event.pointerType === 'mouse' && event.button !== 0) return
    // Buttons inside the drag areas keep working: drags only start after movement.
    controls.start(event)
  }

  const onDragEnd = (_: unknown, info: PanInfo) => {
    if (info.offset.y > DISMISS_OFFSET || info.velocity.y > DISMISS_VELOCITY) close()
  }

  const controlsBlock = (
    <div className="shrink-0">
      {track.isRadio ? <LiveProgress tone="sheet" /> : <Scrubber tone="sheet" />}
      <div className={cn('flex items-center justify-evenly', landscape ? 'my-1' : 'my-3')}>
        <PrevButton tone="sheet" />
        <PlayPauseButton tone="sheet" />
        <NextButton tone="sheet" />
      </div>
      <SheetVolume className={landscape ? 'mb-1' : 'mb-5'} />
      <NowPlayingActions track={track} className="-mx-1.5" />
    </div>
  )

  return (
    <motion.div
      ref={ref}
      role="dialog"
      aria-modal="true"
      aria-label={t('nowPlaying')}
      tabIndex={-1}
      className="ui-chrome fixed inset-0 z-50 flex touch-none flex-col overflow-hidden text-white outline-none"
      style={{ y, borderTopLeftRadius: radius, borderTopRightRadius: radius }}
      // Pixels (not %) so the drag offset and the corner radius share one numeric motion value.
      initial={{ y: offscreen }}
      animate={{ y: 0 }}
      exit={{ y: offscreen }}
      transition={SHEET_SPRING}
      drag="y"
      dragControls={controls}
      dragListener={false}
      dragConstraints={{ top: 0, bottom: 0 }}
      dragElastic={{ top: 0, bottom: 1 }}
      dragMomentum={false}
      onDragEnd={onDragEnd}
    >
      <NowPlayingBackground coverArt={track.coverArt} playing={isPlaying} />

      <div className="relative flex min-h-0 flex-1 flex-col pt-safe pr-safe pb-safe pl-safe">
        {/* Grabber */}
        <div className="flex h-8 shrink-0 items-start justify-center pt-2" onPointerDown={startDrag}>
          <button
            type="button"
            onClick={close}
            aria-label={t('close')}
            className="grid h-6 w-20 place-items-center rounded-full outline-none focus-visible:ring-2 focus-visible:ring-white/60"
          >
            <span className="h-[5px] w-9 rounded-full bg-white/45" />
          </button>
        </div>

        {landscape ? (
          <div className="flex min-h-0 flex-1 gap-6 px-6 pb-2">
            <div className="flex min-h-0 w-[44%] shrink-0 flex-col">
              {panel === 'none' ? (
                <div className="flex min-h-0 flex-1 items-center justify-center [container-type:size]" onPointerDown={startDrag}>
                  <NowPlayingArtwork track={track} playing={isPlaying} className="w-[min(100cqw,100cqh)]" />
                </div>
              ) : (
                <PanelContent panel={panel} track={track} tone="sheet" />
              )}
            </div>
            <div className="flex min-w-0 flex-1 flex-col justify-center gap-2">
              <NowPlayingTitle track={track} size="sm" onPointerDown={startDrag} />
              {controlsBlock}
            </div>
          </div>
        ) : (
          <>
            {panel === 'none' ? (
              <div className="flex min-h-0 flex-1 items-center justify-center px-7 py-4 [container-type:size]" onPointerDown={startDrag}>
                <NowPlayingArtwork track={track} playing={isPlaying} className="w-[min(100cqw,100cqh)]" />
              </div>
            ) : (
              <div className="flex min-h-0 flex-1 flex-col">
                <CompactHeader track={track} onPointerDown={startDrag} />
                <PanelContent panel={panel} track={track} tone="sheet" />
              </div>
            )}
            <div className="shrink-0 px-7 pb-2">
              {panel === 'none' ? <NowPlayingTitle track={track} className="mb-3" onPointerDown={startDrag} /> : null}
              {controlsBlock}
            </div>
          </>
        )}
      </div>
    </motion.div>
  )
}

function CompactHeader({ track, onPointerDown }: { track: PlayableTrack; onPointerDown: (event: ReactPointerEvent) => void }) {
  return (
    <div className="flex shrink-0 items-center gap-3 px-6 pt-2 pb-3" onPointerDown={onPointerDown}>
      <motion.div initial={{ scale: 0.8, opacity: 0 }} animate={{ scale: 1, opacity: 1 }} transition={{ type: 'spring', stiffness: 400, damping: 30 }}>
        <NowPlayingArtworkThumb track={track} />
      </motion.div>
      <NowPlayingTitle track={track} size="sm" className="min-w-0 flex-1" />
    </div>
  )
}

function NowPlayingArtworkThumb({ track }: { track: PlayableTrack }) {
  return <NowPlayingArtwork track={track} playing className="w-16" />
}

function PanelContent({ panel, track, tone }: { panel: PlayerPanel; track: PlayableTrack; tone: 'sheet' | 'stage' }) {
  return (
    <div className="relative min-h-0 flex-1">
      <AnimatePresence mode="wait" initial={false}>
        <motion.div
          key={panel}
          className="absolute inset-0"
          initial={{ opacity: 0, y: 12 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: -8 }}
          transition={{ duration: 0.18 }}
        >
          {panel === 'lyrics' ? <LyricsView track={track} tone={tone} /> : <QueueView tone={tone} />}
        </motion.div>
      </AnimatePresence>
    </div>
  )
}

// ---------------------------------------------------------------------------------------------
// Tablet / desktop: full-window stage
// ---------------------------------------------------------------------------------------------

function StageNowPlaying({ track }: { track: PlayableTrack }) {
  const { t } = useTranslation('player')
  const ref = useRef<HTMLDivElement>(null)
  const panel = usePlayer((s) => s.panel)
  const setPanel = usePlayer((s) => s.setPanel)
  const isPlaying = usePlayer((s) => s.isPlaying)
  const palette = useCoverColor(track.coverArt)
  useModalSurface(ref, palette.top)
  const toggle = (p: PlayerPanel) => setPanel(panel === p ? 'none' : p)
  const split = panel !== 'none'

  return (
    <motion.div
      ref={ref}
      role="dialog"
      aria-modal="true"
      aria-label={t('nowPlaying')}
      tabIndex={-1}
      className="ui-chrome fixed inset-0 z-50 flex flex-col overflow-hidden text-white outline-none"
      initial={{ opacity: 0, y: 40 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: 40 }}
      transition={{ type: 'spring', stiffness: 420, damping: 42 }}
    >
      <NowPlayingBackground coverArt={track.coverArt} playing={isPlaying} />

      <header className="relative flex h-16 shrink-0 items-center justify-between px-5 pt-safe">
        <button
          type="button"
          onClick={close}
          aria-label={t('close')}
          className="grid size-10 place-items-center rounded-full bg-white/10 text-white transition-colors outline-none hover:bg-white/20 focus-visible:ring-2 focus-visible:ring-white/60"
        >
          <ChevronDown className="size-5" />
        </button>
        <div className="flex items-center gap-1">
          {panel === 'lyrics' ? <TranslationToggle track={track} tone="stage" /> : null}
          <StageToggle active={panel === 'lyrics'} onClick={() => toggle('lyrics')} label={t('lyrics.title')}>
            <MessageSquareQuote className="size-[18px]" strokeWidth={1.9} />
          </StageToggle>
          <StageToggle active={panel === 'queue'} onClick={() => toggle('queue')} label={t('queue.title')}>
            <ListMusic className="size-[18px]" strokeWidth={1.9} />
          </StageToggle>
        </div>
      </header>

      <div
        className={cn(
          'relative mx-auto flex min-h-0 w-full flex-1 items-center px-10 pb-10',
          split ? 'max-w-6xl gap-14' : 'max-w-xl justify-center',
        )}
      >
        <motion.div layout="position" className={cn('flex w-full flex-col', split ? 'max-w-[400px] shrink-0' : 'max-w-[440px]')}>
          <div className="mx-auto w-[min(100%,calc(100dvh-380px))] min-w-40">
            <NowPlayingArtwork track={track} playing={isPlaying} />
          </div>
          <NowPlayingTitle track={track} className="mt-7" />
          <div className="mt-4">{track.isRadio ? <LiveProgress tone="sheet" /> : <Scrubber tone="sheet" />}</div>
          <div className="mt-2 flex items-center justify-between">
            <ShuffleButton tone="sheet" className="size-10" />
            <div className="flex items-center gap-4">
              <PrevButton tone="sheet" />
              <PlayPauseButton tone="sheet" />
              <NextButton tone="sheet" />
            </div>
            <RepeatButton tone="sheet" className="size-10" />
          </div>
          <SheetVolume className="mt-4" />
          <AirPlayButton className="mx-auto mt-3" />
        </motion.div>

        {split ? (
          <div className="flex h-full min-h-0 min-w-0 flex-1 flex-col py-4">
            <PanelContent panel={panel} track={track} tone="stage" />
          </div>
        ) : null}
      </div>
    </motion.div>
  )
}

function StageToggle({ active, onClick, label, children }: { active: boolean; onClick: () => void; label: string; children: ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      aria-label={label}
      title={label}
      className={cn(
        'grid size-10 place-items-center rounded-full transition-colors outline-none focus-visible:ring-2 focus-visible:ring-white/60',
        active ? 'bg-white/25 text-white' : 'text-white/70 hover:bg-white/10 hover:text-white',
      )}
    >
      {children}
    </button>
  )
}

// ---------------------------------------------------------------------------------------------
// Desktop side panel (queue / lyrics while browsing with the bottom bar; the floating window
// shows them inside itself)
// ---------------------------------------------------------------------------------------------

function SidePanel({ hidden }: { hidden: boolean }) {
  const { t } = useTranslation('player')
  const panel = usePlayer((s) => s.panel)
  const setPanel = usePlayer((s) => s.setPanel)
  const barMode = usePlayerDock((s) => s.mode === 'bar')
  const track = useCurrentTrack()
  const show = panel !== 'none' && !hidden && barMode

  // Publish the panel width so the shell can make room for it instead of covering the page.
  useEffect(() => {
    if (!show) return
    const root = document.documentElement
    root.style.setProperty('--player-panel-w', `${SIDE_PANEL_WIDTH}px`)
    return () => {
      root.style.removeProperty('--player-panel-w')
    }
  }, [show])

  return (
    <AnimatePresence>
      {show ? (
        <motion.aside
          key="side-panel"
          aria-label={panel === 'lyrics' ? t('lyrics.title') : t('queue.title')}
          className="ui-chrome glass fixed top-0 right-0 bottom-(--player-reserve) z-35 flex w-[360px] max-w-[calc(100vw-4rem)] flex-col border-l border-border/60 pt-safe shadow-[-12px_0_32px_-24px_rgba(0,0,0,0.35)]"
          initial={{ x: '100%' }}
          animate={{ x: 0 }}
          exit={{ x: '100%' }}
          transition={{ type: 'spring', stiffness: 420, damping: 42 }}
        >
          <header className="flex h-14 shrink-0 items-center gap-2 px-3">
            <div role="tablist" aria-label={t('panel')} className="flex flex-1 rounded-lg bg-muted p-0.5">
              {(['queue', 'lyrics'] as const).map((p) => (
                <button
                  key={p}
                  type="button"
                  role="tab"
                  aria-selected={panel === p}
                  onClick={() => setPanel(p)}
                  className={cn(
                    'h-8 flex-1 rounded-md text-sm font-medium transition-[background-color,color,box-shadow] outline-none focus-visible:ring-2 focus-visible:ring-ring/50',
                    panel === p ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground',
                  )}
                >
                  {p === 'queue' ? t('queue.title') : t('lyrics.title')}
                </button>
              ))}
            </div>
            {panel === 'lyrics' ? <TranslationToggle track={track} tone="panel" /> : null}
            <button
              type="button"
              onClick={() => setPanel('none')}
              aria-label={t('closePanel')}
              className="grid size-8 place-items-center rounded-md text-muted-foreground outline-none hover:bg-accent hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50"
            >
              <X className="size-4" />
            </button>
          </header>
          <div className="min-h-0 flex-1">
            {panel === 'lyrics' ? <LyricsView track={track} tone="panel" /> : <QueueView tone="panel" />}
          </div>
        </motion.aside>
      ) : null}
    </AnimatePresence>
  )
}
