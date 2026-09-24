/**
 * Random-access paging over `/api/tracks` for the virtualized manager table: only the pages
 * covering the visible rows are fetched (so jumping to the end of a 50k-track library doesn't
 * load everything in between).
 */
import { useQueries, useQueryClient, type QueryClient } from '@tanstack/react-query'
import { useCallback } from 'react'

import { api, type TrackListParams } from '@/lib/api/endpoints'
import type { Page, Track } from '@/lib/api/types'

import { manageKeys } from '../queries'

export const PAGE_SIZE = 100
const FETCH_ALL_LIMIT = 1000

/** Query options for one page of the manager table (page 0 also yields the total). */
export function trackPageQuery(params: TrackListParams, page: number) {
  return {
    queryKey: manageKeys.trackPage(params, page),
    queryFn: ({ signal }: { signal: AbortSignal }) =>
      api.tracks.list({ ...params, offset: page * PAGE_SIZE, limit: PAGE_SIZE }, { signal }),
    staleTime: 30_000,
  }
}

export interface PagedRows {
  /** Row at `index`, `undefined` while its page loads. */
  rowAt: (index: number) => Track | undefined
  /** Make sure rows `[from, to]` are loaded (e.g. before a shift-range selection) and return them with their indexes. */
  ensureRange: (from: number, to: number) => Promise<{ index: number; track: Track }[]>
}

/** Fetch (and subscribe to) the pages covering `visibleIndexes`. */
export function usePagedRows(params: TrackListParams, visibleIndexes: readonly number[]): PagedRows {
  const queryClient = useQueryClient()
  const pages = new Set<number>()
  for (const i of visibleIndexes) pages.add(Math.floor(i / PAGE_SIZE))
  // Subscribing re-renders the table when a visible page arrives.
  useQueries({ queries: [...pages].sort((a, b) => a - b).map((p) => trackPageQuery(params, p)) })

  const rowAt = useCallback(
    (index: number) => {
      const page = queryClient.getQueryData<Page<Track>>(manageKeys.trackPage(params, Math.floor(index / PAGE_SIZE)))
      return page?.items[index % PAGE_SIZE]
    },
    [queryClient, params],
  )

  const ensureRange = useCallback(
    async (from: number, to: number) => {
      const lo = Math.max(0, Math.min(from, to))
      const hi = Math.max(from, to)
      const needed: number[] = []
      for (let p = Math.floor(lo / PAGE_SIZE); p <= Math.floor(hi / PAGE_SIZE); p++) needed.push(p)
      await Promise.all(needed.map((p) => queryClient.ensureQueryData(trackPageQuery(params, p))))
      const out: { index: number; track: Track }[] = []
      for (let i = lo; i <= hi; i++) {
        const track = rowAt(i)
        if (track) out.push({ index: i, track })
      }
      return out
    },
    [queryClient, params, rowAt],
  )

  return { rowAt, ensureRange }
}

/** Every track matching `params` in table order (for "select all matching" actions). */
export async function fetchAllMatching(queryClient: QueryClient, params: TrackListParams): Promise<Track[]> {
  const out: Track[] = []
  for (let offset = 0; ; offset += FETCH_ALL_LIMIT) {
    const page = await queryClient.fetchQuery({
      queryKey: ['tracks', 'manage', 'all', params, offset],
      queryFn: ({ signal }) => api.tracks.list({ ...params, offset, limit: FETCH_ALL_LIMIT }, { signal }),
      staleTime: 10_000,
    })
    out.push(...page.items)
    if (page.items.length < FETCH_ALL_LIMIT || out.length >= page.total) return out
  }
}
