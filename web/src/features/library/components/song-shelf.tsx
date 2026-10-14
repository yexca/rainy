import { Play } from 'lucide-react'
import { memo, useCallback, useMemo, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { CoverArt } from '@/components/cover-art'
import { Button } from '@/components/ui/button'
import { ContextMenu, ContextMenuContent, ContextMenuTrigger } from '@/components/ui/context-menu'
import { Skeleton } from '@/components/ui/skeleton'
import { usePlayer } from '@/features/player/store'
import type { Track } from '@/lib/api/types'
import { cn } from '@/lib/utils'

import { ActionMenuContext, type ActionMenuContextValue } from '../lib/actions'
import { useTrackState } from '../lib/use-track-state'
import { HorizontalShelf } from './horizontal-shelf'
import { PlayingIndicator } from './playing-indicator'
import { TrackActionItems, TrackActionsMenu } from './track-actions'

/** Songs per column. */
const ROWS = 4

export interface SongShelfProps {
  title: string
  tracks: Track[]
  /** Extra header controls, before the play button. */
  actions?: ReactNode
}

/**
 * A titled shelf of songs (§9.5a): columns of four compact rows that scroll sideways. A row
 * plays the shelf from that song; the header button plays it from the top. Renders nothing
 * without songs.
 */
export function SongShelf({ title, tracks, actions }: SongShelfProps) {
  const { t } = useTranslation('library')
  const columns = useMemo(() => {
    const out: Track[][] = []
    for (let i = 0; i < tracks.length; i += ROWS) out.push(tracks.slice(i, i + ROWS))
    return out
  }, [tracks])
  const play = useCallback((index: number) => usePlayer.getState().playTracks(tracks, index), [tracks])

  if (tracks.length === 0) return null
  return (
    <HorizontalShelf
      title={title}
      itemSize="column"
      actions={
        <>
          {actions}
          <Button
            variant="ghost"
            size="icon"
            onClick={() => usePlayer.getState().playTracks(tracks, 0, { shuffle: false })}
            aria-label={t('home.playSection', { name: title })}
            title={t('home.playSection', { name: title })}
            className="size-11 rounded-full text-primary md:size-9"
          >
            <Play className="size-4" fill="currentColor" strokeWidth={0} aria-hidden />
          </Button>
        </>
      }
    >
      {columns.map((column, c) => (
        <ul key={column[0].id}>
          {column.map((track, r) => (
            <li key={track.id}>
              <SongRow track={track} index={c * ROWS + r} onPlay={play} />
            </li>
          ))}
        </ul>
      ))}
    </HorizontalShelf>
  )
}

const SongRow = memo(function SongRow({ track, index, onPlay }: { track: Track; index: number; onPlay: (index: number) => void }) {
  const state = useTrackState(track.id)
  const current = state !== 'none'
  const [menuOpen, setMenuOpen] = useState(false)
  const contextValue = useMemo<ActionMenuContextValue>(() => ({ kind: 'context', close: () => {} }), [])

  return (
    <ContextMenu onOpenChange={setMenuOpen}>
      <ContextMenuTrigger asChild>
        <div
          className={cn(
            'group -mx-2 flex h-15 items-center rounded-lg px-2 transition-colors active:bg-accent/70 md:h-14',
            menuOpen ? 'bg-accent' : 'md:hover:bg-accent/60',
            track.missing && 'opacity-50',
          )}
        >
          <button
            type="button"
            onClick={() => (current ? usePlayer.getState().togglePlay() : onPlay(index))}
            aria-current={current ? 'true' : undefined}
            className="flex h-full min-w-0 flex-1 items-center gap-3 rounded-md text-left outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
          >
            <span className="relative shrink-0">
              <CoverArt coverArt={track.coverArt} size={44} />
              <span
                className={cn(
                  'absolute inset-0 grid place-items-center rounded-md bg-black/40 text-white transition-opacity',
                  current ? 'opacity-100' : 'opacity-0 md:group-hover:opacity-100',
                )}
              >
                {current ? (
                  <PlayingIndicator playing={state === 'playing'} className="text-white" />
                ) : (
                  <Play className="size-4" fill="currentColor" strokeWidth={0} aria-hidden />
                )}
              </span>
            </span>
            <span className="min-w-0 flex-1">
              <span className={cn('block truncate text-[15px] leading-snug font-medium md:text-sm', current && 'text-primary')}>
                {track.title}
              </span>
              <span className="block truncate text-[13px] leading-snug text-muted-foreground">{track.artist}</span>
            </span>
          </button>
          <TrackActionsMenu tracks={[track]} className="-mr-2 shrink-0 md:mr-0" />
        </div>
      </ContextMenuTrigger>
      <ContextMenuContent className="min-w-56 rounded-xl">
        <ActionMenuContext.Provider value={contextValue}>
          <TrackActionItems tracks={[track]} />
        </ActionMenuContext.Provider>
      </ContextMenuContent>
    </ContextMenu>
  )
})

/** Placeholder for a song shelf that is loading. */
export function SongShelfSkeleton() {
  return (
    <div className="mt-8 md:mt-10" aria-hidden>
      <Skeleton className="mb-4 h-6 w-36" />
      <div className="grid gap-x-5 lg:grid-cols-3">
        {Array.from({ length: 3 }, (_, c) => (
          <div key={c} className={cn(c > 0 && 'hidden lg:block')}>
            {Array.from({ length: ROWS }, (_, r) => (
              <div key={r} className="flex h-15 items-center gap-3 md:h-14">
                <Skeleton className="size-11 rounded-md" />
                <div className="flex flex-1 flex-col gap-1.5">
                  <Skeleton className="h-3.5 w-3/5" />
                  <Skeleton className="h-3 w-2/5" />
                </div>
              </div>
            ))}
          </div>
        ))}
      </div>
    </div>
  )
}
