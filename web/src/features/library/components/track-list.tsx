import { useWindowVirtualizer } from '@tanstack/react-virtual'
import { Clock3, Flame, MessageSquareQuote, Pause, Play, Star } from 'lucide-react'
import { memo, useCallback, useEffect, useMemo, useState, type KeyboardEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'

import { CoverArt } from '@/components/cover-art'
import { ContextMenu, ContextMenuContent, ContextMenuTrigger } from '@/components/ui/context-menu'
import { Skeleton } from '@/components/ui/skeleton'
import { usePlayer } from '@/features/player/store'
import { useIsMobile } from '@/hooks/use-media-query'
import type { Track } from '@/lib/api/types'
import { formatDuration, formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'

import { ActionMenuContext, type ActionMenuContextValue } from '../lib/actions'
import { formatQuality, trackQuality } from '../lib/format'
import type { TrackActionsContext } from '../lib/track-action-items'
import { createTrackRows, type TrackSection } from '../lib/track-rows'
import { useElementGeometry } from '../lib/use-element-geometry'
import { useTrackState } from '../lib/use-track-state'
import { Hairline } from './hairline'
import { PlayingIndicator } from './playing-indicator'
import { StarButton } from './star-button'
import { TrackActionItems, TrackActionsMenu } from './track-actions'

export type { TrackSection } from '../lib/track-rows'

export interface TrackListProps {
  tracks: Track[]
  /** Total rows when paginated; rows beyond `tracks.length` render as skeletons. */
  total?: number
  /** Called when the user scrolls close to the last loaded track (load the next page). */
  onEndReached?: () => void
  /** `album`: track numbers instead of artwork, no album column. */
  variant?: 'default' | 'album'
  /** Album column on desktop (default: true for the default variant). */
  showAlbum?: boolean
  /** Artwork (default: true for the default variant). */
  showArt?: boolean
  /** Desktop column header (default true). */
  showHeader?: boolean
  /** Album variant: artists equal to this are not repeated under each title. */
  albumArtist?: string
  /** Default variant: hide the artist in the subtitle (e.g. top songs on an artist page). */
  hideArtist?: boolean
  /** Group rows under headers (e.g. discs); a header is inserted whenever the key changes. */
  sectionOf?: (track: Track, index: number) => TrackSection | null
  /** Playlist context for the actions (position = row index). */
  playlist?: { id: string; editable: boolean }
  /** Override "play from row" (default: play `tracks` as the queue, starting at the row). */
  onPlay?: (index: number) => void
  /** Rendered instead of the list when there are no tracks (and nothing is loading). */
  empty?: ReactNode
  className?: string
}

const HEIGHT = {
  desktopTrack: 48,
  desktopHeader: 44,
  mobileTrack: 60,
  mobileAlbumTrack: 52,
  mobileHeader: 44,
}

/** How many rows before the end of the loaded tracks `onEndReached` fires. */
const END_THRESHOLD = 30

/**
 * Virtualized track list (the window scrolls). Desktop: dense table with a playing indicator,
 * hover play button, star, "…" menu and a right-click context menu; double-click or Enter plays.
 * Phones: 60px rows with 44px artwork; tap plays. The queue is the list itself.
 */
export function TrackList({
  tracks,
  total,
  onEndReached,
  variant = 'default',
  showAlbum,
  showArt,
  showHeader = true,
  albumArtist,
  hideArtist,
  sectionOf,
  playlist,
  onPlay,
  empty,
  className,
}: TrackListProps) {
  const { t } = useTranslation()
  const isMobile = useIsMobile()
  const [listRef, geometry] = useElementGeometry<HTMLDivElement>()
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const isAlbum = variant === 'album'
  const art = showArt ?? !isAlbum
  const album = (showAlbum ?? !isAlbum) && !isMobile
  const count = Math.max(total ?? tracks.length, tracks.length)
  // Album variant: a plays column relative to the album's most played song, once any song was played.
  const maxPlays = useMemo(() => (isAlbum ? Math.max(0, ...tracks.map((t) => t.playCount)) : 0), [isAlbum, tracks])

  const rows = useMemo(() => createTrackRows(tracks, count, sectionOf), [tracks, count, sectionOf])

  const trackHeight = isMobile ? (isAlbum ? HEIGHT.mobileAlbumTrack : HEIGHT.mobileTrack) : HEIGHT.desktopTrack
  const headerHeight = isMobile ? HEIGHT.mobileHeader : HEIGHT.desktopHeader

  const virtualizer = useWindowVirtualizer({
    count: rows.count,
    estimateSize: (i) => (rows.loaded[i]?.type === 'header' ? headerHeight : trackHeight),
    overscan: 12,
    scrollMargin: geometry.top,
    getItemKey: rows.getItemKey,
  })
  const virtualItems = virtualizer.getVirtualItems()
  const lastVisible = virtualItems.length > 0 ? virtualItems[virtualItems.length - 1].index : -1

  // Size changes (mobile ↔ desktop) change every row height.
  useEffect(() => {
    virtualizer.measure()
  }, [virtualizer, trackHeight, headerHeight])

  useEffect(() => {
    if (!onEndReached || tracks.length >= count || lastVisible < 0) return
    if (lastVisible >= rows.loaded.length - END_THRESHOLD) onEndReached()
  }, [lastVisible, onEndReached, rows, tracks.length, count])

  const play = useCallback(
    (index: number) => {
      if (onPlay) onPlay(index)
      else usePlayer.getState().playTracks(tracks, index)
    },
    [onPlay, tracks],
  )

  if (count === 0 && empty) return <>{empty}</>

  return (
    <div className={cn('w-full', className)} role="list" aria-label={t('library:track.listLabel')}>
      {showHeader && !isMobile ? (
        <div
          aria-hidden
          className={cn(
            'grid h-9 items-center gap-3 border-b border-border/60 px-2 text-xs font-medium tracking-wide text-muted-foreground uppercase',
            gridColumns(isAlbum, album, maxPlays > 0),
          )}
        >
          <span className="text-center">#</span>
          <span>{t('library:track.title')}</span>
          {album ? <span>{t('library:track.album')}</span> : null}
          {isAlbum ? (
            <span className="flex justify-center">
              <MessageSquareQuote className="size-3.5" strokeWidth={1.75} />
            </span>
          ) : null}
          {isAlbum ? <span>{t('library:track.quality')}</span> : null}
          {maxPlays > 0 ? (
            <span className="flex">
              <Flame className="size-3.5" strokeWidth={1.75} />
            </span>
          ) : null}
          <span className="flex justify-end pr-1">
            <Clock3 className="size-3.5" strokeWidth={1.75} />
          </span>
          <span />
          <span />
        </div>
      ) : null}
      <div ref={listRef} className="relative w-full" style={{ height: virtualizer.getTotalSize() }}>
        {virtualItems.map((item) => {
          const row = rows.getRow(item.index)
          if (!row) return null
          const style = {
            height: item.size,
            transform: `translateY(${item.start - virtualizer.options.scrollMargin}px)`,
          }
          if (row.type === 'header') {
            return (
              <div
                key={item.key}
                role="presentation"
                className="absolute inset-x-0 top-0 flex items-end pb-2 text-sm font-semibold text-muted-foreground md:px-2"
                style={style}
              >
                {row.label}
              </div>
            )
          }
          if (row.type === 'skeleton') {
            return (
              <div key={item.key} className="absolute inset-x-0 top-0" style={style}>
                <SkeletonRow mobile={isMobile} art={art} />
              </div>
            )
          }
          const track = tracks[row.index]
          return (
            <div key={item.key} className="absolute inset-x-0 top-0" style={style} role="listitem">
              {isMobile ? (
                <MobileTrackRow
                  track={track}
                  index={row.index}
                  art={art}
                  isAlbum={isAlbum}
                  albumArtist={albumArtist}
                  hideArtist={hideArtist}
                  playlist={playlist}
                  last={row.index === count - 1 || rows.getRow(item.index + 1)?.type === 'header'}
                  onPlay={play}
                />
              ) : (
                <DesktopTrackRow
                  track={track}
                  index={row.index}
                  art={art}
                  isAlbum={isAlbum}
                  showAlbum={album}
                  maxPlays={maxPlays}
                  albumArtist={albumArtist}
                  hideArtist={hideArtist}
                  playlist={playlist}
                  selected={selectedId === track.id}
                  onSelect={setSelectedId}
                  onPlay={play}
                />
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}

function gridColumns(isAlbum: boolean, showAlbum: boolean, plays = false): string {
  if (isAlbum) {
    // # · title · lyrics · quality · [plays] · duration · star · menu
    return plays
      ? 'grid-cols-[2.25rem_minmax(0,1fr)_1.5rem_7rem_3rem_3.5rem_2rem_2rem]'
      : 'grid-cols-[2.25rem_minmax(0,1fr)_1.5rem_7rem_3.5rem_2rem_2rem]'
  }
  if (!showAlbum) return 'grid-cols-[2.25rem_minmax(0,1fr)_3.5rem_2rem_2rem]'
  return 'grid-cols-[2.25rem_minmax(0,1.2fr)_minmax(0,1fr)_3.5rem_2rem_2rem]'
}

function playlistContext(
  playlist: TrackListProps['playlist'],
  index: number,
): TrackActionsContext | undefined {
  if (!playlist) return undefined
  return { playlistId: playlist.id, position: index, canEditPlaylist: playlist.editable }
}

function trackNumberLabel(track: Track, index: number, isAlbum: boolean): string {
  if (isAlbum) return track.trackNumber > 0 ? String(track.trackNumber) : '–'
  return String(index + 1)
}

function LyricsMark() {
  const { t } = useTranslation()
  const label = t('library:track.hasLyrics')
  return (
    <span role="img" aria-label={label} title={label} className="shrink-0 text-muted-foreground">
      <MessageSquareQuote className="size-3.5" strokeWidth={1.75} />
    </span>
  )
}

/** The user's plays of a song relative to the album's most played one. */
function PlaysBar({ plays, max }: { plays: number; max: number }) {
  const { t } = useTranslation()
  const label = t('library:listening.plays', { count: plays, formatted: formatNumber(plays) })
  const width = plays > 0 ? Math.max(8, Math.round((plays / max) * 100)) : 0
  return (
    <div role="img" aria-label={label} title={label} className="h-1 w-9 overflow-hidden rounded-full bg-foreground/10">
      <div className="h-full rounded-full bg-muted-foreground/70" style={{ width: `${width}%` }} />
    </div>
  )
}

interface RowProps {
  track: Track
  index: number
  art: boolean
  isAlbum: boolean
  albumArtist?: string
  hideArtist?: boolean
  playlist?: TrackListProps['playlist']
  onPlay: (index: number) => void
}

const DesktopTrackRow = memo(function DesktopTrackRow({
  track,
  index,
  art,
  isAlbum,
  showAlbum,
  maxPlays,
  albumArtist,
  hideArtist,
  playlist,
  selected,
  onSelect,
  onPlay,
}: RowProps & { showAlbum: boolean; maxPlays: number; selected: boolean; onSelect: (id: string) => void }) {
  const { t } = useTranslation()
  const state = useTrackState(track.id)
  const current = state !== 'none'
  const [menuOpen, setMenuOpen] = useState(false)
  const showArtist = isAlbum ? !!track.artist && track.artist !== albumArtist : !hideArtist
  const context = playlistContext(playlist, index)
  const contextValue = useMemo<ActionMenuContextValue>(() => ({ kind: 'context', close: () => {} }), [])

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.target !== event.currentTarget) return
    if (event.key === 'Enter') {
      event.preventDefault()
      onPlay(index)
    }
  }

  const primaryAction = () => {
    if (current) usePlayer.getState().togglePlay()
    else onPlay(index)
  }

  return (
    <ContextMenu onOpenChange={setMenuOpen}>
      <ContextMenuTrigger asChild>
        <div
          tabIndex={0}
          aria-current={current ? 'true' : undefined}
          onClick={() => onSelect(track.id)}
          onDoubleClick={() => onPlay(index)}
          onKeyDown={onKeyDown}
          className={cn(
            'group grid h-full cursor-default items-center gap-3 rounded-md px-2 text-sm outline-none select-none',
            'focus-visible:ring-2 focus-visible:ring-ring/50',
            selected || menuOpen ? 'bg-accent' : 'hover:bg-accent/60',
            track.missing && 'opacity-50',
            gridColumns(isAlbum, showAlbum, maxPlays > 0),
          )}
        >
          {/* # / playing indicator / hover play button */}
          <div className="relative grid h-full place-items-center">
            <span
              className={cn(
                'tnum text-[13px] text-muted-foreground group-hover:invisible group-focus-within:invisible',
                current && 'invisible',
              )}
            >
              {trackNumberLabel(track, index, isAlbum)}
            </span>
            {current ? (
              <span className="absolute inset-0 grid place-items-center group-hover:invisible group-focus-within:invisible">
                <PlayingIndicator playing={state === 'playing'} />
              </span>
            ) : null}
            <button
              type="button"
              tabIndex={-1}
              aria-label={
                state === 'playing'
                  ? t('common:actions.pause')
                  : t('library:track.playTitle', { title: track.title })
              }
              onClick={(event) => {
                event.stopPropagation()
                primaryAction()
              }}
              onDoubleClick={(event) => event.stopPropagation()}
              className="invisible absolute inset-0 grid place-items-center text-foreground group-hover:visible group-focus-within:visible"
            >
              {state === 'playing' ? (
                <Pause className="size-4" fill="currentColor" strokeWidth={0} />
              ) : (
                <Play className="size-4" fill="currentColor" strokeWidth={0} />
              )}
            </button>
          </div>

          {/* title + artist */}
          <div className="flex min-w-0 items-center gap-3">
            {art ? <CoverArt coverArt={track.coverArt} size={36} rounded="sm" /> : null}
            <div className="min-w-0 flex-1">
              <div className={cn('truncate font-medium', current && 'text-primary')}>{track.title}</div>
              {showArtist ? (
                <div className="truncate text-[13px] text-muted-foreground">
                  {track.artistId ? (
                    <Link
                      to={`/artists/${track.artistId}`}
                      className="hover:text-foreground hover:underline"
                      onClick={(event) => event.stopPropagation()}
                      onDoubleClick={(event) => event.stopPropagation()}
                    >
                      {track.artist}
                    </Link>
                  ) : (
                    track.artist
                  )}
                </div>
              ) : null}
            </div>
          </div>

          {showAlbum ? (
            <div className="min-w-0 truncate text-[13px] text-muted-foreground">
              {track.albumId ? (
                <Link
                  to={`/albums/${track.albumId}`}
                  className="hover:text-foreground hover:underline"
                  onClick={(event) => event.stopPropagation()}
                  onDoubleClick={(event) => event.stopPropagation()}
                >
                  {track.album}
                </Link>
              ) : (
                track.album
              )}
            </div>
          ) : null}

          {isAlbum ? <div className="flex justify-center">{track.hasLyrics ? <LyricsMark /> : null}</div> : null}

          {isAlbum ? (
            <div className="tnum truncate text-xs text-muted-foreground" title={formatQuality([track])}>
              {trackQuality(track)}
            </div>
          ) : null}

          {maxPlays > 0 ? <PlaysBar plays={track.playCount} max={maxPlays} /> : null}

          <div className="tnum pr-1 text-right text-[13px] text-muted-foreground">{formatDuration(track.duration)}</div>

          <div
            className={cn(
              'flex justify-center',
              !track.starred && 'opacity-0 group-hover:opacity-100 group-focus-within:opacity-100',
            )}
          >
            <StarButton type="track" item={track} size="sm" silent />
          </div>

          <div className="flex justify-center opacity-0 group-hover:opacity-100 group-focus-within:opacity-100 has-[[data-state=open]]:opacity-100">
            <TrackActionsMenu tracks={[track]} context={context} />
          </div>
        </div>
      </ContextMenuTrigger>
      <ContextMenuContent className="min-w-56 rounded-xl">
        <ActionMenuContext.Provider value={contextValue}>
          <TrackActionItems tracks={[track]} context={context} />
        </ActionMenuContext.Provider>
      </ContextMenuContent>
    </ContextMenu>
  )
})

const MobileTrackRow = memo(function MobileTrackRow({
  track,
  index,
  art,
  isAlbum,
  albumArtist,
  hideArtist,
  playlist,
  last,
  onPlay,
}: RowProps & { last: boolean }) {
  const { t } = useTranslation()
  const state = useTrackState(track.id)
  const current = state !== 'none'
  const showArtist = !isAlbum || (!!track.artist && track.artist !== albumArtist)
  const quality = isAlbum ? trackQuality(track) : ''
  const subtitle = isAlbum
    ? showArtist
      ? track.artist
      : ''
    : [hideArtist ? '' : track.artist, track.album].filter(Boolean).join(' · ')

  return (
    <div
      className={cn(
        'bleed-x page-x relative flex h-full items-center transition-colors active:bg-accent/70',
        track.missing && 'opacity-50',
      )}
    >
      <button
        type="button"
        onClick={() => (current ? usePlayer.getState().togglePlay() : onPlay(index))}
        aria-current={current ? 'true' : undefined}
        className="flex h-full min-w-0 flex-1 items-center gap-3 text-left outline-none"
      >
        {art ? (
          <span className="relative shrink-0">
            <CoverArt coverArt={track.coverArt} size={44} />
            {current ? (
              <span className="absolute inset-0 grid place-items-center rounded-md bg-black/35">
                <PlayingIndicator playing={state === 'playing'} className="text-white" />
              </span>
            ) : null}
          </span>
        ) : (
          <span className="grid w-7 shrink-0 place-items-center">
            {current ? (
              <PlayingIndicator playing={state === 'playing'} />
            ) : (
              <span className="tnum text-[15px] text-muted-foreground">{trackNumberLabel(track, index, isAlbum)}</span>
            )}
          </span>
        )}
        <span className="min-w-0 flex-1">
          <span className={cn('block truncate text-[16px] leading-snug', current && 'text-primary')}>{track.title}</span>
          {isAlbum ? (
            subtitle || quality ? (
              // Artist · quality; the quality never truncates.
              <span className="flex min-w-0 items-center gap-1.5 text-[13px] leading-snug text-muted-foreground">
                {subtitle ? <span className="min-w-0 truncate">{subtitle}</span> : null}
                {subtitle && quality ? <span aria-hidden>·</span> : null}
                {quality ? <span className="shrink-0">{quality}</span> : null}
              </span>
            ) : null
          ) : showArtist && subtitle ? (
            <span className="block truncate text-[14px] leading-snug text-muted-foreground">{subtitle}</span>
          ) : null}
        </span>
      </button>
      {track.starred ? (
        <Star
          className="ml-2 size-3.5 shrink-0 text-primary"
          fill="currentColor"
          strokeWidth={1.75}
          role="img"
          aria-label={t('library:track.favorited')}
        />
      ) : null}
      {/* Lyrics column: next to the menu, so the marks line up whether or not the song is starred. */}
      {isAlbum ? <span className="ml-2 grid w-3.5 shrink-0 place-items-center">{track.hasLyrics ? <LyricsMark /> : null}</span> : null}
      <TrackActionsMenu tracks={[track]} context={playlistContext(playlist, index)} className="-mr-2.5" />
      {last ? null : (
        <Hairline inset={`calc(var(--page-px) + var(--safe-left) + ${art ? 56 : 40}px)`} />
      )}
    </div>
  )
})

function SkeletonRow({ mobile, art }: { mobile: boolean; art: boolean }) {
  return (
    <div className={cn('flex h-full items-center gap-3', !mobile && 'px-2')}>
      {!mobile ? <Skeleton className="h-3 w-5" /> : null}
      {art ? <Skeleton className={mobile ? 'size-11 rounded-md' : 'size-9 rounded-sm'} /> : null}
      <div className="flex flex-1 flex-col gap-1.5">
        <Skeleton className="h-3.5 w-2/5" />
        <Skeleton className="h-3 w-1/4" />
      </div>
    </div>
  )
}

/** Placeholder list while the first page loads. */
export function TrackListSkeleton({ rows = 8, art = true }: { rows?: number; art?: boolean }) {
  const isMobile = useIsMobile()
  return (
    <div aria-hidden className="w-full">
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} style={{ height: isMobile ? HEIGHT.mobileTrack : HEIGHT.desktopTrack }}>
          <SkeletonRow mobile={isMobile} art={art} />
        </div>
      ))}
    </div>
  )
}
