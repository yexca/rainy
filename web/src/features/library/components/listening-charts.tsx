import { useMemo, useRef, useState, type KeyboardEvent, type PointerEvent } from 'react'
import { useTranslation } from 'react-i18next'

import type { ListeningBucket, ListeningTimelineBucket } from '@/lib/api/types'
import { formatDurationLong, formatNumber } from '@/lib/format'
import { currentLocale } from '@/lib/i18n'
import { cn } from '@/lib/utils'

import { heatLevel, niceMax, weekdayOrder } from '../lib/listening'

export type TimelineMetric = 'plays' | 'duration'

function bucketLabel(start: number, bucket: ListeningBucket, long: boolean): string {
  const locale = currentLocale()
  const d = new Date(start)
  switch (bucket) {
    case 'hour':
      return new Intl.DateTimeFormat(locale, long ? { weekday: 'short', hour: 'numeric', minute: '2-digit' } : { hour: 'numeric' }).format(d)
    case 'day':
      return new Intl.DateTimeFormat(locale, long ? { weekday: 'short', month: 'short', day: 'numeric' } : { month: 'short', day: 'numeric' }).format(d)
    case 'week':
      return new Intl.DateTimeFormat(locale, long ? { year: 'numeric', month: 'short', day: 'numeric' } : { month: 'short', day: 'numeric' }).format(d)
    case 'month':
      return new Intl.DateTimeFormat(locale, long ? { year: 'numeric', month: 'long' } : { month: 'short' }).format(d)
  }
}

function hoursLabel(seconds: number): string {
  const h = seconds / 3600
  return h >= 10 || h === 0 ? formatNumber(Math.round(h)) : new Intl.NumberFormat(currentLocale(), { maximumFractionDigits: 1 }).format(h)
}

/**
 * Plays (or listening hours) over time as thin bars on one axis. Hovering, touching or the
 * arrow keys show a tooltip for a bar; a visually hidden table carries the same data.
 */
export function TimelineChart({
  buckets,
  bucket,
  metric,
}: {
  buckets: readonly ListeningTimelineBucket[]
  bucket: ListeningBucket
  metric: TimelineMetric
}) {
  const { t } = useTranslation('library')
  const [active, setActive] = useState<number | null>(null)
  const plotRef = useRef<HTMLDivElement>(null)
  const values = useMemo(() => buckets.map((b) => (metric === 'plays' ? b.plays : b.duration / 3600)), [buckets, metric])
  const max = niceMax(Math.max(0, ...values))
  const ticks = [max, max / 2, 0]
  const dense = buckets.length > 45
  const format = (v: number) => (metric === 'plays' ? formatNumber(v) : hoursLabel(v * 3600))

  const pick = (event: PointerEvent<HTMLDivElement>) => {
    const el = plotRef.current
    if (!el || buckets.length === 0) return
    const rect = el.getBoundingClientRect()
    const i = Math.floor(((event.clientX - rect.left) / rect.width) * buckets.length)
    setActive(Math.min(buckets.length - 1, Math.max(0, i)))
  }
  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (buckets.length === 0) return
    const step = event.key === 'ArrowRight' ? 1 : event.key === 'ArrowLeft' ? -1 : 0
    if (event.key === 'Home' || event.key === 'End') {
      event.preventDefault()
      setActive(event.key === 'Home' ? 0 : buckets.length - 1)
    } else if (step) {
      event.preventDefault()
      setActive((i) => Math.min(buckets.length - 1, Math.max(0, (i ?? (step > 0 ? -1 : buckets.length)) + step)))
    } else if (event.key === 'Escape') {
      setActive(null)
    }
  }

  const labelAt = new Set(buckets.length <= 8 ? buckets.map((_, i) => i) : [0, Math.floor((buckets.length - 1) / 2), buckets.length - 1])
  const cur = active !== null ? buckets[active] : null

  return (
    <figure className="grid grid-cols-[minmax(0,1fr)] gap-2">
      <div className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-2">
        {/* y axis */}
        <div className="relative h-40 w-7 text-right text-[11px] text-muted-foreground tabular-nums md:h-48" aria-hidden>
          {ticks.map((v, i) => (
            <span key={i} className="absolute right-0 -translate-y-1/2" style={{ top: `${(i / (ticks.length - 1)) * 100}%` }}>
              {format(v)}
            </span>
          ))}
        </div>
        <div
          ref={plotRef}
          role="img"
          tabIndex={0}
          aria-label={t('listening.chart.label', { count: buckets.length })}
          onPointerMove={pick}
          onPointerDown={pick}
          onPointerLeave={(e) => e.pointerType === 'mouse' && setActive(null)}
          onKeyDown={onKeyDown}
          onBlur={() => setActive(null)}
          className="relative h-40 touch-pan-y rounded-sm outline-none focus-visible:ring-2 focus-visible:ring-ring/50 md:h-48"
        >
          {/* gridlines */}
          {ticks.map((_, i) => (
            <div
              key={i}
              aria-hidden
              className={cn('absolute inset-x-0 border-t', i === ticks.length - 1 ? 'border-border' : 'border-dashed border-border/60')}
              style={{ top: `${(i / (ticks.length - 1)) * 100}%` }}
            />
          ))}
          <div className={cn('absolute inset-0 flex items-end', dense ? 'gap-px' : 'gap-[2px]')} aria-hidden>
            {values.map((v, i) => (
              <div key={buckets[i].start} className="flex h-full min-w-0 flex-1 items-end">
                <div
                  className={cn(
                    'w-full transition-[background-color] duration-100',
                    dense ? 'rounded-t-[2px]' : 'rounded-t-[4px]',
                    active === null || active === i ? 'bg-primary' : 'bg-primary/45',
                  )}
                  style={{ height: v > 0 ? `max(2px, ${(v / max) * 100}%)` : 0 }}
                />
              </div>
            ))}
          </div>
          {cur ? (
            <div
              role="status"
              className="pointer-events-none absolute -top-2 z-10 w-max max-w-48 -translate-x-1/2 -translate-y-full rounded-lg border bg-popover px-2.5 py-1.5 text-xs text-popover-foreground shadow-md"
              style={{ left: `clamp(4.5rem, ${((active! + 0.5) / buckets.length) * 100}%, calc(100% - 4.5rem))` }}
            >
              <p className="font-medium">{bucket === 'week' ? t('listening.chart.weekOf', { date: bucketLabel(cur.start, bucket, true) }) : bucketLabel(cur.start, bucket, true)}</p>
              <p className="text-muted-foreground tabular-nums">
                {t('listening.plays', { count: cur.plays, formatted: formatNumber(cur.plays) })} · {formatDurationLong(cur.duration)}
              </p>
            </div>
          ) : null}
        </div>
        {/* x axis */}
        <div />
        <div className="relative mt-1 h-4 text-[11px] text-muted-foreground" aria-hidden>
          {buckets.map((b, i) =>
            labelAt.has(i) ? (
              <span
                key={b.start}
                className={cn(
                  'absolute whitespace-nowrap',
                  i === 0 && buckets.length > 1 ? 'left-0' : i === buckets.length - 1 && buckets.length > 1 ? 'right-0' : '-translate-x-1/2',
                )}
                style={i === 0 || i === buckets.length - 1 ? undefined : { left: `${((i + 0.5) / buckets.length) * 100}%` }}
              >
                {bucketLabel(b.start, bucket, false)}
              </span>
            ) : null,
          )}
        </div>
      </div>
      <table className="sr-only">
        <caption>{t('listening.chart.tableCaption')}</caption>
        <thead>
          <tr>
            <th scope="col">{t('listening.chart.period')}</th>
            <th scope="col">{t('listening.stats.plays')}</th>
            <th scope="col">{t('listening.stats.time')}</th>
          </tr>
        </thead>
        <tbody>
          {buckets.map((b) => (
            <tr key={b.start}>
              <th scope="row">{bucketLabel(b.start, bucket, true)}</th>
              <td>{b.plays}</td>
              <td>{formatDurationLong(b.duration)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </figure>
  )
}

const HEAT = ['bg-muted', 'bg-primary/25', 'bg-primary/50', 'bg-primary/75', 'bg-primary'] as const

/** First weekday of the UI locale (0 = Sunday … 6 = Saturday); Monday when unknown. */
function firstWeekday(): number {
  try {
    const loc = new Intl.Locale(currentLocale()) as Intl.Locale & { weekInfo?: { firstDay: number }; getWeekInfo?: () => { firstDay: number } }
    const info = loc.getWeekInfo?.() ?? loc.weekInfo
    if (info) return info.firstDay % 7
  } catch {
    // older browsers
  }
  return 1
}

function busiestHour(clock: readonly (readonly number[])[]): { day: number; hour: number } | null {
  let best: { day: number; hour: number } | null = null
  let bestValue = 0
  for (let day = 0; day < clock.length; day++) {
    for (let hour = 0; hour < clock[day].length; hour++) {
      if (clock[day][hour] > bestValue) {
        best = { day, hour }
        bestValue = clock[day][hour]
      }
    }
  }
  return best
}

/** Plays by weekday and hour: one hue, lighter to darker, with a legend and a tooltip line. */
export function ClockHeatmap({ clock }: { clock: readonly (readonly number[])[] }) {
  const { t } = useTranslation('library')
  const [active, setActive] = useState<{ day: number; hour: number } | null>(null)
  const max = Math.max(0, ...clock.flat())
  const order = useMemo(() => weekdayOrder(firstWeekday()), [])
  const locale = currentLocale()
  // 2024-01-01 is a Monday: ISO weekday i → Jan (1 + i).
  const dayName = (iso: number, style: 'short' | 'long') =>
    new Intl.DateTimeFormat(locale, { weekday: style }).format(new Date(2024, 0, 1 + iso))
  const hourName = (h: number) => new Intl.DateTimeFormat(locale, { hour: 'numeric' }).format(new Date(2024, 0, 1, h))
  const cur = active ? clock[active.day]?.[active.hour] ?? 0 : null
  const busiest = busiestHour(clock)

  return (
    <figure className="grid grid-cols-[minmax(0,1fr)] gap-2">
      <div
        role="img"
        aria-label={busiest ? t('listening.clock.busiest', { day: dayName(busiest.day, 'long'), hour: hourName(busiest.hour) }) : t('listening.clock.hint')}
        className="grid touch-pan-y grid-cols-[auto_minmax(0,1fr)] gap-x-2 gap-y-[3px]"
        onPointerLeave={(e) => e.pointerType === 'mouse' && setActive(null)}
      >
        {order.map((day) => (
          <div key={day} className="contents">
            <span className="self-center text-[11px] text-muted-foreground" aria-hidden>
              {dayName(day, 'short')}
            </span>
            <div className="grid grid-cols-24 gap-[2px]" aria-hidden>
              {clock[day]?.map((v, hour) => (
                <span
                  key={hour}
                  onPointerEnter={() => setActive({ day, hour })}
                  onPointerDown={() => setActive({ day, hour })}
                  className={cn(
                    'aspect-square min-w-0 rounded-[3px]',
                    HEAT[heatLevel(v, max)],
                    active?.day === day && active.hour === hour && 'ring-2 ring-foreground/70',
                  )}
                />
              ))}
            </div>
          </div>
        ))}
        <span />
        <div className="relative h-4 text-[11px] text-muted-foreground" aria-hidden>
          {[0, 6, 12, 18].map((h) => (
            <span key={h} className="absolute" style={{ left: `${(h / 24) * 100}%` }}>
              {hourName(h)}
            </span>
          ))}
        </div>
      </div>
      <figcaption className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
        <span className="min-h-4 tabular-nums" aria-live="polite">
          {active && cur !== null
            ? t('listening.clock.cell', { day: dayName(active.day, 'long'), hour: hourName(active.hour), count: cur })
            : t('listening.clock.hint')}
        </span>
        <span className="flex items-center gap-1.5" aria-hidden>
          {t('listening.clock.less')}
          {HEAT.map((c) => (
            <span key={c} className={cn('size-3 rounded-[3px]', c)} />
          ))}
          {t('listening.clock.more')}
        </span>
      </figcaption>
    </figure>
  )
}

/** Ranked horizontal bars with the value beside each (genres, players). */
export function BarList({ items }: { items: readonly { key: string; label: string; value: number; detail?: string }[] }) {
  const max = Math.max(1, ...items.map((i) => i.value))
  return (
    <ol className="grid grid-cols-[minmax(0,1fr)] gap-2.5">
      {items.map((item) => (
        <li key={item.key} className="grid grid-cols-[minmax(0,1fr)] gap-1">
          <div className="flex items-baseline justify-between gap-3 text-sm">
            <span className="min-w-0 truncate">{item.label}</span>
            <span className="shrink-0 text-xs text-muted-foreground tabular-nums">{item.detail ?? formatNumber(item.value)}</span>
          </div>
          <div className="h-1.5 overflow-hidden rounded-full bg-muted" aria-hidden>
            <div className="h-full rounded-full bg-primary" style={{ width: `${(item.value / max) * 100}%` }} />
          </div>
        </li>
      ))}
    </ol>
  )
}
