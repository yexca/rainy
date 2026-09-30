import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Download, Link2, Settings2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { toast } from 'sonner'

import { Spinner } from '@/components/spinner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { useAuth } from '@/hooks/use-auth'
import { api } from '@/lib/api/endpoints'
import type { DownloadFormat, DownloadsStatus } from '@/lib/api/types'
import { cn } from '@/lib/utils'

import { toastError } from '../lib/batch'
import { detectSite } from '../lib/downloads'
import { downloadsQuery, manageKeys } from '../queries'
import { useJobNotifications } from '../lib/use-job-notifications'
import { DownloadJobs } from './download-jobs'

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
 * "From a link" section of the Upload tab: the server downloads the audio of a YouTube or
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

  const status = useQuery(downloadsQuery)
  useJobNotifications(status.data?.jobs)

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

      {data ? <DownloadJobs jobs={data.jobs.filter((j) => j.kind !== 'online')} /> : null}
    </section>
  )
}

function SignInHint({ sites }: { sites: DownloadsStatus['sites'] }) {
  const { t } = useTranslation('manage')
  const signedIn = sites.filter((s) => s.cookies).map((s) => t(`download.sites.${s.id}`))
  if (signedIn.length === 0) return null
  return <>{t('download.signedIn', { sites: signedIn.join(', ') })}</>
}
