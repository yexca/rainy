import { useMutation, useQueryClient } from '@tanstack/react-query'
import { CircleAlert, CircleCheck, Download, ExternalLink, ListMusic, LoaderCircle, RotateCw, Tags, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { api } from '@/lib/api/endpoints'
import type { DownloadJob } from '@/lib/api/types'
import { formatBytes } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useUI } from '@/stores/ui'

import { toastError } from '../lib/batch'
import { formatEta, isActiveJob, jobName } from '../lib/downloads'
import { manageKeys } from '../queries'

/** Server-side download jobs (links or online music), newest first. */
export function DownloadJobs({ jobs }: { jobs: DownloadJob[] }) {
  const { t } = useTranslation('manage')
  const queryClient = useQueryClient()
  const finished = jobs.filter((j) => !isActiveJob(j))
  const remove = useMutation({
    mutationFn: async (ids: string[]) => {
      for (const id of ids) await api.manage.downloads.remove(id)
    },
    onSettled: () => void queryClient.invalidateQueries({ queryKey: manageKeys.downloads }),
    onError: toastError,
  })
  if (jobs.length === 0) return null

  return (
    <div className="grid gap-2">
      <div className="flex items-center gap-2">
        <h3 className="min-w-0 flex-1 text-sm font-semibold">{t('download.jobs')}</h3>
        {finished.length > 0 ? (
          <Button variant="ghost" size="sm" className="max-sm:h-10" onClick={() => remove.mutate(finished.map((j) => j.id))} disabled={remove.isPending}>
            {t('upload.clearFinished')}
          </Button>
        ) : null}
      </div>
      <ul className="divide-y rounded-xl border">
        {jobs.map((job) => (
          <DownloadRow key={job.id} job={job} onRemove={() => remove.mutate([job.id])} />
        ))}
      </ul>
    </div>
  )
}

function DownloadRow({ job, onRemove }: { job: DownloadJob; onRemove: () => void }) {
  const { t } = useTranslation('manage')
  const openTagEditor = useUI((s) => s.openTagEditor)
  const queryClient = useQueryClient()
  const active = isActiveJob(job)
  const retry = useMutation({
    mutationFn: async () => {
      if (job.online) {
        const { song, quality, lyrics, cover } = job.online
        await api.manage.online.download({
          songs: [song], quality, lyrics, cover, libraryId: job.libraryId, dir: job.dir || undefined, organize: job.organize,
        })
        return
      }
      await api.manage.downloads.start({
        url: job.url, libraryId: job.libraryId, dir: job.dir || undefined, organize: job.organize,
        format: job.format || undefined, playlist: job.playlist,
      })
    },
    onSuccess: () => {
      onRemove()
      void queryClient.invalidateQueries({ queryKey: manageKeys.downloads })
    },
    onError: toastError,
  })
  const name = jobName(job)

  return (
    <li className="flex items-center gap-3 px-3 py-2.5">
      <JobIcon job={job} />
      <div className="grid min-w-0 flex-1 gap-1">
        <p className="truncate text-sm" title={name}>
          {job.playlist ? <ListMusic className="mr-1 inline size-3.5 align-[-2px] text-muted-foreground" aria-label={t('download.playlist')} /> : null}
          {name}
        </p>
        {job.status === 'running' && job.progress >= 0 ? <Progress value={job.progress * 100} className="h-1" /> : null}
        <p className={cn('truncate text-xs', job.status === 'error' ? 'text-destructive' : 'text-muted-foreground')} title={job.error || undefined}>
          <JobDetail job={job} />
        </p>
      </div>
      {job.online && job.url ? (
        <Button asChild variant="ghost" size="icon-sm" className="text-muted-foreground max-sm:hidden" aria-label={t('onlineMusic.openPage')} title={t('onlineMusic.openPage')}>
          <a href={job.url} target="_blank" rel="noreferrer noopener">
            <ExternalLink />
          </a>
        </Button>
      ) : null}
      {job.status === 'done' && job.trackIds.length > 0 ? (
        <Button variant="ghost" size="icon-sm" className="max-sm:size-10" aria-label={t('actions.editTags')} title={t('actions.editTags')} onClick={() => openTagEditor(job.trackIds)}>
          <Tags />
        </Button>
      ) : null}
      {job.status === 'error' || job.status === 'canceled' ? (
        <Button variant="ghost" size="icon-sm" className="max-sm:size-10" aria-label={t('upload.retry')} title={t('upload.retry')} onClick={() => retry.mutate()} disabled={retry.isPending}>
          <RotateCw />
        </Button>
      ) : null}
      <Button
        variant="ghost"
        size="icon-sm"
        className="text-muted-foreground max-sm:size-10"
        aria-label={active ? t('common:actions.cancel') : t('upload.removeEntry')}
        title={active ? t('common:actions.cancel') : t('upload.removeEntry')}
        onClick={onRemove}
      >
        <X />
      </Button>
    </li>
  )
}

function JobIcon({ job }: { job: DownloadJob }) {
  const { t } = useTranslation('manage')
  switch (job.status) {
    case 'done':
      return <CircleCheck className={cn('size-5 shrink-0', job.error ? 'text-amber-500' : 'text-emerald-500')} aria-label={t('download.status.done')} />
    case 'error':
    case 'canceled':
      return <CircleAlert className="size-5 shrink-0 text-destructive" aria-label={t(`download.status.${job.status}`)} />
    case 'queued':
      return <Download className="size-5 shrink-0 text-muted-foreground" strokeWidth={1.75} aria-label={t('download.status.queued')} />
    default:
      return <LoaderCircle className="size-5 shrink-0 animate-spin text-primary motion-reduce:animate-none" strokeWidth={1.75} aria-label={t(`download.status.${job.status}`)} />
  }
}

function JobDetail({ job }: { job: DownloadJob }) {
  const { t } = useTranslation('manage')
  const parts: string[] = []
  const via =
    job.online?.source && job.online.got
      ? t('onlineMusic.via', { source: job.online.source, quality: t(`onlineMusic.qualities.${job.online.got}`) })
      : ''
  if (job.status === 'running') {
    if (job.online) parts.push(job.phase === 'processing' ? t('onlineMusic.tagging') : via || t('onlineMusic.resolving'))
    else parts.push(job.phase === 'processing' ? t('download.status.processing') : t('download.status.running'))
    if (job.items > 1 && job.item > 0) parts.push(t('download.entry', { item: job.item, items: job.items }))
    if (job.progress >= 0) parts.push(`${Math.round(job.progress * 100)}%`)
    if (job.phase !== 'processing' && job.speed > 0) parts.push(`${formatBytes(job.speed)}/s`)
    const eta = job.phase !== 'processing' ? formatEta(job.eta) : ''
    if (eta) parts.push(t('download.eta', { time: eta }))
  } else if (job.status === 'done') {
    parts.push(t('download.imported', { count: job.trackIds.length }))
    if (via) parts.push(via)
    if (job.error) parts.push(job.error)
  } else if (job.status === 'error' || job.status === 'canceled') {
    parts.push(job.error || t(`download.status.${job.status}`))
  } else {
    parts.push(t(`download.status.${job.status}`))
  }
  return <>{parts.join(' · ')}</>
}
