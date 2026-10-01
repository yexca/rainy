import { useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { Disc3, Star } from 'lucide-react'
import { useCallback, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useSearchParams } from 'react-router'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { Button } from '@/components/ui/button'
import { useIsMobile } from '@/hooks/use-media-query'
import type { AlbumSort, SortOrder } from '@/lib/api/endpoints'
import type { Album } from '@/lib/api/types'
import { formatNumber } from '@/lib/format'

import { AlbumCard, AlbumCardSkeleton } from '../components/album-card'
import { CardGrid, VirtualCardGrid } from '../components/card-grid'
import { ChoiceFilter, FilterChip, ListToolbar, SortMenu, type SortOption } from '../components/list-toolbar'
import { ALBUM_CAPTION_HEIGHT } from '../lib/layout'
import { albumsInfiniteQuery, genresQuery, useFlattenedPages } from '../lib/queries'

const SORTS: readonly AlbumSort[] = ['recent', 'name', 'artist', 'year', 'played', 'frequent', 'starred', 'random']

/** Natural order for each field when none is given (newest / most first for time & counts). */
const DEFAULT_ORDER: Partial<Record<AlbumSort, SortOrder>> = {
  recent: 'desc',
  year: 'desc',
  played: 'desc',
  frequent: 'desc',
  starred: 'desc',
}

function isAlbumSort(value: string | null): value is AlbumSort {
  return !!value && (SORTS as readonly string[]).includes(value)
}

export default function AlbumsPage() {
  const { t } = useTranslation('library')
  const isMobile = useIsMobile()
  const [params, setParams] = useSearchParams()
  const sortParam = params.get('sort')
  const sort: AlbumSort = isAlbumSort(sortParam) ? sortParam : 'recent'
  const orderParam = params.get('order')
  const order: SortOrder = orderParam === 'asc' || orderParam === 'desc' ? orderParam : (DEFAULT_ORDER[sort] ?? 'asc')
  const starred = params.get('starred') === '1'
  const genre = params.get('genre')

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

  const sortOptions = useMemo<SortOption<AlbumSort>[]>(
    () => SORTS.map((value) => ({ value, label: t(`sort.album.${value}`) })),
    [t],
  )

  const genres = useQuery(genresQuery())
  const genreNames = useMemo(() => (genres.data ?? []).filter((g) => g.albumCount > 0).map((g) => g.name), [genres.data])

  const query = useInfiniteQuery(
    albumsInfiniteQuery({ sort, order, starred: starred || undefined, genre: genre ?? undefined }),
  )
  const { items, total } = useFlattenedPages(query.data, query.hasNextPage)
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = query
  const loadMore = useCallback(() => {
    if (hasNextPage && !isFetchingNextPage) void fetchNextPage()
  }, [hasNextPage, isFetchingNextPage, fetchNextPage])

  const filtered = starred || genre !== null

  return (
    <Page>
      <PageHeader
        title={t('common:nav.albums')}
        back={isMobile}
        subtitle={query.data ? t('albums.count', { count: total, formatted: formatNumber(total) }) : undefined}
      />
      <ListToolbar>
        <SortMenu
          options={sortOptions}
          value={sort}
          onChange={(value) => update({ sort: value, order: null })}
          order={sort === 'random' ? undefined : order}
          onOrderChange={(value) => update({ order: value })}
        />
        <FilterChip
          active={starred}
          onToggle={() => update({ starred: starred ? null : '1' })}
          icon={<Star className="size-3.5" fill={starred ? 'currentColor' : 'none'} strokeWidth={1.75} aria-hidden />}
        >
          {t('filter.favorites')}
        </FilterChip>
        {genreNames.length > 0 ? (
          <ChoiceFilter
            label={t('filter.genre')}
            value={genre}
            options={genreNames}
            onChange={(value) => update({ genre: value })}
            allLabel={t('filter.allGenres')}
          />
        ) : null}
      </ListToolbar>

      {query.isPending ? (
        <CardGrid>
          {Array.from({ length: 12 }, (_, i) => (
            <AlbumCardSkeleton key={i} />
          ))}
        </CardGrid>
      ) : query.isError && items.length === 0 ? (
        <ErrorState error={query.error} onRetry={() => void query.refetch()} retrying={query.isFetching} />
      ) : items.length === 0 ? (
        <EmptyState
          icon={Disc3}
          art="empty"
          title={filtered ? t('albums.emptyFilteredTitle') : t('albums.emptyTitle')}
          description={filtered ? t('albums.emptyFilteredDescription') : t('albums.emptyDescription')}
          action={
            filtered ? (
              <Button variant="outline" onClick={() => update({ starred: null, genre: null })}>
                {t('filter.reset')}
              </Button>
            ) : null
          }
        />
      ) : (
        <VirtualCardGrid<Album>
          items={items}
          total={total}
          onEndReached={loadMore}
          getKey={(album) => album.id}
          captionHeight={ALBUM_CAPTION_HEIGHT}
          renderItem={(album, _index, width) => (
            <AlbumCard album={album} sizeHint={width} subtitle={sort === 'year' ? 'artistYear' : 'artist'} />
          )}
          renderSkeleton={() => <AlbumCardSkeleton />}
        />
      )}
    </Page>
  )
}
