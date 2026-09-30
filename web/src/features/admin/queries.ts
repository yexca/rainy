/**
 * React Query keys and shared queries of the admin area. Keys start with the `admin` root so
 * `library` server events refresh them (see `LIBRARY_QUERY_ROOTS`).
 */
import { queryOptions, useQuery } from '@tanstack/react-query'

import { useAuth } from '@/hooks/use-auth'
import { isApiError } from '@/lib/api/client'
import { api } from '@/lib/api/endpoints'
import type { LibraryInfo, Settings } from '@/lib/api/types'

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

/** Placeholder library used when a manager (non-admin) cannot list libraries. */
const FALLBACK_LIBRARY: LibraryInfo = {
  id: 1,
  name: '',
  path: '',
  createdAt: 0,
  updatedAt: 0,
  lastScanAt: 0,
  trackCount: 0,
  exists: true,
  writable: true,
}

export interface LibrariesState {
  libraries: LibraryInfo[]
  isLoading: boolean
  error: unknown
  /** Library details are unavailable to this user (non-admin managers); ids only. */
  limited: boolean
}

/**
 * Libraries for pickers (upload target, folder browser). The list endpoint is admin-only, so
 * managers fall back to the default library (#1).
 */
export function useLibraries(): LibrariesState {
  const { isAdmin } = useAuth()
  const query = useQuery({ ...librariesQuery, enabled: isAdmin })
  if (!isAdmin) return { libraries: [FALLBACK_LIBRARY], isLoading: false, error: null, limited: true }
  const forbidden = isApiError(query.error) && query.error.status === 403
  if (forbidden) return { libraries: [FALLBACK_LIBRARY], isLoading: false, error: null, limited: true }
  return { libraries: query.data ?? [], isLoading: query.isPending, error: query.error, limited: false }
}

/** Server settings when readable (admins); `undefined` otherwise. */
export function useOptionalSettings(): Settings | undefined {
  const { isAdmin } = useAuth()
  return useQuery({ ...settingsQuery, enabled: isAdmin, staleTime: 5 * 60_000 }).data
}

/** The configured rename pattern, or the built-in default for non-admins. */
export function useDefaultRenamePattern(): string {
  return useOptionalSettings()?.renamePattern || DEFAULT_RENAME_PATTERN
}
