/**
 * Query definitions for the library feature. Every key starts with a root from
 * `LIBRARY_QUERY_ROOTS` (see `@/lib/query-keys`) so `library` server events refresh them.
 *
 *   ['home']                                  Home
 *   ['albums', id] / ['albums', 'list', p]    album detail / paged album lists
 *   ['artists', id] / ['artists', 'list', p]  artist detail / paged artist lists
 *   ['tracks', 'list', p]                     paged track lists
 *   ['genres'], ['search', q], ['starred'], ['playlists'], ['playlists', id]
 *   ['recommend', 'daily', day, tz]          daily mix (the server keeps it for the day)
 *   ['radios']                                (not library data; invalidated by the radio page)
 */
import { infiniteQueryOptions, keepPreviousData, queryOptions } from '@tanstack/react-query'
import { useMemo } from 'react'

import {
  api,
  type AlbumListParams,
  type ArtistListParams,
  type SearchParams,
  type TrackListParams,
} from '@/lib/api/endpoints'
import type { Page } from '@/lib/api/types'
import { queryKeys } from '@/lib/query-keys'

/** Albums per page for infinite grids (divisible by 2–6 so rows fill up evenly). */
export const ALBUM_PAGE_SIZE = 120
/** Tracks per page for infinite lists. */
export const TRACK_PAGE_SIZE = 200
/** Artists per page (the artists page loads everything for the A–Z index). */
export const ARTIST_PAGE_SIZE = 1000

type Paged = Omit<AlbumListParams | ArtistListParams | TrackListParams, 'offset' | 'limit'>

/** Next offset for an offset/limit paged endpoint, or `undefined` when everything is loaded. */
export function nextOffset<T>(lastPage: Page<T>, allPages: Page<T>[]): number | undefined {
  const loaded = allPages.reduce((n, p) => n + p.items.length, 0)
  if (lastPage.items.length === 0 || loaded >= lastPage.total) return undefined
  return loaded
}

/** Drop `undefined` / empty values so equal filters share one cache entry. */
function clean<T extends Paged>(params: T): T {
  const out: Record<string, unknown> = {}
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === '' || value === false) continue
    out[key] = value
  }
  return out as T
}

export const homeQuery = () =>
  queryOptions({
    queryKey: queryKeys.home,
    queryFn: ({ signal }) => api.home({ signal }),
  })

/** The browser's IANA time zone ('' when unknown: the server then uses UTC). */
export function browserTimeZone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || ''
  } catch {
    return ''
  }
}

/** Local day (`2026-10-01`) so the daily mix query changes at midnight. */
function localDay(now: Date = new Date()): string {
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`
}

export const dailyMixQuery = () => {
  const tz = browserTimeZone()
  return queryOptions({
    queryKey: [...queryKeys.recommend, 'daily', localDay(), tz],
    queryFn: ({ signal }) => api.recommend.daily({ tz }, { signal }),
    staleTime: 5 * 60_000,
  })
}

export const albumQuery = (id: string) =>
  queryOptions({
    queryKey: queryKeys.album(id),
    queryFn: ({ signal }) => api.albums.get(id, { signal }),
    staleTime: 60_000,
  })

export const artistQuery = (id: string) =>
  queryOptions({
    queryKey: queryKeys.artist(id),
    queryFn: ({ signal }) => api.artists.get(id, { signal }),
    staleTime: 60_000,
  })

export const playlistQuery = (id: string) =>
  queryOptions({
    queryKey: queryKeys.playlist(id),
    queryFn: ({ signal }) => api.playlists.get(id, { signal }),
  })

export const playlistsQuery = () =>
  queryOptions({
    queryKey: queryKeys.playlists,
    queryFn: ({ signal }) => api.playlists.list({ signal }),
  })

export const genresQuery = () =>
  queryOptions({
    queryKey: queryKeys.genres,
    queryFn: ({ signal }) => api.genres({ signal }),
    staleTime: 5 * 60_000,
  })

export const starredQuery = () =>
  queryOptions({
    queryKey: queryKeys.starred,
    queryFn: ({ signal }) => api.starred({ signal }),
  })

export const radiosQuery = () =>
  queryOptions({
    queryKey: queryKeys.radios,
    queryFn: ({ signal }) => api.radios.list({ signal }),
  })

export const searchQuery = (params: SearchParams) =>
  queryOptions({
    queryKey: [...queryKeys.search(params.q.trim()), params.artists ?? 0, params.albums ?? 0, params.tracks ?? 0],
    queryFn: ({ signal }) => api.search({ ...params, q: params.q.trim() }, { signal }),
    enabled: params.q.trim().length > 0,
    placeholderData: keepPreviousData,
    staleTime: 60_000,
  })

/** A single page of albums (shelves such as "More by this artist"). */
export const albumListQuery = (params: AlbumListParams) =>
  queryOptions({
    queryKey: [...queryKeys.albums, 'list', clean(params), params.offset ?? 0, params.limit ?? 0],
    queryFn: ({ signal }) => api.albums.list(params, { signal }),
  })

export const albumsInfiniteQuery = (params: Omit<AlbumListParams, 'offset' | 'limit'>) =>
  infiniteQueryOptions({
    queryKey: [...queryKeys.albums, 'list', clean(params), 'infinite'],
    queryFn: ({ signal, pageParam }) =>
      api.albums.list({ ...params, offset: pageParam, limit: ALBUM_PAGE_SIZE }, { signal }),
    initialPageParam: 0,
    getNextPageParam: nextOffset,
    // Random order must not reshuffle when refetched in the background.
    staleTime: params.sort === 'random' ? Number.POSITIVE_INFINITY : 30_000,
  })

export const artistsInfiniteQuery = (params: Omit<ArtistListParams, 'offset' | 'limit'>) =>
  infiniteQueryOptions({
    queryKey: [...queryKeys.artists, 'list', clean(params), 'infinite'],
    queryFn: ({ signal, pageParam }) =>
      api.artists.list({ ...params, offset: pageParam, limit: ARTIST_PAGE_SIZE }, { signal }),
    initialPageParam: 0,
    getNextPageParam: nextOffset,
  })

export const tracksInfiniteQuery = (params: Omit<TrackListParams, 'offset' | 'limit'>) =>
  infiniteQueryOptions({
    queryKey: [...queryKeys.tracks, 'list', clean(params), 'infinite'],
    queryFn: ({ signal, pageParam }) =>
      api.tracks.list({ ...params, offset: pageParam, limit: TRACK_PAGE_SIZE }, { signal }),
    initialPageParam: 0,
    getNextPageParam: nextOffset,
    staleTime: params.sort === 'random' ? Number.POSITIVE_INFINITY : 30_000,
  })

/**
 * Flatten infinite pages into one list + the server total. Items that shifted into a later page
 * while paging (library changed in between) are de-duplicated by id.
 */
export function flattenPages<T extends { id: string }>(
  data: { pages: Page<T>[] } | undefined,
  hasNextPage: boolean,
): { items: T[]; total: number } {
  if (!data || data.pages.length === 0) return { items: [], total: 0 }
  const seen = new Set<string>()
  const items: T[] = []
  for (const page of data.pages) {
    for (const item of page.items) {
      if (seen.has(item.id)) continue
      seen.add(item.id)
      items.push(item)
    }
  }
  // Once everything is loaded the list is complete, whatever `total` said.
  if (!hasNextPage) return { items, total: items.length }
  const last = data.pages[data.pages.length - 1]
  return { items, total: Math.max(last.total, items.length) }
}

/**
 * Memoized {@link flattenPages}: the flattened list keeps its identity until the pages change, so
 * memoized rows / cards and callbacks derived from it don't re-render on unrelated updates.
 */
export function useFlattenedPages<T extends { id: string }>(
  data: { pages: Page<T>[] } | undefined,
  hasNextPage: boolean,
): { items: T[]; total: number } {
  return useMemo(() => flattenPages(data, hasNextPage), [data, hasNextPage])
}
