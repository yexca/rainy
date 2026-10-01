import { useQuery, useQueryClient } from '@tanstack/react-query'
import { FolderOpen, FolderPlus, HardDrive, Lock, MoreHorizontal, Pencil, ScanSearch, Trash2, TriangleAlert } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { toast } from 'sonner'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { PageLoader } from '@/components/spinner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { ConfirmDialog } from '@/features/manage/components/confirm-dialog'
import { useIsMobile } from '@/hooks/use-media-query'
import { ADMIN_SECTION } from '@/layouts/nav'
import { SectionTabs } from '@/layouts/section-tabs'
import { api } from '@/lib/api/endpoints'
import type { LibraryInfo } from '@/lib/api/types'
import { errorMessage } from '@/lib/errors'
import { formatNumber, formatRelative } from '@/lib/format'

import { LibraryDialog } from '../components/library-dialog'
import { ScanPanel } from '../components/scan-panel'
import { adminKeys, librariesQuery } from '../queries'
import { useScanStatus, useStartScan } from '../scan'

export default function LibrariesPage() {
  const { t } = useTranslation('admin')
  const isMobile = useIsMobile()
  const queryClient = useQueryClient()
  const libraries = useQuery(librariesQuery)
  const [editing, setEditing] = useState<{ library: LibraryInfo | null } | null>(null)
  const [deleting, setDeleting] = useState<LibraryInfo | null>(null)
  const list = libraries.data ?? []

  const remove = async () => {
    if (!deleting) return
    try {
      await api.admin.libraries.delete(deleting.id)
      toast.success(t('libraries.deleted', { name: deleting.name }))
      void queryClient.invalidateQueries({ queryKey: adminKeys.all })
    } catch (error) {
      toast.error(errorMessage(error, t))
      throw error
    }
  }

  const add = () => setEditing({ library: null })

  return (
    <Page>
      <PageHeader
        title={t('libraries.title')}
        subtitle={t('libraries.subtitle')}
        back={isMobile ? '/manage' : undefined}
        actions={
          isMobile ? null : (
            <Button onClick={add}>
              <FolderPlus />
              {t('libraries.add')}
            </Button>
          )
        }
        navActions={
          isMobile ? (
            <Button variant="ghost" size="icon" className="size-11 text-primary" onClick={add} aria-label={t('libraries.add')}>
              <FolderPlus className="size-5" />
            </Button>
          ) : null
        }
      >
        <SectionTabs section={ADMIN_SECTION} />
      </PageHeader>

      <div className="grid gap-8">
        <ScanPanel libraries={list} />

        <section className="grid gap-3">
          <h2 className="text-xl font-semibold tracking-tight">{t('libraries.listTitle')}</h2>
          {libraries.isPending ? (
            <PageLoader className="min-h-40" />
          ) : libraries.isError ? (
            <ErrorState error={libraries.error} onRetry={() => void libraries.refetch()} size="compact" />
          ) : list.length === 0 ? (
            <EmptyState
              icon={HardDrive}
              size="compact"
              title={t('libraries.empty')}
              description={t('libraries.emptyDescription')}
              action={
                <Button onClick={add}>
                  <FolderPlus />
                  {t('libraries.add')}
                </Button>
              }
            />
          ) : (
            <ul className="grid gap-3 md:grid-cols-2">
              {list.map((lib) => (
                <LibraryCard key={lib.id} library={lib} onEdit={() => setEditing({ library: lib })} onDelete={() => setDeleting(lib)} />
              ))}
            </ul>
          )}
        </section>
      </div>

      <LibraryDialog open={editing !== null} onOpenChange={(open) => (open ? undefined : setEditing(null))} library={editing?.library ?? null} />
      <ConfirmDialog
        open={deleting !== null}
        onOpenChange={(open) => (open ? undefined : setDeleting(null))}
        title={t('libraries.deleteTitle', { name: deleting?.name ?? '' })}
        description={t('libraries.deleteDescription')}
        confirmLabel={t('common:actions.delete')}
        destructive
        onConfirm={remove}
      />
    </Page>
  )
}

function LibraryCard({ library, onEdit, onDelete }: { library: LibraryInfo; onEdit: () => void; onDelete: () => void }) {
  const { t } = useTranslation('admin')
  const scan = useScanStatus()
  const start = useStartScan()
  const scanning = !!scan.data?.scanning
  const scanningThis = scanning && (scan.data?.libraryId === 0 || scan.data?.libraryId === library.id)

  return (
    <li className="flex flex-col gap-4 rounded-xl border p-4">
      <div className="flex items-start gap-3">
        <span className="grid size-10 shrink-0 place-items-center rounded-xl bg-primary/10 text-primary">
          <HardDrive className="size-5" strokeWidth={1.75} />
        </span>
        <div className="min-w-0 flex-1">
          <p className="truncate font-semibold">{library.name}</p>
          <p className="truncate font-mono text-xs text-muted-foreground" title={library.path}>
            {library.path}
          </p>
        </div>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="-mt-1 -mr-2 size-10" aria-label={t('common:actions.more')}>
              <MoreHorizontal />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="min-w-52 rounded-xl">
            <DropdownMenuItem asChild>
              <Link to={`/manage/folders?libraryId=${library.id}`}>
                <FolderOpen />
                {t('libraries.browse')}
              </Link>
            </DropdownMenuItem>
            <DropdownMenuItem onSelect={onEdit}>
              <Pencil />
              {t('common:actions.edit')}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem variant="destructive" onSelect={onDelete}>
              <Trash2 />
              {t('common:actions.delete')}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      <div className="flex flex-wrap gap-1.5">
        {library.exists ? null : (
          <Badge variant="destructive" className="gap-1">
            <TriangleAlert />
            {t('libraries.missing')}
          </Badge>
        )}
        {library.exists && !library.writable ? (
          <Badge variant="outline" className="gap-1 text-amber-600 dark:text-amber-400">
            <Lock />
            {t('libraries.readonly')}
          </Badge>
        ) : null}
        {library.exists && library.writable ? <Badge variant="secondary">{t('libraries.writable')}</Badge> : null}
      </div>

      <dl className="grid grid-cols-2 gap-2 text-sm">
        <div>
          <dt className="text-xs text-muted-foreground">{t('libraries.tracks')}</dt>
          <dd className="tnum font-medium">{formatNumber(library.trackCount)}</dd>
        </div>
        <div>
          <dt className="text-xs text-muted-foreground">{t('libraries.lastScan')}</dt>
          <dd className="font-medium">{scanningThis ? t('scan.scanningNow') : formatRelative(library.lastScanAt)}</dd>
        </div>
      </dl>

      <div className="mt-auto flex gap-2">
        <Button variant="secondary" className="flex-1 max-sm:h-11" disabled={scanning || start.isPending || !library.exists} onClick={() => start.mutate({ libraryId: library.id })}>
          <ScanSearch />
          {t('scan.quickScan')}
        </Button>
        <Button variant="outline" className="flex-1 max-sm:h-11" disabled={scanning || start.isPending || !library.exists} onClick={() => start.mutate({ libraryId: library.id, full: true })}>
          {t('scan.fullScan')}
        </Button>
      </div>
    </li>
  )
}
