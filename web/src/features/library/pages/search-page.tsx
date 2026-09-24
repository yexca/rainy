import { useQuery } from '@tanstack/react-query'
import { Clock, SearchX, X } from 'lucide-react'
import { useEffect, useMemo, useState, type MouseEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useSearchParams } from 'react-router'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { Button } from '@/components/ui/button'
import { usePlayer } from '@/features/player/store'
import { useIsMobile } from '@/hooks/use-media-query'
import type { Track } from '@/lib/api/types'
import { cn } from '@/lib/utils'

import { AlbumCard } from '../components/album-card'
import { ArtistCard } from '../components/artist-card'
import { GenreTile } from '../components/genre-tile'
import { HorizontalShelf } from '../components/horizontal-shelf'
import { SearchField } from '../components/search-field'
import { SectionHeader } from '../components/section-header'
import { TrackList, TrackListSkeleton } from '../components/track-list'
import { genresQuery, searchQuery } from '../lib/queries'
import { addRecentSearch, clearRecentSearches, removeRecentSearch, useRecentSearches } from '../lib/recent-searches'

const DEBOUNCE_MS = 250
const SONGS_COLLAPSED = 8
const BROWSE_GENRES = 12

export default function SearchPage() {
  const { t } = useTranslation('library')
  const isMobile = useIsMobile()
  const [params, setParams] = useSearchParams()
  const q = params.get('q') ?? ''
  const [input, setInput] = useState(q)
  // URL → field (back / forward, links). Updates we make ourselves set `syncedQ` first.
  const [syncedQ, setSyncedQ] = useState(q)
  if (q !== syncedQ) {
    setSyncedQ(q)
    setInput(q)
  }

  useEffect(() => {
    const next = input.trim()
    if (next === q.trim()) return
    const timer = window.setTimeout(() => {
      setSyncedQ(next)
      setParams(next ? { q: next } : {}, { replace: true })
    }, DEBOUNCE_MS)
    return () => window.clearTimeout(timer)
  }, [input, q, setParams])

  const term = q.trim()
  const results = useQuery(searchQuery({ q: term, artists: 12, albums: 18, tracks: 50 }))
  const data = term ? results.data : undefined
  const stale = results.isPlaceholderData

  const remember = () => addRecentSearch(term)
  // Clicking anything in the results (open, play, menus) remembers the search.
  const onResultsClick = (event: MouseEvent) => {
    if ((event.target as HTMLElement).closest('a,button')) remember()
  }

  const setQuery = (value: string) => {
    setInput(value)
    setSyncedQ(value.trim())
    setParams(value.trim() ? { q: value.trim() } : {}, { replace: true })
  }

  return (
    <Page>
      <PageHeader title={t('common:nav.search')}>
        <SearchField
          value={input}
          onChange={setInput}
          onSubmit={(value) => {
            setQuery(value)
            addRecentSearch(value)
          }}
          onCancel={() => setQuery('')}
          placeholder={t('search.placeholder')}
          autoFocus={!isMobile && !q}
          className="mb-5 max-w-2xl md:mb-6"
        />
      </PageHeader>

      {!term ? (
        <SearchIdle onPick={(value) => setQuery(value)} />
      ) : results.isPending ? (
        <TrackListSkeleton rows={8} />
      ) : results.isError && !data ? (
        <ErrorState error={results.error} onRetry={() => void results.refetch()} retrying={results.isFetching} />
      ) : data && data.artists.length + data.albums.length + data.tracks.length === 0 ? (
        <EmptyState
          icon={SearchX}
          title={t('search.noResultsTitle', { query: term })}
          description={t('search.noResultsDescription')}
        />
      ) : data ? (
        <div
          onClickCapture={onResultsClick}
          className={cn('transition-opacity duration-150', stale && 'opacity-60')}
          aria-busy={stale}
        >
          {data.artists.length > 0 ? (
            <HorizontalShelf title={t('search.artists')} itemSize="sm">
              {data.artists.map((artist) => (
                <ArtistCard key={artist.id} artist={artist} />
              ))}
            </HorizontalShelf>
          ) : null}
          {data.tracks.length > 0 ? <SongResults key={term} tracks={data.tracks} /> : null}
          {data.albums.length > 0 ? (
            <HorizontalShelf title={t('search.albums')}>
              {data.albums.map((album) => (
                <AlbumCard key={album.id} album={album} subtitle="artistYear" />
              ))}
            </HorizontalShelf>
          ) : null}
        </div>
      ) : null}
    </Page>
  )
}

function SongResults({ tracks }: { tracks: Track[] }) {
  const { t } = useTranslation('library')
  const [expanded, setExpanded] = useState(false)
  const shown = expanded ? tracks : tracks.slice(0, SONGS_COLLAPSED)
  return (
    <section className="mt-8 first:mt-2 md:mt-10">
      <SectionHeader
        title={t('search.songs')}
        actions={
          tracks.length > SONGS_COLLAPSED ? (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setExpanded((v) => !v)}
              className="text-primary hover:bg-primary/10 hover:text-primary"
            >
              {expanded ? t('actions.showLess') : t('actions.showAll', { count: tracks.length })}
            </Button>
          ) : null
        }
      />
      {/* The queue is every matching song, not only the visible ones. */}
      <TrackList
        tracks={shown}
        showHeader={false}
        onPlay={(index) => usePlayer.getState().playTracks(tracks, index)}
      />
    </section>
  )
}

function SearchIdle({ onPick }: { onPick: (value: string) => void }) {
  const { t } = useTranslation('library')
  const recent = useRecentSearches()
  const genres = useQuery(genresQuery())
  const browse = useMemo(
    () => [...(genres.data ?? [])].sort((a, b) => b.songCount - a.songCount).slice(0, BROWSE_GENRES),
    [genres.data],
  )

  return (
    <>
      {recent.length > 0 ? (
        <section className="mb-8">
          <SectionHeader
            title={t('search.recent')}
            actions={
              <Button
                variant="ghost"
                size="sm"
                onClick={clearRecentSearches}
                className="text-primary hover:bg-primary/10 hover:text-primary"
              >
                {t('common:actions.clear')}
              </Button>
            }
          />
          <ul className="bleed-x md:mx-0">
            {recent.map((term) => (
              <li
                key={term}
                className="hairline-inset page-x flex items-center [--hairline-inset:calc(var(--page-px)+var(--safe-left)+2.25rem)] md:px-0 md:[--hairline-inset:2.25rem]"
              >
                <button
                  type="button"
                  onClick={() => onPick(term)}
                  className="flex h-12 min-w-0 flex-1 items-center gap-3 text-left text-[17px] outline-none hover:text-primary focus-visible:text-primary md:h-11 md:text-sm"
                >
                  <Clock className="size-5 shrink-0 text-muted-foreground md:size-4" strokeWidth={1.75} aria-hidden />
                  <span className="truncate">{term}</span>
                </button>
                <button
                  type="button"
                  aria-label={t('search.removeRecent', { term })}
                  onClick={() => removeRecentSearch(term)}
                  className="-mr-2 grid size-11 place-items-center rounded-full text-muted-foreground hover:text-foreground md:size-9"
                >
                  <X className="size-4" aria-hidden />
                </button>
              </li>
            ))}
          </ul>
        </section>
      ) : null}

      {browse.length > 0 ? (
        <section>
          <SectionHeader title={t('search.browse')} to="/genres" />
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:gap-4 lg:grid-cols-4">
            {browse.map((genre) => (
              <GenreTile key={genre.id} genre={genre} />
            ))}
          </div>
        </section>
      ) : null}
    </>
  )
}
