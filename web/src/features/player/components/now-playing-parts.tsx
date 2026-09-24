import { Airplay, ListMusic, MessageSquareQuote, Radio } from 'lucide-react'
import { motion } from 'motion/react'
import { useLayoutEffect, useRef, useState, type PointerEventHandler, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'

import { CoverArt } from '@/components/cover-art'
import { TrackActionsMenu } from '@/features/library/components/track-actions'
import { cn } from '@/lib/utils'

import { showAirPlayPicker, useAirPlay } from '../engine/engine'
import { usePlayer } from '../store'
import type { PlayableTrack, PlayerPanel } from '../types'
import { StarButton } from './star-button'
import { ModeToggle } from './transport'

/** Scrolls text back and forth when it doesn't fit (Apple Music style); static otherwise. */
export function Marquee({ children, className }: { children: ReactNode; className?: string }) {
  const outerRef = useRef<HTMLDivElement>(null)
  const innerRef = useRef<HTMLSpanElement>(null)
  const [overflow, setOverflow] = useState(0)

  useLayoutEffect(() => {
    const outer = outerRef.current
    const inner = innerRef.current
    if (!outer || !inner) return
    const measure = () => setOverflow(Math.max(0, Math.ceil(inner.scrollWidth - outer.clientWidth)))
    measure()
    const observer = new ResizeObserver(measure)
    observer.observe(outer)
    return () => observer.disconnect()
  }, [children])

  const travel = overflow / 28
  const total = 2.5 + travel + 2.5 + travel
  return (
    <div ref={outerRef} className={cn('overflow-hidden whitespace-nowrap', className)}>
      <motion.span
        key={overflow}
        ref={innerRef}
        className="inline-block pr-px"
        initial={{ x: 0 }}
        animate={overflow > 0 ? { x: [0, 0, -overflow, -overflow, 0] } : { x: 0 }}
        transition={
          overflow > 0
            ? {
                duration: total,
                times: [0, 2.5 / total, (2.5 + travel) / total, (5 + travel) / total, 1],
                ease: 'easeInOut',
                repeat: Number.POSITIVE_INFINITY,
              }
            : { duration: 0 }
        }
      >
        {children}
      </motion.span>
    </div>
  )
}

/** Large artwork that springs smaller while paused (Apple Music behaviour). */
export function NowPlayingArtwork({ track, playing, className }: { track: PlayableTrack; playing: boolean; className?: string }) {
  return (
    <motion.div
      className={cn('relative aspect-square', className)}
      initial={false}
      animate={{ scale: playing ? 1 : 0.8 }}
      transition={{ type: 'spring', stiffness: 260, damping: 22 }}
    >
      <CoverArt
        coverArt={track.coverArt}
        size={512}
        fluid
        priority
        flat
        keepPrevious
        rounded="xl"
        icon={track.isRadio ? Radio : undefined}
        alt={track.album || track.title}
        className={cn(
          'ring-1 ring-white/10 transition-shadow duration-500',
          playing ? 'shadow-[0_28px_60px_-12px_rgba(0,0,0,0.6)]' : 'shadow-[0_14px_36px_-12px_rgba(0,0,0,0.45)]',
        )}
      />
    </motion.div>
  )
}

/** Title + artist (→ artist page) + star + actions menu, on artwork backgrounds. */
export function NowPlayingTitle({
  track,
  size = 'lg',
  className,
  onPointerDown,
}: {
  track: PlayableTrack
  size?: 'lg' | 'sm'
  className?: string
  onPointerDown?: PointerEventHandler<HTMLDivElement>
}) {
  const { t } = useTranslation('player')
  const navigate = useNavigate()
  const setNowPlayingOpen = usePlayer((s) => s.setNowPlayingOpen)
  const large = size === 'lg'
  const goToArtist = () => {
    if (!track.artistId || track.isRadio) return
    setNowPlayingOpen(false)
    navigate(`/artists/${encodeURIComponent(track.artistId)}`)
  }

  return (
    <div className={cn('flex items-center gap-3', className)} onPointerDown={onPointerDown}>
      <div className="min-w-0 flex-1">
        <Marquee className={cn('font-semibold text-white', large ? 'text-[21px] leading-7' : 'text-base leading-6')}>{track.title}</Marquee>
        {track.isRadio ? (
          <p className={cn('truncate text-white/60', large ? 'text-[21px] leading-7' : 'text-sm leading-5')}>{t('live.radio')}</p>
        ) : (
          <button
            type="button"
            onClick={goToArtist}
            disabled={!track.artistId}
            className={cn(
              'block max-w-full truncate rounded text-left text-white/60 outline-none hover:text-white/80 focus-visible:ring-2 focus-visible:ring-white/50',
              large ? 'text-[21px] leading-7' : 'text-sm leading-5',
            )}
          >
            {track.artist}
          </button>
        )}
      </div>
      <StarButton track={track} tone="sheet" />
      {!track.isRadio ? (
        <TrackActionsMenu
          tracks={[track]}
          className="size-9 rounded-full bg-white/10 text-white hover:bg-white/20 hover:text-white focus-visible:ring-white/60 dark:hover:bg-white/20"
        />
      ) : null}
    </div>
  )
}

/** Opens the AirPlay device picker (only where Safari supports it and a device is around). */
export function AirPlayButton({ className }: { className?: string }) {
  const { t } = useTranslation('player')
  const available = useAirPlay((s) => s.available)
  if (!available) return null
  return (
    <ModeToggle tone="sheet" active={false} onClick={showAirPlayPicker} aria-label={t('airplay')} className={cn('size-11', className)}>
      <Airplay className="size-[22px]" strokeWidth={1.9} />
    </ModeToggle>
  )
}

/** Lyrics · AirPlay · Queue toggles under the transport. */
export function NowPlayingActions({ className, showAirPlay = true }: { className?: string; showAirPlay?: boolean }) {
  const { t } = useTranslation('player')
  const panel = usePlayer((s) => s.panel)
  const setPanel = usePlayer((s) => s.setPanel)
  const airPlay = useAirPlay((s) => s.available)
  const toggle = (p: PlayerPanel) => setPanel(panel === p ? 'none' : p)

  return (
    <div className={cn('flex items-center justify-between', className)}>
      <ModeToggle tone="sheet" active={panel === 'lyrics'} onClick={() => toggle('lyrics')} aria-label={t('lyrics.title')} className="size-11">
        <MessageSquareQuote className="size-[22px]" strokeWidth={1.9} />
      </ModeToggle>
      {showAirPlay && airPlay ? <AirPlayButton /> : null}
      <ModeToggle tone="sheet" active={panel === 'queue'} onClick={() => toggle('queue')} aria-label={t('queue.title')} className="size-11">
        <ListMusic className="size-[22px]" strokeWidth={1.9} />
      </ModeToggle>
    </div>
  )
}
