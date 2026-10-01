import { useInfiniteQuery } from '@tanstack/react-query'
import { Music, Star } from 'lucide-react'
import { useCallback, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useSearchParams } from 'react-router'
import { toast } from 'sonner'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { Button } from '@/components/ui/button'
import { usePlayer } from '@/features/player/store'
import { useIsMobile } from '@/hooks/use-media-query'
import { api, type SortOrder, type TrackSort } from '@/lib/api/endpoints'
import { errorMessage } from '@/lib/errors'
import { formatNumber } from '@/lib/format'

import { FilterChip, ListToolbar, SortMenu, type SortOption } from '../components/list-toolbar'
import { PlayShuffleButtons } from '../components/play-shuffle-buttons'
import { TrackList, TrackListSkeleton } from '../components/track-list'
import { MAX_QUEUE, usePlayCollection } from '../lib/play'
import { tracksInfiniteQuery, useFlattenedPages } from '../lib/queries'

const SORTS: readonly TrackSort[] = ['title', 'artist', 'album', 'recent', 'year', 'duration', 'frequent', 'played', 'starred']
const DEFAULT_ORDER: Partial<Record<TrackSort, SortOrder>> = {
  recent: 'desc',
  year: 'desc',
  frequent: 'desc',
  played: 'desc',
  starred: 'desc',
}

function isTrackSort(value: string | null): value is TrackSort {
  return !!value && (SORTS as readonly string[]).includes(value)
}

export default function SongsPage() {
  const { t } = useTranslation('library')
  const isMobile = useIsMobile()
  const [params, setParams] = useSearchParams()
  const play = usePlayCollection()
  const sortParam = params.get('sort')
  const sort: TrackSort = isTrackSort(sortParam) ? sortParam : 'title'
  const orderParam = params.get('order')
  const order: SortOrder = orderParam === 'asc' || orderParam === 'desc' ? orderParam : (DEFAULT_ORDER[sort] ?? 'asc')
  const starred = params.get('starred') === '1'

  const update = useCallback(
    (patch: Record<string, string | null>) => {
      setParams(
        (prev) => {
          const next = new URLSearchParams(prev)
          for (const [key, value] of Object.entries(patch)) {
            if (value === null) next.delete(key)
            else next.set(key, value)
          }
          return next
        },
        { replace: true },
      )
    },
    [setParams],
  )

  const sortOptions = useMemo<SortOption<TrackSort>[]>(
    () => SORTS.map((value) => ({ value, label: t(`sort.track.${value}`) })),
    [t],
  )

  const query = useInfiniteQuery(tracksInfiniteQuery({ sort, order, starred: starred || undefined }))
  const { items, total } = useFlattenedPages(query.data, query.hasNextPage)
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = query
  const loadMore = useCallback(() => {
    if (hasNextPage && !isFetchingNextPage) void fetchNextPage()
  }, [hasNextPage, isFetchingNextPage, fetchNextPage])

  // "Play" queues the whole list in the current order (up to MAX_QUEUE), not just the loaded pages.
  const playAll = async () => {
    try {
      const tracks = hasNextPage
        ? (await api.tracks.list({ sort, order, starred: starred || undefined, limit: MAX_QUEUE })).items
        : items
      usePlayer.getState().playTracks(tracks, 0, { shuffle: false })
    } catch (error) {
      toast.error(errorMessage(error, t))
    }
  }

  return (
    <Page>
      <PageHeader
        title={t('common:nav.songs')}
        back={isMobile}
        subtitle={query.data ? t('songs.count', { count: total, formatted: formatNumber(total) }) : undefined}
        actions={
          items.length > 0 ? (
            <PlayShuffleButtons
              onPlay={playAll}
              onShuffle={() => (starred ? play({ kind: 'starred' }, 'shuffle') : play({ kind: 'library' }, 'shuffle'))}
            />
          ) : undefined
        }
      />
      <ListToolbar>
        <SortMenu
          options={sortOptions}
          value={sort}
          onChange={(value) => update({ sort: value === 'title' ? null : value, order: null })}
          order={order}
          onOrderChange={(value) => update({ order: value })}
        />
        <FilterChip
          active={starred}
          onToggle={() => update({ starred: starred ? null : '1' })}
          icon={<Star className="size-3.5" fill={starred ? 'currentColor' : 'none'} strokeWidth={1.75} aria-hidden />}
        >
          {t('filter.favorites')}
        </FilterChip>
      </ListToolbar>

      {query.isPending ? (
        <TrackListSkeleton rows={14} />
      ) : query.isError && items.length === 0 ? (
        <ErrorState error={query.error} onRetry={() => void query.refetch()} retrying={query.isFetching} />
      ) : items.length === 0 ? (
        <EmptyState
          icon={Music}
          art="empty"
          title={starred ? t('songs.emptyFavoritesTitle') : t('songs.emptyTitle')}
          description={starred ? t('songs.emptyFavoritesDescription') : t('songs.emptyDescription')}
          action={
            starred ? (
              <Button variant="outline" onClick={() => update({ starred: null })}>
                {t('filter.reset')}
              </Button>
            ) : null
          }
        />
      ) : (
        <TrackList tracks={items} total={total} onEndReached={loadMore} />
      )}
    </Page>
  )
}
