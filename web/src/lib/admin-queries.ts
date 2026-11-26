/**
 * React Query keys and queries of the admin API, shared by the admin and manage areas. Keys start
 * with the `admin` root so `library` server events refresh them (see `LIBRARY_QUERY_ROOTS`).
 */
import { queryOptions } from '@tanstack/react-query'

import { api } from '@/lib/api/endpoints'

export const adminKeys = {
  all: ['admin'] as const,
  users: ['admin', 'users'] as const,
  libraries: ['admin', 'libraries'] as const,
  scan: ['admin', 'scan'] as const,
  settings: ['admin', 'settings'] as const,
  stats: ['admin', 'stats'] as const,
  system: ['admin', 'system'] as const,
  ytdlp: ['admin', 'ytdlp'] as const,
  sources: ['admin', 'sources'] as const,
  scrobbling: ['admin', 'scrobbling'] as const,
}

export const librariesQuery = queryOptions({
  queryKey: adminKeys.libraries,
  queryFn: ({ signal }) => api.admin.libraries.list({ signal }),
})

export const settingsQuery = queryOptions({
  queryKey: adminKeys.settings,
  queryFn: ({ signal }) => api.admin.settings.get({ signal }),
})

export const sourcesQuery = queryOptions({
  queryKey: adminKeys.sources,
  queryFn: ({ signal }) => api.admin.sources.get({ signal }),
})

export const scrobblingQuery = queryOptions({
  queryKey: adminKeys.scrobbling,
  queryFn: ({ signal }) => api.admin.scrobbling.get({ signal }),
})

export const ytdlpQuery = queryOptions({
  queryKey: adminKeys.ytdlp,
  queryFn: ({ signal }) => api.admin.ytdlp.get({ signal }),
})

/** Default rename pattern (mirrors `model.DefaultSettings`). */
export const DEFAULT_RENAME_PATTERN = '{albumartist}/{album}/[{disc}-]{track:2} {title}'

