import { Maximize2 } from 'lucide-react'
import { AnimatePresence, motion } from 'motion/react'
import { useTranslation } from 'react-i18next'

import { usePlayerDock } from '../dock'
import { usePlayer } from '../store'
import type { PlayableTrack } from '../types'
import { DockModeMenu } from './dock-mode-menu'
import { LiveProgress } from './live-badge'
import { LyricsView } from './lyrics-view'
import { NowPlayingBackground } from './now-playing-background'
import { NowPlayingActions, NowPlayingArtwork, NowPlayingTitle } from './now-playing-parts'
import { QueueView } from './queue-view'
import { Scrubber } from './scrubber'
import { ModeToggle, NextButton, PlayPauseButton, PrevButton, RepeatButton, ShuffleButton } from './transport'
import { SheetVolume } from './volume-control'

const FLOATING_SPRING = { type: 'spring', stiffness: 420, damping: 38 } as const

/**
 * Floating Now Playing window (`window` dock mode): a bottom-right card with the artwork
 * backdrop, artwork (springs smaller while paused), title, scrubber, transport, volume and the
 * lyrics / queue toggles, which swap the artwork for the panel. The grabber shrinks it to the
 * mini bar; the header also has the layout menu and full screen. Not modal: the page stays
 * usable and the global shortcuts keep working.
 */
export function PlayerWindow({ track }: { track: PlayableTrack }) {
  const { t } = useTranslation('player')
  const panel = usePlayer((s) => s.panel)
  const isPlaying = usePlayer((s) => s.isPlaying)
  const position = usePlayer((s) => s.index + 1)
  const total = usePlayer((s) => s.queue.length)
  const setNowPlayingOpen = usePlayer((s) => s.setNowPlayingOpen)
  const setMode = usePlayerDock((s) => s.setMode)

  return (
    <motion.section
      aria-label={t('nowPlaying')}
      className="ui-chrome fixed right-[calc(1.5rem+var(--safe-right))] bottom-[calc(1.5rem+var(--safe-bottom))] z-40 flex h-[min(640px,calc(100dvh-3rem-var(--safe-top)-var(--safe-bottom)))] w-[min(var(--player-window-w),calc(100vw-3rem))] flex-col overflow-hidden rounded-3xl text-white shadow-2xl shadow-black/40 ring-1 ring-black/10 dark:ring-white/10"
      initial={{ opacity: 0, y: 24, scale: 0.97 }}
      animate={{ opacity: 1, y: 0, scale: 1 }}
      exit={{ opacity: 0, y: 24, scale: 0.97 }}
      transition={FLOATING_SPRING}
      style={{ transformOrigin: 'bottom right' }}
    >
      <NowPlayingBackground coverArt={track.coverArt} playing={isPlaying} />

      <header className="relative grid h-12 shrink-0 grid-cols-[1fr_auto_1fr] items-center px-3">
        <span className="tnum pl-2 text-[11px] font-medium text-white/55">
          {position} / {total}
        </span>
        <button
          type="button"
          onClick={() => setMode('compact')}
          aria-label={t('dock.shrink')}
          title={t('dock.shrink')}
          className="group/grab grid h-8 w-16 place-items-center rounded-full outline-none focus-visible:ring-2 focus-visible:ring-white/60"
        >
          <span className="h-[5px] w-9 rounded-full bg-white/40 transition-colors group-hover/grab:bg-white/70" />
        </button>
        <div className="flex items-center justify-end gap-1">
          <DockModeMenu tone="sheet" />
          <ModeToggle
            tone="sheet"
            active={false}
            onClick={() => setNowPlayingOpen(true)}
            aria-label={t('expand')}
            title={t('expand')}
            className="size-9 rounded-full"
          >
            <Maximize2 className="size-4" strokeWidth={1.9} />
          </ModeToggle>
        </div>
      </header>

      <div className="relative flex min-h-0 flex-1 flex-col">
        {panel === 'none' ? (
          <div className="flex min-h-0 flex-1 items-center justify-center px-8 py-2 [container-type:size]">
            <NowPlayingArtwork track={track} playing={isPlaying} className="w-[min(100cqw,100cqh)]" />
          </div>
        ) : (
          <>
            <div className="flex shrink-0 items-center gap-3 px-6 pb-2">
              <NowPlayingArtwork track={track} playing className="w-14 shrink-0" />
              <NowPlayingTitle track={track} size="sm" className="min-w-0 flex-1" />
            </div>
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
                  {panel === 'lyrics' ? <LyricsView track={track} tone="sheet" /> : <QueueView tone="sheet" />}
                </motion.div>
              </AnimatePresence>
            </div>
          </>
        )}

        <div className="shrink-0 px-6 pb-4">
          {panel === 'none' ? <NowPlayingTitle track={track} className="mb-2" /> : null}
          {track.isRadio ? <LiveProgress tone="sheet" /> : <Scrubber tone="sheet" />}
          <div className="my-1 flex items-center justify-between">
            <ShuffleButton tone="sheet" className="size-10" />
            <PrevButton tone="sheet" />
            <PlayPauseButton tone="sheet" />
            <NextButton tone="sheet" />
            <RepeatButton tone="sheet" className="size-10" />
          </div>
          <SheetVolume className="mb-1" />
          <NowPlayingActions track={track} className="-mx-1.5" />
        </div>
      </div>
    </motion.section>
  )
}
