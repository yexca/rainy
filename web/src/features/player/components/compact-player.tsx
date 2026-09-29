import { Radio } from 'lucide-react'
import { motion } from 'motion/react'
import { useEffect, useRef, useState, type PointerEvent as ReactPointerEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { CoverArt } from '@/components/cover-art'
import { formatDuration } from '@/lib/format'
import { cn } from '@/lib/utils'

import { usePlayerDock } from '../dock'
import { COMPACT_SCRUB_THRESHOLD, compactScrubTarget } from '../lib/dock'
import { entryKey, usePlayback, type PlaybackState } from '../store'
import type { PlayableTrack } from '../types'
import { DockModeMenu } from './dock-mode-menu'
import { LiveBadge } from './live-badge'
import { Marquee } from './now-playing-parts'
import { NextButton, PlayPauseButton } from './transport'

const FLOATING_SPRING = { type: 'spring', stiffness: 420, damping: 38 } as const

interface Scrub {
  /** Queue entry the scrub belongs to: a track change mid-drag drops it. */
  key: string
  pointerId: number
  startX: number
  width: number
  originTime: number
  previewTime: number
  dragging: boolean
}

/**
 * Floating mini bar (`compact` dock mode): bottom-right glass card with artwork, title, layout
 * menu, play/pause and next, and a thin progress line. Tapping the artwork / title opens the
 * floating window; dragging sideways anywhere on the card scrubs relative to the current
 * position (preview above the card, Escape cancels).
 */
export function CompactPlayer({ track }: { track: PlayableTrack }) {
  const { t } = useTranslation('player')
  const setMode = usePlayerDock((s) => s.setMode)
  const duration = usePlayback((s) => s.duration)
  const [scrub, setScrub] = useState<Scrub | null>(null)
  const scrubRef = useRef<Scrub | null>(null)
  const suppressClick = useRef(false)
  const key = entryKey(track)
  const canScrub = !track.isRadio && duration > 0
  const active = scrub?.dragging && scrub.key === key ? scrub : null

  const update = (next: Scrub | null) => {
    scrubRef.current = next
    setScrub(next)
  }

  const swallowNextClick = () => {
    suppressClick.current = true
    window.setTimeout(() => {
      suppressClick.current = false
    }, 0)
  }

  useEffect(() => {
    if (!active) return
    const cancel = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return
      event.preventDefault()
      event.stopPropagation()
      update(null)
      swallowNextClick()
    }
    window.addEventListener('keydown', cancel, true)
    return () => window.removeEventListener('keydown', cancel, true)
  }, [active])

  const onPointerDown = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (!canScrub || event.button !== 0) return
    if ((event.target as HTMLElement).closest('[data-compact-control]')) return
    const origin = usePlayback.getState().currentTime
    update({
      key,
      pointerId: event.pointerId,
      startX: event.clientX,
      width: Math.max(1, event.currentTarget.getBoundingClientRect().width),
      originTime: origin,
      previewTime: origin,
      dragging: false,
    })
  }

  const onPointerMove = (event: ReactPointerEvent<HTMLDivElement>) => {
    const state = scrubRef.current
    if (!state || state.pointerId !== event.pointerId) return
    const deltaX = event.clientX - state.startX
    if (!state.dragging && Math.abs(deltaX) < COMPACT_SCRUB_THRESHOLD) return
    event.preventDefault()
    if (!event.currentTarget.hasPointerCapture(event.pointerId)) event.currentTarget.setPointerCapture(event.pointerId)
    const total = usePlayback.getState().duration
    update({ ...state, dragging: true, previewTime: compactScrubTarget(state.originTime, deltaX, state.width, total) })
  }

  const finish = (event: ReactPointerEvent<HTMLDivElement>, commit: boolean) => {
    const state = scrubRef.current
    if (!state || state.pointerId !== event.pointerId) return
    if (event.currentTarget.hasPointerCapture(event.pointerId)) event.currentTarget.releasePointerCapture(event.pointerId)
    update(null)
    if (!state.dragging) return
    swallowNextClick()
    if (commit && state.key === key) usePlayback.getState().seek(state.previewTime)
  }

  const percent = (seconds: number) => (duration > 0 ? Math.min(100, Math.max(0, (seconds / duration) * 100)) : 0)
  const origin = active ? percent(active.originTime) : 0
  const preview = active ? percent(active.previewTime) : 0
  const delta = active ? active.previewTime - active.originTime : 0

  return (
    <motion.div
      role="region"
      aria-label={t('region')}
      className="fixed right-[calc(1.5rem+var(--safe-right))] bottom-[calc(1.5rem+var(--safe-bottom))] z-40 w-[min(390px,calc(100vw-3rem))]"
      initial={{ opacity: 0, y: 16, scale: 0.98 }}
      animate={{ opacity: 1, y: 0, scale: 1 }}
      exit={{ opacity: 0, y: 16, scale: 0.98 }}
      transition={FLOATING_SPRING}
    >
      {active ? (
        <div className="pointer-events-none absolute inset-x-0 bottom-full h-11" aria-live="polite">
          <div
            className="tnum absolute top-0 -translate-x-1/2 rounded-full bg-popover px-2.5 py-1 text-xs font-medium whitespace-nowrap text-popover-foreground shadow-lg ring-1 ring-border"
            style={{ left: `${Math.min(78, Math.max(22, origin))}%` }}
          >
            {formatDuration(active.originTime)} → {formatDuration(active.previewTime)}{' '}
            <span className={delta >= 0 ? 'text-primary' : 'text-destructive'}>
              {delta >= 0 ? '+' : '−'}
              {formatDuration(Math.abs(delta))}
            </span>
          </div>
        </div>
      ) : null}

      <div
        className="ui-chrome glass relative touch-pan-y overflow-hidden rounded-2xl border border-border/60 shadow-xl shadow-black/10 dark:shadow-black/40"
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={(event) => finish(event, true)}
        onPointerCancel={(event) => finish(event, false)}
      >
        {active ? (
          <div aria-hidden className="pointer-events-none absolute inset-0">
            <div className="absolute inset-y-0 left-0 bg-primary/10" style={{ width: `${Math.min(origin, preview)}%` }} />
            <div
              className="absolute inset-y-0 bg-primary/20"
              style={{ left: `${Math.min(origin, preview)}%`, width: `${Math.abs(preview - origin)}%` }}
            />
            <div className="absolute inset-y-0 w-px bg-primary/70" style={{ left: `${preview}%` }} />
          </div>
        ) : null}

        <div className="relative flex h-(--compact-player-h) items-center gap-0.5 pr-1.5 pl-2">
          <button
            type="button"
            onClick={() => {
              if (suppressClick.current) return
              setMode('window')
            }}
            aria-label={t('dock.openWindow')}
            title={t('dock.openWindow')}
            className="flex min-w-0 flex-1 items-center gap-3 rounded-xl p-1 text-left transition-colors outline-none hover:bg-foreground/[0.04] focus-visible:ring-2 focus-visible:ring-ring/60 active:bg-foreground/[0.07]"
          >
            <CoverArt coverArt={track.coverArt} size={44} rounded="md" keepPrevious icon={track.isRadio ? Radio : undefined} />
            <span className="min-w-0 flex-1 leading-tight">
              <Marquee className="text-sm font-semibold">{track.title}</Marquee>
              <span className="flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground">
                {track.isRadio ? <LiveBadge tone="bar" className="px-1.5 py-0 text-[9px]" /> : null}
                <span className="truncate">{track.isRadio ? t('live.radio') : track.artist}</span>
              </span>
            </span>
          </button>
          <span data-compact-control className="contents">
            <DockModeMenu tone="bar" className="size-9 rounded-full" />
            <PlayPauseButton tone="bar" size="sm" />
            <NextButton tone="bar" size="sm" className="size-11" />
          </span>
        </div>

        {canScrub ? <CompactProgress hidden={!!active} /> : null}
      </div>
    </motion.div>
  )
}

/** Thin progress line along the bottom edge (DOM-driven, no re-renders). */
function CompactProgress({ hidden }: { hidden: boolean }) {
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const apply = (s: PlaybackState) => {
      const ratio = s.duration > 0 ? Math.min(1, Math.max(0, s.currentTime / s.duration)) : 0
      if (ref.current) ref.current.style.transform = `scaleX(${ratio})`
    }
    apply(usePlayback.getState())
    return usePlayback.subscribe(apply)
  }, [])
  return (
    <div
      aria-hidden
      className={cn(
        'pointer-events-none absolute inset-x-4 bottom-1 h-[3px] overflow-hidden rounded-full bg-foreground/10 transition-opacity',
        hidden && 'opacity-0',
      )}
    >
      <div ref={ref} className="h-full origin-left rounded-full bg-primary" style={{ transform: 'scaleX(0)' }} />
    </div>
  )
}
