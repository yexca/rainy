import type { VirtualItem } from '@tanstack/react-virtual'
import { ArrowDown, ArrowUp, CircleAlert } from 'lucide-react'
import type { CSSProperties, MouseEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { CoverArt } from '@/components/cover-art'
import { Checkbox } from '@/components/ui/checkbox'
import { Skeleton } from '@/components/ui/skeleton'
import type { SortOrder, TrackSort } from '@/lib/api/endpoints'
import type { Track } from '@/lib/api/types'
import { formatBitrate, formatBytes, formatDate, formatDuration } from '@/lib/format'
import { cn } from '@/lib/utils'

import type { ColumnDef } from '../lib/columns'

export const DESKTOP_ROW_HEIGHT = 48

/** CSS grid template: checkbox + visible columns. */
function gridTemplate(columns: readonly ColumnDef[]): CSSProperties {
  return { gridTemplateColumns: ['2rem', ...columns.map((c) => c.width)].join(' ') }
}

export interface TrackTableHeaderProps {
  columns: readonly ColumnDef[]
  sort: TrackSort
  order: SortOrder
  onSort: (sort: TrackSort) => void
  /** Header checkbox. */
  checked: boolean | 'indeterminate'
  onCheckedChange: (checked: boolean) => void
}

export function TrackTableHeader({ columns, sort, order, onSort, checked, onCheckedChange }: TrackTableHeaderProps) {
  const { t } = useTranslation('manage')
  return (
    <div
      role="row"
      className="grid h-9 items-center gap-x-3 px-2 text-xs font-medium text-muted-foreground"
      style={gridTemplate(columns)}
    >
      <div role="columnheader" className="flex justify-center">
        <Checkbox checked={checked} onCheckedChange={(v) => onCheckedChange(v === true)} aria-label={t('table.selectAllVisible')} />
      </div>
      {columns.map((col) => {
        const active = col.sort !== undefined && col.sort === sort
        const label = col.id === 'cover' ? '' : t(`columns.${col.id}`)
        return (
          <div
            key={col.id}
            role="columnheader"
            aria-sort={active ? (order === 'asc' ? 'ascending' : 'descending') : undefined}
            className={cn('min-w-0', col.align === 'right' && 'text-right')}
          >
            {col.sort ? (
              <button
                type="button"
                onClick={() => onSort(col.sort!)}
                className={cn(
                  'inline-flex max-w-full items-center gap-1 rounded-sm py-1 transition-colors hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50 focus-visible:outline-none',
                  active && 'text-foreground',
                  col.align === 'right' && 'flex-row-reverse',
                )}
              >
                <span className="truncate">{label}</span>
                {active ? order === 'asc' ? <ArrowUp className="size-3 shrink-0" /> : <ArrowDown className="size-3 shrink-0" /> : null}
              </button>
            ) : (
              <span className="truncate">{label}</span>
            )}
          </div>
        )
      })}
    </div>
  )
}

export interface TrackTableRowsProps {
  columns: readonly ColumnDef[]
  items: VirtualItem[]
  totalSize: number
  scrollMargin: number
  rowAt: (index: number) => Track | undefined
  isSelected: (id: string) => boolean
  onRowClick: (index: number, track: Track, event: MouseEvent) => void
  onRowDoubleClick: (index: number, track: Track) => void
  onToggle: (index: number, track: Track, event: MouseEvent) => void
}

/** Virtualized desktop rows (positioned by the page's window virtualizer). */
export function TrackTableRows({
  columns,
  items,
  totalSize,
  scrollMargin,
  rowAt,
  isSelected,
  onRowClick,
  onRowDoubleClick,
  onToggle,
}: TrackTableRowsProps) {
  const { t } = useTranslation('manage')
  const template = gridTemplate(columns)
  return (
    <div role="rowgroup" className="relative" style={{ height: totalSize }}>
      {items.map((item) => {
        const track = rowAt(item.index)
        const style: CSSProperties = {
          ...template,
          height: item.size,
          transform: `translateY(${item.start - scrollMargin}px)`,
        }
        if (!track) {
          return (
            <div key={item.key} role="row" className="absolute inset-x-0 top-0 grid items-center gap-x-3 px-2" style={style}>
              <div />
              {columns.map((col) => (
                <Skeleton key={col.id} className={cn('h-3.5 rounded', col.id === 'cover' ? 'size-8 rounded-md' : 'w-3/4')} />
              ))}
            </div>
          )
        }
        const selected = isSelected(track.id)
        return (
          <div
            key={item.key}
            role="row"
            aria-selected={selected}
            data-index={item.index}
            onClick={(e) => onRowClick(item.index, track, e)}
            onDoubleClick={() => onRowDoubleClick(item.index, track)}
            className={cn(
              'absolute inset-x-0 top-0 grid cursor-default items-center gap-x-3 rounded-md px-2 text-sm select-none',
              selected ? 'bg-primary/10 hover:bg-primary/15' : 'hover:bg-accent/60',
              track.missing && 'text-muted-foreground',
            )}
            style={style}
          >
            <div className="flex justify-center" onClick={(e) => e.stopPropagation()} onDoubleClick={(e) => e.stopPropagation()}>
              <Checkbox
                checked={selected}
                onClick={(e) => onToggle(item.index, track, e)}
                aria-label={t('table.selectTrack', { title: track.title })}
              />
            </div>
            {columns.map((col) => (
              <Cell key={col.id} column={col} track={track} missingLabel={t('table.missing')} />
            ))}
          </div>
        )
      })}
    </div>
  )
}

function Cell({ column, track, missingLabel }: { column: ColumnDef; track: Track; missingLabel: string }) {
  const right = column.align === 'right'
  const base = cn('min-w-0 truncate', right && 'text-right tnum')
  switch (column.id) {
    case 'cover':
      return <CoverArt coverArt={track.coverArt} size={32} />
    case 'title':
      return (
        <div className="flex min-w-0 items-center gap-1.5">
          {track.missing ? <CircleAlert className="size-3.5 shrink-0 text-amber-500" aria-label={missingLabel} /> : null}
          <span className={cn('truncate font-medium', track.missing ? 'text-muted-foreground' : 'text-foreground')} title={track.title}>
            {track.title}
          </span>
        </div>
      )
    case 'artist':
      return <span className={cn(base, 'text-muted-foreground')} title={track.artist}>{track.artist}</span>
    case 'album':
      return <span className={cn(base, 'text-muted-foreground')} title={track.album}>{track.album}</span>
    case 'albumArtist':
      return <span className={cn(base, 'text-muted-foreground')} title={track.albumArtist}>{track.albumArtist}</span>
    case 'track':
      return <span className={cn(base, 'text-muted-foreground')}>{track.trackNumber || ''}</span>
    case 'disc':
      return <span className={cn(base, 'text-muted-foreground')}>{track.discNumber || ''}</span>
    case 'year':
      return <span className={cn(base, 'text-muted-foreground')}>{track.year || ''}</span>
    case 'genre':
      return <span className={cn(base, 'text-muted-foreground')} title={track.genre}>{track.genres.join(', ')}</span>
    case 'format':
      return <span className={cn(base, 'text-xs font-medium tracking-wide text-muted-foreground uppercase')}>{track.suffix}</span>
    case 'bitrate':
      return <span className={cn(base, 'text-muted-foreground')}>{formatBitrate(track.bitrate)}</span>
    case 'duration':
      return <span className={cn(base, 'text-muted-foreground')}>{formatDuration(track.duration)}</span>
    case 'size':
      return <span className={cn(base, 'text-muted-foreground')}>{formatBytes(track.size)}</span>
    case 'path':
      return <span className={cn(base, 'font-mono text-xs text-muted-foreground')} title={track.path}>{track.path}</span>
    case 'updated':
      return <span className={cn(base, 'text-muted-foreground')}>{formatDate(track.updatedAt)}</span>
  }
}
