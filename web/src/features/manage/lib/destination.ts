/**
 * Where added music goes (Tracks → Upload, Links and Online share it, so switching tabs keeps
 * the chosen library and folder). Whether files are organized by tags is a server setting.
 */
import { useQuery } from '@tanstack/react-query'
import { create } from 'zustand'

import { DEFAULT_RENAME_PATTERN, useLibraries } from '@/features/admin/queries'
import type { LibraryInfo } from '@/lib/api/types'

import { placementQuery } from '../queries'

interface DestinationState {
  /** `null` = the first library. */
  libraryId: number | null
  /** Folder as typed (see `cleanDir`). */
  dir: string
  setLibraryId: (id: number | null) => void
  setDir: (dir: string) => void
}

export const useDestination = create<DestinationState>((set) => ({
  libraryId: null,
  dir: '',
  setLibraryId: (libraryId) => set({ libraryId }),
  setDir: (dir) => set({ dir }),
}))

/** Library-relative folder without surrounding slashes (backslashes become slashes). */
export function cleanDir(dir: string): string {
  return dir.trim().replace(/\\/g, '/').replace(/^\/+|\/+$/g, '')
}

export interface DestinationTarget {
  libraryId: number
  library: LibraryInfo | undefined
  /** Cleaned folder (`cleanDir`). */
  dir: string
  /** The folder has "." or ".." segments. */
  invalidDir: boolean
  /**
   * The server organizes added files by tags (`settings.organizeByTags`; assumed on until it
   * answers). Shown only: requests leave `organize` out, so the server decides.
   */
  organize: boolean
  /** The pattern organized files are placed by. */
  renamePattern: string
}

/** The destination as the API wants it. */
export function useDestinationTarget(): DestinationTarget {
  const { libraries } = useLibraries()
  const libraryId = useDestination((s) => s.libraryId)
  const dir = useDestination((s) => s.dir)
  const placement = useQuery({ ...placementQuery, staleTime: 60_000 }).data
  const effective = libraryId ?? libraries[0]?.id ?? 1
  const clean = cleanDir(dir)
  return {
    libraryId: effective,
    library: libraries.find((l) => l.id === effective),
    dir: clean,
    invalidDir: clean.split('/').some((p) => p === '..' || p === '.'),
    organize: placement?.organize ?? true,
    renamePattern: placement?.renamePattern || DEFAULT_RENAME_PATTERN,
  }
}
