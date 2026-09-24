import { useCallback, useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { formatDuration } from '@/lib/format'
import { cn } from '@/lib/utils'

import { useSliderDrag } from '../hooks/use-slider-drag'
import { usePlayback, usePlayer, type PlaybackState } from '../store'

export type PlayerTone = 'sheet' | 'bar'

const KEY_STEP = 5
const PAGE_STEP = 30

interface ScrubberProps {
  /** `sheet`: light-on-artwork (Now Playing), times below. `bar`: themed, times at the sides. */
  tone: PlayerTone
  disabled?: boolean
  className?: string
}

/**
 * Playback position slider. The fill is written straight to the DOM from the playback store
 * (no React render per `timeupdate`); the track thickens while dragging (transform only).
 */
export function Scrubber({ tone, disabled, className }: ScrubberProps) {
  const { t } = useTranslation('player')
  const duration = usePlayback((s) => s.duration)
  const seconds = usePlayback((s) => Math.floor(s.currentTime))
  const hasTrack = usePlayer((s) => s.index >= 0)
  const [drag, setDrag] = useState<number | null>(null)
  const dragRef = useRef<number | null>(null)
  const fillRef = useRef<HTMLDivElement>(null)
  const bufferRef = useRef<HTMLDivElement>(null)
  const thumbRef = useRef<HTMLDivElement>(null)
  const inactive = disabled || !hasTrack || !(duration > 0)

  const paint = useCallback((ratio: number, buffered?: number) => {
    const r = Number.isFinite(ratio) ? Math.min(1, Math.max(0, ratio)) : 0
    if (fillRef.current) fillRef.current.style.transform = `scaleX(${r})`
    if (thumbRef.current) thumbRef.current.style.transform = `translateX(${r * 100}%)`
    if (buffered !== undefined && bufferRef.current) {
      const b = Number.isFinite(buffered) ? Math.min(1, Math.max(0, buffered)) : 0
      bufferRef.current.style.transform = `scaleX(${b})`
    }
  }, [])

  useEffect(() => {
    const apply = (s: PlaybackState) => {
      const ratio = s.duration > 0 ? s.currentTime / s.duration : 0
      const buffered = s.duration > 0 ? s.buffered / s.duration : 0
      if (dragRef.current === null) paint(ratio, buffered)
      else paint(dragRef.current, buffered)
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
    onStart: (ratio) => {
      // Mouse jumps immediately; touch waits for movement (relative drag).
      setDragging(ratio)
    },
    onMove: (ratio) => setDragging(ratio),
    onEnd: (ratio, committed) => {
      const { duration: d } = usePlayback.getState()
      setDragging(null)
      if (committed && d > 0) usePlayback.getState().seek(ratio * d)
      else {
        const s = usePlayback.getState()
        paint(s.duration > 0 ? s.currentTime / s.duration : 0)
      }
    },
  })

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (inactive) return
    const s = usePlayback.getState()
    let target: number | null = null
    if (event.key === 'ArrowRight' || event.key === 'ArrowUp') target = s.currentTime + KEY_STEP
    else if (event.key === 'ArrowLeft' || event.key === 'ArrowDown') target = s.currentTime - KEY_STEP
    else if (event.key === 'PageUp') target = s.currentTime + PAGE_STEP
    else if (event.key === 'PageDown') target = s.currentTime - PAGE_STEP
    else if (event.key === 'Home') target = 0
    else if (event.key === 'End') target = Math.max(0, s.duration - 1)
    if (target === null) return
    event.preventDefault()
    event.stopPropagation()
    s.seek(target)
  }

  const dragging = drag !== null
  const shown = dragging ? drag * duration : seconds
  const elapsed = formatDuration(shown)
  const remaining = `−${formatDuration(Math.max(0, duration - shown))}`
  const total = formatDuration(duration)
  const sheet = tone === 'sheet'

  const slider = (
    <div
      role="slider"
      tabIndex={inactive ? -1 : 0}
      aria-label={t('seek')}
      aria-valuemin={0}
      aria-valuemax={Math.round(duration)}
      aria-valuenow={Math.round(shown)}
      aria-valuetext={t('position', { elapsed, total })}
      aria-disabled={inactive || undefined}
      onKeyDown={onKeyDown}
      {...handlers}
      className={cn(
        'group/scrub relative flex min-w-0 flex-1 touch-none items-center outline-none select-none',
        sheet ? 'h-7' : 'h-4',
        inactive ? 'cursor-default' : 'cursor-pointer',
        'focus-visible:[&>div:first-child]:ring-2 focus-visible:[&>div:first-child]:ring-ring/60',
      )}
    >
      <div
        className={cn(
          'relative w-full overflow-hidden rounded-full transition-transform duration-200 ease-out',
          sheet ? 'h-[9px] bg-white/25' : 'h-[5px] bg-foreground/15',
          dragging ? 'scale-y-100' : sheet ? 'scale-y-[0.5]' : 'scale-y-[0.8] group-hover/scrub:scale-y-100',
        )}
      >
        <div
          ref={bufferRef}
          className={cn('absolute inset-0 origin-left', sheet ? 'bg-white/15' : 'bg-foreground/10')}
          style={{ transform: 'scaleX(0)' }}
        />
        <div
          ref={fillRef}
          className={cn(
            'absolute inset-0 origin-left',
            sheet
              ? dragging
                ? 'bg-white'
                : 'bg-white/80'
              : dragging
                ? 'bg-primary'
                : 'bg-foreground/60 group-hover/scrub:bg-primary',
          )}
          style={{ transform: 'scaleX(0)' }}
        />
      </div>
      {!sheet ? (
        <div ref={thumbRef} className="pointer-events-none absolute inset-x-0 top-1/2 -mt-1.5 h-3" style={{ transform: 'translateX(0%)' }}>
          <div
            className={cn(
              '-ml-1.5 size-3 rounded-full bg-foreground shadow-sm transition-[opacity,scale] duration-150',
              dragging ? 'scale-100 opacity-100' : 'scale-50 opacity-0 group-hover/scrub:scale-100 group-hover/scrub:opacity-100',
              inactive && 'hidden',
            )}
          />
        </div>
      ) : null}
    </div>
  )

  if (sheet) {
    return (
      <div className={cn('w-full', className)}>
        {slider}
        <div
          className={cn(
            'tnum flex justify-between text-[11px] leading-4 font-medium transition-[color,transform] duration-200',
            dragging ? 'translate-y-1 text-white/85' : 'text-white/55',
          )}
          aria-hidden
        >
          <span>{elapsed}</span>
          <span>{remaining}</span>
        </div>
      </div>
    )
  }

  return (
    <div className={cn('flex w-full items-center gap-2', className)}>
      <span className="tnum w-10 shrink-0 text-right text-[11px] text-muted-foreground" aria-hidden>
        {hasTrack ? elapsed : '–:––'}
      </span>
      {slider}
      <span className="tnum w-10 shrink-0 text-[11px] text-muted-foreground" aria-hidden>
        {hasTrack ? (duration > 0 ? total : '–:––') : '–:––'}
      </span>
    </div>
  )
}
