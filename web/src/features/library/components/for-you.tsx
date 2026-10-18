import { keepPreviousData, useQuery, type UseQueryResult } from '@tanstack/react-query'
import { CalendarDays, ChartColumn, Play, Sparkles, type LucideIcon } from 'lucide-react'
import { useMemo, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'

import { CoverArt } from '@/components/cover-art'
import { MascotArt } from '@/components/mascot'
import { Skeleton } from '@/components/ui/skeleton'
import { usePlayer } from '@/features/player/store'
import { useMascotArt } from '@/hooks/use-mascot-art'
import { api } from '@/lib/api/endpoints'
import type { DailyMix, Track, WeeklyMix } from '@/lib/api/types'
import { formatDurationLong } from '@/lib/format'
import { currentLocale } from '@/lib/i18n'
import { queryKeys } from '@/lib/query-keys'
import { cn } from '@/lib/utils'

import { monthOf, rangeBounds, recapMonth } from '../lib/listening'
import { weekDays } from '../lib/mix-period'
import { browserTimeZone, dailyMixQuery, weeklyMixQuery } from '../lib/queries'
import { SectionHeader } from './section-header'

/**
 * Home "Made for You" (§9.5a): today's daily mix, this week's weekly mix and, during the first
 * week of a month, last month's listening recap.
 */
export function ForYou() {
  const { t } = useTranslation('library')
  const daily = useQuery(dailyMixQuery())
  const weekly = useQuery(weeklyMixQuery())
  const recap = useLastMonthRecap()

  const cards = [
    { key: 'daily', node: mixSlot(daily, (mix) => <DailyMixCard mix={mix} />) },
    { key: 'weekly', node: mixSlot(weekly, (mix) => <WeeklyMixCard mix={mix} />) },
    { key: 'recap', node: recap },
  ].filter((card) => card.node != null)
  if (cards.length === 0) return null
  return (
    <section aria-labelledby="home-for-you" className="mt-8 md:mt-10">
      <SectionHeader id="home-for-you" title={t('home.forYou')} />
      <div className={cn('grid gap-3 sm:grid-cols-2', cards.length > 2 && 'xl:grid-cols-3')}>
        {cards.map((card) => (
          <div key={card.key} className="min-w-0">
            {card.node}
          </div>
        ))}
      </div>
    </section>
  )
}

/** A mix's card, a placeholder while it loads, or nothing when it failed or has no songs. */
function mixSlot<T extends { tracks: Track[] }>(query: UseQueryResult<T>, render: (mix: T) => ReactNode): ReactNode {
  if (query.data) return query.data.tracks.length > 0 ? render(query.data) : null
  return query.isPending ? <Skeleton className="h-[104px] rounded-2xl md:h-[120px]" /> : null
}

function Card({ media, to, label, children, action }: { media: ReactNode; to: string; label: string; children: ReactNode; action?: ReactNode }) {
  return (
    <div className="group/card relative flex items-center gap-4 rounded-2xl border bg-card p-3 pr-4 transition-colors hover:bg-accent/50 has-[a:focus-visible]:ring-[3px] has-[a:focus-visible]:ring-ring/50 md:p-4">
      <div className="relative size-20 shrink-0 overflow-hidden rounded-xl md:size-22">{media}</div>
      <div className="min-w-0 flex-1">
        <Link to={to} className="outline-none after:absolute after:inset-0 after:rounded-2xl" aria-label={label}>
          {children}
        </Link>
      </div>
      {action}
    </div>
  )
}

interface MixCardProps {
  tracks: Track[]
  to: string
  icon: LucideIcon
  eyebrow: string
  title: string
  openLabel: string
  playLabel: string
  /** Shown over the cover mosaic (the day or the week). */
  badge: ReactNode
  /** Tint over the mosaic, so the mixes are told apart at a glance. */
  tint: string
}

/** A stored mix: a 2×2 mosaic of its first albums under a badge, its first artists and a play button. */
function MixCard({ tracks, to, icon: Icon, eyebrow, title, openLabel, playLabel, badge, tint }: MixCardProps) {
  const { t } = useTranslation('library')
  const covers = useMemo(() => {
    const seen = new Set<string>()
    const out: string[] = []
    for (const tr of tracks) {
      if (tr.coverArt && !seen.has(tr.albumId)) {
        seen.add(tr.albumId)
        out.push(tr.coverArt)
      }
      if (out.length === 4) break
    }
    return out
  }, [tracks])
  const artists = useMemo(() => [...new Set(tracks.map((tr) => tr.artist))].slice(0, 3), [tracks])

  const media = (
    <>
      {covers.length === 4 ? (
        <div className="grid size-full grid-cols-2 grid-rows-2">
          {covers.map((c) => (
            <CoverArt key={c} coverArt={c} size={64} flat rounded="none" className="size-full" alt="" />
          ))}
        </div>
      ) : (
        <CoverArt coverArt={covers[0] ?? ''} size={128} flat rounded="none" className="size-full" alt="" />
      )}
      <div className={cn('absolute inset-0 grid place-content-center text-center text-white', tint)}>{badge}</div>
    </>
  )

  return (
    <Card
      media={media}
      to={to}
      label={openLabel}
      action={
        <button
          type="button"
          onClick={() => usePlayer.getState().playTracks(tracks, undefined, { shuffle: false })}
          aria-label={playLabel}
          className="relative z-10 grid size-11 shrink-0 place-items-center rounded-full bg-primary text-primary-foreground shadow-sm transition-transform outline-none hover:brightness-110 focus-visible:ring-[3px] focus-visible:ring-ring/50 active:scale-90"
        >
          <Play className="size-5 translate-x-px" fill="currentColor" strokeWidth={0} aria-hidden />
        </button>
      }
    >
      <span className="flex items-center gap-1 text-xs font-medium text-primary">
        <Icon className="size-3.5" strokeWidth={2} aria-hidden />
        {eyebrow}
      </span>
      <span className="mt-0.5 block truncate text-lg leading-6 font-semibold">{title}</span>
      <span className="mt-0.5 line-clamp-2 text-[13px] leading-5 text-muted-foreground">
        {t('mix.artists', { artists: artists.join(t('mix.separator')), count: tracks.length })}
      </span>
    </Card>
  )
}

function DailyMixCard({ mix }: { mix: DailyMix }) {
  const { t } = useTranslation('library')
  const [y, m, d] = mix.date.split('-').map(Number)
  const weekday = new Intl.DateTimeFormat(currentLocale(), { weekday: 'short' }).format(new Date(y, m - 1, d))
  return (
    <MixCard
      tracks={mix.tracks}
      to="/daily"
      icon={Sparkles}
      eyebrow={t('daily.eyebrow')}
      title={t('daily.title')}
      openLabel={t('daily.open')}
      playLabel={t('daily.play')}
      tint="bg-linear-to-br from-primary/80 to-black/50"
      badge={
        <>
          <span className="text-3xl leading-none font-bold tabular-nums">{d}</span>
          <span className="mt-0.5 text-xs font-medium tracking-wide uppercase opacity-90">{weekday}</span>
        </>
      }
    />
  )
}

function WeeklyMixCard({ mix }: { mix: WeeklyMix }) {
  const { t } = useTranslation('library')
  const days = weekDays(mix.week)
  // "28–4" over "Sep – Oct": the week's first and last day, then its month(s).
  const months = days ? new Intl.DateTimeFormat(currentLocale(), { month: 'short' }).formatRange(days[0], days[1]) : ''
  return (
    <MixCard
      tracks={mix.tracks}
      to="/weekly"
      icon={CalendarDays}
      eyebrow={t('weekly.eyebrow')}
      title={t('weekly.title')}
      openLabel={t('weekly.open')}
      playLabel={t('weekly.play')}
      tint="bg-linear-to-tr from-black/65 to-primary/70"
      badge={
        days ? (
          <>
            <span className="text-xl leading-none font-bold tabular-nums">
              {days[0].getDate()}–{days[1].getDate()}
            </span>
            <span className="mt-1 text-[11px] font-medium tracking-wide uppercase opacity-90">{months}</span>
          </>
        ) : null
      }
    />
  )
}

/** Last month's recap card during the first week of a month, when that month had plays. */
function useLastMonthRecap(): ReactNode {
  const { t } = useTranslation('library')
  const showArt = useMascotArt()
  const [range] = useState(() => recapMonth())
  const bounds = useMemo(() => (range ? rangeBounds(range) : null), [range])
  const tz = useMemo(() => browserTimeZone(), [])
  const report = useQuery({
    queryKey: [...queryKeys.listening, 'report', range, bounds, tz, 'recap'],
    queryFn: ({ signal }) => api.listening.report({ ...bounds, tz, limit: 1 }, { signal }),
    enabled: !!range && !!bounds,
    placeholderData: keepPreviousData,
  })
  const month = range ? monthOf(range) : null
  if (!range || !month || !report.data || report.data.totals.plays === 0) return null

  const name = new Intl.DateTimeFormat(currentLocale(), { month: 'long' }).format(new Date(month.year, month.month - 1, 1))
  const top = report.data.topArtists[0]
  const media = (
    <div className={cn('grid size-full place-items-center bg-linear-to-br from-primary/25 to-primary/5 text-primary')}>
      {showArt ? <MascotArt pose="report" className="h-[92%] translate-y-1" /> : <ChartColumn className="size-8" strokeWidth={1.75} aria-hidden />}
    </div>
  )
  return (
    <Card media={media} to={`/listening?range=${range}`} label={t('home.recap.open', { month: name })}>
      <span className="flex items-center gap-1 text-xs font-medium text-primary">
        <ChartColumn className="size-3.5" strokeWidth={2} aria-hidden />
        {t('home.recap.eyebrow')}
      </span>
      <span className="mt-0.5 block truncate text-lg leading-6 font-semibold">{t('home.recap.title', { month: name })}</span>
      <span className="mt-0.5 line-clamp-2 text-[13px] leading-5 text-muted-foreground">
        {top
          ? t('home.recap.summaryTop', { time: formatDurationLong(report.data.totals.duration), artist: top.name })
          : t('home.recap.summary', { time: formatDurationLong(report.data.totals.duration) })}
      </span>
    </Card>
  )
}
