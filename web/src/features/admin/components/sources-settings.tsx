import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  ArrowDown,
  ArrowUp,
  BellOff,
  BellRing,
  CircleAlert,
  CloudDownload,
  ExternalLink,
  Import,
  MoreHorizontal,
  RefreshCw,
  Trash2,
} from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ErrorState } from '@/components/error-state'
import { PageLoader, Spinner } from '@/components/spinner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { ConfirmDialog } from '@/features/manage/components/confirm-dialog'
import { api } from '@/lib/api/endpoints'
import type { LxSource, LxSourceMode, LxSourcesInfo, Settings } from '@/lib/api/types'
import { errorMessage } from '@/lib/errors'
import { formatBytes, formatRelative } from '@/lib/format'
import { cn } from '@/lib/utils'

import { adminKeys, sourcesQuery } from '../queries'
import { SourceImportDialog } from './source-import-dialog'

/**
 * Admin → Settings → Sources: turn online music on, choose automatic fallback or one fixed
 * source, and import / order / enable lx-music custom source scripts.
 */
export function SourcesSettings() {
  const { t } = useTranslation('admin')
  const queryClient = useQueryClient()
  const info = useQuery({
    ...sourcesQuery,
    refetchInterval: (query) => (query.state.data?.sources.some((s) => s.status === 'loading') ? 1000 : false),
  })
  const [importing, setImporting] = useState(false)
  const [removing, setRemoving] = useState<LxSource | null>(null)

  const setInfo = (update: (prev: LxSourcesInfo) => LxSourcesInfo) =>
    queryClient.setQueryData<LxSourcesInfo>(adminKeys.sources, (prev) => (prev ? update(prev) : prev))
  const refreshOthers = () => {
    void queryClient.invalidateQueries({ queryKey: ['manage', 'online'] })
  }

  const settings = useMutation({
    mutationFn: (patch: Partial<Settings>) => api.admin.settings.update(patch),
    onSuccess: (saved) => {
      queryClient.setQueryData<Settings>(adminKeys.settings, saved)
      setInfo((prev) => ({ ...prev, enabled: saved.lxSourcesEnabled, mode: saved.lxSourceMode, sourceId: saved.lxSourceId }))
      refreshOthers()
    },
    onError: (error) => toast.error(errorMessage(error, t)),
  })
  const reorder = useMutation({
    mutationFn: (ids: string[]) => api.admin.sources.reorder(ids),
    onMutate: (ids) =>
      setInfo((prev) => ({ ...prev, sources: ids.map((id) => prev.sources.find((s) => s.id === id)).filter((s): s is LxSource => !!s) })),
    onSuccess: (next) => queryClient.setQueryData(adminKeys.sources, next),
    onError: (error) => {
      toast.error(errorMessage(error, t))
      void queryClient.invalidateQueries({ queryKey: adminKeys.sources })
    },
  })

  if (!info.data) {
    return info.isError ? <ErrorState error={info.error} onRetry={() => void info.refetch()} retrying={info.isFetching} /> : <PageLoader />
  }
  const d = info.data
  const pending = settings.isPending ? settings.variables : undefined
  const enabled = pending?.lxSourcesEnabled ?? d.enabled
  const mode: LxSourceMode = pending?.lxSourceMode ?? d.mode
  const fixedId = pending?.lxSourceId ?? d.sourceId
  const ids = d.sources.map((s) => s.id)
  const move = (index: number, delta: number) => {
    const next = [...ids]
    const [id] = next.splice(index, 1)
    next.splice(index + delta, 0, id)
    reorder.mutate(next)
  }

  return (
    <div className="grid gap-8">
      <Section title={t('sources.sections.online')}>
        <Row label={t('sources.enable')} description={t('sources.enableHelp')} inline>
          <Switch
            checked={enabled}
            onCheckedChange={(v) => settings.mutate({ lxSourcesEnabled: v })}
            disabled={settings.isPending}
            aria-label={t('sources.enable')}
          />
        </Row>
      </Section>

      <Section title={t('sources.sections.mode')}>
        <div className="grid gap-4 p-4">
          <RadioGroup
            value={mode}
            onValueChange={(v) => {
              const next: LxSourceMode = v === 'fixed' ? 'fixed' : 'auto'
              const firstEnabled = d.sources.find((s) => s.enabled)?.id ?? ''
              settings.mutate(next === 'fixed' && !fixedId ? { lxSourceMode: next, lxSourceId: firstEnabled } : { lxSourceMode: next })
            }}
            disabled={settings.isPending}
            className="gap-4"
          >
            <ModeOption value="auto" label={t('sources.modes.auto')} description={t('sources.modes.autoHelp')} />
            <ModeOption value="fixed" label={t('sources.modes.fixed')} description={t('sources.modes.fixedHelp')} />
          </RadioGroup>
          {mode === 'fixed' ? (
            <div className="grid gap-1.5 pl-7">
              <Label className="text-[13px] font-medium text-foreground/75">{t('sources.modes.fixedSource')}</Label>
              <Select value={fixedId || undefined} onValueChange={(v) => settings.mutate({ lxSourceId: v })} disabled={settings.isPending || d.sources.length === 0}>
                <SelectTrigger className="w-full sm:w-72">
                  <SelectValue placeholder={t('sources.modes.choose')} />
                </SelectTrigger>
                <SelectContent>
                  {d.sources.map((s) => (
                    <SelectItem key={s.id} value={s.id} disabled={!s.enabled}>
                      {s.name}
                      {s.enabled ? '' : ` (${t('sources.disabled')})`}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {fixedId && !d.sources.some((s) => s.id === fixedId && s.enabled) ? (
                <p className="text-xs text-destructive">{t('sources.modes.fixedUnavailable')}</p>
              ) : null}
            </div>
          ) : null}
        </div>
      </Section>

      <Section
        title={t('sources.sections.sources', { count: d.sources.length })}
        description={t('sources.help')}
        action={
          <Button size="sm" onClick={() => setImporting(true)} className="max-sm:h-10">
            <Import />
            {t('sources.importButton')}
          </Button>
        }
      >
        {d.sources.length === 0 ? (
          <p className="p-4 text-sm text-muted-foreground">{t('sources.empty')}</p>
        ) : (
          d.sources.map((source, index) => (
            <SourceRow
              key={source.id}
              source={source}
              enabled={enabled}
              fixed={mode === 'fixed' && source.id === fixedId}
              first={index === 0}
              last={index === d.sources.length - 1}
              reordering={reorder.isPending}
              onMove={(delta) => move(index, delta)}
              onRemove={() => setRemoving(source)}
              onChanged={(next) => {
                setInfo((prev) => ({ ...prev, sources: prev.sources.map((s) => (s.id === next.id ? next : s)) }))
                refreshOthers()
              }}
            />
          ))
        )}
      </Section>

      <SourceImportDialog open={importing} onOpenChange={setImporting} enabled={enabled} />
      <ConfirmDialog
        open={removing !== null}
        onOpenChange={(open) => !open && setRemoving(null)}
        title={t('sources.removeTitle', { name: removing?.name ?? '' })}
        description={t('sources.removeDescription')}
        confirmLabel={t('sources.remove')}
        destructive
        onConfirm={async () => {
          if (!removing) return
          try {
            await api.admin.sources.remove(removing.id)
            await queryClient.invalidateQueries({ queryKey: adminKeys.sources })
            refreshOthers()
            toast.success(t('sources.removed', { name: removing.name }))
          } catch (error) {
            toast.error(errorMessage(error, t))
            throw error
          }
        }}
      />
    </div>
  )
}

function ModeOption({ value, label, description }: { value: LxSourceMode; label: string; description: string }) {
  const id = `source-mode-${value}`
  return (
    <div className="flex items-start gap-3">
      <RadioGroupItem value={value} id={id} className="mt-0.5" />
      <Label htmlFor={id} className="grid gap-0.5 font-normal">
        <span className="text-sm font-medium">{label}</span>
        <span className="text-xs text-muted-foreground">{description}</span>
      </Label>
    </div>
  )
}

interface SourceRowProps {
  source: LxSource
  /** Online music is on (starting or refreshing a script contacts other servers). */
  enabled: boolean
  fixed: boolean
  first: boolean
  last: boolean
  reordering: boolean
  onMove: (delta: number) => void
  onRemove: () => void
  onChanged: (next: LxSource) => void
}

function SourceRow({ source, enabled, fixed, first, last, reordering, onMove, onRemove, onChanged }: SourceRowProps) {
  const { t } = useTranslation('admin')
  const update = useMutation({
    mutationFn: (body: { enabled?: boolean; allowUpdateAlert?: boolean }) => api.admin.sources.update(source.id, body),
    onSuccess: onChanged,
    onError: (error) => toast.error(errorMessage(error, t)),
  })
  const reload = useMutation({
    mutationFn: () => api.admin.sources.reload(source.id),
    onSuccess: (next) => {
      onChanged(next)
      if (next.status === 'error') toast.error(t('sources.reloadFailed', { name: next.name }), { description: next.error })
      else toast.success(t('sources.reloaded', { name: next.name }))
    },
    onError: (error) => toast.error(errorMessage(error, t)),
  })
  const refresh = useMutation({
    mutationFn: () => api.admin.sources.refresh(source.id),
    onSuccess: (next) => {
      onChanged(next)
      toast.success(t('sources.refreshed', { name: next.name, version: next.version || '—' }))
    },
    onError: (error) => toast.error(errorMessage(error, t)),
  })
  const busy = reload.isPending || refresh.isPending
  const status = busy ? 'loading' : source.status
  const meta = [
    source.author ? t('sources.by', { author: source.author }) : '',
    source.sourceUrl ? t('sources.fromUrl') : t('sources.fromFile'),
    formatBytes(source.size),
    source.loadedAt ? t('sources.loadedAt', { when: formatRelative(source.loadedAt) }) : '',
  ].filter(Boolean)

  return (
    <div className={cn('grid gap-3 p-4 sm:grid-cols-[auto_minmax(0,1fr)_auto] sm:items-start', !source.enabled && 'opacity-70')}>
      <div className="flex gap-1 sm:flex-col">
        <Button variant="ghost" size="icon-sm" className="max-sm:size-10" onClick={() => onMove(-1)} disabled={first || reordering} aria-label={t('sources.moveUp', { name: source.name })} title={t('sources.moveUp', { name: source.name })}>
          <ArrowUp />
        </Button>
        <Button variant="ghost" size="icon-sm" className="max-sm:size-10" onClick={() => onMove(1)} disabled={last || reordering} aria-label={t('sources.moveDown', { name: source.name })} title={t('sources.moveDown', { name: source.name })}>
          <ArrowDown />
        </Button>
      </div>
      <div className="grid min-w-0 gap-1.5">
        <p className="flex flex-wrap items-center gap-2 text-sm font-medium">
          <span className="break-all">{source.name}</span>
          {source.version ? <span className="font-mono text-xs text-muted-foreground">v{source.version}</span> : null}
          <StatusBadge status={status} />
          {fixed ? <Badge variant="outline">{t('sources.fixedBadge')}</Badge> : null}
        </p>
        {source.description ? <p className="text-xs break-words text-muted-foreground">{source.description}</p> : null}
        <p className="text-xs text-muted-foreground">{meta.join(' · ')}</p>
        {source.platforms.length > 0 ? (
          <ul className="flex flex-wrap gap-1.5" aria-label={t('sources.platforms')}>
            {source.platforms.map((p) => (
              <li key={p.platform}>
                <Badge variant="secondary" className="font-normal">
                  {t(`sources.platformNames.${p.platform}`)} · {t(`sources.qualities.${p.qualities[p.qualities.length - 1]}`)}
                </Badge>
              </li>
            ))}
          </ul>
        ) : null}
        {source.error && status === 'error' ? (
          <p className="flex items-start gap-1.5 text-xs text-destructive">
            <CircleAlert className="mt-px size-3.5 shrink-0" aria-hidden />
            <span className="break-words">{source.error}</span>
          </p>
        ) : null}
        {source.updateAlert && source.allowUpdateAlert ? (
          <div className="grid gap-1.5 rounded-lg bg-muted/60 p-3 text-xs">
            <p className="flex items-center gap-1.5 font-medium">
              <BellRing className="size-3.5 text-primary" aria-hidden />
              {t('sources.updateAlert', { when: formatRelative(source.updateAlert.at) })}
            </p>
            <p className="break-words whitespace-pre-wrap text-muted-foreground">{source.updateAlert.log}</p>
            <div className="flex flex-wrap gap-2">
              {source.updateAlert.url ? (
                <Button asChild variant="outline" size="sm" className="max-sm:h-10">
                  <a href={source.updateAlert.url} target="_blank" rel="noreferrer noopener">
                    <ExternalLink />
                    {t('sources.openUpdate')}
                  </a>
                </Button>
              ) : null}
              {source.sourceUrl ? (
                <Button variant="outline" size="sm" className="max-sm:h-10" onClick={() => refresh.mutate()} disabled={!enabled || busy}>
                  <CloudDownload />
                  {t('sources.refresh')}
                </Button>
              ) : null}
            </div>
          </div>
        ) : null}
      </div>
      <div className="flex items-center gap-2 max-sm:justify-end">
        <Switch
          checked={source.enabled}
          onCheckedChange={(v) => update.mutate({ enabled: v })}
          disabled={update.isPending}
          aria-label={t('sources.enableSource', { name: source.name })}
        />
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon-sm" className="max-sm:size-10" aria-label={t('sources.more', { name: source.name })}>
              {busy ? <Spinner size="sm" className="text-current" /> : <MoreHorizontal />}
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="min-w-56">
            <DropdownMenuItem onSelect={() => reload.mutate()} disabled={!enabled || busy}>
              <RefreshCw />
              {t('sources.reload')}
            </DropdownMenuItem>
            {source.sourceUrl ? (
              <DropdownMenuItem onSelect={() => refresh.mutate()} disabled={!enabled || busy}>
                <CloudDownload />
                {t('sources.refresh')}
              </DropdownMenuItem>
            ) : null}
            <DropdownMenuItem onSelect={() => update.mutate({ allowUpdateAlert: !source.allowUpdateAlert })}>
              {source.allowUpdateAlert ? <BellOff /> : <BellRing />}
              {source.allowUpdateAlert ? t('sources.muteAlerts') : t('sources.showAlerts')}
            </DropdownMenuItem>
            {source.homepage ? (
              <DropdownMenuItem asChild>
                <a href={source.homepage} target="_blank" rel="noreferrer noopener">
                  <ExternalLink />
                  {t('sources.homepage')}
                </a>
              </DropdownMenuItem>
            ) : null}
            <DropdownMenuSeparator />
            <DropdownMenuItem variant="destructive" onSelect={onRemove}>
              <Trash2 />
              {t('sources.remove')}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </div>
  )
}

function StatusBadge({ status }: { status: LxSource['status'] }) {
  const { t } = useTranslation('admin')
  switch (status) {
    case 'ready':
      return <Badge variant="secondary" className="text-emerald-700 dark:text-emerald-400">{t('sources.status.ready')}</Badge>
    case 'loading':
      return (
        <Badge variant="outline">
          <Spinner size="sm" className="size-3 text-current" />
          {t('sources.status.loading')}
        </Badge>
      )
    case 'error':
      return <Badge variant="outline" className="border-destructive/40 text-destructive">{t('sources.status.error')}</Badge>
    default:
      return <Badge variant="outline" className="text-muted-foreground">{t('sources.status.idle')}</Badge>
  }
}

function Section({ title, description, action, children }: { title: string; description?: string; action?: ReactNode; children: ReactNode }) {
  return (
    <section className="grid gap-2">
      <div className="flex items-end gap-2 px-1">
        <h2 className="min-w-0 flex-1 text-xs font-medium tracking-wide text-muted-foreground uppercase">{title}</h2>
        {action}
      </div>
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
