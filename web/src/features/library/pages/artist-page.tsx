import { useQuery } from '@tanstack/react-query'
import { MicVocal } from 'lucide-react'
import { useCallback, useMemo, useState, type CSSProperties } from 'react'
import { useTranslation } from 'react-i18next'
import { useParams } from 'react-router'

import { Page } from '@/components/page'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { usePlayer } from '@/features/player/store'
import { useIsDesktop, useIsMobile } from '@/hooks/use-media-query'
import type { ArtistDetail, Track } from '@/lib/api/types'
import { cn } from '@/lib/utils'

import { AlbumCard } from '../components/album-card'
import { CardGrid } from '../components/card-grid'
import { ArtistActionsMenu } from '../components/collection-actions'
import { ArtworkBackdrop, CollectionHero, CollectionHeroSkeleton } from '../components/collection-hero'
import { DetailNavBar } from '../components/detail-nav-bar'
import { HorizontalShelf } from '../components/horizontal-shelf'
import { PlayShuffleButtons } from '../components/play-shuffle-buttons'
import { QueryError } from '../components/query-error'
import { SectionHeader } from '../components/section-header'
import { SongRow } from '../components/song-shelf'
import { StarButton } from '../components/star-button'
import { TrackList, TrackListSkeleton } from '../components/track-list'
import { usePlayCollection } from '../lib/play'
import { artistQuery } from '../lib/queries'
import { useCoverTheme } from '../lib/use-cover-theme'

/** Top songs shown before "Show all" on phones and tablets. */
const TOP_COLLAPSED = 5

export default function ArtistPage() {
  const { t } = useTranslation('library')
  const { id = '' } = useParams()
  const isMobile = useIsMobile()
  const isDesktop = useIsDesktop()
  const query = useQuery(artistQuery(id))
  const artist = query.data
  const [titleEl, setTitleEl] = useState<HTMLHeadingElement | null>(null)
  const coverTheme = useCoverTheme(artist?.coverArt, query.isPending)

  return (
    // Like album pages, the app takes on the artwork's colours in immersive mode.
    <Page {...coverTheme}>
      <div className="relative isolate">
        {artist ? (
          <ArtworkBackdrop key={artist.coverArt} coverArt={artist.coverArt} className="bleed-x h-[30rem] md:h-[24rem]" />
        ) : null}
        <DetailNavBar
          title={artist?.name ?? ''}
          watch={titleEl}
          actions={
            artist && isMobile ? (
              <>
                <StarButton type="artist" item={artist} size="lg" className="text-primary hover:text-primary/80" />
                <ArtistActionsMenu artist={artist} className="-mr-2.5 text-primary" />
              </>
            ) : undefined
          }
        />
        {query.isPending ? (
          <>
            <CollectionHeroSkeleton shape="circle" />
            {isDesktop ? <TopSongsGridSkeleton /> : <TrackListSkeleton rows={5} />}
          </>
        ) : query.isError ? (
          <QueryError
            error={query.error}
            onRetry={() => void query.refetch()}
            retrying={query.isFetching}
            notFound={{
              icon: MicVocal,
              title: t('artist.notFound'),
              backTo: '/artists',
              backLabel: t('artist.backToArtists'),
            }}
          />
        ) : (
          <ArtistContent artist={query.data} titleRef={setTitleEl} />
        )}
      </div>
    </Page>
  )
}

function ArtistContent({
  artist,
  titleRef,
}: {
  artist: ArtistDetail
  titleRef: (el: HTMLHeadingElement | null) => void
}) {
  const { t } = useTranslation('library')
  const play = usePlayCollection()
  const isDesktop = useIsDesktop()
  const [expanded, setExpanded] = useState(false)
  const top = artist.topTracks
  const shownTop = expanded ? top : top.slice(0, TOP_COLLAPSED)
  const albums = useMemo(
    () => [...artist.albums].sort((a, b) => b.year - a.year || a.name.localeCompare(b.name)),
    [artist.albums],
  )
  // The counts cover music only; an artist known only from programs counts its shows instead.
  const meta = [
    artist.albumCount > 0 ? t('common:count.albums', { count: artist.albumCount }) : '',
    artist.songCount > 0 || artist.programs.length === 0 ? t('common:count.songs', { count: artist.songCount }) : '',
    artist.songCount === 0 && artist.programs.length > 0 ? t('programs.count', { count: artist.programs.length }) : '',
  ]
    .filter(Boolean)
    .join(' · ')

  return (
    <>
      <CollectionHero
        coverArt={artist.coverArt}
        shape="circle"
        icon={MicVocal}
        kind={t('artist.kind')}
        title={artist.name}
        titleRef={titleRef}
        meta={meta}
        playButtons={(layout) => (
          <PlayShuffleButtons
            layout={layout}
            // Program episodes are not played with the artist's songs.
            disabled={top.length === 0}
            onPlay={() => play({ kind: 'artist', id: artist.id }, 'play')}
            onShuffle={() => play({ kind: 'artist', id: artist.id }, 'shuffle')}
          />
        )}
        trailing={
          <>
            <StarButton type="artist" item={artist} />
            <ArtistActionsMenu artist={artist} className="md:size-9" />
          </>
        }
      />

      {top.length > 0 ? (
        <section className="mt-2">
          <SectionHeader
            title={t('artist.topSongs')}
            actions={
              !isDesktop && top.length > TOP_COLLAPSED ? (
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setExpanded((v) => !v)}
                  className="text-primary hover:bg-primary/10 hover:text-primary"
                >
                  {expanded ? t('actions.showLess') : t('actions.showAll', { count: top.length })}
                </Button>
              ) : null
            }
          />
          {isDesktop ? (
            <TopSongsGrid tracks={top} />
          ) : (
            <TrackList
              tracks={shownTop}
              showHeader={false}
              hideArtist
              onPlay={(index) => usePlayer.getState().playTracks(top, index)}
            />
          )}
        </section>
      ) : null}

      {albums.length > 0 ? (
        <section className="mt-10">
          <SectionHeader title={t('artist.albums')} />
          <CardGrid>
            {albums.map((album) => (
              <AlbumCard key={album.id} album={album} subtitle="year" />
            ))}
          </CardGrid>
        </section>
      ) : null}

      {artist.programs.length > 0 ? (
        <section className={albums.length > 0 || top.length > 0 ? 'mt-10' : 'mt-2'}>
          <SectionHeader title={t('artist.programs')} />
          <CardGrid>
            {artist.programs.map((show) => (
              <AlbumCard key={show.id} album={show} subtitle="year" />
            ))}
          </CardGrid>
        </section>
      ) : null}

      {artist.appearsOn.length > 0 ? (
        <HorizontalShelf title={t('artist.appearsOn')} className="mt-12">
          {artist.appearsOn.map((album) => (
            <AlbumCard key={album.id} album={album} subtitle="artistYear" />
          ))}
        </HorizontalShelf>
      ) : null}
    </>
  )
}

/**
 * Desktop top songs: every song in columns filled top to bottom (two, three on wide screens, like
 * the home song shelves), so the ranks read down one column and then the next. A row plays all
 * top songs from that song.
 */
function TopSongsGrid({ tracks }: { tracks: Track[] }) {
  const play = useCallback((index: number) => usePlayer.getState().playTracks(tracks, index), [tracks])
  const rows = {
    '--rows-2': Math.ceil(tracks.length / 2),
    '--rows-3': Math.ceil(tracks.length / 3),
  } as CSSProperties
  return (
    <ul
      className="grid grid-flow-col grid-cols-2 grid-rows-[repeat(var(--rows-2),auto)] gap-x-5 xl:grid-cols-3 xl:grid-rows-[repeat(var(--rows-3),auto)]"
      style={rows}
    >
      {tracks.map((track, index) => (
        <li key={track.id} className="min-w-0">
          <SongRow
            track={track}
            index={index}
            onPlay={play}
            rank={index + 1}
            subtitle={[track.album, track.year > 0 ? String(track.year) : ''].filter(Boolean).join(' · ')}
            star
          />
        </li>
      ))}
    </ul>
  )
}

function TopSongsGridSkeleton() {
  return (
    <div aria-hidden className="mt-2">
      <Skeleton className="mb-4 h-6 w-36" />
      <div className="grid grid-cols-2 gap-x-5 xl:grid-cols-3">
        {Array.from({ length: 3 }, (_, c) => (
          <div key={c} className={cn(c === 2 && 'hidden xl:block')}>
            {Array.from({ length: 5 }, (_, r) => (
              <div key={r} className={cn('flex h-14 items-center gap-3', r === 4 && 'xl:hidden')}>
                <Skeleton className="h-3 w-5" />
                <Skeleton className="size-11 rounded-md" />
                <div className="flex flex-1 flex-col gap-1.5">
                  <Skeleton className="h-3.5 w-2/5" />
                  <Skeleton className="h-3 w-1/4" />
                </div>
              </div>
            ))}
          </div>
        ))}
      </div>
    </div>
  )
}
