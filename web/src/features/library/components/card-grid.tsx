import { useWindowVirtualizer } from '@tanstack/react-virtual'
import { useEffect, type ReactNode } from 'react'

import { useIsMobile } from '@/hooks/use-media-query'
import { cn } from '@/lib/utils'

import { useElementGeometry } from '../lib/use-element-geometry'

export type CardGridDensity = 'albums' | 'artists'

const STATIC_CLASSES: Record<CardGridDensity, string> = {
  albums:
    'grid-cols-[repeat(auto-fill,minmax(140px,1fr))] gap-x-4 gap-y-5 md:grid-cols-[repeat(auto-fill,minmax(164px,1fr))] md:gap-x-6 md:gap-y-7',
  artists:
    'grid-cols-[repeat(auto-fill,minmax(104px,1fr))] gap-x-4 gap-y-5 md:grid-cols-[repeat(auto-fill,minmax(150px,1fr))] md:gap-x-6 md:gap-y-7',
}

const MIN_WIDTH: Record<CardGridDensity, { mobile: number; desktop: number }> = {
  albums: { mobile: 140, desktop: 164 },
  artists: { mobile: 104, desktop: 150 },
}

/** Responsive auto-fill grid for a bounded number of cards (artist albums, favorites, …). */
export function CardGrid({
  density = 'albums',
  className,
  children,
}: {
  density?: CardGridDensity
  className?: string
  children: ReactNode
}) {
  return <div className={cn('grid', STATIC_CLASSES[density], className)}>{children}</div>
}

export interface VirtualCardGridProps<T> {
  items: T[]
  /** Total items (rows beyond `items` render skeletons). */
  total: number
  onEndReached?: () => void
  getKey: (item: T) => string
  /** `width` = the card width in px (for the cover resolution). */
  renderItem: (item: T, index: number, width: number) => ReactNode
  renderSkeleton: () => ReactNode
  /** Fixed height of the text block under each square card. */
  captionHeight: number
  density?: CardGridDensity
  className?: string
}

/** Rows left to render before the end of the loaded items when `onEndReached` fires. */
const END_ROWS = 4

/**
 * Window-virtualized card grid for large libraries: rows of square cards with a fixed caption,
 * the column count follows the container width. Unloaded rows up to `total` show skeletons.
 */
export function VirtualCardGrid<T>({
  items,
  total,
  onEndReached,
  getKey,
  renderItem,
  renderSkeleton,
  captionHeight,
  density = 'albums',
  className,
}: VirtualCardGridProps<T>) {
  const isMobile = useIsMobile()
  const [ref, geometry] = useElementGeometry<HTMLDivElement>()
  const gap = isMobile ? 16 : 24
  const rowGap = isMobile ? 20 : 28
  const minWidth = isMobile ? MIN_WIDTH[density].mobile : MIN_WIDTH[density].desktop
  // Until measured, assume a phone-sized container so the first paint is close.
  const width = geometry.width || 343
  const columns = Math.max(2, Math.floor((width + gap) / (minWidth + gap)))
  const itemWidth = (width - gap * (columns - 1)) / columns
  const count = Math.max(total, items.length)
  const rowCount = Math.ceil(count / columns)
  const rowHeight = Math.round(itemWidth + captionHeight + rowGap)

  const virtualizer = useWindowVirtualizer({
    count: rowCount,
    estimateSize: () => rowHeight,
    overscan: 3,
    scrollMargin: geometry.top,
  })

  useEffect(() => {
    virtualizer.measure()
  }, [virtualizer, rowHeight, columns])

  const virtualRows = virtualizer.getVirtualItems()
  const lastRow = virtualRows.length > 0 ? virtualRows[virtualRows.length - 1].index : -1
  const loadedRows = Math.ceil(items.length / columns)

  useEffect(() => {
    if (!onEndReached || items.length >= count || lastRow < 0) return
    if (lastRow >= loadedRows - END_ROWS) onEndReached()
  }, [lastRow, loadedRows, onEndReached, items.length, count])

  return (
    <div ref={ref} className={cn('relative w-full', className)} style={{ height: virtualizer.getTotalSize() }}>
      {virtualRows.map((row) => {
        const start = row.index * columns
        const cells = []
        for (let i = start; i < Math.min(start + columns, count); i++) {
          const item = items[i]
          cells.push(
            <div key={item === undefined ? `s${i}` : getKey(item)} className="min-w-0">
              {item === undefined ? renderSkeleton() : renderItem(item, i, itemWidth)}
            </div>,
          )
        }
        return (
          <div
            key={row.key}
            className="absolute inset-x-0 top-0 grid"
            style={{
              height: rowHeight - rowGap,
              gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))`,
              columnGap: gap,
              transform: `translateY(${row.start - virtualizer.options.scrollMargin}px)`,
            }}
          >
            {cells}
          </div>
        )
      })}
    </div>
  )
}
