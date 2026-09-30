import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CircleAlert, CircleCheck, Download, RefreshCw, Trash2 } from 'lucide-react'
import { useEffect, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ErrorState } from '@/components/error-state'
import { PageLoader, Spinner } from '@/components/spinner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { ConfirmDialog } from '@/features/manage/components/confirm-dialog'
import { api } from '@/lib/api/endpoints'
import type { CookieInfo, DownloadSite, Settings, YtdlpInfo } from '@/lib/api/types'
import { errorMessage } from '@/lib/errors'
import { formatDate, formatNumber, formatRelative } from '@/lib/format'
import { cn } from '@/lib/utils'

import { adminKeys, ytdlpQuery } from '../queries'
import { CookieDialog } from './cookie-dialog'

/** Newer-than comparison of yt-dlp versions ("2026.08.19", "2026.08.19.1"). */
function isNewer(latest: string, installed: string): boolean {
  const a = latest.split('.').map(Number)
  const b = installed.split('.').map(Number)
  for (let i = 0; i < Math.max(a.length, b.length); i++) {
    const d = (a[i] ?? 0) - (b[i] ?? 0)
    if (d !== 0) return d > 0
  }
  return false
}

/** Admin → Settings → yt-dlp: enable downloads, install / update yt-dlp, sign-in cookies. */
export function YtdlpSettings() {
  const { t } = useTranslation('admin')
  const queryClient = useQueryClient()
  const info = useQuery({
    ...ytdlpQuery,
    refetchInterval: (query) => (query.state.data?.install.running ? 1000 : false),
  })
  const [cookieSite, setCookieSite] = useState<DownloadSite | null>(null)
  const [removeSite, setRemoveSite] = useState<DownloadSite | null>(null)
  useInstallNotifications(info.data)

  const setInfo = (next: YtdlpInfo) => queryClient.setQueryData(adminKeys.ytdlp, next)

  const toggle = useMutation({
    mutationFn: (on: boolean) => api.admin.settings.update({ ytdlpEnabled: on }),
    onSuccess: (saved) => {
      queryClient.setQueryData<Settings>(adminKeys.settings, (prev) => (prev ? { ...prev, ytdlpEnabled: saved.ytdlpEnabled } : prev))
      void queryClient.invalidateQueries({ queryKey: adminKeys.ytdlp })
      void queryClient.invalidateQueries({ queryKey: ['manage', 'downloads'] })
    },
    onError: (error) => toast.error(errorMessage(error, t)),
  })
  const check = useMutation({
    mutationFn: () => api.admin.ytdlp.check(),
    onSuccess: (next) => {
      setInfo(next)
      if (next.installed && next.latest && !isNewer(next.latest, next.version)) toast.success(t('ytdlp.upToDate', { version: next.version }))
    },
    onError: (error) => toast.error(errorMessage(error, t)),
  })
  const install = useMutation({
    mutationFn: () => api.admin.ytdlp.install(),
    onSuccess: setInfo,
    onError: (error) => toast.error(errorMessage(error, t)),
  })

  if (!info.data) {
    return info.isError ? <ErrorState error={info.error} onRetry={() => void info.refetch()} retrying={info.isFetching} /> : <PageLoader />
  }
  const d = info.data
  const enabled = toggle.isPending ? toggle.variables : d.enabled
  const installing = d.install.running || install.isPending
  const updateAvailable = d.installed && !!d.latest && isNewer(d.latest, d.version)

  return (
    <div className="grid gap-8">
      <Section title={t('ytdlp.sections.downloads')}>
        <Row label={t('ytdlp.enable')} description={t('ytdlp.enableHelp')} inline>
          <Switch checked={enabled} onCheckedChange={(v) => toggle.mutate(v)} disabled={toggle.isPending} aria-label={t('ytdlp.enable')} />
        </Row>
      </Section>

      <Section title={t('ytdlp.sections.binary')}>
        <Row
          label={t('ytdlp.version')}
          description={d.managed ? t('ytdlp.managedHelp') : t('ytdlp.unmanagedHelp')}
        >
          <div className="grid gap-1 text-sm sm:justify-items-end sm:text-right">
            <p className="flex flex-wrap items-center gap-2 sm:justify-end">
              {d.installed ? (
                <span className="font-mono tnum">{d.version}</span>
              ) : (
                <span className="text-muted-foreground">{t('ytdlp.notInstalled')}</span>
              )}
              {updateAvailable ? <Badge variant="secondary">{t('ytdlp.updateAvailable', { version: d.latest })}</Badge> : null}
            </p>
            {d.error ? <p className="text-xs text-destructive">{d.error}</p> : null}
            {d.latest || d.checkedAt ? (
              <p className="text-xs text-muted-foreground">
                {t('ytdlp.latest', { version: d.latest || '—', when: formatRelative(d.checkedAt) })}
              </p>
            ) : null}
          </div>
        </Row>
        <Row label={t('ytdlp.updates')} description={enabled ? t('ytdlp.updatesHelp') : t('ytdlp.enableFirst')}>
          <div className="flex flex-wrap gap-2 sm:justify-end">
            <Button variant="outline" onClick={() => check.mutate()} disabled={!enabled || check.isPending || installing}>
              {check.isPending ? <Spinner size="sm" className="text-current" /> : <RefreshCw />}
              {t('ytdlp.check')}
            </Button>
            {d.managed && (!d.installed || updateAvailable || installing) ? (
              <Button onClick={() => install.mutate()} disabled={!enabled || installing}>
                {installing ? <Spinner size="sm" className="text-current" /> : <Download />}
                {installing ? t('ytdlp.installing') : d.installed ? t('ytdlp.update', { version: d.latest }) : t('ytdlp.install')}
              </Button>
            ) : null}
          </div>
          {d.install.error && !installing ? <p className="text-xs text-destructive sm:text-right">{d.install.error}</p> : null}
        </Row>
        <Row label={t('ytdlp.requirements')} description={t('ytdlp.requirementsHelp')}>
          <ul className="grid gap-1 text-sm sm:justify-items-end">
            <Requirement ok={d.ffmpeg} label={d.ffmpeg ? t('ytdlp.ffmpegOk') : t('ytdlp.ffmpegMissing')} />
            <Requirement
              ok={d.jsRuntime !== ''}
              label={d.jsRuntime ? t('ytdlp.jsRuntime', { name: t(`ytdlp.runtimes.${d.jsRuntime}`) }) : t('ytdlp.jsRuntimeMissing')}
            />
          </ul>
        </Row>
      </Section>

      <Section title={t('ytdlp.sections.cookies')} description={t('ytdlp.cookies.help')}>
        {d.cookies.map((c) => (
          <CookieRow key={c.site} info={c} onSet={() => setCookieSite(c.site)} onRemove={() => setRemoveSite(c.site)} />
        ))}
      </Section>

      <CookieDialog site={cookieSite} onOpenChange={(open) => !open && setCookieSite(null)} />
      <ConfirmDialog
        open={removeSite !== null}
        onOpenChange={(open) => !open && setRemoveSite(null)}
        title={t('ytdlp.cookies.removeTitle', { site: removeSite ? t(`ytdlp.sites.${removeSite}`) : '' })}
        description={t('ytdlp.cookies.removeDescription')}
        confirmLabel={t('ytdlp.cookies.remove')}
        destructive
        onConfirm={async () => {
          if (!removeSite) return
          try {
            await api.admin.ytdlp.deleteCookies(removeSite)
            await queryClient.invalidateQueries({ queryKey: adminKeys.ytdlp })
            void queryClient.invalidateQueries({ queryKey: ['manage', 'downloads'] })
            toast.success(t('ytdlp.cookies.removed'))
          } catch (error) {
            toast.error(errorMessage(error, t))
            throw error
          }
        }}
      />
    </div>
  )
}

/** Toasts when a background install finishes while the page is open. */
function useInstallNotifications(data: YtdlpInfo | undefined) {
  const { t } = useTranslation('admin')
  const wasRunning = useRef(false)
  useEffect(() => {
    if (!data) return
    if (wasRunning.current && !data.install.running) {
      if (data.install.error) toast.error(t('ytdlp.installFailed'), { description: data.install.error })
      else toast.success(t('ytdlp.installed', { version: data.version }))
    }
    wasRunning.current = data.install.running
  }, [data, t])
}

function Requirement({ ok, label }: { ok: boolean; label: string }) {
  const Icon = ok ? CircleCheck : CircleAlert
  return (
    <li className={cn('flex items-center gap-1.5', ok ? 'text-muted-foreground' : 'text-amber-600 dark:text-amber-400')}>
      <Icon className={cn('size-4 shrink-0', ok && 'text-emerald-500')} strokeWidth={1.75} aria-hidden />
      {label}
    </li>
  )
}

function CookieRow({ info, onSet, onRemove }: { info: CookieInfo; onSet: () => void; onRemove: () => void }) {
  const { t } = useTranslation('admin')
  const name = t(`ytdlp.sites.${info.site}`)
  return (
    <div className="grid gap-3 p-4 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
      <div className="grid gap-1">
        <p className="flex flex-wrap items-center gap-2 text-sm font-medium">
          {name}
          {info.configured ? (
            <Badge variant={info.signedIn ? 'secondary' : 'outline'} className={cn(!info.signedIn && 'text-amber-600 dark:text-amber-400')}>
              {info.signedIn ? t('ytdlp.cookies.signedIn') : t('ytdlp.cookies.notSignedIn')}
            </Badge>
          ) : null}
        </p>
        <p className="text-xs text-muted-foreground">
          {info.configured
            ? [
                t('ytdlp.cookies.count', { count: info.count, formatted: formatNumber(info.count) }),
                t('ytdlp.cookies.updated', { when: formatRelative(info.updatedAt) }),
                info.expiresAt ? t('ytdlp.cookies.expires', { date: formatDate(info.expiresAt) }) : '',
              ]
                .filter(Boolean)
                .join(' · ')
            : t('ytdlp.cookies.none')}
        </p>
      </div>
      <div className="flex gap-2">
        <Button variant="outline" onClick={onSet} className="max-sm:h-11 max-sm:flex-1">
          {info.configured ? t('ytdlp.cookies.replace') : t('ytdlp.cookies.set')}
        </Button>
        {info.configured ? (
          <Button variant="ghost" onClick={onRemove} className="text-destructive max-sm:size-11" aria-label={t('ytdlp.cookies.removeLabel', { site: name })} title={t('ytdlp.cookies.remove')}>
            <Trash2 />
          </Button>
        ) : null}
      </div>
    </div>
  )
}

function Section({ title, description, children }: { title: string; description?: string; children: ReactNode }) {
  return (
    <section className="grid gap-2">
      <h2 className="px-1 text-xs font-medium tracking-wide text-muted-foreground uppercase">{title}</h2>
      {description ? <p className="px-1 text-xs text-muted-foreground">{description}</p> : null}
      <div className="divide-y rounded-xl border">{children}</div>
    </section>
  )
}

function Row({ label, description, inline, children }: { label: string; description?: string; inline?: boolean; children: ReactNode }) {
  return (
    <div className={cn('grid gap-3 p-4', inline ? 'grid-cols-[minmax(0,1fr)_auto] items-center' : 'sm:grid-cols-[minmax(0,1fr)_minmax(0,1.1fr)] sm:items-center')}>
      <div className="grid gap-0.5">
        <Label className="text-sm font-medium">{label}</Label>
        {description ? <p className="text-xs text-muted-foreground">{description}</p> : null}
      </div>
      <div className="grid gap-1.5">{children}</div>
    </div>
  )
}
