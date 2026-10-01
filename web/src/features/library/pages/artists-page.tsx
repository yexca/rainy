import { useInfiniteQuery } from '@tanstack/react-query'
import { useWindowVirtualizer } from '@tanstack/react-virtual'
import { ChevronRight, MicVocal, Users } from 'lucide-react'
import { useCallback, useEffect, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useSearchParams } from 'react-router'

import { CoverArt } from '@/components/cover-art'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { Skeleton } from '@/components/ui/skeleton'
import { useIsMobile } from '@/hooks/use-media-query'
import type { ArtistSort, SortOrder } from '@/lib/api/endpoints'
import type { Artist } from '@/lib/api/types'
import { formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'

import { ArtistCard, ArtistCardSkeleton } from '../components/artist-card'
import { CardGrid } from '../components/card-grid'
import { Hairline } from '../components/hairline'
import { IndexRail } from '../components/index-rail'
import { FilterChip, ListToolbar, SortMenu, type SortOption } from '../components/list-toolbar'
import { ARTIST_CAPTION_HEIGHT } from '../lib/layout'
import { artistsInfiniteQuery, useFlattenedPages } from '../lib/queries'
import { useElementGeometry } from '../lib/use-element-geometry'

const SORTS: readonly ArtistSort[] = ['name', 'albumCount', 'songCount', 'recent', 'frequent', 'played']
const DEFAULT_ORDER: Partial<Record<ArtistSort, SortOrder>> = {
  albumCount: 'desc',
  songCount: 'desc',
  recent: 'desc',
  frequent: 'desc',
  played: 'desc',
}
const LETTERS = ['#', ...'ABCDEFGHIJKLMNOPQRSTUVWXYZ'] as const
/**
 * Desktop grids only split into letter sections (with the A–Z rail) for larger libraries: with
 * a handful of artists, one-card rows under every letter look broken.
 */
const GROUP_MIN_DESKTOP = 24
/** Space kept above a jumped-to section (sticky nav bar + breathing room). */
const JUMP_PADDING = 64

function isArtistSort(value: string | null): value is ArtistSort {
  return !!value && (SORTS as readonly string[]).includes(value)
}

type Row =
  | { type: 'header'; letter: string }
  | { type: 'artists'; artists: Artist[] }
  | { type: 'artist'; artist: Artist; last: boolean }

/** Bucket by index letter ('#', A–Z, then anything else), keeping the server order inside. */
function groupByLetter(artists: readonly Artist[]): [string, Artist[]][] {
  const groups = new Map<string, Artist[]>()
  for (const artist of artists) {
    const key = (artist.indexKey || '#').toUpperCase()
    const list = groups.get(key)
    if (list) list.push(artist)
    else groups.set(key, [artist])
  }
  const order = (key: string) => {
    const i = (LETTERS as readonly string[]).indexOf(key)
    return i === -1 ? LETTERS.length : i
  }
  return [...groups.entries()].sort((a, b) => order(a[0]) - order(b[0]) || a[0].localeCompare(b[0]))
}

export default function ArtistsPage() {
  const { t } = useTranslation('library')
  const isMobile = useIsMobile()
  const [params, setParams] = useSearchParams()
  const sortParam = params.get('sort')
  const sort: ArtistSort = isArtistSort(sortParam) ? sortParam : 'name'
  const orderParam = params.get('order')
  const order: SortOrder = orderParam === 'asc' || orderParam === 'desc' ? orderParam : (DEFAULT_ORDER[sort] ?? 'asc')
  const all = params.get('all') === '1'

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

  const query = useInfiniteQuery(artistsInfiniteQuery({ sort, order, all: all || undefined }))
  const { items, total } = useFlattenedPages(query.data, query.hasNextPage)
  // The A–Z index needs everything: keep loading pages in the background.
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = query
  useEffect(() => {
    if (hasNextPage && !isFetchingNextPage) void fetchNextPage()
  }, [hasNextPage, isFetchingNextPage, fetchNextPage])

  const grouped = sort === 'name' && (isMobile || items.length >= GROUP_MIN_DESKTOP)
  const sortOptions = useMemo<SortOption<ArtistSort>[]>(
    () => SORTS.map((value) => ({ value, label: t(`sort.artist.${value}`) })),
    [t],
  )

  return (
    <Page>
      <PageHeader
        title={t('common:nav.artists')}
        back={isMobile}
        subtitle={query.data ? t('artists.count', { count: total, formatted: formatNumber(total) }) : undefined}
      />
      <ListToolbar>
        <SortMenu
          options={sortOptions}
          value={sort}
          onChange={(value) => update({ sort: value === 'name' ? null : value, order: null })}
          order={order}
          onOrderChange={(value) => update({ order: value })}
        />
        <FilterChip
          active={all}
          onToggle={() => update({ all: all ? null : '1' })}
          icon={<Users className="size-3.5" strokeWidth={1.75} aria-hidden />}
        >
          {t('artists.includeTrackArtists')}
        </FilterChip>
      </ListToolbar>

      {query.isPending ? (
        isMobile ? (
          <div aria-hidden>
            {Array.from({ length: 10 }, (_, i) => (
              <div key={i} className="flex h-[60px] items-center gap-3">
                <Skeleton className="size-11 rounded-full" />
                <Skeleton className="h-4 w-1/2" />
              </div>
            ))}
          </div>
        ) : (
          <CardGrid density="artists">
            {Array.from({ length: 12 }, (_, i) => (
              <ArtistCardSkeleton key={i} />
            ))}
          </CardGrid>
        )
      ) : query.isError && items.length === 0 ? (
        <ErrorState error={query.error} onRetry={() => void query.refetch()} retrying={query.isFetching} />
      ) : items.length === 0 ? (
        <EmptyState icon={MicVocal} art="empty" title={t('artists.emptyTitle')} description={t('artists.emptyDescription')} />
      ) : (
        <ArtistsList artists={items} grouped={grouped} mobile={isMobile} />
      )}
    </Page>
  )
}

function ArtistsList({ artists, grouped, mobile }: { artists: Artist[]; grouped: boolean; mobile: boolean }) {
  const [ref, geometry] = useElementGeometry<HTMLDivElement>()
  const gap = 24
  const minWidth = 150
  const width = geometry.width || 900
  const columns = mobile ? 1 : Math.max(2, Math.floor((width + gap) / (minWidth + gap)))
  const cardWidth = mobile ? 0 : (width - gap * (columns - 1)) / columns
  const cardRowHeight = Math.round(cardWidth + ARTIST_CAPTION_HEIGHT + 28)
  const headerHeight = mobile ? 32 : 52
  const itemHeight = 60

  const rows = useMemo<Row[]>(() => {
    const out: Row[] = []
    const groups: [string | null, Artist[]][] = grouped ? groupByLetter(artists) : [[null, artists]]
    for (const [letter, list] of groups) {
      if (letter !== null) out.push({ type: 'header', letter })
      if (mobile) {
        list.forEach((artist, i) => out.push({ type: 'artist', artist, last: i === list.length - 1 }))
      } else {
        for (let i = 0; i < list.length; i += columns) out.push({ type: 'artists', artists: list.slice(i, i + columns) })
      }
    }
    return out
  }, [artists, grouped, mobile, columns])

  const letterRows = useMemo(() => {
    const map = new Map<string, number>()
    rows.forEach((row, i) => {
      if (row.type === 'header') map.set(row.letter, i)
    })
    return map
  }, [rows])

  const virtualizer = useWindowVirtualizer({
    count: rows.length,
    estimateSize: (i) => {
      const row = rows[i]
      if (row?.type === 'header') return headerHeight
      if (row?.type === 'artists') return cardRowHeight
      return itemHeight
    },
    overscan: mobile ? 12 : 3,
    scrollMargin: geometry.top,
    scrollPaddingStart: JUMP_PADDING,
  })

  useEffect(() => {
    virtualizer.measure()
  }, [virtualizer, cardRowHeight, columns, mobile])

  const jump = (letter: string) => {
    const index = letterRows.get(letter)
    if (index !== undefined) virtualizer.scrollToIndex(index, { align: 'start' })
  }

  const showRail = grouped && letterRows.size > 1 && artists.length > 20

  return (
    <div className={cn('relative', showRail && 'pr-5 md:pr-8')}>
      <div ref={ref} className="relative w-full" style={{ height: virtualizer.getTotalSize() }}>
        {virtualizer.getVirtualItems().map((item) => {
          const row = rows[item.index]
          if (!row) return null
          const style = { height: item.size, transform: `translateY(${item.start - virtualizer.options.scrollMargin}px)` }
          if (row.type === 'header') {
            return (
              <div
                key={item.key}
                className={cn(
                  'absolute inset-x-0 top-0 flex items-end',
                  mobile ? 'pb-1 text-[15px] font-semibold text-muted-foreground' : 'pb-3 text-xl font-semibold',
                )}
                style={style}
                id={`artists-${row.letter}`}
              >
                {row.letter}
              </div>
            )
          }
          if (row.type === 'artists') {
            return (
              <div
                key={item.key}
                className="absolute inset-x-0 top-0 grid"
                style={{
                  ...style,
                  height: item.size - 28,
                  gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))`,
                  columnGap: gap,
                }}
              >
                {row.artists.map((artist) => (
                  <ArtistCard key={artist.id} artist={artist} sizeHint={cardWidth} />
                ))}
              </div>
            )
          }
          return (
            <div key={item.key} className="absolute inset-x-0 top-0" style={style}>
              <ArtistRow artist={row.artist} last={row.last} />
            </div>
          )
        })}
      </div>
      {showRail ? (
        <IndexRail
          letters={LETTERS}
          available={new Set(letterRows.keys())}
          onJump={jump}
          className="fixed top-1/2 right-[calc(var(--safe-right)+2px)] z-20 -translate-y-1/2 md:right-[calc(var(--safe-right)+10px)]"
        />
      ) : null}
    </div>
  )
}

function ArtistRow({ artist, last }: { artist: Artist; last: boolean }) {
  const { t } = useTranslation()
  return (
    <Link
      to={`/artists/${artist.id}`}
      className="relative flex h-full items-center gap-3 transition-colors outline-none active:bg-accent/70"
    >
      <CoverArt coverArt={artist.coverArt} size={44} shape="circle" icon={MicVocal} />
      <span className="min-w-0 flex-1">
        <span className="block truncate text-[17px] leading-snug">{artist.name}</span>
        <span className="block truncate text-[13px] text-muted-foreground">
          {artist.albumCount > 0
            ? t('common:count.albums', { count: artist.albumCount })
            : t('common:count.songs', { count: artist.songCount })}
        </span>
      </span>
      <ChevronRight className="size-4 shrink-0 text-muted-foreground/60" aria-hidden />
      {last ? null : <Hairline inset="56px" />}
    </Link>
  )
}
