import { useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { Disc3, Music, Shapes } from 'lucide-react'
import { useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useParams, useSearchParams } from 'react-router'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent } from '@/components/ui/tabs'
import type { Album } from '@/lib/api/types'

import { ActionList, ActionMenu } from '../components/action-menu'
import { AlbumCard, AlbumCardSkeleton } from '../components/album-card'
import { CardGrid, VirtualCardGrid } from '../components/card-grid'
import { PlayShuffleButtons } from '../components/play-shuffle-buttons'
import { SegmentedTabs } from '../components/tab-bar'
import { TrackList, TrackListSkeleton } from '../components/track-list'
import { useCollectionQueueGroups } from '../lib/collection-action-items'
import { seedGradient } from '../lib/colors'
import { ALBUM_CAPTION_HEIGHT } from '../lib/layout'
import { usePlayCollection } from '../lib/play'
import { albumsInfiniteQuery, genresQuery, tracksInfiniteQuery, useFlattenedPages } from '../lib/queries'

type Tab = 'albums' | 'songs'

function GenreActionItems({ name }: { name: string }) {
  return <ActionList groups={useCollectionQueueGroups({ kind: 'genre', name })} />
}

export default function GenrePage() {
  const { t } = useTranslation('library')
  const { name = '' } = useParams()
  const [params, setParams] = useSearchParams()
  const tab: Tab = params.get('tab') === 'songs' ? 'songs' : 'albums'
  const play = usePlayCollection()
  const genres = useQuery(genresQuery())
  const genre = genres.data?.find((g) => g.name === name)
  const missing = !!genres.data && !genre

  const subtitle = genre
    ? [t('common:count.albums', { count: genre.albumCount }), t('common:count.songs', { count: genre.songCount })].join(
        ' · ',
      )
    : undefined

  return (
    <Page>
      <div className="relative isolate">
        <div
          aria-hidden
          hidden={missing}
          className="bleed-x pointer-events-none absolute inset-x-0 top-0 -z-10 h-72 opacity-25 [mask-image:linear-gradient(to_bottom,black,transparent)] dark:opacity-30"
          style={{ backgroundImage: seedGradient(name) }}
        />
        <PageHeader
          title={name}
          back
          subtitle={subtitle}
          navActions={
            missing ? undefined : (
              <ActionMenu label={t('common:actions.more')} className="text-primary md:text-muted-foreground">
                <GenreActionItems name={name} />
              </ActionMenu>
            )
          }
          actions={
            missing ? undefined : (
              <PlayShuffleButtons
                onPlay={() => play({ kind: 'genre', name }, 'play')}
                onShuffle={() => play({ kind: 'genre', name }, 'shuffle')}
              />
            )
          }
        />
        {missing ? (
          <EmptyState
            icon={Shapes}
            title={t('genre.notFound')}
            description={t('common:errors.not_found')}
            action={
              <Button variant="outline" asChild>
                <Link to="/genres">{t('genre.backToGenres')}</Link>
              </Button>
            }
          />
        ) : (
          <Tabs
            value={tab}
            onValueChange={(value) => setParams(value === 'songs' ? { tab: 'songs' } : {}, { replace: true })}
          >
            <SegmentedTabs<Tab>
              items={[
                { value: 'albums', label: t('genre.albums') },
                { value: 'songs', label: t('genre.songs') },
              ]}
            />
            <TabsContent value="albums">
              <GenreAlbums name={name} />
            </TabsContent>
            <TabsContent value="songs">
              <GenreSongs name={name} />
            </TabsContent>
          </Tabs>
        )}
      </div>
    </Page>
  )
}

function GenreAlbums({ name }: { name: string }) {
  const { t } = useTranslation('library')
  const query = useInfiniteQuery(albumsInfiniteQuery({ genre: name, sort: 'name' }))
  const { items, total } = useFlattenedPages(query.data, query.hasNextPage)
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = query
  const loadMore = useCallback(() => {
    if (hasNextPage && !isFetchingNextPage) void fetchNextPage()
  }, [hasNextPage, isFetchingNextPage, fetchNextPage])

  if (query.isPending) {
    return (
      <CardGrid>
        {Array.from({ length: 8 }, (_, i) => (
          <AlbumCardSkeleton key={i} />
        ))}
      </CardGrid>
    )
  }
  if (query.isError && items.length === 0) {
    return <ErrorState error={query.error} onRetry={() => void query.refetch()} retrying={query.isFetching} />
  }
  if (items.length === 0) {
    return <EmptyState icon={Disc3} size="compact" title={t('genre.noAlbums')} />
  }
  return (
    <VirtualCardGrid<Album>
      items={items}
      total={total}
      onEndReached={loadMore}
      getKey={(album) => album.id}
      captionHeight={ALBUM_CAPTION_HEIGHT}
      renderItem={(album, _i, width) => <AlbumCard album={album} sizeHint={width} />}
      renderSkeleton={() => <AlbumCardSkeleton />}
    />
  )
}

function GenreSongs({ name }: { name: string }) {
  const { t } = useTranslation('library')
  const query = useInfiniteQuery(tracksInfiniteQuery({ genre: name, sort: 'album' }))
  const { items, total } = useFlattenedPages(query.data, query.hasNextPage)
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = query
  const loadMore = useCallback(() => {
    if (hasNextPage && !isFetchingNextPage) void fetchNextPage()
  }, [hasNextPage, isFetchingNextPage, fetchNextPage])

  if (query.isPending) return <TrackListSkeleton rows={10} />
  if (query.isError && items.length === 0) {
    return <ErrorState error={query.error} onRetry={() => void query.refetch()} retrying={query.isFetching} />
  }
  return (
    <TrackList
      tracks={items}
      total={total}
      onEndReached={loadMore}
      empty={<EmptyState icon={Music} size="compact" title={t('genre.noSongs')} />}
    />
  )
}
