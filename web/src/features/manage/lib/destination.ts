/**
 * Where added music goes (Tracks → Upload and Tracks → Online share it, so switching tabs keeps
 * the chosen library and folder). Only "organize by tags" is remembered across visits.
 */
import { create } from 'zustand'

import { useLibraries } from '@/features/admin/queries'
import type { LibraryInfo } from '@/lib/api/types'

const ORGANIZE_KEY = 'rainy.manage.uploadOrganize'

function loadOrganize(): boolean {
  try {
    return localStorage.getItem(ORGANIZE_KEY) === '1'
  } catch {
    return false
  }
}

interface DestinationState {
  /** `null` = the first library. */
  libraryId: number | null
  /** Folder as typed (see `cleanDir`). */
  dir: string
  organize: boolean
  setLibraryId: (id: number | null) => void
  setDir: (dir: string) => void
  setOrganize: (organize: boolean) => void
}

export const useDestination = create<DestinationState>((set) => ({
  libraryId: null,
  dir: '',
  organize: loadOrganize(),
  setLibraryId: (libraryId) => set({ libraryId }),
  setDir: (dir) => set({ dir }),
  setOrganize: (organize) => {
    try {
      localStorage.setItem(ORGANIZE_KEY, organize ? '1' : '0')
    } catch {
      // not remembered
    }
    set({ organize })
  },
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
  organize: boolean
}

/** The destination as the API wants it. */
export function useDestinationTarget(): DestinationTarget {
  const { libraries } = useLibraries()
  const libraryId = useDestination((s) => s.libraryId)
  const dir = useDestination((s) => s.dir)
  const organize = useDestination((s) => s.organize)
  const effective = libraryId ?? libraries[0]?.id ?? 1
  const clean = cleanDir(dir)
  return {
    libraryId: effective,
    library: libraries.find((l) => l.id === effective),
    dir: clean,
    invalidDir: clean.split('/').some((p) => p === '..' || p === '.'),
    organize,
  }
}
