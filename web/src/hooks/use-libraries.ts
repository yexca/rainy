/** Library and server-settings hooks shared by the admin and manage areas. */
import { useQuery } from '@tanstack/react-query'

import { useAuth } from '@/hooks/use-auth'
import { DEFAULT_RENAME_PATTERN, librariesQuery, settingsQuery } from '@/lib/admin-queries'
import { isApiError } from '@/lib/api/client'
import type { LibraryInfo, Settings } from '@/lib/api/types'

/** Placeholder library used when a manager (non-admin) cannot list libraries. */
const FALLBACK_LIBRARY: LibraryInfo = {
  id: 1,
  name: '',
  path: '',
  program: false,
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
