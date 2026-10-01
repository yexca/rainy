import { useInfiniteQuery, useQuery } from '@tanstack/react-query'
import {
  ArchiveRestore,
  ArrowRight,
  CloudDownload,
  FilterX,
  History,
  ImageIcon,
  Languages,
  ListFilter,
  MicVocal,
  PencilLine,
  RefreshCw,
  Tags,
  Trash2,
  Upload,
  Eraser,
  type LucideIcon,
} from 'lucide-react'
import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { useSearchParams } from 'react-router'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { PageLoader, Spinner } from '@/components/spinner'
import { Button } from '@/components/ui/button'
import { useIsMobile } from '@/hooks/use-media-query'
import { api } from '@/lib/api/endpoints'
import type { EditLogEntry } from '@/lib/api/types'
import { formatBytes, formatDate, formatDateTime } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useUI } from '@/stores/ui'

import { readDetails } from '../lib/log-details'
import { manageKeys } from '../queries'

const PAGE = 50
const ACTION_ICONS: Record<string, LucideIcon> = {
  tags: Tags,
  tag_rebuild: RefreshCw,
  cover: ImageIcon,
  lyrics: MicVocal,
  rename: PencilLine,
  delete: Trash2,
  restore: ArchiveRestore,
  upload: Upload,
  download: CloudDownload,
  purge: Eraser,
  encoding: Languages,
}
const KNOWN_ACTIONS = new Set(Object.keys(ACTION_ICONS))

export default function HistoryPage() {
  const { t } = useTranslation('manage')
  const isMobile = useIsMobile()
  const [searchParams, setSearchParams] = useSearchParams()
  const trackId = searchParams.get('trackId') ?? ''
  const sentinel = useRef<HTMLDivElement>(null)

  const setTrack = (id: string) => {
    setSearchParams(id ? { trackId: id } : {}, { replace: true })
    window.scrollTo({ top: 0 })
  }

  const track = useQuery({
    queryKey: manageKeys.track(trackId),
    queryFn: ({ signal }) => api.tracks.get(trackId, { signal }),
    enabled: !!trackId,
    retry: false,
  })

  const log = useInfiniteQuery({
    queryKey: manageKeys.log(trackId),
    queryFn: ({ pageParam, signal }) => api.manage.log({ trackId: trackId || undefined, offset: pageParam, limit: PAGE }, { signal }),
    initialPageParam: 0,
    getNextPageParam: (last, pages) => {
      const loaded = pages.reduce((n, p) => n + p.items.length, 0)
      return loaded < last.total && last.items.length > 0 ? loaded : undefined
    },
  })

  // Infinite scroll.
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = log
  useEffect(() => {
    const el = sentinel.current
    if (!el || !hasNextPage) return
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting && !isFetchingNextPage) void fetchNextPage()
      },
      { rootMargin: '400px' },
    )
    observer.observe(el)
    return () => observer.disconnect()
  }, [hasNextPage, isFetchingNextPage, fetchNextPage])

  const entries = log.data?.pages.flatMap((p) => p.items) ?? []
  const total = log.data?.pages[0]?.total ?? 0

  // Group by day (entries are newest first).
  const groups: { day: string; items: EditLogEntry[] }[] = []
  for (const entry of entries) {
    const day = formatDate(entry.createdAt)
    const last = groups[groups.length - 1]
    if (last?.day === day) last.items.push(entry)
    else groups.push({ day, items: [entry] })
  }

  return (
    <Page>
      <PageHeader title={t('history.title')} subtitle={log.data ? t('history.count', { count: total }) : t('history.subtitle')} back={isMobile ? '/manage' : undefined}>
        {trackId ? (
          <div className="flex items-center gap-2 pb-4">
            <span className="inline-flex h-8 max-w-full items-center gap-2 rounded-full bg-secondary pr-1 pl-3 text-sm">
              <ListFilter className="size-3.5 shrink-0 text-muted-foreground" />
              <span className="truncate">{t('history.filteredBy', { title: track.data?.title ?? trackId })}</span>
              <Button variant="ghost" size="icon-xs" className="rounded-full" onClick={() => setTrack('')} aria-label={t('history.clearFilter')}>
                <FilterX />
              </Button>
            </span>
          </div>
        ) : null}
      </PageHeader>

      {log.isPending ? (
        <PageLoader />
      ) : log.isError ? (
        <ErrorState error={log.error} onRetry={() => void log.refetch()} retrying={log.isFetching} />
      ) : entries.length === 0 ? (
        <EmptyState icon={History} art="empty" title={t('history.empty')} description={t('history.emptyDescription')} />
      ) : (
        <div className="grid gap-6">
          {groups.map((group) => (
            <section key={group.day}>
              <h2 className="sticky top-[calc(var(--safe-top)+2.75rem)] z-10 -mx-1 mb-2 bg-background/90 px-1 py-1 text-xs font-medium tracking-wide text-muted-foreground uppercase backdrop-blur md:top-[calc(var(--safe-top)+3rem)]">
                {group.day}
              </h2>
              <ol className="grid gap-2">
                {group.items.map((entry) => (
                  <LogEntry key={entry.id} entry={entry} onFilter={trackId ? undefined : setTrack} />
                ))}
              </ol>
            </section>
          ))}
          <div ref={sentinel} className="flex justify-center py-2">
            {log.isFetchingNextPage ? <Spinner /> : null}
          </div>
        </div>
      )}
    </Page>
  )
}

function LogEntry({ entry, onFilter }: { entry: EditLogEntry; onFilter?: (trackId: string) => void }) {
  const { t } = useTranslation('manage')
  const openTagEditor = useUI((s) => s.openTagEditor)
  const Icon = ACTION_ICONS[entry.action] ?? History
  const details = readDetails(entry.details)
  // A restore's "from" is the internal trash location; the entry path already says where it went.
  if (entry.action === 'restore') details.move = undefined
  details.fields.sort((a, b) => fieldRank(a.key) - fieldRank(b.key))
  const hasBody = details.changes.length > 0 || details.move || details.fields.length > 0
  const time = new Date(entry.createdAt).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })

  return (
    <li className="rounded-xl border">
      <div className="flex items-start gap-3 p-3">
        <span
          className={cn(
            'grid size-8 shrink-0 place-items-center rounded-lg',
            entry.action === 'delete' || entry.action === 'purge' ? 'bg-destructive/10 text-destructive' : 'bg-primary/10 text-primary',
          )}
        >
          <Icon className="size-4" strokeWidth={1.75} />
        </span>
        <div className="grid min-w-0 flex-1 gap-0.5">
          <p className="text-sm">
            <span className="font-medium">{KNOWN_ACTIONS.has(entry.action) ? t(`history.actions.${entry.action}`) : entry.action}</span>
            <span className="text-muted-foreground">
              {' '}
              · {entry.username || t('history.system')} ·{' '}
              <time dateTime={new Date(entry.createdAt).toISOString()} title={formatDateTime(entry.createdAt)}>
                {time}
              </time>
            </span>
          </p>
          {entry.path ? (
            <p className="truncate font-mono text-[11px] text-muted-foreground" title={entry.path}>
              {entry.path}
            </p>
          ) : null}
        </div>
        {entry.trackId ? (
          <div className="flex shrink-0 gap-0.5">
            {onFilter ? (
              <Button variant="ghost" size="icon-sm" className="text-muted-foreground max-sm:size-10" onClick={() => onFilter(entry.trackId)} aria-label={t('history.showTrack')} title={t('history.showTrack')}>
                <ListFilter />
              </Button>
            ) : null}
            {entry.action !== 'delete' && entry.action !== 'purge' ? (
              <Button variant="ghost" size="icon-sm" className="text-muted-foreground max-sm:size-10" onClick={() => openTagEditor([entry.trackId])} aria-label={t('actions.editTags')} title={t('actions.editTags')}>
                <Tags />
              </Button>
            ) : null}
          </div>
        ) : null}
      </div>
      {hasBody ? (
        <div className="grid gap-1.5 border-t bg-muted/30 px-3 py-2.5 text-[13px]">
          {details.move ? (
            <p className="flex min-w-0 flex-wrap items-baseline gap-x-1.5 font-mono text-xs">
              <span className="break-all text-muted-foreground">{details.move.from}</span>
              <ArrowRight className="size-3 shrink-0 self-center text-muted-foreground" aria-hidden />
              <span className="break-all">{details.move.to}</span>
            </p>
          ) : null}
          {details.changes.map((c) => (
            <div key={c.key} className="grid gap-0.5 sm:grid-cols-[8rem_minmax(0,1fr)] sm:gap-3">
              <span className="font-mono text-[11px] text-muted-foreground sm:pt-0.5">{c.key}</span>
              <span className="flex min-w-0 flex-wrap items-baseline gap-x-1.5">
                {c.old.length > 0 ? (
                  <del className="max-h-40 overflow-y-auto break-all whitespace-pre-wrap text-muted-foreground decoration-destructive/50">{c.old.join('; ')}</del>
                ) : (
                  <span className="text-muted-foreground italic">{t('tools.empty')}</span>
                )}
                <ArrowRight className="size-3 shrink-0 self-center text-muted-foreground" aria-hidden />
                {c.new.length > 0 ? (
                  <ins className="max-h-40 overflow-y-auto font-medium break-all whitespace-pre-wrap no-underline">{c.new.join('; ')}</ins>
                ) : (
                  <span className="text-muted-foreground italic">{t('history.removed')}</span>
                )}
              </span>
            </div>
          ))}
          {details.fields.map((f) => {
            const field = describeField(f.key, f.value, t)
            return (
              <div key={f.key} className="grid gap-0.5 sm:grid-cols-[8rem_minmax(0,1fr)] sm:gap-3">
                <span className="text-xs text-muted-foreground">{field.label}</span>
                <span className={cn('break-all', field.mono && 'font-mono text-xs')}>{field.value}</span>
              </div>
            )
          })}
        </div>
      ) : null}
    </li>
  )
}

type TFunction = (key: string, options?: Record<string, unknown>) => string

/** Known `details` keys of the edit log (see the manage service) with readable labels / values. */
const FIELD_ORDER = ['op', 'embedded', 'folderImage', 'target', 'encoding', 'name', 'title', 'source', 'quality', 'via', 'size', 'missing']
const FIELD_KEYS = new Set(FIELD_ORDER)

function fieldRank(key: string): number {
  const i = FIELD_ORDER.indexOf(key)
  return i < 0 ? FIELD_ORDER.length : i
}

function describeField(key: string, value: string, t: TFunction): { label: string; value: string; mono?: boolean } {
  if (!FIELD_KEYS.has(key)) return { label: key, value }
  const label = t(`history.fields.${key}`)
  const yesNo = (v: string) => (v === 'true' ? t('history.values.yes') : v === 'false' ? t('history.values.no') : v)
  switch (key) {
    case 'op':
      return { label, value: value === 'set' || value === 'remove' ? t(`history.values.op_${value}`) : value }
    case 'target':
      return { label, value: value === 'lrc' || value === 'embedded' ? t(`history.values.target_${value}`) : value }
    case 'embedded':
    case 'missing':
      return { label, value: yesNo(value) }
    case 'size': {
      const n = Number(value)
      return { label, value: Number.isFinite(n) ? formatBytes(n) : value }
    }
    case 'encoding':
      return { label, value: value.toUpperCase() }
    case 'folderImage':
    case 'name':
    case 'source':
      return { label, value, mono: true }
    default:
      return { label, value }
  }
}
