import { useQuery } from '@tanstack/react-query'
import { ChevronRight } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Spinner } from '@/components/spinner'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { adminKeys } from '@/lib/admin-queries'
import { api } from '@/lib/api/endpoints'
import type { FolderEntry } from '@/lib/api/types'
import { cn } from '@/lib/utils'

/** Whether the library-relative `path` is `folder` or lies below it. */
function folderContains(folder: string, path: string): boolean {
  return path === folder || path.startsWith(`${folder}/`)
}

/** Adds `path` (dropping the folders it contains) or removes it. */
function toggleFolder(paths: readonly string[], path: string, on: boolean): string[] {
  return on ? [...paths.filter((p) => !folderContains(path, p)), path] : paths.filter((p) => p !== path)
}

interface FolderGrantPickerProps {
  libraryId: number
  libraryName: string
  /** Granted folders of this library (library-relative paths). */
  value: readonly string[]
  onChange: (paths: string[]) => void
}

/** The folder tree of a library, loaded as it is expanded; checking a folder grants it and everything below. */
export function FolderGrantPicker({ libraryId, libraryName, value, onChange }: FolderGrantPickerProps) {
  const { t } = useTranslation('admin')
  return (
    <div role="group" aria-label={t('users.access.folders', { name: libraryName })} className="grid">
      <FolderChildren
        libraryId={libraryId}
        dir=""
        depth={0}
        value={value}
        onToggle={(path, on) => onChange(toggleFolder(value, path, on))}
      />
    </div>
  )
}

interface ChildrenProps {
  libraryId: number
  dir: string
  depth: number
  value: readonly string[]
  onToggle: (path: string, on: boolean) => void
}

function FolderChildren({ libraryId, dir, depth, value, onToggle }: ChildrenProps) {
  const { t } = useTranslation('admin')
  const listing = useQuery({
    queryKey: [...adminKeys.libraryFolders, libraryId, dir],
    queryFn: ({ signal }) => api.manage.folders.list({ libraryId, dir }, { signal }),
    select: (l) => l.folders,
  })
  const indent = { paddingInlineStart: `${depth * 1.25 + 0.75}rem` }
  if (listing.isPending) {
    return (
      <div className="flex min-h-11 items-center" style={indent}>
        <Spinner size="sm" />
      </div>
    )
  }
  if (listing.isError) {
    return (
      <p className="py-2 text-sm text-muted-foreground" style={indent}>
        {t('users.access.foldersError')}
      </p>
    )
  }
  if (listing.data.length === 0) {
    return depth === 0 ? (
      <p className="py-2 text-sm text-muted-foreground" style={indent}>
        {t('users.access.noFolders')}
      </p>
    ) : null
  }
  return listing.data.map((entry) => (
    <FolderRow key={entry.path} libraryId={libraryId} entry={entry} depth={depth} value={value} onToggle={onToggle} />
  ))
}

function FolderRow({
  libraryId,
  entry,
  depth,
  value,
  onToggle,
}: Omit<ChildrenProps, 'dir'> & { entry: FolderEntry }) {
  const { t } = useTranslation('admin')
  const [open, setOpen] = useState(false)
  const granted = value.includes(entry.path)
  // A granted parent already covers this folder.
  const implied = !granted && value.some((p) => folderContains(p, entry.path))
  const id = `grant-${libraryId}-${entry.path}`
  return (
    <>
      <div
        className="flex min-h-11 items-center gap-1 rounded-lg pe-3 hover:bg-accent/40"
        style={{ paddingInlineStart: `${depth * 1.25}rem` }}
      >
        {entry.hasSubfolders ? (
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="size-11 shrink-0"
            aria-expanded={open}
            aria-label={t(open ? 'users.access.collapse' : 'users.access.expand', { name: entry.name })}
            onClick={() => setOpen((v) => !v)}
          >
            <ChevronRight className={cn('transition-transform', open && 'rotate-90')} />
          </Button>
        ) : (
          <span className="size-11 shrink-0" aria-hidden />
        )}
        <Checkbox
          id={id}
          checked={granted || implied}
          disabled={implied}
          onCheckedChange={(on) => onToggle(entry.path, on === true)}
        />
        <label htmlFor={id} className="flex min-h-11 min-w-0 flex-1 cursor-pointer items-center gap-2 ps-2 text-sm">
          <span className="min-w-0 flex-1 truncate">{entry.name}</span>
          <span className="shrink-0 text-xs text-muted-foreground tabular-nums">{entry.trackCount}</span>
        </label>
      </div>
      {open ? (
        <FolderChildren libraryId={libraryId} dir={entry.path} depth={depth + 1} value={value} onToggle={onToggle} />
      ) : null}
    </>
  )
}
