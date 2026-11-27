import { useQueryClient } from '@tanstack/react-query'
import { Loader2, MoreHorizontal, Play } from 'lucide-react'
import { memo, type PointerEvent, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'

import { CoverArt } from '@/components/cover-art'
import { Skeleton } from '@/components/ui/skeleton'
import type { Album } from '@/lib/api/types'
import { cn } from '@/lib/utils'

import { ALBUM_CAPTION_HEIGHT } from '../lib/layout'
import { usePlayCollection } from '../lib/play'
import { albumQuery, artistQuery } from '../lib/queries'
import { coverSeedsQuery, useImmersive } from '../lib/use-cover-theme'
import { AlbumActionsMenu } from './collection-actions'

export type AlbumCardSubtitle = 'artist' | 'year' | 'artistYear'

export interface AlbumCardProps {
  album: Album
  /** Second caption line (default: artist). */
  subtitle?: AlbumCardSubtitle
  /** Approximate rendered width, picks the cover resolution. */
  sizeHint?: number
  priority?: boolean
  className?: string
}

const PREFETCH_DELAY = 150

/**
 * Album tile: square artwork with title + artist below. The artwork and title open the album; the
 * artist name opens the artist. Desktop hover reveals a play button and a "…" menu over the
 * artwork; phones get press feedback. Hovering or pressing prefetches the page a link opens (and,
 * in immersive mode, the album cover's colours).
 */
export const AlbumCard = memo(function AlbumCard({
  album,
  subtitle = 'artist',
  sizeHint = 200,
  priority,
  className,
}: AlbumCardProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const play = usePlayCollection()
  const [starting, setStarting] = useState(false)
  const [menuOpen, setMenuOpen] = useState(false)
  const prefetchTimer = useRef<number | undefined>(undefined)
  const showArtist = subtitle !== 'year' && !!album.artist
  const year = subtitle !== 'artist' && album.year > 0 ? String(album.year) : ''
  const immersive = useImmersive()

  useEffect(() => () => window.clearTimeout(prefetchTimer.current), [])

  // In immersive mode the cover's colours are sampled ahead too, so the album opens in them.
  const prefetchAlbum = () => {
    void queryClient.prefetchQuery(albumQuery(album.id))
    if (immersive && album.coverArt) void queryClient.prefetchQuery(coverSeedsQuery(album.coverArt))
  }
  const prefetchArtist = () => void queryClient.prefetchQuery(artistQuery(album.artistId))

  // Mouse hover prefetches after a short delay; a press prefetches at once.
  const hoverPrefetch = (event: PointerEvent, prefetch: () => void) => {
    if (event.pointerType !== 'mouse') return
    window.clearTimeout(prefetchTimer.current)
    prefetchTimer.current = window.setTimeout(prefetch, PREFETCH_DELAY)
  }
  const cancelPrefetch = () => window.clearTimeout(prefetchTimer.current)

  const onPlay = () => {
    setStarting(true)
    void play({ kind: 'album', id: album.id }).finally(() => setStarting(false))
  }

  return (
    <div className={cn('group/card relative min-w-0', className)}>
      <Link
        to={`/albums/${album.id}`}
        className="block rounded-lg outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
        onPointerEnter={(event) => hoverPrefetch(event, prefetchAlbum)}
        onPointerDown={prefetchAlbum}
        onPointerLeave={cancelPrefetch}
      >
        <div className="transition-transform duration-150 ease-out active:scale-[0.97] md:active:scale-100">
          <CoverArt coverArt={album.coverArt} size={sizeHint} fluid alt={album.name} priority={priority} />
        </div>
      </Link>
      <div className="mt-2 min-w-0" style={{ height: ALBUM_CAPTION_HEIGHT - 8 }}>
        {/* The artwork link is the album's tab stop and accessible name; the title repeats it for pointers. */}
        <Link
          to={`/albums/${album.id}`}
          tabIndex={-1}
          aria-hidden
          className="block truncate text-[14px] leading-5 font-medium md:text-[13px]"
          onPointerEnter={(event) => hoverPrefetch(event, prefetchAlbum)}
          onPointerDown={prefetchAlbum}
          onPointerLeave={cancelPrefetch}
        >
          {album.name}
        </Link>
        {showArtist || year ? (
          <p className="truncate text-[14px] leading-5 text-muted-foreground md:text-[13px]">
            {showArtist && album.artistId ? (
              <Link
                to={`/artists/${album.artistId}`}
                className="rounded-sm outline-none hover:text-foreground hover:underline focus-visible:ring-[3px] focus-visible:ring-ring/50"
                onPointerEnter={(event) => hoverPrefetch(event, prefetchArtist)}
                onPointerDown={prefetchArtist}
                onPointerLeave={cancelPrefetch}
              >
                {album.artist}
              </Link>
            ) : showArtist ? (
              album.artist
            ) : null}
            {showArtist && year ? ' · ' : null}
            {year}
          </p>
        ) : null}
      </div>

      {/* Desktop hover controls over the artwork (hidden on touch-only devices). */}
      <div
        className={cn(
          'pointer-events-none absolute inset-x-0 top-0 hidden aspect-square rounded-lg bg-linear-to-t from-black/45 via-black/5 to-transparent opacity-0 transition-opacity duration-200 [@media(hover:hover)]:block',
          'group-hover/card:opacity-100 group-focus-within/card:opacity-100',
          (menuOpen || starting) && 'opacity-100',
        )}
      >
        <button
          type="button"
          onClick={onPlay}
          aria-label={t('library:album.playAlbum', { name: album.name })}
          className="pointer-events-auto absolute bottom-2.5 left-2.5 grid size-10 place-items-center rounded-full bg-primary text-primary-foreground shadow-lg transition-transform outline-none hover:scale-105 focus-visible:ring-[3px] focus-visible:ring-white/70 active:scale-95"
        >
          {starting ? (
            <Loader2 className="size-5 animate-spin" aria-hidden />
          ) : (
            <Play className="ml-0.5 size-5" fill="currentColor" strokeWidth={0} aria-hidden />
          )}
        </button>
        <div className="pointer-events-auto absolute right-2.5 bottom-2.5">
          <AlbumActionsMenu
            album={album}
            responsive={false}
            onOpenChange={setMenuOpen}
            trigger={
              <button
                type="button"
                aria-label={t('common:actions.more')}
                className="grid size-8 place-items-center rounded-full bg-black/45 text-white backdrop-blur-md transition-colors outline-none hover:bg-black/60 focus-visible:ring-[3px] focus-visible:ring-white/70"
              >
                <MoreHorizontal className="size-4" strokeWidth={2} />
              </button>
            }
          />
        </div>
      </div>
    </div>
  )
})

export function AlbumCardSkeleton({ className }: { className?: string }) {
  return (
    <div className={cn('min-w-0', className)} aria-hidden>
      <Skeleton className="aspect-square w-full rounded-lg" />
      <div className="mt-2 flex flex-col gap-1.5" style={{ height: ALBUM_CAPTION_HEIGHT - 8 }}>
        <Skeleton className="h-3.5 w-4/5" />
        <Skeleton className="h-3 w-1/2" />
      </div>
    </div>
  )
}
