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
import { HorizontalShelf } from './horizontal-shelf'

/**
 * Home "Made for You" (§9.5a): Apple Music-style editorial poster cards in a horizontal shelf —
 * today's daily mix, this week's weekly mix, and during the first week of a month, last month's
 * listening recap.
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
    <HorizontalShelf id="home-for-you" title={t('home.forYou')} itemSize="poster">
      {cards.map((card) => (
        <div key={card.key} className="min-w-0">
          {card.node}
        </div>
      ))}
    </HorizontalShelf>
  )
}

/** A mix's poster card, a placeholder while it loads, or nothing when it failed or has no songs. */
function mixSlot<T extends { tracks: Track[] }>(query: UseQueryResult<T>, render: (mix: T) => ReactNode): ReactNode {
  if (query.data) return query.data.tracks.length > 0 ? render(query.data) : null
  return query.isPending ? (
    <div className="space-y-2" aria-hidden>
      <Skeleton className="aspect-[16/10] w-full rounded-2xl" />
      <Skeleton className="h-4 w-3/4" />
    </div>
  ) : null
}

function PosterCard({
  artwork,
  scrim,
  icon: Icon,
  eyebrow,
  badge,
  title,
  caption,
  to,
  label,
  action,
}: {
  artwork: ReactNode
  scrim: string
  icon: LucideIcon
  eyebrow: string
  badge?: ReactNode
  title: string
  caption: string
  to: string
  label: string
  action?: ReactNode
}) {
  return (
    <div className="group/card relative rounded-2xl has-[a:focus-visible]:ring-[3px] has-[a:focus-visible]:ring-ring/50">
      <div className="relative aspect-[16/10] w-full overflow-hidden rounded-2xl shadow-sm ring-1 ring-black/5 dark:ring-white/10">
        <div className="size-full transition-transform duration-500 ease-out md:group-hover/card:scale-[1.03]">
          {artwork}
        </div>
        <div className={cn('pointer-events-none absolute inset-0', scrim)} aria-hidden />
        <div className="pointer-events-none absolute inset-0 flex flex-col justify-between p-3.5 text-white md:p-4">
          <div className="flex items-center justify-between gap-2">
            <span className="inline-flex min-w-0 items-center gap-1 text-[11px] font-semibold tracking-wide text-white/85 uppercase">
              <Icon className="size-3.5 shrink-0" strokeWidth={2} aria-hidden />
              <span className="truncate">{eyebrow}</span>
            </span>
            {badge ? (
              <span className="shrink-0 rounded-full bg-black/35 px-2.5 py-0.5 text-xs font-semibold text-white/95 ring-1 ring-white/15 backdrop-blur-md tabular-nums">
                {badge}
              </span>
            ) : null}
          </div>
          <div className="flex items-end justify-between gap-3">
            <span className="min-w-0 line-clamp-2 text-lg leading-tight font-bold tracking-tight drop-shadow-xs md:text-xl">
              {title}
            </span>
            {action}
          </div>
        </div>
      </div>
      <Link to={to} className="outline-none after:absolute after:inset-0 after:rounded-2xl" aria-label={label}>
        <span className="mt-2 block truncate px-0.5 text-[13px] leading-5 text-muted-foreground">{caption}</span>
      </Link>
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
  badge?: ReactNode
  scrim: string
}

/** A stored mix: editorial poster over a 2×2 cover mosaic, frosted play button, and artist line beneath. */
function MixCard({ tracks, to, icon, eyebrow, title, openLabel, playLabel, badge, scrim }: MixCardProps) {
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

  const artwork =
    covers.length === 4 ? (
      <div className="grid size-full grid-cols-2 grid-rows-2">
        {covers.map((c) => (
          <CoverArt key={c} coverArt={c} size={256} flat rounded="none" className="size-full" alt="" />
        ))}
      </div>
    ) : (
      <CoverArt coverArt={covers[0] ?? ''} size={512} flat rounded="none" className="size-full" alt="" />
    )

  return (
    <PosterCard
      artwork={artwork}
      scrim={scrim}
      icon={icon}
      eyebrow={eyebrow}
      badge={badge}
      title={title}
      caption={t('mix.artists', { artists: artists.join(t('mix.separator')), count: tracks.length })}
      to={to}
      label={openLabel}
      action={
        <button
          type="button"
          onClick={() => usePlayer.getState().playTracks(tracks, undefined, { shuffle: false })}
          aria-label={playLabel}
          className="pointer-events-auto relative z-10 grid size-11 shrink-0 place-items-center rounded-full bg-white/22 text-white ring-1 ring-white/25 backdrop-blur-md transition-[transform,background-color] outline-none hover:bg-white/32 focus-visible:ring-[3px] focus-visible:ring-white/75 active:scale-90 md:size-10"
        >
          <Play className="size-4.5 translate-x-px" fill="currentColor" strokeWidth={0} aria-hidden />
        </button>
      }
    />
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
      scrim="bg-linear-to-t from-black/80 via-black/30 to-black/45"
      badge={`${d} · ${weekday}`}
    />
  )
}

function WeeklyMixCard({ mix }: { mix: WeeklyMix }) {
  const { t } = useTranslation('library')
  const days = weekDays(mix.week)
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
      scrim="bg-linear-to-t from-black/80 via-black/30 to-black/45"
      badge={days ? `${days[0].getDate()}–${days[1].getDate()} ${months}` : undefined}
    />
  )
}

/** Last month's recap poster during the first week of a month, when that month had plays. */
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
  const artwork = (
    <div className="relative size-full bg-linear-to-br from-slate-800 via-slate-900 to-black">
      <div className="absolute inset-0 bg-linear-to-br from-primary/55 via-primary/20 to-transparent" />
      {showArt ? (
        <MascotArt pose="report" className="absolute right-3 -bottom-1 h-[88%]" />
      ) : (
        <ChartColumn
          className="absolute right-4 bottom-4 size-20 text-white/15"
          strokeWidth={1.5}
          aria-hidden
        />
      )}
    </div>
  )
  const caption = top
    ? t('home.recap.summaryTop', { time: formatDurationLong(report.data.totals.duration), artist: top.name })
    : t('home.recap.summary', { time: formatDurationLong(report.data.totals.duration) })

  return (
    <PosterCard
      artwork={artwork}
      scrim="bg-linear-to-t from-black/65 via-transparent to-black/25"
      icon={ChartColumn}
      eyebrow={t('home.recap.eyebrow')}
      title={t('home.recap.title', { month: name })}
      caption={caption}
      to={`/listening?range=${range}`}
      label={t('home.recap.open', { month: name })}
    />
  )
}
