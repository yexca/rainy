import { ListMusic, Maximize2, MessageSquareQuote, Music, Radio } from 'lucide-react'
import { useCallback, useEffect, useRef, useState, type PointerEvent as ReactPointerEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { CoverArt } from '@/components/cover-art'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { TrackActionsMenu } from '@/features/library/components/track-actions'
import { formatDuration } from '@/lib/format'
import { cn } from '@/lib/utils'

import { usePlayerDock } from '../dock'
import { seekByKey } from '../hooks/seek-by-key'
import { useSliderDrag } from '../hooks/use-slider-drag'
import { pointerRatio } from '../lib/dock'
import { useCurrentTrack, usePlayback, usePlayer, type PlaybackState } from '../store'
import type { PlayableTrack, PlayerPanel } from '../types'
import { DockModeMenu } from './dock-mode-menu'
import { LiveBadge } from './live-badge'
import { Marquee } from './now-playing-parts'
import { PipLyricsButton } from './pip-lyrics-button'
import { StarButton } from './star-button'
import { StopTimerButton } from './stop-timer-button'
import { ModeToggle, NextButton, PlayPauseButton, PrevButton, RepeatButton, ShuffleButton } from './transport'
import { BarVolume } from './volume-control'

/**
 * Desktop / tablet bottom bar (the `bar` dock mode), laid out like the Apple Music web player:
 * a floating, fully rounded glass pill (56px high, at most 880px wide, centred over the content) with the transport on the
 * left, the current song in the middle (artwork, left-aligned title over "artist — album", star
 * and "…", the progress line underneath across the whole middle) and the toggles on the right.
 * The artwork opens full-screen Now Playing and the title opens the floating window; the
 * progress seeks to where the pointer is, with a time preview while hovering. Rendered by
 * `<PlayerDock/>` inside a `fixed inset-x-0 bottom-0` slot that adds the `--playerbar-gap`
 * margins and safe areas; owns its height (`--playerbar-h`) and always renders (idle state when
 * empty). `onMenuOpenChange` lets the auto-hiding slot stay up while the layout menu, the
 * auto-stop timer panel or the "…" menu is open.
 */
export function PlayerBar({ onMenuOpenChange }: { onMenuOpenChange?: (open: boolean) => void }) {
  const { t } = useTranslation('player')
  const track = useCurrentTrack()
  const panel = usePlayer((s) => s.panel)
  const setPanel = usePlayer((s) => s.setPanel)
  const togglePanel = (p: PlayerPanel) => setPanel(panel === p ? 'none' : p)

  return (
    <div
      role="region"
      aria-label={t('region')}
      className="ui-chrome glass mx-auto grid h-(--playerbar-h) w-full max-w-[880px] grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-3 rounded-full border border-border/60 px-4 shadow-xl shadow-black/10 lg:gap-5 dark:shadow-black/40"
    >
      <div className="flex items-center gap-0.5 lg:gap-1.5">
        {/* From `lg`: the narrower bar keeps them in full-screen Now Playing and the window. */}
        <ShuffleButton tone="bar" className="hidden size-8 lg:grid" />
        <PrevButton tone="bar" size="sm" />
        <PlayPauseButton tone="bar" size="md" />
        <NextButton tone="bar" size="sm" />
        <RepeatButton tone="bar" className="hidden size-8 lg:grid" />
      </div>

      <BarNowPlaying track={track} onMenuOpenChange={onMenuOpenChange} />

      <div className="flex items-center justify-end gap-0.5">
        <BarToggle
          label={t('lyrics.title')}
          active={panel === 'lyrics'}
          disabled={!track}
          onClick={() => togglePanel('lyrics')}
        >
          <MessageSquareQuote className="size-[18px]" strokeWidth={1.9} />
        </BarToggle>
        <PipLyricsButton tone="bar" />
        <BarToggle label={t('queue.title')} active={panel === 'queue'} onClick={() => togglePanel('queue')}>
          <ListMusic className="size-[18px]" strokeWidth={1.9} />
        </BarToggle>
        {/* From `lg`: the narrower bar has no room for it (full-screen Now Playing has one). */}
        <StopTimerButton tone="bar" className="hidden lg:grid" onOpenChange={onMenuOpenChange} />
        <BarVolume className="ml-1" />
        <DockModeMenu tone="bar" onOpenChange={onMenuOpenChange} />
      </div>
    </div>
  )
}

function BarToggle({
  label,
  active,
  disabled,
  onClick,
  children,
}: {
  label: string
  active: boolean
  disabled?: boolean
  onClick: () => void
  children: ReactNode
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <ModeToggle tone="bar" active={active} disabled={disabled} onClick={onClick} aria-label={label} className="size-8 rounded-md">
          {children}
        </ModeToggle>
      </TooltipTrigger>
      <TooltipContent side="top">{label}</TooltipContent>
    </Tooltip>
  )
}

/** Middle of the bar: artwork → full screen, title → floating window, progress underneath. */
function BarNowPlaying({
  track,
  onMenuOpenChange,
}: {
  track: PlayableTrack | undefined
  onMenuOpenChange?: (open: boolean) => void
}) {
  const { t } = useTranslation('player')
  const setNowPlayingOpen = usePlayer((s) => s.setNowPlayingOpen)
  const setMode = usePlayerDock((s) => s.setMode)

  if (!track) {
    return (
      <div className="flex h-full min-w-0 flex-col pt-2">
        <div className="flex h-[34px] min-w-0 items-center gap-2">
          <div className="grid size-[34px] shrink-0 place-items-center rounded-md bg-muted text-muted-foreground/60">
            <Music className="size-4" strokeWidth={1.5} />
          </div>
          <div className="min-w-0 leading-tight">
            <p className="truncate text-[13px] font-medium text-muted-foreground">{t('idle.title')}</p>
            <p className="truncate text-xs text-muted-foreground/70">{t('idle.hint')}</p>
          </div>
        </div>
        <div aria-hidden className="h-3 py-1">
          <div className="h-1 rounded-full bg-foreground/10" />
        </div>
      </div>
    )
  }

  return (
    <div className="group/np flex h-full min-w-0 flex-col pt-2">
      <div className="flex h-[34px] min-w-0 items-center gap-2">
        <Tooltip>
          <TooltipTrigger asChild>
            <button
              type="button"
              onClick={() => setNowPlayingOpen(true)}
              aria-label={t('openNowPlaying')}
              className="group/art relative shrink-0 rounded-md outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
            >
              <CoverArt coverArt={track.coverArt} size={34} rounded="md" keepPrevious icon={track.isRadio ? Radio : undefined} />
              <span className="absolute inset-0 grid place-items-center rounded-md bg-black/40 text-white opacity-0 transition-opacity group-hover/art:opacity-100 group-focus-visible/art:opacity-100">
                <Maximize2 className="size-3.5" />
              </span>
            </button>
          </TooltipTrigger>
          <TooltipContent side="top">{t('expand')}</TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger asChild>
            <button
              type="button"
              onClick={() => setMode('window')}
              aria-label={t('dock.openWindow')}
              className="min-w-0 flex-1 rounded-md text-left leading-tight outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
            >
              <Marquee className="text-[13px] font-medium">{track.title}</Marquee>
              {track.isRadio ? (
                <span className="flex min-w-0 items-center gap-1.5 text-xs font-medium text-muted-foreground">
                  <LiveBadge tone="bar" className="px-1.5 py-0 text-[9px]" />
                  <span className="truncate">{t('live.radio')}</span>
                </span>
              ) : (
                <Marquee className="text-xs font-medium text-muted-foreground">
                  {track.album ? `${track.artist} — ${track.album}` : track.artist}
                </Marquee>
              )}
            </button>
          </TooltipTrigger>
          <TooltipContent side="top">{t('dock.openWindow')}</TooltipContent>
        </Tooltip>

        {track.isRadio ? null : <BarTimes />}
        <StarButton track={track} tone="bar" className="hidden lg:grid" />
        {track.isRadio ? null : (
          <TrackActionsMenu tracks={[track]} onOpenChange={onMenuOpenChange} className="size-8 rounded-full" />
        )}
      </div>
      {track.isRadio ? (
        <div aria-hidden className="h-3 py-1">
          <div className="h-1 rounded-full bg-foreground/10" />
        </div>
      ) : (
        <BarProgress />
      )}
    </div>
  )
}

/** Elapsed / −remaining beside the title, shown while the middle is hovered or the progress has focus. */
function BarTimes() {
  const duration = usePlayback((s) => s.duration)
  const seconds = usePlayback((s) => Math.floor(s.currentTime))
  if (!(duration > 0)) return null
  return (
    <span
      aria-hidden
      className="tnum shrink-0 text-[11px] text-muted-foreground opacity-0 transition-opacity group-hover/np:opacity-100 group-has-[[role=slider]:focus-visible]/np:opacity-100"
    >
      {formatDuration(seconds)} / −{formatDuration(Math.max(0, duration - seconds))}
    </span>
  )
}

/**
 * Progress line under the song. The mouse seeks to the position under the pointer (press, or
 * press and drag) and hovering previews that time; touch drags move relative to where the
 * finger went down, as everywhere else. The fill is written straight to the DOM from the
 * playback store (no React render per `timeupdate`).
 */
function BarProgress() {
  const { t } = useTranslation('player')
  const duration = usePlayback((s) => s.duration)
  const seconds = usePlayback((s) => Math.floor(s.currentTime))
  const [drag, setDrag] = useState<number | null>(null)
  const [hover, setHover] = useState<number | null>(null)
  const dragRef = useRef<number | null>(null)
  const fillRef = useRef<HTMLDivElement>(null)
  const bufferRef = useRef<HTMLDivElement>(null)
  const inactive = !(duration > 0)

  const paint = useCallback((ratio: number, buffered?: number) => {
    const r = Number.isFinite(ratio) ? Math.min(1, Math.max(0, ratio)) : 0
    if (fillRef.current) fillRef.current.style.transform = `scaleX(${r})`
    if (buffered !== undefined && bufferRef.current) {
      const b = Number.isFinite(buffered) ? Math.min(1, Math.max(0, buffered)) : 0
      bufferRef.current.style.transform = `scaleX(${b})`
    }
  }, [])

  useEffect(() => {
    const apply = (s: PlaybackState) => {
      const buffered = s.duration > 0 ? s.buffered / s.duration : 0
      paint(dragRef.current ?? (s.duration > 0 ? s.currentTime / s.duration : 0), buffered)
    }
    apply(usePlayback.getState())
    return usePlayback.subscribe(apply)
  }, [paint])

  const setDragging = (value: number | null) => {
    dragRef.current = value
    setDrag(value)
    if (value !== null) paint(value)
  }

  const handlers = useSliderDrag({
    disabled: inactive,
    getValue: () => {
      const s = usePlayback.getState()
      return s.duration > 0 ? s.currentTime / s.duration : 0
    },
    onStart: setDragging,
    onMove: setDragging,
    onEnd: (ratio, committed) => {
      setDragging(null)
      const s = usePlayback.getState()
      if (committed && s.duration > 0) s.seek(ratio * s.duration)
      else paint(s.duration > 0 ? s.currentTime / s.duration : 0)
    },
  })

  const onPointerMove = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (event.pointerType !== 'touch' && !inactive) {
      const rect = event.currentTarget.getBoundingClientRect()
      setHover(pointerRatio(event.clientX, rect.left, rect.width))
    }
    handlers.onPointerMove(event)
  }

  const dragging = drag !== null
  const shown = dragging ? drag * duration : seconds
  const elapsed = formatDuration(shown)
  const total = formatDuration(duration)
  const preview = dragging ? drag : hover

  return (
    <div
      role="slider"
      tabIndex={inactive ? -1 : 0}
      aria-label={t('seek')}
      aria-valuemin={0}
      aria-valuemax={Math.round(duration)}
      aria-valuenow={Math.round(shown)}
      aria-valuetext={t('position', { elapsed, total })}
      aria-disabled={inactive || undefined}
      onKeyDown={(event) => {
        if (!inactive) seekByKey(event)
      }}
      {...handlers}
      onPointerMove={onPointerMove}
      onPointerLeave={() => setHover(null)}
      className={cn(
        'group/progress relative h-3 w-full touch-none rounded-full outline-none select-none focus-visible:ring-2 focus-visible:ring-ring/60',
        inactive ? 'cursor-default' : 'cursor-pointer',
      )}
    >
      <div
        className={cn(
          'absolute inset-x-0 top-1/2 -mt-[3px] h-1.5 overflow-hidden rounded-full bg-foreground/10 transition-transform duration-150 ease-out',
          dragging ? 'scale-y-100' : 'scale-y-[0.67] group-hover/progress:scale-y-100',
        )}
      >
        <div ref={bufferRef} className="absolute inset-0 origin-left bg-foreground/10" style={{ transform: 'scaleX(0)' }} />
        {preview !== null && !dragging ? (
          <div className="absolute inset-y-0 left-0 bg-foreground/15" style={{ width: `${preview * 100}%` }} />
        ) : null}
        <div
          ref={fillRef}
          className={cn(
            'absolute inset-0 origin-left',
            dragging ? 'bg-primary' : 'bg-foreground/60 group-hover/progress:bg-primary',
          )}
          style={{ transform: 'scaleX(0)' }}
        />
      </div>
      {preview !== null ? (
        <span
          aria-hidden
          className="tnum pointer-events-none absolute bottom-full mb-1 -translate-x-1/2 rounded-md bg-popover px-1.5 py-0.5 text-[10px] leading-none font-medium whitespace-nowrap text-popover-foreground shadow-md ring-1 ring-border"
          style={{ left: `clamp(1.25rem, ${preview * 100}%, calc(100% - 1.25rem))` }}
        >
          {formatDuration(preview * duration)}
        </span>
      ) : null}
    </div>
  )
}
