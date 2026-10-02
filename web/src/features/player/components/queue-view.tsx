import {
  DndContext,
  DragOverlay,
  KeyboardSensor,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragStartEvent,
} from '@dnd-kit/core'
import { restrictToFirstScrollableAncestor, restrictToVerticalAxis } from '@dnd-kit/modifiers'
import { SortableContext, sortableKeyboardCoordinates, useSortable, verticalListSortingStrategy } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { defaultRangeExtractor, useVirtualizer, type Range } from '@tanstack/react-virtual'
import { GripHorizontal, Infinity as InfinityIcon, ListMusic, Radio, Trash2, X } from 'lucide-react'
import { motion, useMotionValue, useTransform, type PanInfo } from 'motion/react'
import { memo, useCallback, useMemo, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { CoverArt } from '@/components/cover-art'
import { formatDuration } from '@/lib/format'
import { isTouch } from '@/lib/platform'
import { cn } from '@/lib/utils'

import { entryKey, useCurrentTrack, usePlayer } from '../store'
import type { PlayableTrack } from '../types'
import { LiveBadge } from './live-badge'
import { InfiniteButton, RepeatButton, ShuffleButton } from './transport'

export type QueueTone = 'sheet' | 'panel' | 'stage'

const ROW_HEIGHT = 56
/** Swipe further than this (px) to remove a row. */
const SWIPE_REMOVE = 96

interface QueueViewProps {
  /** `sheet` / `stage`: white on artwork. `panel`: themed side panel. */
  tone: QueueTone
  className?: string
}

/** "Playing Next": the upcoming entries, reorderable (dnd-kit), virtualized. */
export function QueueView({ tone, className }: QueueViewProps) {
  const { t } = useTranslation('player')
  const queue = usePlayer((s) => s.queue)
  const index = usePlayer((s) => s.index)
  const clearUpcoming = usePlayer((s) => s.clearUpcoming)
  const refilling = usePlayer((s) => s.infinite && s.repeat === 'off')
  const current = useCurrentTrack()
  const onArt = tone !== 'panel'
  const start = index + 1
  const upcoming = useMemo(() => queue.slice(start), [queue, start])
  const suggested = useMemo(() => upcoming.filter((t) => t.autoAdded).length, [upcoming])

  return (
    <div className={cn('flex h-full min-h-0 flex-col', className)}>
      {tone === 'panel' && current ? (
        <section className="px-3 pb-2">
          <h3 className="px-2 pt-1 pb-2 text-xs font-semibold tracking-wide text-muted-foreground uppercase">{t('queue.nowPlaying')}</h3>
          <div className="flex h-14 items-center gap-3 rounded-lg bg-accent/60 px-2">
            <CoverArt coverArt={current.coverArt} size={40} icon={current.isRadio ? Radio : undefined} />
            <div className="min-w-0 flex-1 leading-tight">
              <p className="truncate text-sm font-medium text-primary">{current.title}</p>
              <p className="truncate text-xs text-muted-foreground">{current.isRadio ? t('live.radio') : current.artist}</p>
            </div>
          </div>
        </section>
      ) : null}

      <div className={cn('flex items-center gap-2 pb-2', tone === 'panel' ? 'px-5 pt-2' : tone === 'stage' ? 'px-2' : 'px-6')}>
        <div className="min-w-0 flex-1">
          <h3 className={cn('text-base font-semibold', onArt ? 'text-white' : 'text-foreground')}>{t('queue.playingNext')}</h3>
          {upcoming.length > 0 ? (
            <p className={cn('text-xs', onArt ? 'text-white/55' : 'text-muted-foreground')}>
              {t('queue.upcoming', { count: upcoming.length })}
              {suggested > 0 ? ` · ${t('queue.suggested', { count: suggested })}` : null}
            </p>
          ) : null}
        </div>
        {upcoming.length > 0 ? (
          <button
            type="button"
            onClick={clearUpcoming}
            className={cn(
              'h-8 rounded-full px-3 text-sm font-medium transition-[background-color,scale] outline-none active:scale-95',
              onArt
                ? 'bg-white/15 text-white hover:bg-white/25 focus-visible:ring-2 focus-visible:ring-white/60'
                : 'text-primary hover:bg-primary/10 focus-visible:ring-2 focus-visible:ring-ring/50',
            )}
          >
            {t('queue.clear')}
          </button>
        ) : null}
        {tone !== 'panel' ? (
          <>
            <ShuffleButton tone="sheet" className="size-8" />
            <RepeatButton tone="sheet" className="size-8" />
            <InfiniteButton tone="sheet" className="size-8" />
          </>
        ) : (
          <InfiniteButton tone="bar" className="size-8" />
        )}
      </div>

      {upcoming.length === 0 ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-2 px-8 pb-10 text-center">
          <ListMusic className={cn('size-8', onArt ? 'text-white/45' : 'text-muted-foreground/60')} strokeWidth={1.5} />
          <p className={cn('text-sm font-medium', onArt ? 'text-white/70' : 'text-muted-foreground')}>
            {refilling && current && !current.isRadio ? t('queue.findingMore') : t('queue.empty')}
          </p>
        </div>
      ) : (
        <SortableQueue entries={upcoming} offset={start} tone={tone} />
      )}
    </div>
  )
}

function SortableQueue({ entries, offset, tone }: { entries: PlayableTrack[]; offset: number; tone: QueueTone }) {
  const { t } = useTranslation('player')
  const move = usePlayer((s) => s.move)
  const scrollRef = useRef<HTMLDivElement>(null)
  const [activeId, setActiveId] = useState<string | null>(null)
  const ids = useMemo(() => entries.map(entryKey), [entries])
  const getItemKey = useCallback((index: number) => ids[index], [ids])
  const activeIndex = activeId ? ids.indexOf(activeId) : -1

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  )

  // Keep the dragged row mounted even when it scrolls out of the virtual window.
  const rangeExtractor = useCallback(
    (range: Range) => {
      const indexes = defaultRangeExtractor(range)
      if (activeIndex >= 0 && !indexes.includes(activeIndex)) indexes.push(activeIndex)
      return indexes.sort((a, b) => a - b)
    },
    [activeIndex],
  )

  // TanStack Virtual's getters aren't memoizable; the rows read positions straight from it each render.
  // eslint-disable-next-line react-hooks/incompatible-library -- TanStack Virtual is used as documented
  const virtualizer = useVirtualizer({
    count: entries.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 8,
    getItemKey,
    rangeExtractor,
  })

  const onDragStart = (event: DragStartEvent) => setActiveId(String(event.active.id))
  const onDragEnd = (event: DragEndEvent) => {
    setActiveId(null)
    const { active, over } = event
    if (!over || active.id === over.id) return
    const from = ids.indexOf(String(active.id))
    const to = ids.indexOf(String(over.id))
    if (from >= 0 && to >= 0) move(offset + from, offset + to)
  }

  const activeEntry = activeIndex >= 0 ? entries[activeIndex] : undefined

  return (
    <div
      ref={scrollRef}
      className="scrollbar-thin relative min-h-0 flex-1 touch-pan-y overflow-y-auto overscroll-contain pb-6"
      aria-label={t('queue.playingNext')}
    >
      <DndContext
        sensors={sensors}
        collisionDetection={closestCenter}
        modifiers={[restrictToVerticalAxis, restrictToFirstScrollableAncestor]}
        onDragStart={onDragStart}
        onDragEnd={onDragEnd}
        onDragCancel={() => setActiveId(null)}
      >
        <SortableContext items={ids} strategy={verticalListSortingStrategy}>
          <ul className="relative" style={{ height: virtualizer.getTotalSize() }}>
            {virtualizer.getVirtualItems().map((item) => (
              <SortableRow
                key={item.key}
                id={ids[item.index]}
                entry={entries[item.index]}
                queueIndex={offset + item.index}
                tone={tone}
                style={{ position: 'absolute', top: item.start, left: 0, right: 0, height: ROW_HEIGHT }}
              />
            ))}
          </ul>
        </SortableContext>
        <DragOverlay dropAnimation={{ duration: 180, easing: 'cubic-bezier(0.2, 0, 0, 1)' }}>
          {activeEntry ? (
            <div
              className={cn(
                'rounded-xl shadow-2xl',
                tone === 'panel' ? 'bg-popover ring-1 ring-border' : 'bg-white/20 ring-1 ring-white/20 backdrop-blur-xl',
              )}
            >
              <RowContent entry={activeEntry} tone={tone} dragging />
            </div>
          ) : null}
        </DragOverlay>
      </DndContext>
    </div>
  )
}

interface SortableRowProps {
  id: string
  entry: PlayableTrack
  queueIndex: number
  tone: QueueTone
  style: CSSProperties
}

const SortableRow = memo(function SortableRow({ id, entry, queueIndex, tone, style }: SortableRowProps) {
  const { t } = useTranslation('player')
  const { attributes, listeners, setNodeRef, setActivatorNodeRef, transform, transition, isDragging } = useSortable({ id })
  const jumpTo = usePlayer((s) => s.jumpTo)
  const removeAt = usePlayer((s) => s.removeAt)
  const onArt = tone !== 'panel'

  return (
    <li
      ref={setNodeRef}
      style={{ ...style, transform: CSS.Translate.toString(transform), transition }}
      className={cn('group/row', isDragging && 'opacity-30')}
    >
      <SwipeToRemove onRemove={() => removeAt(queueIndex)} enabled={isTouch && !isDragging} label={t('queue.remove')}>
        <div className="flex h-full items-center">
          <button
            type="button"
            onClick={() => jumpTo(queueIndex)}
            className={cn(
              'flex h-full min-w-0 flex-1 items-center rounded-lg text-left outline-none',
              onArt ? 'focus-visible:bg-white/10 active:bg-white/10' : 'hover:bg-accent/60 focus-visible:bg-accent active:bg-accent',
            )}
          >
            <RowContent entry={entry} tone={tone} />
          </button>
          {!isTouch ? (
            <button
              type="button"
              onClick={() => removeAt(queueIndex)}
              aria-label={t('queue.removeTrack', { title: entry.title })}
              className={cn(
                'grid size-8 shrink-0 place-items-center rounded-md opacity-0 transition-opacity outline-none group-hover/row:opacity-100 focus-visible:opacity-100',
                onArt ? 'text-white/60 hover:bg-white/10 hover:text-white' : 'text-muted-foreground hover:bg-accent hover:text-foreground',
              )}
            >
              <X className="size-4" />
            </button>
          ) : null}
          <button
            type="button"
            ref={setActivatorNodeRef}
            {...attributes}
            {...listeners}
            aria-label={t('queue.reorder', { title: entry.title })}
            className={cn(
              'grid h-full w-11 shrink-0 cursor-grab touch-none place-items-center outline-none active:cursor-grabbing',
              onArt ? 'text-white/45 focus-visible:text-white' : 'text-muted-foreground/60 focus-visible:text-foreground',
              tone === 'panel' ? 'mr-2' : tone === 'sheet' ? 'mr-3' : '',
            )}
          >
            <GripHorizontal className="size-5" strokeWidth={1.75} />
          </button>
        </div>
      </SwipeToRemove>
    </li>
  )
})

function RowContent({ entry, tone, dragging }: { entry: PlayableTrack; tone: QueueTone; dragging?: boolean }) {
  const { t } = useTranslation('player')
  const onArt = tone !== 'panel'
  return (
    <div className={cn('flex h-14 min-w-0 flex-1 items-center gap-3', tone === 'panel' ? 'px-3' : tone === 'sheet' ? 'pl-6' : 'pl-2', dragging && 'pr-4')}>
      <CoverArt coverArt={entry.coverArt} size={40} flat={onArt} icon={entry.isRadio ? Radio : undefined} />
      <div className="min-w-0 flex-1 leading-tight">
        <p className={cn('truncate text-[15px] font-medium md:text-sm', onArt ? 'text-white' : 'text-foreground')}>{entry.title}</p>
        <p className={cn('flex items-center gap-1 text-[13px] md:text-xs', onArt ? 'text-white/55' : 'text-muted-foreground')}>
          {entry.autoAdded ? (
            <InfinityIcon className="size-3.5 shrink-0" strokeWidth={2} role="img" aria-label={t('queue.suggestion')} />
          ) : null}
          <span className="truncate">{entry.isRadio ? t('live.radio') : entry.artist}</span>
        </p>
      </div>
      {entry.isRadio ? (
        <LiveBadge tone={onArt ? 'sheet' : 'bar'} />
      ) : tone === 'panel' ? (
        <span className="tnum shrink-0 text-xs text-muted-foreground">{formatDuration(entry.duration)}</span>
      ) : null}
    </div>
  )
}

/** iOS-style swipe-left-to-delete (touch devices). */
function SwipeToRemove({
  children,
  onRemove,
  enabled,
  label,
}: {
  children: ReactNode
  onRemove: () => void
  enabled: boolean
  label: string
}) {
  const x = useMotionValue(0)
  const revealWidth = useTransform(x, (v) => Math.max(0, -v))
  const iconOpacity = useTransform(x, [-SWIPE_REMOVE, -24, 0], [1, 0.4, 0])

  if (!enabled) return <div className="h-full">{children}</div>

  const onDragEnd = (_: unknown, info: PanInfo) => {
    if (info.offset.x < -SWIPE_REMOVE || info.velocity.x < -800) onRemove()
  }

  return (
    <div className="relative h-full overflow-hidden">
      <motion.div
        aria-hidden
        className="absolute inset-y-0 right-0 overflow-hidden bg-destructive text-white"
        style={{ width: revealWidth }}
      >
        <motion.span
          style={{ opacity: iconOpacity }}
          className="absolute inset-y-0 right-5 flex items-center gap-1.5 text-sm font-medium whitespace-nowrap"
        >
          <Trash2 className="size-4" />
          {label}
        </motion.span>
      </motion.div>
      <motion.div
        drag="x"
        dragDirectionLock
        dragConstraints={{ left: 0, right: 0 }}
        dragElastic={{ left: 0.9, right: 0 }}
        dragMomentum={false}
        onDragEnd={onDragEnd}
        style={{ x, touchAction: 'pan-y' }}
        className="relative h-full"
      >
        {children}
      </motion.div>
    </div>
  )
}
