import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CircleAlert, CircleCheck, Download, Link2, ListMusic, LoaderCircle, RotateCw, Settings2, Tags, X } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { toast } from 'sonner'

import { Spinner } from '@/components/spinner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Progress } from '@/components/ui/progress'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { useAuth } from '@/hooks/use-auth'
import { api } from '@/lib/api/endpoints'
import type { DownloadFormat, DownloadJob, DownloadsStatus } from '@/lib/api/types'
import { formatBytes } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useUI } from '@/stores/ui'

import { toastError } from '../lib/batch'
import { detectSite, formatEta, isActiveJob } from '../lib/downloads'
import { invalidateLibrary, manageKeys } from '../queries'

const FORMAT_KEY = 'rainy.manage.downloadFormat'
const FORMATS: readonly DownloadFormat[] = ['best', 'm4a', 'mp3', 'opus']

function loadFormat(): DownloadFormat {
  try {
    const v = localStorage.getItem(FORMAT_KEY)
    return FORMATS.includes(v as DownloadFormat) ? (v as DownloadFormat) : 'best'
  } catch {
    return 'best'
  }
}

export interface LinkDownloadProps {
  libraryId: number
  dir: string
  organize: boolean
  invalidDir: boolean
}

/**
 * "From a link" section of the Upload page: the server downloads the audio of a YouTube or
 * bilibili video with yt-dlp and imports it like an upload (same destination and organize
 * options). Jobs run on the server, so they keep going when the page is closed.
 */
export function LinkDownload({ libraryId, dir, organize, invalidDir }: LinkDownloadProps) {
  const { t } = useTranslation('manage')
  const { isAdmin } = useAuth()
  const queryClient = useQueryClient()
  const [url, setUrl] = useState('')
  const [format, setFormat] = useState<DownloadFormat>(loadFormat)
  const [playlist, setPlaylist] = useState(false)

  const status = useQuery({
    queryKey: manageKeys.downloads,
    queryFn: ({ signal }) => api.manage.downloads.status({ signal }),
    refetchInterval: (query) => (query.state.data?.jobs.some(isActiveJob) ? 1000 : false),
  })
  useJobNotifications(status.data)

  const start = useMutation({
    mutationFn: () => api.manage.downloads.start({ url, libraryId, dir: dir || undefined, organize, format, playlist }),
    onSuccess: (job) => {
      setUrl('')
      queryClient.setQueryData<DownloadsStatus>(manageKeys.downloads, (prev) => (prev ? { ...prev, jobs: [job, ...prev.jobs] } : prev))
      void queryClient.invalidateQueries({ queryKey: manageKeys.downloads })
      toast(t('download.started'))
    },
    onError: toastError,
  })

  const site = detectSite(url)
  const data = status.data
  const canStart = !!data?.enabled && data.ready && url.trim() !== '' && !invalidDir && !start.isPending

  return (
    <section className="grid gap-4 rounded-xl border p-4 sm:p-5">
      <div className="flex items-start gap-3">
        <div className="grid size-9 shrink-0 place-items-center rounded-xl bg-primary/10 text-primary">
          <Link2 className="size-[18px]" strokeWidth={1.75} />
        </div>
        <div className="grid gap-0.5">
          <h2 className="text-sm font-semibold">{t('download.title')}</h2>
          <p className="text-xs text-muted-foreground">{t('download.subtitle')}</p>
        </div>
      </div>

      {!data ? (
        status.isError ? (
          <p className="text-sm text-destructive">{t('download.statusError')}</p>
        ) : (
          <Spinner />
        )
      ) : !data.enabled || !data.ready ? (
        <div className="flex flex-wrap items-center gap-3 rounded-lg bg-muted/50 px-3 py-2.5 text-sm">
          <p className="min-w-0 flex-1 text-muted-foreground">
            {!data.enabled ? t('download.disabled') : t('download.notInstalled')} {isAdmin ? null : t('download.askAdmin')}
          </p>
          {isAdmin ? (
            <Button asChild variant="outline" size="sm" className="max-sm:h-10">
              <Link to="/admin/settings?tab=ytdlp">
                <Settings2 />
                {t('download.openSettings')}
              </Link>
            </Button>
          ) : null}
        </div>
      ) : (
        <form
          className="grid gap-3"
          onSubmit={(e) => {
            e.preventDefault()
            if (canStart) start.mutate()
          }}
        >
          <div className="grid gap-1.5">
            <Label htmlFor="download-url" className="text-[13px] font-medium text-foreground/75">
              {t('download.link')}
            </Label>
            <div className="relative">
              <Input
                id="download-url"
                type="text"
                inputMode="url"
                enterKeyHint="go"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                placeholder={t('download.linkPlaceholder')}
                autoComplete="off"
                autoCapitalize="none"
                autoCorrect="off"
                spellCheck={false}
                className={cn('font-mono text-sm', site && 'pr-24')}
              />
              {site ? (
                <span className="pointer-events-none absolute inset-y-0 right-2 my-auto h-fit rounded-md bg-muted px-1.5 py-0.5 text-[11px] font-medium text-muted-foreground">
                  {t(`download.sites.${site}`)}
                </span>
              ) : null}
            </div>
          </div>
          <div className="flex flex-wrap items-end gap-x-4 gap-y-3">
            <div className="grid gap-1.5">
              <Label className="text-[13px] font-medium text-foreground/75">{t('download.format')}</Label>
              <Select
                value={format}
                onValueChange={(v) => {
                  setFormat(v as DownloadFormat)
                  try {
                    localStorage.setItem(FORMAT_KEY, v)
                  } catch {
                    // not remembered
                  }
                }}
              >
                <SelectTrigger className="w-52">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {FORMATS.map((f) => (
                    <SelectItem key={f} value={f}>
                      {t(`download.formats.${f}`)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <Label className="flex min-h-9 items-center gap-2.5 font-normal max-sm:min-h-11">
              <Switch checked={playlist} onCheckedChange={setPlaylist} />
              <span className="text-sm">{t('download.playlist')}</span>
            </Label>
            <Button type="submit" disabled={!canStart} className="ml-auto max-sm:h-11 max-sm:w-full">
              {start.isPending ? <Spinner size="sm" className="text-current" /> : <Download />}
              {t('download.start')}
            </Button>
          </div>
          <p className="text-xs text-muted-foreground">
            {playlist ? t('download.playlistHint') : null} <SignInHint sites={data.sites} /> {t('download.rights')}
          </p>
        </form>
      )}

      {data && data.jobs.length > 0 ? <DownloadJobs jobs={data.jobs} /> : null}
    </section>
  )
}

function SignInHint({ sites }: { sites: DownloadsStatus['sites'] }) {
  const { t } = useTranslation('manage')
  const signedIn = sites.filter((s) => s.cookies).map((s) => t(`download.sites.${s.id}`))
  if (signedIn.length === 0) return null
  return <>{t('download.signedIn', { sites: signedIn.join(', ') })}</>
}

/** Toasts when a job this page saw running finishes, and refreshes the library. */
function useJobNotifications(data: DownloadsStatus | undefined) {
  const { t } = useTranslation('manage')
  const queryClient = useQueryClient()
  const seen = useRef(new Map<string, DownloadJob['status']>())
  useEffect(() => {
    if (!data) return
    let changed = false
    for (const job of data.jobs) {
      const before = seen.current.get(job.id)
      seen.current.set(job.id, job.status)
      if (!before || !isActiveJob({ status: before }) || isActiveJob(job)) continue
      const name = job.title || job.url
      if (job.status === 'done') {
        changed = true
        if (job.error) toast.warning(t('download.finishedPartly', { name, count: job.trackIds.length }), { description: job.error })
        else toast.success(t('download.finished', { name, count: job.trackIds.length }))
      } else if (job.status === 'error') {
        toast.error(t('download.failed', { name }), { description: job.error })
      }
    }
    if (changed) void invalidateLibrary(queryClient)
  }, [data, queryClient, t])
}

function DownloadJobs({ jobs }: { jobs: DownloadJob[] }) {
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

  return (
    <div className="grid gap-2">
      <div className="flex items-center gap-2">
        <h3 className="min-w-0 flex-1 text-sm font-semibold">{t('download.jobs')}</h3>
        {finished.length > 0 ? (
          <Button variant="ghost" size="sm" onClick={() => remove.mutate(finished.map((j) => j.id))} disabled={remove.isPending}>
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
    mutationFn: () =>
      api.manage.downloads.start({
        url: job.url, libraryId: job.libraryId, dir: job.dir || undefined, organize: job.organize,
        format: job.format, playlist: job.playlist,
      }),
    onSuccess: () => {
      onRemove()
      void queryClient.invalidateQueries({ queryKey: manageKeys.downloads })
    },
    onError: toastError,
  })

  return (
    <li className="flex items-center gap-3 px-3 py-2.5">
      <JobIcon job={job} />
      <div className="grid min-w-0 flex-1 gap-1">
        <p className="truncate text-sm" title={job.title || job.url}>
          {job.playlist ? <ListMusic className="mr-1 inline size-3.5 align-[-2px] text-muted-foreground" aria-label={t('download.playlist')} /> : null}
          {job.title || job.url}
        </p>
        {job.status === 'running' && job.progress >= 0 ? <Progress value={job.progress * 100} className="h-1" /> : null}
        <p className={cn('truncate text-xs', job.status === 'error' ? 'text-destructive' : 'text-muted-foreground')} title={job.error || undefined}>
          <JobDetail job={job} />
        </p>
      </div>
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
  if (job.status === 'running') {
    parts.push(job.phase === 'processing' ? t('download.status.processing') : t('download.status.running'))
    if (job.items > 1 && job.item > 0) parts.push(t('download.entry', { item: job.item, items: job.items }))
    if (job.progress >= 0) parts.push(`${Math.round(job.progress * 100)}%`)
    if (job.phase !== 'processing' && job.speed > 0) parts.push(`${formatBytes(job.speed)}/s`)
    const eta = job.phase !== 'processing' ? formatEta(job.eta) : ''
    if (eta) parts.push(t('download.eta', { time: eta }))
  } else if (job.status === 'done') {
    parts.push(t('download.imported', { count: job.trackIds.length }))
    if (job.error) parts.push(job.error)
  } else if (job.status === 'error' || job.status === 'canceled') {
    parts.push(job.error || t(`download.status.${job.status}`))
  } else {
    parts.push(t(`download.status.${job.status}`))
  }
  return <>{parts.join(' · ')}</>
}
