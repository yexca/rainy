import { useQueryClient } from '@tanstack/react-query'
import { CircleAlert, CircleCheck, RefreshCw, ScanSearch } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { MascotArt } from '@/components/mascot'
import { Button } from '@/components/ui/button'
import { Progress } from '@/components/ui/progress'
import { Skeleton } from '@/components/ui/skeleton'
import { useMascotArt } from '@/hooks/use-mascot-art'
import type { LibraryInfo } from '@/lib/api/types'
import { formatDateTime, formatDuration, formatNumber, formatRelative } from '@/lib/format'
import { cn } from '@/lib/utils'

import { adminKeys } from '../queries'
import { useScanStatus, useStartScan } from '../scan'

function useNow(enabled: boolean): number {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    if (!enabled) return
    const id = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(id)
  }, [enabled])
  return now
}

export function ScanPanel({ libraries }: { libraries: LibraryInfo[] }) {
  const { t } = useTranslation('admin')
  const showArt = useMascotArt()
  const queryClient = useQueryClient()
  const status = useScanStatus()
  const start = useStartScan()
  const s = status.data
  const scanning = !!s?.scanning
  const now = useNow(scanning)

  // Refresh library cards (track counts, last scan) when a scan finishes.
  const wasScanning = useRef(scanning)
  useEffect(() => {
    if (wasScanning.current && !scanning) void queryClient.invalidateQueries({ queryKey: adminKeys.libraries })
    wasScanning.current = scanning
  }, [scanning, queryClient])

  const scopeName = s && s.libraryId ? libraries.find((l) => l.id === s.libraryId)?.name : undefined
  const elapsed = s?.startedAt ? ((scanning ? now : s.finishedAt || now) - s.startedAt) / 1000 : 0

  const counters: [string, number][] = s
    ? [
        [t('scan.filesSeen'), s.filesSeen],
        [t('scan.added'), s.added],
        [t('scan.updated'), s.updated],
        [t('scan.removed'), s.removed],
        [t('scan.moved'), s.moved],
        [t('scan.errors'), s.errors],
      ]
    : []

  return (
    <section className="grid gap-4 rounded-xl border p-4 sm:p-5" aria-live="polite">
      <div className="flex flex-wrap items-start gap-3">
        <span
          className={cn(
            'grid size-10 shrink-0 place-items-center rounded-xl',
            scanning ? 'bg-primary/12 text-primary' : s?.phase === 'error' ? 'bg-destructive/10 text-destructive' : 'bg-emerald-500/12 text-emerald-600 dark:text-emerald-400',
          )}
        >
          {scanning ? (
            <RefreshCw className="size-5 animate-spin [animation-duration:2s]" strokeWidth={1.75} />
          ) : s?.phase === 'error' ? (
            <CircleAlert className="size-5" strokeWidth={1.75} />
          ) : (
            <CircleCheck className="size-5" strokeWidth={1.75} />
          )}
        </span>
        <div className="min-w-0 flex-1">
          <h2 className="text-base font-semibold">
            {status.isPending ? <Skeleton className="h-5 w-32" /> : scanning ? t(`scan.phase.${s?.phase ?? 'walking'}`) : t('scan.idleTitle')}
          </h2>
          <p className="text-sm text-muted-foreground">
            {scanning
              ? [s?.full ? t('scan.full') : t('scan.quick'), scopeName ?? t('scan.allLibraries'), formatDuration(elapsed)].join(' · ')
              : s?.finishedAt
                ? t('scan.lastFinished', { when: formatRelative(s.finishedAt), duration: formatDuration(elapsed) })
                : t('scan.never')}
          </p>
        </div>
        <div className="flex w-full gap-2 sm:w-auto">
          <Button className="flex-1 max-sm:h-11 sm:flex-none" disabled={scanning || start.isPending} onClick={() => start.mutate({})}>
            <ScanSearch />
            {t('scan.quickScan')}
          </Button>
          <Button variant="outline" className="flex-1 max-sm:h-11 sm:flex-none" disabled={scanning || start.isPending} onClick={() => start.mutate({ full: true })}>
            {t('scan.fullScan')}
          </Button>
        </div>
      </div>

      {scanning ? <Progress className="h-1 [&>[data-slot=progress-indicator]]:animate-pulse" value={100} /> : null}

      {scanning && showArt ? (
        <div className="flex items-center gap-4 rounded-lg bg-muted/40 px-4 py-2">
          <MascotArt pose="scanning" className="h-20 shrink-0 sm:h-24" />
          <p className="text-sm text-pretty text-muted-foreground">{t('scan.mascot')}</p>
        </div>
      ) : null}

      {s && (scanning || s.finishedAt > 0) ? (
        <dl className="grid grid-cols-3 gap-2 sm:grid-cols-6">
          {counters.map(([label, value]) => (
            <div key={label} className="rounded-lg bg-muted/50 px-3 py-2">
              <dt className="text-[11px] text-muted-foreground">{label}</dt>
              <dd className={cn('tnum text-lg font-semibold', label === t('scan.errors') && value > 0 && 'text-destructive')}>{formatNumber(value)}</dd>
            </div>
          ))}
        </dl>
      ) : null}

      {s?.lastError ? (
        <p className="rounded-lg bg-destructive/10 px-3 py-2 text-sm break-words text-destructive" title={s.finishedAt ? formatDateTime(s.finishedAt) : undefined}>
          {s.lastError}
        </p>
      ) : null}
      {status.isError ? <p className="text-sm text-destructive">{t('scan.statusUnavailable')}</p> : null}
    </section>
  )
}
