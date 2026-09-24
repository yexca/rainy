import { useMutation, useQuery } from '@tanstack/react-query'
import {
  ChevronRight,
  FileAudio,
  FileImage,
  FileText,
  Folder,
  FolderOpen,
  FolderSync,
  HardDrive,
  LibraryBig,
  Lock,
  MoreHorizontal,
  PencilLine,
  Tags,
  Trash2,
  Upload,
} from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useSearchParams } from 'react-router'
import { toast } from 'sonner'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { PageLoader } from '@/components/spinner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useLibraries } from '@/features/admin/queries'
import { useIsMobile } from '@/hooks/use-media-query'
import { api } from '@/lib/api/endpoints'
import type { FileEntry } from '@/lib/api/types'
import { formatBytes, formatDate } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useUI } from '@/stores/ui'

import { BatchDialogs, type BatchDialogState } from '../components/batch-dialogs'
import { toastError } from '../lib/batch'
import { manageKeys } from '../queries'

export default function FoldersPage() {
  const { t } = useTranslation('manage')
  const isMobile = useIsMobile()
  const [searchParams, setSearchParams] = useSearchParams()
  const { libraries, limited } = useLibraries()
  const openTagEditor = useUI((s) => s.openTagEditor)
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set())
  const [dialog, setDialog] = useState<BatchDialogState | null>(null)

  const urlLib = Number.parseInt(searchParams.get('libraryId') ?? '', 10)
  const libraryId = Number.isFinite(urlLib) && urlLib > 0 ? urlLib : (libraries[0]?.id ?? 1)
  const library = libraries.find((l) => l.id === libraryId)
  const libraryName = library?.name || t('library.default')
  const dir = searchParams.get('dir') ?? ''

  const navigate = (next: { libraryId?: number; dir?: string }) => {
    setSelected(new Set())
    setSearchParams((prev) => {
      const sp = new URLSearchParams(prev)
      if (next.libraryId !== undefined) sp.set('libraryId', String(next.libraryId))
      if (next.dir !== undefined) {
        if (next.dir) sp.set('dir', next.dir)
        else sp.delete('dir')
      }
      return sp
    })
    window.scrollTo({ top: 0 })
  }

  const listing = useQuery({
    queryKey: manageKeys.folders(libraryId, dir),
    queryFn: ({ signal }) => api.manage.folders.list({ libraryId, dir }, { signal }),
  })
  const data = listing.data
  const trackFiles = useMemo(() => (data?.files ?? []).filter((f): f is FileEntry & { trackId: string } => !!f.trackId), [data])
  const selectedIds = trackFiles.filter((f) => selected.has(f.trackId)).map((f) => f.trackId)
  const targetIds = selectedIds.length > 0 ? selectedIds : trackFiles.map((f) => f.trackId)

  const rescan = useMutation({
    mutationFn: () => api.manage.folders.rescan({ libraryId, dir }),
    onSuccess: () => toast.success(t('rescan.started', { count: 1 })),
    onError: toastError,
  })

  const parts = dir ? dir.split('/') : []
  const parent = parts.length > 1 ? parts.slice(0, -1).join('/') : ''
  const title = parts.length > 0 ? parts[parts.length - 1] : libraryName

  const toggle = (id: string) =>
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })

  const managerLink = `/manage?${new URLSearchParams({ ...(dir ? { dirPrefix: dir } : {}), ...(limited ? {} : { libraryId: String(libraryId) }) })}`
  const uploadLink = `/manage/upload?${new URLSearchParams({ libraryId: String(libraryId), ...(dir ? { dir } : {}) })}`

  const menu = (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size={isMobile ? 'icon' : 'default'} className={cn(isMobile && 'size-11')} aria-label={t('common:actions.more')}>
          <MoreHorizontal />
          {isMobile ? null : t('folders.actions')}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-56 rounded-xl">
        <DropdownMenuItem onSelect={() => rescan.mutate()} disabled={rescan.isPending}>
          <FolderSync />
          {t('folders.rescan')}
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link to={managerLink}>
            <LibraryBig />
            {t('folders.openInManager')}
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link to={uploadLink}>
            <Upload />
            {t('folders.uploadHere')}
          </Link>
        </DropdownMenuItem>
        {trackFiles.length > 0 ? (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem onSelect={() => openTagEditor(targetIds)}>
              <Tags />
              {selectedIds.length > 0 ? t('folders.editSelected', { count: selectedIds.length }) : t('folders.editAll', { count: trackFiles.length })}
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={() => setDialog({ kind: 'rename', trackIds: targetIds })}>
              <PencilLine />
              {t('actions.rename')}
            </DropdownMenuItem>
            {selectedIds.length > 0 ? (
              <DropdownMenuItem variant="destructive" onSelect={() => setDialog({ kind: 'delete', trackIds: selectedIds })}>
                <Trash2 />
                {t('actions.delete')}
              </DropdownMenuItem>
            ) : null}
          </>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  )

  return (
    <Page>
      <PageHeader
        title={title}
        subtitle={data && !data.writable ? <ReadonlyBadge /> : t('folders.subtitle')}
        back={parts.length > 0 ? `/manage/folders?${new URLSearchParams({ libraryId: String(libraryId), ...(parent ? { dir: parent } : {}) })}` : isMobile ? '/manage' : undefined}
        actions={isMobile ? null : menu}
        navActions={isMobile ? menu : null}
      >
        <div className="flex flex-wrap items-center gap-2 pb-4">
          {!limited && libraries.length > 1 ? (
            <Select value={String(libraryId)} onValueChange={(v) => navigate({ libraryId: Number(v), dir: '' })}>
              <SelectTrigger className="h-9 w-auto min-w-40">
                <HardDrive className="size-4" />
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {libraries.map((lib) => (
                  <SelectItem key={lib.id} value={String(lib.id)}>
                    {lib.name || `#${lib.id}`}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          ) : null}
          <nav aria-label={t('folders.breadcrumbs')} className="scrollbar-none flex min-w-0 flex-1 items-center gap-0.5 overflow-x-auto text-sm">
            <button
              type="button"
              onClick={() => navigate({ dir: '' })}
              className={cn('flex shrink-0 items-center gap-1.5 rounded-md px-2 py-1.5 hover:bg-accent', parts.length === 0 && 'font-medium')}
            >
              <HardDrive className="size-4 text-muted-foreground" />
              {libraryName}
            </button>
            {parts.map((part, i) => (
              <span key={i} className="flex shrink-0 items-center gap-0.5">
                <ChevronRight className="size-3.5 text-muted-foreground/70" />
                <button
                  type="button"
                  onClick={() => navigate({ dir: parts.slice(0, i + 1).join('/') })}
                  className={cn('rounded-md px-2 py-1.5 hover:bg-accent', i === parts.length - 1 && 'font-medium')}
                  aria-current={i === parts.length - 1 ? 'page' : undefined}
                >
                  {part}
                </button>
              </span>
            ))}
          </nav>
        </div>
      </PageHeader>

      {listing.isPending ? (
        <PageLoader />
      ) : listing.isError ? (
        <ErrorState error={listing.error} onRetry={() => void listing.refetch()} retrying={listing.isFetching} />
      ) : data!.folders.length === 0 && data!.files.length === 0 ? (
        <EmptyState
          icon={FolderOpen}
          title={t('folders.empty')}
          description={t('folders.emptyDescription')}
          action={
            <Button asChild variant="outline">
              <Link to={uploadLink}>
                <Upload />
                {t('folders.uploadHere')}
              </Link>
            </Button>
          }
        />
      ) : (
        <div className="grid gap-6">
          {data!.folders.length > 0 ? (
            <section>
              <h2 className="mb-1 text-xs font-medium tracking-wide text-muted-foreground uppercase">
                {t('folders.folders', { count: data!.folders.length })}
              </h2>
              <ul className="bleed-x md:mx-0">
                {data!.folders.map((f) => (
                  <li key={f.path} className="hairline-inset [--hairline-inset:calc(var(--page-px)+2.75rem)] md:[--hairline-inset:2.75rem]">
                    <button
                      type="button"
                      onClick={() => navigate({ dir: f.path })}
                      className="page-x flex h-12 w-full items-center gap-3 text-left transition-colors hover:bg-accent/50 active:bg-accent md:rounded-md md:px-2"
                    >
                      <Folder className="size-5 shrink-0 text-primary" fill="currentColor" fillOpacity={0.15} strokeWidth={1.75} />
                      <span className="min-w-0 flex-1 truncate text-[15px] md:text-sm">{f.name}</span>
                      <ChevronRight className="size-4 text-muted-foreground/60" />
                    </button>
                  </li>
                ))}
              </ul>
            </section>
          ) : null}

          {data!.files.length > 0 ? (
            <section>
              <div className="mb-1 flex items-center gap-3">
                <h2 className="flex-1 text-xs font-medium tracking-wide text-muted-foreground uppercase">
                  {t('folders.files', { count: data!.files.length })}
                </h2>
                {trackFiles.length > 0 ? (
                  <Button variant="ghost" size="sm" onClick={() => openTagEditor(targetIds)}>
                    <Tags />
                    {selectedIds.length > 0 ? t('folders.editSelected', { count: selectedIds.length }) : t('folders.editAll', { count: trackFiles.length })}
                  </Button>
                ) : null}
              </div>
              <ul className="bleed-x md:mx-0">
                {data!.files.map((f) => (
                  <FileRow
                    key={f.path}
                    file={f}
                    selected={!!f.trackId && selected.has(f.trackId)}
                    onToggle={f.trackId ? () => toggle(f.trackId!) : undefined}
                    onOpen={f.trackId ? () => openTagEditor([f.trackId!]) : undefined}
                  />
                ))}
              </ul>
            </section>
          ) : null}
        </div>
      )}

      <BatchDialogs dialog={dialog} onClose={() => setDialog(null)} onDone={() => setSelected(new Set())} />
    </Page>
  )
}

function ReadonlyBadge() {
  const { t } = useTranslation('manage')
  return (
    <Badge variant="outline" className="gap-1 text-amber-600 dark:text-amber-400">
      <Lock />
      {t('file.readonly')}
    </Badge>
  )
}

function FileRow({ file, selected, onToggle, onOpen }: { file: FileEntry; selected: boolean; onToggle?: () => void; onOpen?: () => void }) {
  const { t } = useTranslation('manage')
  const Icon = file.isAudio ? FileAudio : file.isImage ? FileImage : FileText
  return (
    <li
      className={cn(
        'hairline-inset page-x flex h-14 items-center gap-3 [--hairline-inset:calc(var(--page-px)+4.5rem)] md:h-12 md:rounded-md md:px-2 md:[--hairline-inset:4.5rem]',
        selected && 'bg-primary/8',
      )}
    >
      <span className="grid w-6 place-items-center">
        {onToggle ? <Checkbox checked={selected} onCheckedChange={onToggle} aria-label={t('table.selectTrack', { title: file.name })} /> : null}
      </span>
      <Icon className={cn('size-5 shrink-0', file.isAudio ? 'text-primary' : 'text-muted-foreground')} strokeWidth={1.75} />
      <button
        type="button"
        onClick={onOpen}
        disabled={!onOpen}
        className="grid min-w-0 flex-1 text-left enabled:hover:underline disabled:cursor-default"
      >
        <span className="truncate text-[15px] md:text-sm">{file.name}</span>
        <span className="tnum truncate text-xs text-muted-foreground">
          {formatBytes(file.size)} · {formatDate(file.mtime)}
          {file.isAudio && !file.trackId ? ` · ${t('folders.notIndexed')}` : ''}
        </span>
      </button>
      {onOpen ? (
        <Button variant="ghost" size="icon" className="size-10 text-muted-foreground" onClick={onOpen} aria-label={t('actions.editTags')}>
          <Tags />
        </Button>
      ) : null}
    </li>
  )
}
