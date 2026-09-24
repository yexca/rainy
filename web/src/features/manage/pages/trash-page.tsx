import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArchiveRestore, Trash2 } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { PageLoader, Spinner } from '@/components/spinner'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { useIsMobile } from '@/hooks/use-media-query'
import { api } from '@/lib/api/endpoints'
import type { TrashEntry } from '@/lib/api/types'
import { formatBytes, formatDateTime, formatRelative } from '@/lib/format'
import { cn } from '@/lib/utils'

import { ConfirmDialog } from '../components/confirm-dialog'
import { reportBatch, toastError } from '../lib/batch'
import { invalidateLibrary, manageKeys } from '../queries'

export default function TrashPage() {
  const { t } = useTranslation('manage')
  const isMobile = useIsMobile()
  const queryClient = useQueryClient()
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set())
  const [confirm, setConfirm] = useState<'selected' | 'all' | null>(null)

  const trash = useQuery({ queryKey: manageKeys.trash, queryFn: ({ signal }) => api.manage.trash.list({ signal }) })
  const entries = useMemo(() => [...(trash.data ?? [])].sort((a, b) => b.deletedAt - a.deletedAt), [trash.data])
  const selectedIds = entries.filter((e) => selected.has(e.id)).map((e) => e.id)
  const totalSize = entries.reduce((n, e) => n + e.size, 0)

  const refresh = () => {
    setSelected(new Set())
    void queryClient.invalidateQueries({ queryKey: manageKeys.trash })
    void invalidateLibrary(queryClient)
  }

  const restore = useMutation({
    mutationFn: (ids: string[]) => api.manage.trash.restore(ids),
    onSuccess: (result) => {
      reportBatch(result, 'manage:trash.restored')
      refresh()
    },
    onError: toastError,
  })

  const purge = async (ids?: string[]) => {
    try {
      const res = await api.manage.trash.purge(ids)
      toast.success(t('trash.purged', { count: res.purged }))
      refresh()
    } catch (error) {
      toastError(error)
      throw error
    }
  }

  const toggle = (id: string) =>
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })

  const allChecked = entries.length > 0 && selectedIds.length === entries.length

  return (
    <Page>
      <PageHeader
        title={t('trash.title')}
        subtitle={entries.length > 0 ? t('trash.summary', { count: entries.length, size: formatBytes(totalSize) }) : t('trash.subtitle')}
        back={isMobile ? '/manage' : undefined}
        actions={
          entries.length > 0 ? (
            <Button variant="outline" className="text-destructive hover:text-destructive" onClick={() => setConfirm('all')}>
              <Trash2 />
              {t('trash.empty')}
            </Button>
          ) : null
        }
      />

      {trash.isPending ? (
        <PageLoader />
      ) : trash.isError ? (
        <ErrorState error={trash.error} onRetry={() => void trash.refetch()} retrying={trash.isFetching} />
      ) : entries.length === 0 ? (
        <EmptyState icon={Trash2} title={t('trash.emptyTitle')} description={t('trash.emptyDescription')} />
      ) : (
        <div className="grid gap-3">
          <div className="bleed-x page-x hairline-b sticky top-[calc(var(--safe-top)+2.75rem)] z-20 flex h-12 items-center gap-3 bg-background/90 backdrop-blur-xl md:top-[calc(var(--safe-top)+3rem)]">
            <Checkbox
              checked={allChecked ? true : selectedIds.length > 0 ? 'indeterminate' : false}
              onCheckedChange={(v) => setSelected(v === true ? new Set(entries.map((e) => e.id)) : new Set())}
              aria-label={t('common:actions.selectAll')}
            />
            <span className="tnum min-w-0 flex-1 truncate text-sm text-muted-foreground">
              {selectedIds.length > 0 ? t('selection.count', { count: selectedIds.length, formatted: selectedIds.length }) : t('trash.selectHint')}
            </span>
            <Button
              variant="ghost"
              size="sm"
              disabled={selectedIds.length === 0 || restore.isPending}
              onClick={() => restore.mutate(selectedIds)}
            >
              {restore.isPending ? <Spinner size="sm" /> : <ArchiveRestore />}
              {t('trash.restore')}
            </Button>
            <Button
              variant="ghost"
              size="sm"
              className="text-destructive hover:text-destructive"
              disabled={selectedIds.length === 0}
              onClick={() => setConfirm('selected')}
            >
              <Trash2 />
              <span className="max-sm:sr-only">{t('trash.deleteForever')}</span>
            </Button>
          </div>
          <ul className="bleed-x md:mx-0">
            {entries.map((entry) => (
              <TrashRow key={entry.id} entry={entry} selected={selected.has(entry.id)} onToggle={() => toggle(entry.id)} onRestore={() => restore.mutate([entry.id])} restoring={restore.isPending} />
            ))}
          </ul>
        </div>
      )}

      <ConfirmDialog
        open={confirm !== null}
        onOpenChange={(open) => (open ? undefined : setConfirm(null))}
        title={confirm === 'all' ? t('trash.emptyConfirmTitle') : t('trash.deleteConfirmTitle', { count: selectedIds.length })}
        description={confirm === 'all' ? t('trash.emptyConfirmDescription', { count: entries.length }) : t('trash.deleteConfirmDescription')}
        confirmLabel={confirm === 'all' ? t('trash.empty') : t('trash.deleteForever')}
        destructive
        onConfirm={() => purge(confirm === 'all' ? undefined : selectedIds)}
      />
    </Page>
  )
}

function TrashRow({
  entry,
  selected,
  onToggle,
  onRestore,
  restoring,
}: {
  entry: TrashEntry
  selected: boolean
  onToggle: () => void
  onRestore: () => void
  restoring: boolean
}) {
  const { t } = useTranslation('manage')
  return (
    <li
      className={cn(
        'hairline-inset page-x flex items-center gap-3 py-2.5 [--hairline-inset:calc(var(--page-px)+2.25rem)] md:rounded-md md:px-2 md:[--hairline-inset:2.25rem]',
        selected && 'bg-primary/8',
      )}
    >
      <Checkbox checked={selected} onCheckedChange={onToggle} aria-label={t('table.selectTrack', { title: entry.title || entry.originalPath })} />
      <div className="grid min-w-0 flex-1 gap-0.5">
        <p className="truncate text-[15px] md:text-sm">
          <span className="font-medium">{entry.title || entry.originalPath.split('/').pop()}</span>
          {entry.artist ? <span className="text-muted-foreground"> — {entry.artist}</span> : null}
          {entry.album ? <span className="text-muted-foreground"> · {entry.album}</span> : null}
        </p>
        <p className="truncate font-mono text-[11px] text-muted-foreground" title={entry.originalPath}>
          {entry.originalPath}
        </p>
        <p className="tnum text-xs text-muted-foreground" title={formatDateTime(entry.deletedAt)}>
          {formatBytes(entry.size)} · {t('trash.deletedAgo', { when: formatRelative(entry.deletedAt) })}
          {entry.deletedBy ? ` · ${entry.deletedBy}` : ''}
        </p>
      </div>
      <Button variant="ghost" size="icon" className="size-10 shrink-0 text-muted-foreground" onClick={onRestore} disabled={restoring} aria-label={t('trash.restore')}>
        <ArchiveRestore />
      </Button>
    </li>
  )
}
