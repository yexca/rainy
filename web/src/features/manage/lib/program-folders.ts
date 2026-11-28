/** Program folders (docs/architecture/contract.md §9.5): which folders hold program episodes. */
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { api } from '@/lib/api/endpoints'
import type { Programs } from '@/lib/api/types'
import { toastError } from '@/lib/batch'

import { invalidateLibrary, manageKeys } from '../queries'
import { dirWithin } from './folder-explorer'

/**
 * How a folder relates to the programs: in a program library (any folder, the root included),
 * marked itself, inside a marked one, or neither.
 */
export type ProgramStatus = 'library' | 'marked' | 'inherited' | null

export function programStatus(programs: Programs, libraryId: number, dir: string): ProgramStatus {
  if (programs.libraries.includes(libraryId)) return 'library'
  if (!dir) return null
  let status: ProgramStatus = null
  for (const f of programs.folders) {
    if (f.libraryId !== libraryId) continue
    if (f.path === dir) return 'marked'
    if (dirWithin(dir, f.path)) status = 'inherited'
  }
  return status
}

const NO_PROGRAMS: Programs = { libraries: [], folders: [] }

/** The programs status of the library's folders and a mutation that marks or unmarks one. */
export function useProgramFolders(libraryId: number) {
  const { t } = useTranslation('manage')
  const queryClient = useQueryClient()
  const folders = useQuery({
    queryKey: manageKeys.programFolders,
    queryFn: ({ signal }) => api.manage.programFolders.list({ signal }),
    staleTime: 60_000,
  })
  const toggle = useMutation({
    mutationFn: ({ path, mark }: { path: string; mark: boolean }) =>
      mark
        ? api.manage.programFolders.mark({ libraryId, path })
        : api.manage.programFolders.unmark({ libraryId, path }),
    onSuccess: (next, { mark }) => {
      queryClient.setQueryData(manageKeys.programFolders, next)
      toast.success(mark ? t('folders.program.marked') : t('folders.program.unmarked'))
      void invalidateLibrary(queryClient)
    },
    onError: toastError,
  })
  const programs = folders.data ?? NO_PROGRAMS
  return {
    status: (dir: string) => programStatus(programs, libraryId, dir),
    toggle,
  }
}
