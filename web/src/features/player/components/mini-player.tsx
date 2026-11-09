import { Radio } from 'lucide-react'
import { AnimatePresence, motion, type PanInfo } from 'motion/react'
import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'

import { CoverArt } from '@/components/cover-art'

import { useCurrentTrack, usePlayback, usePlayer, type PlaybackState } from '../store'
import type { PlayableTrack } from '../types'
import { LiveBadge } from './live-badge'
import { NextButton, PlayPauseButton } from './transport'

/** Upward swipe distance / velocity that opens Now Playing. */
const OPEN_OFFSET = -24
const OPEN_VELOCITY = -300

/**
 * Mobile floating mini player (docs/architecture/contract.md §9.4): glass pill above the tab bar with a thin
 * progress line; tap or swipe up opens Now Playing. Rendered by the shell inside a fixed slot
 * 8px above the tab bar (8px side margins); owns its height (`--miniplayer-h`). Idle → nothing.
 */
export function MiniPlayer() {
  const track = useCurrentTrack()
  return <AnimatePresence initial={false}>{track ? <MiniPlayerPill key="mini" track={track} /> : null}</AnimatePresence>
}

function MiniPlayerPill({ track }: { track: PlayableTrack }) {
  const { t } = useTranslation('player')
  const setNowPlayingOpen = usePlayer((s) => s.setNowPlayingOpen)
  const open = () => setNowPlayingOpen(true)

  const onDragEnd = (_: unknown, info: PanInfo) => {
    if (info.offset.y < OPEN_OFFSET || info.velocity.y < OPEN_VELOCITY) open()
  }

  return (
    <motion.div
      className="ui-chrome glass relative h-(--miniplayer-h) overflow-hidden rounded-xl border border-border/60 shadow-lg shadow-black/10 dark:shadow-black/40"
      initial={{ y: 72, opacity: 0 }}
      animate={{ y: 0, opacity: 1 }}
      exit={{ y: 72, opacity: 0 }}
      drag="y"
      dragConstraints={{ top: 0, bottom: 0 }}
      dragElastic={{ top: 0.35, bottom: 0.08 }}
      dragMomentum={false}
      onDragEnd={onDragEnd}
      style={{ touchAction: 'none' }}
    >
      <div className="flex h-full items-center gap-1 pr-1 pl-2">
        <button
          type="button"
          onClick={open}
          aria-label={t('openNowPlaying')}
          className="flex h-full min-w-0 flex-1 items-center gap-3 rounded-lg text-left outline-none transition-transform duration-150 active:scale-[0.98] focus-visible:ring-2 focus-visible:ring-ring/50"
        >
          <CoverArt coverArt={track.coverArt} size={40} rounded="md" keepPrevious icon={track.isRadio ? Radio : undefined} />
          <span className="min-w-0 flex-1 leading-tight">
            <span className="block truncate text-[15px] font-medium">{track.title}</span>
            <span className="flex min-w-0 items-center gap-1.5 text-[13px] text-muted-foreground">
              {track.isRadio ? <LiveBadge tone="bar" className="px-1.5 py-0 text-[9px]" /> : null}
              <span className="truncate">{track.isRadio ? t('live.radio') : track.artist}</span>
            </span>
          </span>
        </button>
        <PlayPauseButton tone="bar" size="sm" />
        <NextButton tone="bar" size="sm" className="size-11" />
      </div>
      {track.isRadio ? null : <MiniProgress />}
    </motion.div>
  )
}

/** Thin progress line along the bottom edge (DOM-driven, no re-renders). */
function MiniProgress() {
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
    <div aria-hidden className="absolute inset-x-3 bottom-0 h-[2px] overflow-hidden rounded-full bg-foreground/10">
      <div ref={ref} className="h-full origin-left bg-primary" style={{ transform: 'scaleX(0)' }} />
    </div>
  )
}
