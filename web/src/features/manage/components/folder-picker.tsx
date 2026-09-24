import { useQuery } from '@tanstack/react-query'
import { ChevronRight, Folder, FolderOpen, HardDrive } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { Spinner } from '@/components/spinner'
import { Button } from '@/components/ui/button'
import { api } from '@/lib/api/endpoints'

import { manageKeys } from '../queries'
import { ResponsiveDialog } from './responsive-dialog'

export interface FolderPickerProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  libraryId: number
  libraryName: string
  /** Initially shown folder. */
  initialDir: string
  onSelect: (dir: string) => void
}

/** Browse a library's folders and pick one (upload target, filters). */
export function FolderPicker({ open, onOpenChange, libraryId, libraryName, initialDir, onSelect }: FolderPickerProps) {
  const { t } = useTranslation('manage')
  return (
    <ResponsiveDialog open={open} onOpenChange={onOpenChange} title={t('folderPicker.title')} size="md">
      {open ? (
        <PickerBody
          libraryId={libraryId}
          libraryName={libraryName}
          initialDir={initialDir}
          onSelect={(dir) => {
            onSelect(dir)
            onOpenChange(false)
          }}
          onCancel={() => onOpenChange(false)}
        />
      ) : null}
    </ResponsiveDialog>
  )
}

function PickerBody({
  libraryId,
  libraryName,
  initialDir,
  onSelect,
  onCancel,
}: {
  libraryId: number
  libraryName: string
  initialDir: string
  onSelect: (dir: string) => void
  onCancel: () => void
}) {
  const { t } = useTranslation('manage')
  const [dir, setDir] = useState(initialDir)
  const listing = useQuery({
    queryKey: manageKeys.folders(libraryId, dir),
    queryFn: ({ signal }) => api.manage.folders.list({ libraryId, dir }, { signal }),
    retry: false,
  })
  const parts = dir ? dir.split('/') : []

  return (
    <div className="grid gap-3">
      <nav className="scrollbar-none flex items-center gap-1 overflow-x-auto text-sm" aria-label={t('folders.breadcrumbs')}>
        <button type="button" onClick={() => setDir('')} className="flex shrink-0 items-center gap-1.5 rounded-md px-1.5 py-1 hover:bg-accent">
          <HardDrive className="size-4 text-muted-foreground" />
          {libraryName || t('library.default')}
        </button>
        {parts.map((part, i) => (
          <span key={i} className="flex shrink-0 items-center gap-1">
            <ChevronRight className="size-3.5 text-muted-foreground" />
            <button type="button" onClick={() => setDir(parts.slice(0, i + 1).join('/'))} className="rounded-md px-1.5 py-1 hover:bg-accent">
              {part}
            </button>
          </span>
        ))}
      </nav>
      <div className="h-[min(45dvh,20rem)] overflow-y-auto rounded-lg border">
        {listing.isPending ? (
          <div className="flex h-full items-center justify-center">
            <Spinner />
          </div>
        ) : listing.isError ? (
          <ErrorState error={listing.error} size="compact" onRetry={() => void listing.refetch()} />
        ) : listing.data.folders.length === 0 ? (
          <p className="flex h-full items-center justify-center text-sm text-muted-foreground">{t('folderPicker.noSubfolders')}</p>
        ) : (
          <ul className="divide-y">
            {listing.data.folders.map((f) => (
              <li key={f.path}>
                <button type="button" onClick={() => setDir(f.path)} className="flex h-11 w-full items-center gap-3 px-3 text-left text-sm hover:bg-accent/60">
                  <Folder className="size-4 shrink-0 text-primary" fill="currentColor" fillOpacity={0.15} />
                  <span className="min-w-0 flex-1 truncate">{f.name}</span>
                  <ChevronRight className="size-4 text-muted-foreground/60" />
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
      <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end [&_button]:max-sm:h-11">
        <Button variant="outline" onClick={onCancel}>
          {t('common:actions.cancel')}
        </Button>
        <Button onClick={() => onSelect(dir)}>
          <FolderOpen />
          {t('folderPicker.select')}
        </Button>
      </div>
    </div>
  )
}
