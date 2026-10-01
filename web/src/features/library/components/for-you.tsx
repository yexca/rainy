import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { ChartColumn, Play, Sparkles } from 'lucide-react'
import { useMemo, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'

import { CoverArt } from '@/components/cover-art'
import { MascotArt } from '@/components/mascot'
import { Skeleton } from '@/components/ui/skeleton'
import { usePlayer } from '@/features/player/store'
import { useMascotArt } from '@/hooks/use-mascot-art'
import { api } from '@/lib/api/endpoints'
import type { DailyMix } from '@/lib/api/types'
import { formatDurationLong } from '@/lib/format'
import { currentLocale } from '@/lib/i18n'
import { queryKeys } from '@/lib/query-keys'
import { cn } from '@/lib/utils'

import { monthOf, rangeBounds, recapMonth } from '../lib/listening'
import { browserTimeZone, dailyMixQuery } from '../lib/queries'
import { SectionHeader } from './section-header'

/**
 * Home "Made for You" (§9.5a): today's daily mix and, during the first week of a month, last
 * month's listening recap.
 */
export function ForYou() {
  const { t } = useTranslation('library')
  const daily = useQuery(dailyMixQuery())
  const recap = useLastMonthRecap()

  if (daily.isError && !recap) return null
  if (daily.data && daily.data.tracks.length === 0 && !recap) return null
  return (
    <section aria-labelledby="home-for-you" className="mt-8 md:mt-10">
      <SectionHeader id="home-for-you" title={t('home.forYou')} />
      <div className="grid gap-3 sm:grid-cols-2">
        {daily.data ? (
          daily.data.tracks.length > 0 ? (
            <DailyMixCard mix={daily.data} />
          ) : null
        ) : daily.isPending ? (
          <Skeleton className="h-[104px] rounded-2xl md:h-[120px]" />
        ) : null}
        {recap}
      </div>
    </section>
  )
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

function DailyMixCard({ mix }: { mix: DailyMix }) {
  const { t } = useTranslation('library')
  const covers = useMemo(() => {
    const seen = new Set<string>()
    const out: string[] = []
    for (const tr of mix.tracks) {
      if (tr.coverArt && !seen.has(tr.albumId)) {
        seen.add(tr.albumId)
        out.push(tr.coverArt)
      }
      if (out.length === 4) break
    }
    return out
  }, [mix.tracks])
  const artists = useMemo(() => [...new Set(mix.tracks.map((tr) => tr.artist))].slice(0, 3), [mix.tracks])
  const [y, m, d] = mix.date.split('-').map(Number)
  const date = new Date(y, m - 1, d)
  const weekday = new Intl.DateTimeFormat(currentLocale(), { weekday: 'short' }).format(date)

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
      <div className="absolute inset-0 grid place-content-center bg-linear-to-br from-primary/80 to-black/50 text-center text-white">
        <span className="text-3xl leading-none font-bold tabular-nums">{d}</span>
        <span className="mt-0.5 text-xs font-medium tracking-wide uppercase opacity-90">{weekday}</span>
      </div>
    </>
  )

  return (
    <Card
      media={media}
      to="/daily"
      label={t('daily.open')}
      action={
        <button
          type="button"
          onClick={() => usePlayer.getState().playTracks(mix.tracks, 0, { shuffle: false })}
          aria-label={t('daily.play')}
          className="relative z-10 grid size-11 shrink-0 place-items-center rounded-full bg-primary text-primary-foreground shadow-sm transition-transform outline-none hover:brightness-110 focus-visible:ring-[3px] focus-visible:ring-ring/50 active:scale-90"
        >
          <Play className="size-5 translate-x-px" fill="currentColor" strokeWidth={0} aria-hidden />
        </button>
      }
    >
      <span className="flex items-center gap-1 text-xs font-medium text-primary">
        <Sparkles className="size-3.5" strokeWidth={2} aria-hidden />
        {t('daily.eyebrow')}
      </span>
      <span className="mt-0.5 block truncate text-lg leading-6 font-semibold">{t('daily.title')}</span>
      <span className="mt-0.5 line-clamp-2 text-[13px] leading-5 text-muted-foreground">
        {t('daily.artists', { artists: artists.join(t('daily.separator')), count: mix.tracks.length })}
      </span>
    </Card>
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
