import type { VirtualItem } from '@tanstack/react-virtual'
import { Check, CircleAlert } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { CoverArt } from '@/components/cover-art'
import { Skeleton } from '@/components/ui/skeleton'
import type { Track } from '@/lib/api/types'
import { formatDuration } from '@/lib/format'
import { cn } from '@/lib/utils'

export const MOBILE_ROW_HEIGHT = 60

export interface MobileTrackRowsProps {
  items: VirtualItem[]
  totalSize: number
  scrollMargin: number
  rowAt: (index: number) => Track | undefined
  selecting: boolean
  isSelected: (id: string) => boolean
  onTap: (index: number, track: Track) => void
}

/** iOS-style list rows (60px, 44px artwork, inset hairlines) with a selection mode. */
export function MobileTrackRows({ items, totalSize, scrollMargin, rowAt, selecting, isSelected, onTap }: MobileTrackRowsProps) {
  const { t } = useTranslation('manage')
  return (
    <ul className="bleed-x relative" style={{ height: totalSize }}>
      {items.map((item) => {
        const track = rowAt(item.index)
        const style = { height: item.size, transform: `translateY(${item.start - scrollMargin}px)` }
        if (!track) {
          return (
            <li key={item.key} className="page-x absolute inset-x-0 top-0 flex items-center gap-3" style={style}>
              <Skeleton className="size-11 rounded-md" />
              <div className="grid flex-1 gap-1.5">
                <Skeleton className="h-3.5 w-2/3" />
                <Skeleton className="h-3 w-1/2" />
              </div>
            </li>
          )
        }
        const selected = selecting && isSelected(track.id)
        return (
          <li
            key={item.key}
            className="hairline-inset absolute inset-x-0 top-0 [--hairline-inset:calc(var(--page-px)+var(--safe-left)+3.5rem)]"
            style={style}
          >
            <button
              type="button"
              onClick={() => onTap(item.index, track)}
              aria-pressed={selecting ? selected : undefined}
              className={cn(
                'page-x flex size-full items-center gap-3 text-left transition-colors active:bg-accent/60',
                selected && 'bg-primary/8',
              )}
            >
              {selecting ? (
                <span
                  aria-hidden
                  className={cn(
                    'grid size-[22px] shrink-0 place-items-center rounded-full border-[1.5px] transition-colors',
                    selected ? 'border-primary bg-primary text-primary-foreground' : 'border-muted-foreground/40',
                  )}
                >
                  {selected ? <Check className="size-3.5" strokeWidth={3} /> : null}
                </span>
              ) : null}
              <CoverArt coverArt={track.coverArt} size={44} />
              <span className="min-w-0 flex-1">
                <span className="flex items-center gap-1.5">
                  {track.missing ? <CircleAlert className="size-3.5 shrink-0 text-amber-500" aria-label={t('table.missing')} /> : null}
                  <span className={cn('truncate text-[15px] leading-5', track.missing && 'text-muted-foreground')}>{track.title}</span>
                </span>
                <span className="block truncate text-[13px] leading-5 text-muted-foreground">
                  {[track.artist, track.album].filter(Boolean).join(' · ')}
                </span>
              </span>
              <span className="flex shrink-0 flex-col items-end gap-0.5 text-[11px] text-muted-foreground">
                <span className="font-medium tracking-wide uppercase">{track.suffix}</span>
                <span className="tnum">{formatDuration(track.duration)}</span>
              </span>
            </button>
          </li>
        )
      })}
    </ul>
  )
}
