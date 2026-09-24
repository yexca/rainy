import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CircleCheck, CircleX, Eraser } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ErrorState } from '@/components/error-state'
import { PageLoader, Spinner } from '@/components/spinner'
import { Button } from '@/components/ui/button'
import { api } from '@/lib/api/endpoints'
import { errorMessage } from '@/lib/errors'
import { formatBytes, formatDateTime, formatDurationLong, formatNumber } from '@/lib/format'

import { adminKeys } from '../queries'

/** Versions, ffmpeg, storage sizes, library statistics, cache clearing. */
export function SystemInfo() {
  const { t } = useTranslation('admin')
  const queryClient = useQueryClient()
  const system = useQuery({ queryKey: adminKeys.system, queryFn: ({ signal }) => api.admin.system({ signal }) })
  const stats = useQuery({ queryKey: adminKeys.stats, queryFn: ({ signal }) => api.admin.stats({ signal }) })

  const clear = useMutation({
    mutationFn: () => api.admin.clearCache(),
    onSuccess: (res) => {
      toast.success(t('system.cacheCleared', { size: formatBytes(res.freed) }))
      void queryClient.invalidateQueries({ queryKey: adminKeys.system })
    },
    onError: (error) => toast.error(errorMessage(error, t)),
  })

  if (system.isPending) return <PageLoader />
  if (system.isError) return <ErrorState error={system.error} onRetry={() => void system.refetch()} />
  const s = system.data
  const buildDate = Number.isFinite(Date.parse(s.buildDate)) ? formatDateTime(Date.parse(s.buildDate)) : s.buildDate || '—'

  const st = stats.data
  const maxFormat = Math.max(1, ...(st?.formats ?? []).map((f) => f.count))

  return (
    <div className="grid gap-8">
      <Group title={t('system.server')}>
        <Item label={t('system.version')} value={<span className="font-mono">{s.version || '—'}</span>} />
        <Item label={t('system.commit')} value={<span className="font-mono">{s.commit ? s.commit.slice(0, 12) : '—'}</span>} />
        <Item label={t('system.buildDate')} value={buildDate} />
        <Item label={t('system.runtime')} value={`${s.goVersion} · ${s.os}/${s.arch}`} />
        <Item label={t('system.uptime')} value={formatDurationLong(s.uptimeSec)} />
      </Group>

      <Group title={t('system.transcoding')}>
        <Item
          label="ffmpeg"
          value={
            <span className="inline-flex items-center gap-1.5">
              {s.ffmpeg.available ? <CircleCheck className="size-4 text-emerald-500" /> : <CircleX className="size-4 text-destructive" />}
              {s.ffmpeg.available ? s.ffmpeg.version || t('system.available') : t('system.unavailable')}
            </span>
          }
        />
        <Item label={t('system.ffmpegPath')} value={<span className="font-mono text-xs break-all">{s.ffmpeg.path || '—'}</span>} />
      </Group>

      <Group
        title={t('system.storage')}
        action={
          <Button variant="outline" size="sm" onClick={() => clear.mutate()} disabled={clear.isPending}>
            {clear.isPending ? <Spinner size="sm" /> : <Eraser />}
            {t('system.clearCache')}
          </Button>
        }
      >
        <Item label={t('system.dataDir')} value={<span className="font-mono text-xs break-all">{s.dataDir}</span>} />
        <Item label={t('system.database')} value={formatBytes(s.dbSize)} />
        <Item label={t('system.cache')} value={formatBytes(s.cacheSize)} />
        <Item label={t('system.trash')} value={formatBytes(s.trashSize)} />
      </Group>

      {st ? (
        <Group title={t('system.library')}>
          <div className="grid grid-cols-2 gap-px bg-border sm:grid-cols-4">
            {(
              [
                ['tracks', st.tracks],
                ['albums', st.albums],
                ['artists', st.artists],
                ['genres', st.genres],
                ['playlists', st.playlists],
                ['users', st.users],
                ['missing', st.missingTracks],
                ['plays30', st.playsLast30Days],
              ] as const
            ).map(([key, value]) => (
              <div key={key} className="bg-background p-4">
                <p className="text-xs text-muted-foreground">{t(`system.stats.${key}`)}</p>
                <p className="tnum text-xl font-semibold tracking-tight">{formatNumber(value)}</p>
              </div>
            ))}
          </div>
          <Item label={t('system.totalDuration')} value={formatDurationLong(st.totalDuration)} />
          <Item label={t('system.totalSize')} value={formatBytes(st.totalSize)} />
          {st.formats.length > 0 ? (
            <div className="grid gap-2 p-4">
              <p className="text-xs text-muted-foreground">{t('system.formats')}</p>
              {[...st.formats]
                .sort((a, b) => b.count - a.count)
                .map((f) => (
                  <div key={f.suffix} className="grid grid-cols-[3.5rem_minmax(0,1fr)_auto] items-center gap-3 text-sm">
                    <span className="text-xs font-medium tracking-wide uppercase">{f.suffix}</span>
                    <span className="h-2 overflow-hidden rounded-full bg-muted">
                      <span className="block h-full rounded-full bg-primary" style={{ width: `${(f.count / maxFormat) * 100}%` }} />
                    </span>
                    <span className="tnum text-xs text-muted-foreground">
                      {formatNumber(f.count)} · {formatBytes(f.size)}
                    </span>
                  </div>
                ))}
            </div>
          ) : null}
        </Group>
      ) : null}
    </div>
  )
}

function Group({ title, action, children }: { title: string; action?: ReactNode; children: ReactNode }) {
  return (
    <section className="grid gap-2">
      <div className="flex items-end justify-between gap-2 px-1">
        <h2 className="text-xs font-medium tracking-wide text-muted-foreground uppercase">{title}</h2>
        {action}
      </div>
      <div className="divide-y overflow-hidden rounded-xl border">{children}</div>
    </section>
  )
}

function Item({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1.6fr)] items-baseline gap-3 px-4 py-3 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span className="min-w-0 text-right">{value}</span>
    </div>
  )
}
