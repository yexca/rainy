/**
 * React Query keys + helpers of the manage area. Every key starts with a library root
 * (`manage` or `tracks`) so `library` server events refresh it.
 */
import type { QueryClient } from '@tanstack/react-query'

import type { TrackListParams } from '@/lib/api/endpoints'
import type { IssueType } from '@/lib/api/types'
import { LIBRARY_QUERY_ROOTS } from '@/lib/query-keys'

export const manageKeys = {
  all: ['manage'] as const,
  trash: ['manage', 'trash'] as const,
  folders: (libraryId: number, dir: string) => ['manage', 'folders', libraryId, dir] as const,
  issuesSummary: ['manage', 'issues', 'summary'] as const,
  issues: (type: IssueType) => ['manage', 'issues', 'list', type] as const,
  log: (trackId: string) => ['manage', 'log', trackId] as const,
  downloads: ['manage', 'downloads'] as const,
  /** Paged track table of the library manager (`tracks` root). */
  trackPage: (params: TrackListParams, page: number) => ['tracks', 'manage', params, page] as const,
  trackTotal: (params: TrackListParams) => ['tracks', 'manage', params] as const,
  track: (id: string) => ['tracks', id] as const,
}

/** Refresh everything derived from the library (after edits, deletes, uploads, …). */
export function invalidateLibrary(queryClient: QueryClient): Promise<void> {
  return queryClient.invalidateQueries({
    predicate: (query) => LIBRARY_QUERY_ROOTS.has(String(query.queryKey[0])),
  })
}
