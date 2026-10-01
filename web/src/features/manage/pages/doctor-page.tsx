import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  CircleCheck,
  Copy,
  FileQuestion,
  ImageOff,
  Languages,
  RefreshCw,
  Tags,
  Trash2,
  type LucideIcon,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useSearchParams } from 'react-router'
import { toast } from 'sonner'

import { CoverArt } from '@/components/cover-art'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { PageLoader, Spinner } from '@/components/spinner'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useMascotArt } from '@/hooks/use-mascot-art'
import { useIsMobile } from '@/hooks/use-media-query'
import { api } from '@/lib/api/endpoints'
import type { Issue, IssueType, Track } from '@/lib/api/types'
import { formatBytes, formatDuration, formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useUI } from '@/stores/ui'

import { BatchDialogs, type BatchDialogState } from '../components/batch-dialogs'
import { ConfirmDialog } from '../components/confirm-dialog'
import { toastError } from '../lib/batch'
import { invalidateLibrary, manageKeys } from '../queries'

const ISSUE_TYPES: readonly IssueType[] = ['missing_tags', 'no_cover', 'duplicates', 'missing_files', 'encoding']
const ICONS: Record<IssueType, LucideIcon> = {
  missing_tags: Tags,
  no_cover: ImageOff,
  duplicates: Copy,
  missing_files: FileQuestion,
  encoding: Languages,
}
const PAGE = 30

export default function DoctorPage() {
  const { t } = useTranslation('manage')
  const isMobile = useIsMobile()
  const [searchParams, setSearchParams] = useSearchParams()
  const typeParam = searchParams.get('type')
  const type = ISSUE_TYPES.includes(typeParam as IssueType) ? (typeParam as IssueType) : null

  const summary = useQuery({
    queryKey: manageKeys.issuesSummary,
    queryFn: ({ signal }) => api.manage.issues.summary({ signal }),
  })

  const select = (next: IssueType) =>
    setSearchParams(
      (prev) => {
        const sp = new URLSearchParams(prev)
        sp.set('type', next)
        return sp
      },
      { replace: true },
    )

  const totalIssues = summary.data ? ISSUE_TYPES.reduce((n, k) => n + (summary.data[k] ?? 0), 0) : 0

  return (
    <Page>
      <PageHeader
        title={t('doctor.title')}
        subtitle={summary.data ? (totalIssues === 0 ? t('doctor.healthy') : t('doctor.issueCount', { count: totalIssues })) : t('doctor.subtitle')}
        back={isMobile ? '/manage' : undefined}
        actions={
          <Button variant="outline" onClick={() => void summary.refetch()} disabled={summary.isFetching}>
            <RefreshCw className={cn(summary.isFetching && 'animate-spin')} />
            {t('common:actions.refresh')}
          </Button>
        }
      />

      {summary.isError ? (
        <ErrorState error={summary.error} onRetry={() => void summary.refetch()} />
      ) : (
        <div className="grid gap-8">
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
            {ISSUE_TYPES.map((k) => {
              const Icon = ICONS[k]
              const n = summary.data?.[k] ?? 0
              const active = type === k
              return (
                <button
                  key={k}
                  type="button"
                  onClick={() => select(k)}
                  aria-pressed={active}
                  className={cn(
                    'group flex flex-col gap-3 rounded-xl border p-4 text-left transition-[background-color,border-color,transform] hover:bg-accent/40 active:scale-[0.98]',
                    active && 'border-primary/60 bg-primary/5 ring-1 ring-primary/30',
                  )}
                >
                  <span className="flex items-center justify-between">
                    <span className={cn('grid size-9 place-items-center rounded-lg', n > 0 ? 'bg-amber-500/12 text-amber-600 dark:text-amber-400' : 'bg-emerald-500/12 text-emerald-600 dark:text-emerald-400')}>
                      {n > 0 ? <Icon className="size-[18px]" strokeWidth={1.75} /> : <CircleCheck className="size-[18px]" strokeWidth={1.75} />}
                    </span>
                    {summary.isPending ? (
                      <Skeleton className="h-7 w-10" />
                    ) : (
                      <span className="tnum text-2xl font-semibold tracking-tight">{formatNumber(n)}</span>
                    )}
                  </span>
                  <span className="grid gap-0.5">
                    <span className="text-sm font-medium">{t(`doctor.types.${k}.title`)}</span>
                    <span className="line-clamp-2 text-xs text-muted-foreground">{t(`doctor.types.${k}.description`)}</span>
                  </span>
                </button>
              )
            })}
          </div>

          {type ? <IssueList key={type} type={type} count={summary.data?.[type] ?? 0} /> : (
            <p className="text-center text-sm text-muted-foreground">{t('doctor.pick')}</p>
          )}
        </div>
      )}
    </Page>
  )
}

function IssueList({ type, count }: { type: IssueType; count: number }) {
  const { t } = useTranslation('manage')
  const showArt = useMascotArt()
  const queryClient = useQueryClient()
  const openTagEditor = useUI((s) => s.openTagEditor)
  const [dialog, setDialog] = useState<BatchDialogState | null>(null)
  const [purgeAll, setPurgeAll] = useState(false)
  const [purgeIds, setPurgeIds] = useState<string[] | null>(null)

  const issues = useInfiniteQuery({
    queryKey: manageKeys.issues(type),
    queryFn: ({ pageParam, signal }) => api.manage.issues.list({ type, offset: pageParam, limit: PAGE }, { signal }),
    initialPageParam: 0,
    getNextPageParam: (last, pages) => {
      const loaded = pages.reduce((n, p) => n + p.items.length, 0)
      return loaded < last.total && last.items.length > 0 ? loaded : undefined
    },
  })

  const purge = useMutation({
    mutationFn: (trackIds?: string[]) => api.manage.missing.purge(trackIds),
    onSuccess: (res) => {
      toast.success(t('doctor.purged', { count: res.purged }))
      void invalidateLibrary(queryClient)
    },
    onError: toastError,
  })

  const all = issues.data?.pages.flatMap((p) => p.items) ?? []
  const allTrackIds = [...new Set(all.flatMap((i) => i.tracks.map((tr) => tr.id)))]
  const Icon = ICONS[type]

  let bulk = null
  if (all.length > 0) {
    if (type === 'missing_files') {
      bulk = (
        <Button variant="destructive" onClick={() => setPurgeAll(true)}>
          <Trash2 />
          {t('doctor.purgeAll')}
        </Button>
      )
    } else if (type === 'encoding') {
      bulk = (
        // Empty track ids = every flagged track, including pages not loaded yet.
        <Button onClick={() => setDialog({ kind: 'encoding', trackIds: [] })}>
          <Languages />
          {t('doctor.fixAll')}
        </Button>
      )
    } else if (type === 'no_cover') {
      bulk = (
        <Button variant="outline" onClick={() => setDialog({ kind: 'cover', trackIds: allTrackIds })}>
          <ImageOff />
          {t('doctor.coverShown', { count: allTrackIds.length })}
        </Button>
      )
    } else if (type === 'missing_tags') {
      bulk = (
        <Button variant="outline" onClick={() => openTagEditor(allTrackIds)}>
          <Tags />
          {t('doctor.editShown', { count: allTrackIds.length })}
        </Button>
      )
    }
  }

  return (
    <section className="grid gap-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 className="text-xl font-semibold tracking-tight">{t(`doctor.types.${type}.title`)}</h2>
          <p className="text-sm text-muted-foreground">{t(`doctor.types.${type}.help`)}</p>
        </div>
        {bulk}
      </div>

      {issues.isPending ? (
        <PageLoader className="min-h-40" />
      ) : issues.isError ? (
        <ErrorState error={issues.error} onRetry={() => void issues.refetch()} size="compact" />
      ) : all.length === 0 ? (
        <EmptyState icon={CircleCheck} art="happy" size={showArt ? 'default' : 'compact'} title={t('doctor.none')} description={t('doctor.noneDescription')} />
      ) : (
        <>
          <ul className="grid gap-3">
            {all.map((issue) => (
              <IssueCard
                key={issue.key}
                issue={issue}
                icon={Icon}
                onEdit={(ids) => openTagEditor(ids)}
                onDialog={setDialog}
                onPurge={setPurgeIds}
                purging={purge.isPending}
              />
            ))}
          </ul>
          {issues.hasNextPage ? (
            <Button variant="outline" className="justify-self-center" onClick={() => void issues.fetchNextPage()} disabled={issues.isFetchingNextPage}>
              {issues.isFetchingNextPage ? <Spinner size="sm" /> : null}
              {t('doctor.loadMore', { count: Math.max(0, count - all.length) })}
            </Button>
          ) : null}
        </>
      )}

      <BatchDialogs dialog={dialog} onClose={() => setDialog(null)} />
      <ConfirmDialog
        open={purgeAll}
        onOpenChange={setPurgeAll}
        title={t('doctor.purgeAllTitle')}
        description={t('doctor.purgeAllDescription', { count })}
        confirmLabel={t('doctor.purgeAll')}
        destructive
        onConfirm={() => purge.mutateAsync(undefined)}
      />
      <ConfirmDialog
        open={purgeIds !== null}
        onOpenChange={(open) => (open ? undefined : setPurgeIds(null))}
        title={t('doctor.purgeTitle', { count: purgeIds?.length ?? 0 })}
        description={t('doctor.purgeAllDescription', { count: purgeIds?.length ?? 0 })}
        confirmLabel={t('doctor.purge')}
        destructive
        onConfirm={() => purge.mutateAsync(purgeIds ?? [])}
      />
    </section>
  )
}

interface IssueCardProps {
  issue: Issue
  icon: LucideIcon
  onEdit: (ids: string[]) => void
  onDialog: (dialog: BatchDialogState) => void
  onPurge: (ids: string[]) => void
  purging: boolean
}

function IssueCard({ issue, icon: Icon, onEdit, onDialog, onPurge, purging }: IssueCardProps) {
  const { t } = useTranslation('manage')
  const ids = issue.tracks.map((tr) => tr.id)
  return (
    <li className="overflow-hidden rounded-xl border">
      <div className="flex items-start gap-3 p-3 sm:p-4">
        {issue.album ? (
          <CoverArt coverArt={issue.album.coverArt} size={44} />
        ) : (
          <span className="grid size-11 shrink-0 place-items-center rounded-md bg-muted text-muted-foreground">
            <Icon className="size-5" strokeWidth={1.75} />
          </span>
        )}
        <div className="min-w-0 flex-1">
          <p className="text-sm font-medium">
            {issue.album ? (
              <Link to={`/albums/${issue.album.id}`} className="hover:underline">
                {issue.album.name}
              </Link>
            ) : (
              issue.message
            )}
          </p>
          <p className="text-xs text-muted-foreground">{issue.album ? issue.message : t('common:count.tracks', { count: issue.tracks.length })}</p>
        </div>
        <div className="flex shrink-0 flex-wrap justify-end gap-1">
          {issue.type === 'missing_files' ? (
            <Button variant="ghost" size="sm" className="text-destructive hover:text-destructive" disabled={purging} onClick={() => onPurge(ids)}>
              <Trash2 />
              <span className="max-sm:sr-only">{t('doctor.purge')}</span>
            </Button>
          ) : null}
          {issue.type === 'encoding' ? (
            <Button variant="ghost" size="sm" onClick={() => onDialog({ kind: 'encoding', trackIds: ids })}>
              <Languages />
              <span className="max-sm:sr-only">{t('doctor.fix')}</span>
            </Button>
          ) : null}
          {issue.type === 'no_cover' ? (
            <Button variant="ghost" size="sm" onClick={() => onDialog({ kind: 'cover', trackIds: ids })}>
              <ImageOff />
              <span className="max-sm:sr-only">{t('actions.cover')}</span>
            </Button>
          ) : null}
          {issue.type !== 'missing_files' ? (
            <Button variant="ghost" size="sm" onClick={() => onEdit(ids)}>
              <Tags />
              <span className="max-sm:sr-only">{t('actions.editTags')}</span>
            </Button>
          ) : null}
        </div>
      </div>
      <ul className="border-t bg-muted/30">
        {issue.tracks.slice(0, 20).map((track) => (
          <IssueTrack
            key={track.id}
            track={track}
            showDelete={issue.type === 'duplicates'}
            onDelete={() => onDialog({ kind: 'delete', trackIds: [track.id] })}
            onEdit={() => onEdit([track.id])}
          />
        ))}
        {issue.tracks.length > 20 ? (
          <li className="px-4 py-2 text-xs text-muted-foreground">{t('doctor.moreTracks', { count: issue.tracks.length - 20 })}</li>
        ) : null}
      </ul>
    </li>
  )
}

function IssueTrack({ track, showDelete, onDelete, onEdit }: { track: Track; showDelete: boolean; onDelete: () => void; onEdit: () => void }) {
  const { t } = useTranslation('manage')
  return (
    <li className="flex items-center gap-3 border-b px-3 py-2 last:border-b-0 sm:px-4">
      <button type="button" onClick={onEdit} className="grid min-w-0 flex-1 text-left" disabled={track.missing}>
        <span className="truncate text-[13px]">
          {track.title}
          {track.artist ? <span className="text-muted-foreground"> — {track.artist}</span> : null}
        </span>
        <span className="truncate font-mono text-[11px] text-muted-foreground" title={track.path}>
          {track.path}
        </span>
      </button>
      <span className="tnum hidden shrink-0 text-right text-xs text-muted-foreground sm:block">
        {track.suffix.toUpperCase()} · {formatDuration(track.duration)} · {formatBytes(track.size)}
      </span>
      {showDelete ? (
        <Button variant="ghost" size="icon-sm" className="text-muted-foreground hover:text-destructive" aria-label={t('actions.delete')} onClick={onDelete}>
          <Trash2 />
        </Button>
      ) : null}
    </li>
  )
}
