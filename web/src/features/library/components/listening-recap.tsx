import { CalendarHeart, Clock3, Flame, MicVocal, Sparkles, TrendingDown, TrendingUp } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'

import { CoverArt } from '@/components/cover-art'
import { MascotArt } from '@/components/mascot'
import { useMascotArt } from '@/hooks/use-mascot-art'
import type { ListeningReport } from '@/lib/api/types'
import { formatDurationLong, formatNumber } from '@/lib/format'
import { currentLocale } from '@/lib/i18n'
import { cn } from '@/lib/utils'

import { busiestTimes, percentChange } from '../lib/listening'

/** ISO weekday (0 = Monday) as a localized name. 5 Jan 2026 is a Monday. */
function weekdayName(weekday: number): string {
  return new Intl.DateTimeFormat(currentLocale(), { weekday: 'long' }).format(new Date(2026, 0, 5 + weekday))
}

function hourLabel(hour: number): string {
  return new Intl.DateTimeFormat(currentLocale(), { hour: 'numeric', minute: '2-digit' }).format(new Date(2026, 0, 5, hour))
}

/**
 * The headline card of a calendar month or year report (§9.5a): listening time, the top artist
 * and song, discoveries, when the user listens most, the longest streak and the change against
 * the period before. Everything comes from the report itself.
 */
export function ListeningRecap({ report, period, kind }: { report: ListeningReport; period: string; kind: 'month' | 'year' }) {
  const { t } = useTranslation('library')
  const showArt = useMascotArt()
  const { totals } = report
  const artist = report.topArtists[0]
  const song = report.topTracks[0]
  const busiest = busiestTimes(report.clock)
  const change = percentChange(totals.duration, report.previous?.duration)

  return (
    <section
      aria-labelledby="listening-recap"
      className="relative overflow-hidden rounded-2xl border bg-linear-to-br from-primary/15 via-primary/5 to-transparent p-5 md:p-6"
    >
      <div className="flex items-start gap-6">
        <div className="min-w-0 flex-1">
          <p className="text-sm font-medium text-primary">{t('listening.recap.eyebrow', { period })}</p>
          <h2 id="listening-recap" className="mt-1 text-2xl font-bold tracking-tight text-balance md:text-3xl">
            {t('listening.recap.headline', { time: formatDurationLong(totals.duration) })}
          </h2>
          <p className="mt-1 text-sm text-muted-foreground">
            {t('listening.recap.summary', {
              plays: formatNumber(totals.plays),
              songs: formatNumber(totals.tracks),
              artists: formatNumber(totals.artists),
              count: totals.plays,
            })}
          </p>
        </div>
        {showArt ? <MascotArt pose="report" className="-my-2 hidden h-32 shrink-0 sm:block md:h-36" /> : null}
      </div>

      <ul className="mt-5 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {artist ? (
          <Fact
            label={t('listening.recap.topArtist')}
            media={<CoverArt coverArt={artist.coverArt} size={44} shape="circle" icon={MicVocal} alt="" />}
            to={artist.available ? `/artists/${artist.id}` : undefined}
          >
            <span className="block truncate font-medium">{artist.name}</span>
            <span className="block truncate text-xs text-muted-foreground">
              {t('listening.plays', { count: artist.plays, formatted: formatNumber(artist.plays) })}
            </span>
          </Fact>
        ) : null}
        {song ? (
          <Fact
            label={t('listening.recap.topSong')}
            media={<CoverArt coverArt={song.coverArt} size={44} alt="" />}
            to={song.track ? `/albums/${song.track.albumId}` : undefined}
          >
            <span className="block truncate font-medium">{song.name}</span>
            <span className="block truncate text-xs text-muted-foreground">
              {song.artist} · {t('listening.plays', { count: song.plays, formatted: formatNumber(song.plays) })}
            </span>
          </Fact>
        ) : null}
        <Fact label={t('listening.recap.discoveries')} icon={Sparkles}>
          <span className="block font-medium">
            {t('listening.recap.newArtists', { count: report.newArtists, formatted: formatNumber(report.newArtists) })}
          </span>
          <span className="block text-xs text-muted-foreground">
            {t('listening.recap.newSongs', { count: report.newTracks, formatted: formatNumber(report.newTracks) })}
          </span>
        </Fact>
        {busiest ? (
          <Fact label={t('listening.recap.busiest')} icon={Clock3}>
            <span className="block font-medium">{weekdayName(busiest.weekday)}</span>
            <span className="block text-xs text-muted-foreground">{t('listening.recap.around', { time: hourLabel(busiest.hour) })}</span>
          </Fact>
        ) : null}
        <Fact label={t('listening.recap.streak')} icon={Flame}>
          <span className="block font-medium">
            {t('listening.recap.days', { count: report.longestStreak, formatted: formatNumber(report.longestStreak) })}
          </span>
          <span className="block text-xs text-muted-foreground">
            {t('listening.recap.activeDays', { count: report.activeDays, formatted: formatNumber(report.activeDays) })}
          </span>
        </Fact>
        {change !== null ? (
          <Fact label={t(kind === 'month' ? 'listening.recap.vsMonth' : 'listening.recap.vsYear')} icon={change >= 0 ? TrendingUp : TrendingDown}>
            <span className="block font-medium tabular-nums">
              {change >= 0 ? t('listening.recap.more', { pct: change }) : t('listening.recap.less', { pct: -change })}
            </span>
            <span className="block text-xs text-muted-foreground">{t('listening.recap.listeningTime')}</span>
          </Fact>
        ) : (
          <Fact label={t('listening.recap.since')} icon={CalendarHeart}>
            <span className="block font-medium">
              {new Intl.DateTimeFormat(currentLocale(), { dateStyle: 'medium' }).format(new Date(report.firstPlayAt))}
            </span>
            <span className="block text-xs text-muted-foreground">{t('listening.recap.firstPlay')}</span>
          </Fact>
        )}
      </ul>
    </section>
  )
}

function Fact({
  label,
  icon: Icon,
  media,
  to,
  children,
}: {
  label: string
  icon?: typeof Sparkles
  media?: ReactNode
  to?: string
  children: ReactNode
}) {
  const body = (
    <>
      {media ?? (
        <span className="grid size-11 shrink-0 place-items-center rounded-full bg-primary/10 text-primary" aria-hidden>
          {Icon ? <Icon className="size-5" strokeWidth={1.75} /> : null}
        </span>
      )}
      <span className="min-w-0 flex-1 text-sm leading-5">
        <span className="block text-xs text-muted-foreground">{label}</span>
        {children}
      </span>
    </>
  )
  const box = 'flex min-h-16 items-center gap-3 rounded-xl bg-background/70 p-3 ring-1 ring-border/60 dark:bg-background/40'
  return (
    <li>
      {to ? (
        <Link
          to={to}
          className={cn(box, 'outline-none hover:bg-background focus-visible:ring-[3px] focus-visible:ring-ring/50 active:scale-[0.99]')}
        >
          {body}
        </Link>
      ) : (
        <div className={box}>{body}</div>
      )}
    </li>
  )
}
