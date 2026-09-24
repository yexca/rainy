import { useQuery } from '@tanstack/react-query'
import { Disc3 } from 'lucide-react'
import { useCallback, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useParams } from 'react-router'

import { Page } from '@/components/page'
import { usePlayer } from '@/features/player/store'
import { useIsMobile } from '@/hooks/use-media-query'
import type { AlbumDetail, Track } from '@/lib/api/types'
import { formatDurationLong } from '@/lib/format'
import { currentLocale } from '@/lib/i18n'

import { AlbumCard } from '../components/album-card'
import { AlbumActionsMenu } from '../components/collection-actions'
import { ArtworkBackdrop, CollectionHero, CollectionHeroSkeleton } from '../components/collection-hero'
import { DetailNavBar } from '../components/detail-nav-bar'
import { HorizontalShelf } from '../components/horizontal-shelf'
import { PlayShuffleButtons } from '../components/play-shuffle-buttons'
import { QueryError } from '../components/query-error'
import { StarButton } from '../components/star-button'
import { TrackList, TrackListSkeleton, type TrackSection } from '../components/track-list'
import { formatGenres, formatQuality, formatReleaseDate, releaseDate, totalDuration } from '../lib/format'
import { albumListQuery, albumQuery } from '../lib/queries'

export default function AlbumPage() {
  const { t } = useTranslation('library')
  const { id = '' } = useParams()
  const isMobile = useIsMobile()
  const query = useQuery(albumQuery(id))
  const album = query.data
  const [titleEl, setTitleEl] = useState<HTMLHeadingElement | null>(null)

  return (
    <Page>
      <div className="relative isolate">
        {album ? <ArtworkBackdrop key={album.coverArt} coverArt={album.coverArt} className="bleed-x h-[34rem] md:h-[26rem]" /> : null}
        <DetailNavBar
          title={album?.name ?? ''}
          watch={titleEl}
          actions={
            album && isMobile ? (
              <>
                <StarButton type="album" item={album} size="lg" className="text-primary hover:text-primary/80" />
                <AlbumActionsMenu album={album} className="-mr-2.5 text-primary" />
              </>
            ) : undefined
          }
        />
        {query.isPending ? (
          <>
            <CollectionHeroSkeleton />
            <TrackListSkeleton rows={10} art={false} />
          </>
        ) : query.isError ? (
          <QueryError
            error={query.error}
            onRetry={() => void query.refetch()}
            retrying={query.isFetching}
            notFound={{
              icon: Disc3,
              title: t('album.notFound'),
              backTo: '/albums',
              backLabel: t('album.backToAlbums'),
            }}
          />
        ) : (
          <AlbumContent album={query.data} titleRef={setTitleEl} />
        )}
      </div>
    </Page>
  )
}

function AlbumContent({
  album,
  titleRef,
}: {
  album: AlbumDetail
  titleRef: (el: HTMLHeadingElement | null) => void
}) {
  const { t } = useTranslation('library')
  const tracks = album.tracks
  const multiDisc = album.discs.length > 1 || album.discCount > 1

  const sectionOf = useCallback(
    (track: Track): TrackSection => {
      const disc = track.discNumber || 1
      return {
        key: String(disc),
        label: (
          <span className="flex items-center gap-2">
            <Disc3 className="size-4" strokeWidth={1.75} aria-hidden />
            {t('album.disc', { number: disc })}
            {track.discSubtitle ? <span className="font-normal">· {track.discSubtitle}</span> : null}
          </span>
        ),
      }
    },
    [t],
  )

  const meta = [formatGenres(album.genre), album.year > 0 ? String(album.year) : ''].filter(Boolean).join(' · ')
  const quality = useMemo(() => formatQuality(tracks), [tracks])

  return (
    <>
      <CollectionHero
        coverArt={album.coverArt}
        kind={album.compilation ? t('album.compilation') : t('album.kind')}
        title={album.name}
        titleRef={titleRef}
        byline={
          album.artistId ? (
            <Link to={`/artists/${album.artistId}`} className="hover:underline">
              {album.artist}
            </Link>
          ) : (
            album.artist
          )
        }
        meta={<span className="uppercase md:normal-case">{meta}</span>}
        playButtons={(layout) => (
          <PlayShuffleButtons
            layout={layout}
            disabled={tracks.length === 0}
            onPlay={() => usePlayer.getState().playTracks(tracks, 0, { shuffle: false })}
            onShuffle={() => usePlayer.getState().playTracks(tracks, undefined, { shuffle: true })}
          />
        )}
        trailing={
          <>
            <StarButton type="album" item={album} />
            <AlbumActionsMenu album={album} className="md:size-9" />
          </>
        }
      />

      <TrackList
        tracks={tracks}
        variant="album"
        albumArtist={album.artist}
        sectionOf={multiDisc ? sectionOf : undefined}
      />

      <AlbumFooter album={album} quality={quality} />
      <MoreByArtist album={album} />
    </>
  )
}

function AlbumFooter({ album, quality }: { album: AlbumDetail; quality: string }) {
  const { t } = useTranslation('library')
  const date = releaseDate(album.tracks)
  const released = date ? formatReleaseDate(date, currentLocale()) : album.year > 0 ? String(album.year) : ''
  const duration = album.duration || totalDuration(album.tracks)
  return (
    <footer className="mt-6 space-y-0.5 text-[13px] text-muted-foreground md:px-2">
      {released ? <p>{released}</p> : null}
      <p>
        {t('songsAndDuration', {
          songs: t('common:count.songs', { count: album.tracks.length }),
          duration: formatDurationLong(duration),
        })}
      </p>
      {quality ? <p>{quality}</p> : null}
      {album.year > 0 && album.artist ? (
        <p className="pt-2 text-xs">
          ℗ {album.year} {album.artist}
        </p>
      ) : null}
    </footer>
  )
}

function MoreByArtist({ album }: { album: AlbumDetail }) {
  const { t } = useTranslation('library')
  const more = useQuery({
    ...albumListQuery({ artistId: album.artistId, sort: 'year', order: 'desc', limit: 16 }),
    enabled: !!album.artistId,
  })
  const others = (more.data?.items ?? []).filter((a) => a.id !== album.id)
  if (others.length === 0) return null
  return (
    <HorizontalShelf
      title={t('album.moreBy', { artist: album.artist })}
      to={`/artists/${album.artistId}`}
      className="mt-12 md:mt-14"
    >
      {others.map((other) => (
        <AlbumCard key={other.id} album={other} subtitle="year" />
      ))}
    </HorizontalShelf>
  )
}
