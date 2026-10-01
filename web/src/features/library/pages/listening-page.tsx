import { keepPreviousData, useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { ArrowDown, ArrowUp, ChartColumn, Disc3, History, MicVocal, Music } from 'lucide-react'
import { useMemo, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useSearchParams } from 'react-router'

import { CoverArt } from '@/components/cover-art'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { Spinner } from '@/components/spinner'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectSeparator, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent } from '@/components/ui/tabs'
import { usePlayer } from '@/features/player/store'
import { useIsMobile } from '@/hooks/use-media-query'
import { api } from '@/lib/api/endpoints'
import type { ListeningReport, ListeningTopEntry, ListeningTopTrack, ListeningTotals, Play, Track } from '@/lib/api/types'
import { formatDurationLong, formatNumber } from '@/lib/format'
import { currentLocale } from '@/lib/i18n'
import { queryKeys } from '@/lib/query-keys'
import { cn } from '@/lib/utils'

import { BarList, ClockHeatmap, TimelineChart, type TimelineMetric } from '../components/listening-charts'
import { SectionHeader } from '../components/section-header'
import { SegmentedTabs } from '../components/tab-bar'
import { TrackActionsMenu } from '../components/track-actions'
import { HorizontalShelf } from '../components/horizontal-shelf'
import { ListeningRecap } from '../components/listening-recap'
import {
  ROLLING_RANGES,
  groupByDay,
  isListeningRange,
  monthOf,
  monthsSince,
  percentChange,
  rangeBounds,
  relativeDay,
  yearOf,
  yearsSince,
  type ListeningRange,
} from '../lib/listening'

type Tab = 'overview' | 'history'
const TABS: readonly Tab[] = ['overview', 'history']
const HISTORY_PAGE = 100

/** "September 2026" / "2026年9月". */
function monthLabel(year: number, month: number): string {
  return new Intl.DateTimeFormat(currentLocale(), { year: 'numeric', month: 'long' }).format(new Date(year, month - 1, 1))
}

function timeZone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || ''
  } catch {
    return ''
  }
}

/** `/listening`: the signed-in user's listening report (Last.fm style) and play history. */
export default function ListeningPage() {
  const { t } = useTranslation('library')
  const isMobile = useIsMobile()
  const [params, setParams] = useSearchParams()
  const tabParam = params.get('tab') as Tab | null
  const tab: Tab = tabParam && TABS.includes(tabParam) ? tabParam : 'overview'
  const rangeParam = params.get('range')
  const range: ListeningRange = isListeningRange(rangeParam) ? rangeParam : '30d'
  // Bounds are recomputed per range change only, so the query key stays stable.
  const bounds = useMemo(() => rangeBounds(range), [range])
  const tz = useMemo(() => timeZone(), [])

  const report = useQuery({
    queryKey: [...queryKeys.listening, 'report', range, bounds, tz],
    queryFn: ({ signal }) => api.listening.report({ ...bounds, tz }, { signal }),
    placeholderData: keepPreviousData,
  })

  const update = (next: { tab?: Tab; range?: ListeningRange }) => {
    const out = new URLSearchParams(params)
    const nextTab = next.tab ?? tab
    const nextRange = next.range ?? range
    if (nextTab === 'overview') out.delete('tab')
    else out.set('tab', nextTab)
    if (nextRange === '30d') out.delete('range')
    else out.set('range', nextRange)
    setParams(out, { replace: true })
  }

  const years = yearsSince(report.data?.firstPlayAt ?? 0)
  const months = monthsSince(report.data?.firstPlayAt ?? 0)
  // A month opened from a link (the home page recap) stays selectable before the report loads.
  if (monthOf(range) && !months.includes(range)) months.unshift(range)
  const rangeLabel = (r: ListeningRange) => {
    const month = monthOf(r)
    if (month) return monthLabel(month.year, month.month)
    const year = yearOf(r)
    return year !== null ? String(year) : t(`listening.ranges.${r}`)
  }
  const recapKind = monthOf(range) ? 'month' : yearOf(range) !== null ? 'year' : null

  return (
    <Page>
      <PageHeader
        title={t('listening.title')}
        back={isMobile}
        subtitle={t('listening.subtitle')}
        actions={
          <Select value={range} onValueChange={(v) => update({ range: v as ListeningRange })}>
            <SelectTrigger className="w-full sm:w-48" aria-label={t('listening.rangeLabel')}>
              <SelectValue>{rangeLabel(range)}</SelectValue>
            </SelectTrigger>
            <SelectContent>
              {ROLLING_RANGES.map((r) => (
                <SelectItem key={r} value={r}>
                  {rangeLabel(r)}
                </SelectItem>
              ))}
              {months.length > 0 ? <SelectSeparator /> : null}
              {months.map((m) => (
                <SelectItem key={m} value={m}>
                  {rangeLabel(m)}
                </SelectItem>
              ))}
              {years.length > 0 ? <SelectSeparator /> : null}
              {years.map((y) => (
                <SelectItem key={y} value={`y${y}`}>
                  {String(y)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        }
      />
      <Tabs value={tab} onValueChange={(v) => update({ tab: v as Tab })}>
        <SegmentedTabs<Tab> items={TABS.map((value) => ({ value, label: t(`listening.tabs.${value}`) }))} />
        <TabsContent value="overview">
          {report.data ? (
            report.data.totals.plays === 0 ? (
              <NoPlays allTime={range === 'all'} />
            ) : (
              <Overview
                report={report.data}
                stale={report.isPlaceholderData}
                recap={recapKind ? { kind: recapKind, period: rangeLabel(range) } : null}
              />
            )
          ) : report.isError ? (
            <ErrorState error={report.error} onRetry={() => void report.refetch()} retrying={report.isFetching} />
          ) : (
            <OverviewSkeleton />
          )}
        </TabsContent>
        <TabsContent value="history">
          <HistoryList from={bounds.from} to={bounds.to} />
        </TabsContent>
      </Tabs>
    </Page>
  )
}

function NoPlays({ allTime }: { allTime: boolean }) {
  const { t } = useTranslation('library')
  return (
    <EmptyState
      icon={ChartColumn}
      art="report"
      title={allTime ? t('listening.empty.title') : t('listening.empty.periodTitle')}
      description={allTime ? t('listening.empty.description') : t('listening.empty.periodDescription')}
    />
  )
}

// ---- overview

function Overview({
  report,
  stale,
  recap,
}: {
  report: ListeningReport
  stale: boolean
  /** Calendar months and years open with a recap card. */
  recap: { kind: 'month' | 'year'; period: string } | null
}) {
  const { t } = useTranslation('library')
  const [metric, setMetric] = useState<TimelineMetric>('plays')
  const tracks = report.topTracks
  const playable = tracks.filter((tt): tt is ListeningTopTrack & { track: Track } => tt.track !== null)

  return (
    <div className={cn('grid grid-cols-[minmax(0,1fr)] gap-10 transition-opacity', stale && 'opacity-60')}>
      {recap ? <ListeningRecap report={report} period={recap.period} kind={recap.kind} /> : null}
      <StatTiles report={report} />

      <section aria-labelledby="listening-timeline">
        <SectionHeader
          id="listening-timeline"
          title={t('listening.sections.timeline')}
          actions={
            <div role="radiogroup" aria-label={t('listening.chart.metric')} className="flex rounded-lg bg-muted p-0.5">
              {(['plays', 'duration'] as const).map((m) => (
                <button
                  key={m}
                  type="button"
                  role="radio"
                  aria-checked={metric === m}
                  onClick={() => setMetric(m)}
                  className={cn(
                    'h-8 rounded-md px-3 text-[13px] font-medium outline-none focus-visible:ring-2 focus-visible:ring-ring/50 max-md:h-9',
                    metric === m ? 'bg-background shadow-sm dark:bg-accent' : 'text-muted-foreground hover:text-foreground',
                  )}
                >
                  {m === 'plays' ? t('listening.stats.plays') : t('listening.chart.hours')}
                </button>
              ))}
            </div>
          }
        />
        <TimelineChart buckets={report.timeline} bucket={report.bucket} metric={metric} />
      </section>

      {report.topArtists.length > 0 ? (
        <HorizontalShelf title={t('listening.sections.topArtists')} itemSize="sm" className="mt-0! md:mt-0!">
          {report.topArtists.map((a, i) => (
            <TopArtist key={a.id} entry={a} rank={i + 1} />
          ))}
        </HorizontalShelf>
      ) : null}

      {report.topAlbums.length > 0 ? (
        <HorizontalShelf title={t('listening.sections.topAlbums')} className="mt-0! md:mt-0!">
          {report.topAlbums.map((a, i) => (
            <TopAlbum key={a.id} entry={a} rank={i + 1} />
          ))}
        </HorizontalShelf>
      ) : null}

      <section aria-labelledby="listening-tracks">
        <SectionHeader id="listening-tracks" title={t('listening.sections.topTracks')} />
        <ol>
          {tracks.map((tt, i) => (
            <TopTrackRow
              key={tt.id}
              entry={tt}
              rank={i + 1}
              onPlay={() => {
                const start = tt.track ? playable.findIndex((p) => p.id === tt.id) : -1
                if (start >= 0)
                  usePlayer.getState().playTracks(
                    playable.map((p) => p.track),
                    start,
                  )
              }}
            />
          ))}
        </ol>
      </section>

      <div className="grid grid-cols-[minmax(0,1fr)] gap-10 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
        <section aria-labelledby="listening-clock">
          <SectionHeader id="listening-clock" title={t('listening.sections.clock')} />
          <ClockHeatmap clock={report.clock} />
        </section>
        <div className="grid content-start gap-10">
          {report.topGenres.length > 0 ? (
            <section aria-labelledby="listening-genres">
              <SectionHeader id="listening-genres" title={t('listening.sections.genres')} />
              <BarList
                items={report.topGenres.map((g) => ({
                  key: g.id,
                  label: g.name,
                  value: g.plays,
                  detail: t('listening.plays', { count: g.plays, formatted: formatNumber(g.plays) }),
                }))}
              />
            </section>
          ) : null}
          {report.clients.length > 0 ? (
            <section aria-labelledby="listening-clients">
              <SectionHeader id="listening-clients" title={t('listening.sections.clients')} />
              <BarList
                items={report.clients.map((c) => ({
                  key: c.id,
                  label: c.name || t('listening.unknownClient'),
                  value: c.plays,
                  detail: t('listening.plays', { count: c.plays, formatted: formatNumber(c.plays) }),
                }))}
              />
            </section>
          ) : null}
        </div>
      </div>

      <p className="text-[13px] text-muted-foreground">
        {t('listening.scrobbleHint')}{' '}
        <Link to="/settings#scrobbling" className="font-medium text-primary hover:underline">
          {t('listening.scrobbleLink')}
        </Link>
      </p>
    </div>
  )
}

function Delta({ current, previous }: { current: number; previous: number | null | undefined }) {
  const { t } = useTranslation('library')
  const pct = percentChange(current, previous)
  if (pct === null) return null
  const Icon = pct >= 0 ? ArrowUp : ArrowDown
  return (
    <span className="inline-flex items-center gap-0.5 text-xs text-muted-foreground tabular-nums">
      <Icon className="size-3" strokeWidth={2} aria-hidden />
      <span className="sr-only">{pct >= 0 ? t('listening.stats.up') : t('listening.stats.down')}</span>
      {Math.abs(pct)}%
    </span>
  )
}

function Tile({ label, value, extra }: { label: string; value: ReactNode; extra?: ReactNode }) {
  return (
    <div className="grid content-start gap-1 rounded-xl border bg-card p-4">
      <p className="text-[13px] text-muted-foreground">{label}</p>
      <p className="text-2xl font-semibold tracking-tight tabular-nums">{value}</p>
      {extra ? <div className="flex flex-wrap items-center gap-x-2 text-xs text-muted-foreground">{extra}</div> : null}
    </div>
  )
}

function StatTiles({ report }: { report: ListeningReport }) {
  const { t } = useTranslation('library')
  const { totals, previous } = report
  const prev = (key: keyof ListeningTotals) => previous?.[key]
  const vsPrevious = previous ? <span>{t('listening.stats.vsPrevious')}</span> : null
  return (
    <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
      <Tile
        label={t('listening.stats.plays')}
        value={formatNumber(totals.plays)}
        extra={
          <>
            <Delta current={totals.plays} previous={prev('plays')} />
            {vsPrevious}
          </>
        }
      />
      <Tile
        label={t('listening.stats.time')}
        value={formatDurationLong(totals.duration)}
        extra={
          <>
            <Delta current={totals.duration} previous={prev('duration')} />
            {vsPrevious}
          </>
        }
      />
      <Tile
        label={t('listening.stats.artists')}
        value={formatNumber(totals.artists)}
        extra={report.newArtists > 0 ? t('listening.stats.new', { count: report.newArtists, formatted: formatNumber(report.newArtists) }) : null}
      />
      <Tile
        label={t('listening.stats.tracks')}
        value={formatNumber(totals.tracks)}
        extra={report.newTracks > 0 ? t('listening.stats.new', { count: report.newTracks, formatted: formatNumber(report.newTracks) }) : null}
      />
      <Tile label={t('listening.stats.albums')} value={formatNumber(totals.albums)} />
      <Tile
        label={t('listening.stats.activeDays')}
        value={formatNumber(report.activeDays)}
        extra={t('listening.stats.streak', { count: report.longestStreak, formatted: formatNumber(report.longestStreak) })}
      />
      <Tile
        label={t('listening.stats.perDay')}
        value={formatNumber(Math.round(totals.plays / Math.max(1, Math.ceil((report.to - report.from) / 86_400_000))))}
      />
      <Tile label={t('listening.stats.since')} value={new Intl.DateTimeFormat(currentLocale(), { dateStyle: 'medium' }).format(new Date(report.firstPlayAt))} />
    </div>
  )
}

function Rank({ n, className }: { n: number; className?: string }) {
  return <span className={cn('w-6 shrink-0 text-center text-sm font-semibold text-muted-foreground tabular-nums', className)}>{n}</span>
}

/** "12 plays" in the active language. */
function usePlaysLabel(): (plays: number) => string {
  const { t } = useTranslation('library')
  return (plays) => t('listening.plays', { count: plays, formatted: formatNumber(plays) })
}

function TopArtist({ entry, rank }: { entry: ListeningTopEntry; rank: number }) {
  const playsLabel = usePlaysLabel()
  const body = (
    <>
      <CoverArt coverArt={entry.coverArt} size={160} fluid shape="circle" icon={MicVocal} alt={entry.name} />
      <p className="mt-2 truncate text-[14px] leading-5 font-medium md:text-[13px]">
        <span className="text-muted-foreground tabular-nums">{rank} </span>
        {entry.name}
      </p>
      <p className="truncate text-[13px] leading-5 text-muted-foreground md:text-xs">{playsLabel(entry.plays)}</p>
    </>
  )
  return entry.available ? (
    <Link to={`/artists/${entry.id}`} className="block min-w-0 rounded-xl text-center outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 active:scale-[0.97]">
      {body}
    </Link>
  ) : (
    <div className="min-w-0 text-center opacity-70">{body}</div>
  )
}

function TopAlbum({ entry, rank }: { entry: ListeningTopEntry; rank: number }) {
  const playsLabel = usePlaysLabel()
  const body = (
    <>
      <CoverArt coverArt={entry.coverArt} size={200} fluid icon={Disc3} alt={entry.name} />
      <p className="mt-2 truncate text-[14px] leading-5 font-medium md:text-[13px]">
        <span className="text-muted-foreground tabular-nums">{rank} </span>
        {entry.name}
      </p>
      <p className="truncate text-[13px] leading-5 text-muted-foreground md:text-xs">
        {entry.artist ? `${entry.artist} · ` : ''}
        {playsLabel(entry.plays)}
      </p>
    </>
  )
  return entry.available ? (
    <Link to={`/albums/${entry.id}`} className="block min-w-0 rounded-lg outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 active:scale-[0.97]">
      {body}
    </Link>
  ) : (
    <div className="min-w-0 opacity-70">{body}</div>
  )
}

function TopTrackRow({ entry, rank, onPlay }: { entry: ListeningTopTrack; rank: number; onPlay: () => void }) {
  const { t } = useTranslation('library')
  const playsLabel = usePlaysLabel()
  const track = entry.track
  return (
    <li className="flex min-h-[60px] items-center gap-3 border-b border-border/60 last:border-b-0 md:min-h-12">
      <Rank n={rank} />
      <button
        type="button"
        onClick={onPlay}
        disabled={!track}
        title={track ? undefined : t('listening.unavailable')}
        className="flex min-w-0 flex-1 items-center gap-3 rounded-md py-2 text-left outline-none focus-visible:ring-2 focus-visible:ring-ring/50 enabled:active:scale-[0.99] disabled:cursor-default"
      >
        <CoverArt coverArt={entry.coverArt} size={44} className={cn('md:size-9!', !track && 'opacity-60')} />
        <span className="min-w-0 flex-1">
          <span className={cn('block truncate text-[15px] md:text-sm', !track && 'text-muted-foreground')}>{entry.name}</span>
          <span className="block truncate text-[13px] text-muted-foreground md:text-xs">{entry.artist}</span>
        </span>
      </button>
      <span className="shrink-0 text-[13px] text-muted-foreground tabular-nums">{playsLabel(entry.plays)}</span>
      {track ? <TrackActionsMenu tracks={[track]} /> : <span className="size-9 shrink-0" aria-hidden />}
    </li>
  )
}

function OverviewSkeleton() {
  return (
    <div className="grid grid-cols-[minmax(0,1fr)] gap-10" aria-hidden>
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        {Array.from({ length: 8 }, (_, i) => (
          <Skeleton key={i} className="h-24 rounded-xl" />
        ))}
      </div>
      <Skeleton className="h-52 rounded-xl" />
      <div className="flex gap-4 overflow-hidden">
        {Array.from({ length: 6 }, (_, i) => (
          <Skeleton key={i} className="aspect-square w-32 shrink-0 rounded-full" />
        ))}
      </div>
    </div>
  )
}

// ---- history

function HistoryList({ from, to }: { from: number; to: number }) {
  const { t } = useTranslation('library')
  const history = useInfiniteQuery({
    queryKey: [...queryKeys.listening, 'history', from, to],
    queryFn: ({ pageParam, signal }) => api.listening.history({ from, to, offset: pageParam, limit: HISTORY_PAGE }, { signal }),
    initialPageParam: 0,
    getNextPageParam: (last, pages) => {
      const loaded = pages.reduce((n, p) => n + p.items.length, 0)
      return last.items.length === 0 || loaded >= last.total ? undefined : loaded
    },
  })
  const plays = useMemo(() => history.data?.pages.flatMap((p) => p.items) ?? [], [history.data])
  const groups = useMemo(() => groupByDay(plays, (p) => p.playedAt), [plays])
  const total = history.data?.pages[0]?.total ?? 0

  if (history.isPending) {
    return (
      <div className="grid gap-2" aria-hidden>
        {Array.from({ length: 10 }, (_, i) => (
          <Skeleton key={i} className="h-12 rounded-md" />
        ))}
      </div>
    )
  }
  if (history.isError) {
    return <ErrorState error={history.error} onRetry={() => void history.refetch()} retrying={history.isFetching} />
  }
  if (plays.length === 0) {
    return <EmptyState icon={History} title={t('listening.history.empty')} description={t('listening.empty.description')} />
  }

  const dayTitle = (key: string, at: number) => {
    const rel = relativeDay(key)
    if (rel) return t(`listening.history.${rel}`)
    return new Intl.DateTimeFormat(currentLocale(), { weekday: 'long', year: 'numeric', month: 'long', day: 'numeric' }).format(new Date(at))
  }

  return (
    <div className="grid grid-cols-[minmax(0,1fr)] gap-6">
      <p className="text-[13px] text-muted-foreground">{t('listening.history.count', { count: total, formatted: formatNumber(total) })}</p>
      {groups.map((g) => (
        <section key={g.key} aria-label={dayTitle(g.key, g.items[0].playedAt)}>
          <h3 className="mb-1 text-[13px] font-semibold tracking-wide text-muted-foreground uppercase">{dayTitle(g.key, g.items[0].playedAt)}</h3>
          <ul>
            {g.items.map((p) => (
              <HistoryRow key={p.id} play={p} />
            ))}
          </ul>
        </section>
      ))}
      {history.hasNextPage ? (
        <div className="flex justify-center">
          <Button variant="outline" onClick={() => void history.fetchNextPage()} disabled={history.isFetchingNextPage} className="max-md:h-11">
            {history.isFetchingNextPage ? <Spinner size="sm" className="text-current" /> : null}
            {t('listening.history.more')}
          </Button>
        </div>
      ) : null}
    </div>
  )
}

function HistoryRow({ play }: { play: Play }) {
  const { t } = useTranslation('library')
  const track = play.track
  const time = new Intl.DateTimeFormat(currentLocale(), { hour: 'numeric', minute: '2-digit' }).format(new Date(play.playedAt))
  const subtitle = [play.artist, play.album].filter(Boolean).join(' · ')
  return (
    <li className="flex min-h-[60px] items-center gap-3 border-b border-border/60 last:border-b-0 md:min-h-12">
      <button
        type="button"
        disabled={!track}
        onClick={() => track && usePlayer.getState().playTracks([track], 0)}
        title={track ? undefined : t('listening.unavailable')}
        className="flex min-w-0 flex-1 items-center gap-3 rounded-md py-2 text-left outline-none focus-visible:ring-2 focus-visible:ring-ring/50 enabled:active:scale-[0.99] disabled:cursor-default"
      >
        <CoverArt coverArt={track?.coverArt} size={44} icon={Music} className={cn('md:size-9!', !track && 'opacity-60')} />
        <span className="min-w-0 flex-1">
          <span className={cn('block truncate text-[15px] md:text-sm', !track && 'text-muted-foreground')}>{play.title || t('listening.unknownTrack')}</span>
          <span className="block truncate text-[13px] text-muted-foreground md:text-xs">{subtitle}</span>
        </span>
      </button>
      <span className="grid shrink-0 justify-items-end text-[13px] text-muted-foreground tabular-nums md:text-xs">
        <span>{time}</span>
        {play.client ? <span className="max-w-24 truncate text-[11px]">{play.client}</span> : null}
      </span>
      {track ? <TrackActionsMenu tracks={[track]} /> : <span className="size-9 shrink-0" aria-hidden />}
    </li>
  )
}
