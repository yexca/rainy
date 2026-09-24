import { useQuery } from '@tanstack/react-query'
import { Shapes } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useSearchParams } from 'react-router'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { Skeleton } from '@/components/ui/skeleton'
import { useIsMobile } from '@/hooks/use-media-query'
import { formatNumber } from '@/lib/format'

import { GenreTile } from '../components/genre-tile'
import { ListToolbar, SortMenu, type SortOption } from '../components/list-toolbar'
import { genresQuery } from '../lib/queries'

type GenreSort = 'name' | 'songs' | 'albums'
const SORTS: readonly GenreSort[] = ['name', 'songs', 'albums']
const GRID = 'grid grid-cols-2 gap-3 sm:grid-cols-3 md:gap-4 lg:grid-cols-4 xl:grid-cols-5'

export default function GenresPage() {
  const { t } = useTranslation('library')
  const isMobile = useIsMobile()
  const [params, setParams] = useSearchParams()
  const sortParam = params.get('sort')
  const sort: GenreSort = SORTS.includes(sortParam as GenreSort) ? (sortParam as GenreSort) : 'name'
  const query = useQuery(genresQuery())

  const genres = useMemo(() => {
    const list = (query.data ?? []).filter((g) => g.songCount > 0)
    if (sort === 'songs') return [...list].sort((a, b) => b.songCount - a.songCount || a.name.localeCompare(b.name))
    if (sort === 'albums') return [...list].sort((a, b) => b.albumCount - a.albumCount || a.name.localeCompare(b.name))
    return list
  }, [query.data, sort])

  const sortOptions = useMemo<SortOption<GenreSort>[]>(
    () => SORTS.map((value) => ({ value, label: t(`sort.genre.${value}`) })),
    [t],
  )

  return (
    <Page>
      <PageHeader
        title={t('common:nav.genres')}
        back={isMobile}
        subtitle={
          query.data ? t('genres.count', { count: genres.length, formatted: formatNumber(genres.length) }) : undefined
        }
      />
      <ListToolbar>
        <SortMenu
          options={sortOptions}
          value={sort}
          onChange={(value) => setParams(value === 'name' ? {} : { sort: value }, { replace: true })}
        />
      </ListToolbar>
      {query.isPending ? (
        <div className={GRID} aria-hidden>
          {Array.from({ length: 12 }, (_, i) => (
            <Skeleton key={i} className="aspect-[16/10] rounded-xl" />
          ))}
        </div>
      ) : query.isError ? (
        <ErrorState error={query.error} onRetry={() => void query.refetch()} retrying={query.isFetching} />
      ) : genres.length === 0 ? (
        <EmptyState icon={Shapes} title={t('genres.emptyTitle')} description={t('genres.emptyDescription')} />
      ) : (
        <div className={GRID}>
          {genres.map((genre) => (
            <GenreTile key={genre.id} genre={genre} />
          ))}
        </div>
      )}
    </Page>
  )
}
