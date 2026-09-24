import { Lock } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import type { TrackTags } from '@/lib/api/types'
import { formatBitrate, formatBytes, formatDateTime, formatDuration, formatSampleRate } from '@/lib/format'

const PAGE = 50

/** Read-only technical info for each file. */
export function FileTab({ items }: { items: readonly TrackTags[] }) {
  const { t } = useTranslation('manage')
  const [shown, setShown] = useState(PAGE)
  const totalSize = items.reduce((sum, i) => sum + (i.file.size || i.track.size), 0)
  const totalDuration = items.reduce((sum, i) => sum + (i.file.duration || i.track.duration), 0)

  return (
    <div className="grid gap-3 pb-2">
      {items.length > 1 ? (
        <p className="text-sm text-muted-foreground">
          {t('file.summary', { count: items.length, size: formatBytes(totalSize), duration: formatDuration(totalDuration) })}
        </p>
      ) : null}
      {items.slice(0, shown).map((item) => (
        <FileCard key={item.track.id} item={item} />
      ))}
      {items.length > shown ? (
        <Button variant="outline" onClick={() => setShown((n) => n + PAGE)}>
          {t('file.showMore', { count: items.length - shown })}
        </Button>
      ) : null}
    </div>
  )
}

function FileCard({ item }: { item: TrackTags }) {
  const { t } = useTranslation('manage')
  const f = item.file
  const rows: [string, ReactNode][] = [
    [t('file.format'), [f.format || item.track.suffix, f.codec || item.track.codec].filter(Boolean).join(' · ').toUpperCase()],
    [t('file.duration'), formatDuration(f.duration || item.track.duration)],
    [t('file.size'), formatBytes(f.size || item.track.size)],
    [t('file.bitrate'), formatBitrate(f.bitrate || item.track.bitrate) || '—'],
    [t('file.sampleRate'), formatSampleRate(f.sampleRate || item.track.sampleRate) || '—'],
    [t('file.bitDepth'), (f.bitDepth || item.track.bitDepth) > 0 ? `${f.bitDepth || item.track.bitDepth}-bit` : '—'],
    [t('file.channels'), channelsLabel(f.channels || item.track.channels, t)],
    [t('file.modified'), formatDateTime(f.mtime || item.track.mtime)],
    [t('file.added'), formatDateTime(item.track.createdAt)],
  ]
  return (
    <section className="rounded-lg border p-3">
      <div className="mb-2 flex items-start gap-2">
        <p className="min-w-0 flex-1 font-mono text-xs break-all text-foreground/85">{f.path || item.track.path}</p>
        {item.writable ? null : (
          <Badge variant="outline" className="shrink-0 gap-1 text-amber-600 dark:text-amber-400">
            <Lock />
            {t('file.readonly')}
          </Badge>
        )}
      </div>
      <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-1 text-[13px] sm:grid-cols-[auto_minmax(0,1fr)_auto_minmax(0,1fr)]">
        {rows.map(([label, value]) => (
          <div key={label} className="contents">
            <dt className="text-muted-foreground">{label}</dt>
            <dd className="tnum min-w-0 truncate">{value}</dd>
          </div>
        ))}
      </dl>
    </section>
  )
}

function channelsLabel(n: number, t: (key: string, o?: Record<string, unknown>) => string): string {
  if (n === 1) return t('file.mono')
  if (n === 2) return t('file.stereo')
  return n > 0 ? t('file.channelsN', { count: n }) : '—'
}
