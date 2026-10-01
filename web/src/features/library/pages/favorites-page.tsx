import { useQuery } from '@tanstack/react-query'
import { Disc3, MicVocal, Star } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useSearchParams } from 'react-router'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { Tabs, TabsContent } from '@/components/ui/tabs'
import { usePlayer } from '@/features/player/store'
import { useIsMobile } from '@/hooks/use-media-query'

import { AlbumCard, AlbumCardSkeleton } from '../components/album-card'
import { ArtistCard, ArtistCardSkeleton } from '../components/artist-card'
import { CardGrid } from '../components/card-grid'
import { PlayShuffleButtons } from '../components/play-shuffle-buttons'
import { SegmentedTabs } from '../components/tab-bar'
import { TrackList, TrackListSkeleton } from '../components/track-list'
import { starredQuery } from '../lib/queries'

type Tab = 'songs' | 'albums' | 'artists'
const TABS: readonly Tab[] = ['songs', 'albums', 'artists']

export default function FavoritesPage() {
  const { t } = useTranslation('library')
  const isMobile = useIsMobile()
  const [params, setParams] = useSearchParams()
  const tabParam = params.get('tab') as Tab | null
  const tab: Tab = tabParam && TABS.includes(tabParam) ? tabParam : 'songs'
  const query = useQuery(starredQuery())
  const data = query.data

  const tracks = data?.tracks ?? []

  return (
    <Page>
      <PageHeader
        title={t('common:nav.favorites')}
        back={isMobile}
        subtitle={
          data
            ? [
                t('common:count.songs', { count: data.tracks.length }),
                t('common:count.albums', { count: data.albums.length }),
                t('common:count.artists', { count: data.artists.length }),
              ].join(' · ')
            : undefined
        }
        actions={
          tab === 'songs' && tracks.length > 0 ? (
            <PlayShuffleButtons
              onPlay={() => usePlayer.getState().playTracks(tracks, 0, { shuffle: false })}
              onShuffle={() => usePlayer.getState().playTracks(tracks, undefined, { shuffle: true })}
            />
          ) : undefined
        }
      />
      <Tabs value={tab} onValueChange={(value) => setParams(value === 'songs' ? {} : { tab: value }, { replace: true })}>
        <SegmentedTabs<Tab> items={TABS.map((value) => ({ value, label: t(`favorites.${value}`) }))} />

        {query.isPending ? (
          tab === 'songs' ? (
            <TrackListSkeleton rows={10} />
          ) : (
            <CardGrid density={tab === 'artists' ? 'artists' : 'albums'}>
              {Array.from({ length: 8 }, (_, i) =>
                tab === 'artists' ? <ArtistCardSkeleton key={i} /> : <AlbumCardSkeleton key={i} />,
              )}
            </CardGrid>
          )
        ) : query.isError ? (
          <ErrorState error={query.error} onRetry={() => void query.refetch()} retrying={query.isFetching} />
        ) : (
          <>
            <TabsContent value="songs">
              <TrackList
                tracks={tracks}
                empty={
                  <EmptyState icon={Star} art="empty" title={t('favorites.emptySongs')} description={t('favorites.emptyHint')} />
                }
              />
            </TabsContent>
            <TabsContent value="albums">
              {data && data.albums.length > 0 ? (
                <CardGrid>
                  {data.albums.map((album) => (
                    <AlbumCard key={album.id} album={album} />
                  ))}
                </CardGrid>
              ) : (
                <EmptyState icon={Disc3} art="empty" title={t('favorites.emptyAlbums')} description={t('favorites.emptyHint')} />
              )}
            </TabsContent>
            <TabsContent value="artists">
              {data && data.artists.length > 0 ? (
                <CardGrid density="artists">
                  {data.artists.map((artist) => (
                    <ArtistCard key={artist.id} artist={artist} />
                  ))}
                </CardGrid>
              ) : (
                <EmptyState icon={MicVocal} art="empty" title={t('favorites.emptyArtists')} description={t('favorites.emptyHint')} />
              )}
            </TabsContent>
          </>
        )}
      </Tabs>
    </Page>
  )
}
