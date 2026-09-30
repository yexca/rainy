import { useQuery } from '@tanstack/react-query'
import { MicVocal } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useParams } from 'react-router'

import { Page } from '@/components/page'
import { Button } from '@/components/ui/button'
import { usePlayer } from '@/features/player/store'
import { useIsMobile } from '@/hooks/use-media-query'
import type { ArtistDetail } from '@/lib/api/types'

import { AlbumCard } from '../components/album-card'
import { CardGrid } from '../components/card-grid'
import { ArtistActionsMenu } from '../components/collection-actions'
import { ArtworkBackdrop, CollectionHero, CollectionHeroSkeleton } from '../components/collection-hero'
import { DetailNavBar } from '../components/detail-nav-bar'
import { HorizontalShelf } from '../components/horizontal-shelf'
import { PlayShuffleButtons } from '../components/play-shuffle-buttons'
import { QueryError } from '../components/query-error'
import { SectionHeader } from '../components/section-header'
import { StarButton } from '../components/star-button'
import { TrackList, TrackListSkeleton } from '../components/track-list'
import { usePlayCollection } from '../lib/play'
import { artistQuery } from '../lib/queries'
import { useCoverTheme } from '../lib/use-cover-theme'

const TOP_COLLAPSED = 5

export default function ArtistPage() {
  const { t } = useTranslation('library')
  const { id = '' } = useParams()
  const isMobile = useIsMobile()
  const query = useQuery(artistQuery(id))
  const artist = query.data
  const [titleEl, setTitleEl] = useState<HTMLHeadingElement | null>(null)
  const coverTheme = useCoverTheme(artist?.coverArt)

  return (
    // Like album pages, the page takes on the artwork's colours.
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
            <TrackListSkeleton rows={5} />
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
  const [expanded, setExpanded] = useState(false)
  const top = artist.topTracks
  const shownTop = expanded ? top : top.slice(0, TOP_COLLAPSED)
  const albums = useMemo(
    () => [...artist.albums].sort((a, b) => b.year - a.year || a.name.localeCompare(b.name)),
    [artist.albums],
  )
  const meta = [
    artist.albumCount > 0 ? t('common:count.albums', { count: artist.albumCount }) : '',
    t('common:count.songs', { count: artist.songCount }),
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
            disabled={artist.songCount === 0}
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
              top.length > TOP_COLLAPSED ? (
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
          <TrackList
            tracks={shownTop}
            showHeader={false}
            hideArtist
            onPlay={(index) => usePlayer.getState().playTracks(top, index)}
          />
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
