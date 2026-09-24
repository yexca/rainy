import {
  DndContext,
  KeyboardSensor,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core'
import { restrictToParentElement, restrictToVerticalAxis } from '@dnd-kit/modifiers'
import {
  SortableContext,
  arrayMove,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { GripVertical, Loader2, MinusCircle } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { CoverArt } from '@/components/cover-art'
import { Button } from '@/components/ui/button'
import type { Track } from '@/lib/api/types'
import { formatDuration } from '@/lib/format'
import { cn } from '@/lib/utils'

import { planPlaylistEdit, type PlaylistEditPlan } from '../lib/playlist-edits'

interface Entry {
  key: string
  /** Position in the original (visible) list. */
  index: number
  track: Track
}

export interface PlaylistEditorProps {
  tracks: Track[]
  saving: boolean
  onCancel: () => void
  /** Called with the edit plan and the resulting track list (only when something changed). */
  onSave: (plan: PlaylistEditPlan, tracks: Track[]) => void
}

/**
 * Edit mode for a playlist the user owns: drag handles (pointer, touch and keyboard via dnd-kit)
 * to reorder, minus buttons to remove. Nothing is sent until "Done".
 */
export function PlaylistEditor({ tracks, saving, onCancel, onSave }: PlaylistEditorProps) {
  const { t } = useTranslation('library')
  // Keys must be unique even when a track appears twice.
  const [entries, setEntries] = useState<Entry[]>(() =>
    tracks.map((track, index) => ({ key: `${index}:${track.id}`, index, track })),
  )
  const done = () => {
    const plan = planPlaylistEdit(
      tracks.length,
      entries.map((e) => e.index),
      entries.map((e) => e.track.id),
    )
    if (!plan) onCancel()
    else onSave(plan, entries.map((e) => e.track))
  }

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  )

  const onDragEnd = (event: DragEndEvent) => {
    const { active, over } = event
    if (!over || active.id === over.id) return
    setEntries((list) => {
      const from = list.findIndex((e) => e.key === active.id)
      const to = list.findIndex((e) => e.key === over.id)
      return from < 0 || to < 0 ? list : arrayMove(list, from, to)
    })
  }

  return (
    <div>
      <div className="mb-3 flex flex-wrap items-center justify-between gap-3 rounded-xl bg-muted/60 px-4 py-3">
        <p className="text-sm text-muted-foreground">{t('playlist.editHint')}</p>
        <div className="flex items-center gap-2">
          <Button variant="ghost" onClick={onCancel} disabled={saving} className="h-9">
            {t('common:actions.cancel')}
          </Button>
          <Button
            onClick={done}
            disabled={saving}
            className="h-9 min-w-20 font-semibold"
          >
            {saving ? <Loader2 className="animate-spin" aria-hidden /> : null}
            {t('common:actions.done')}
          </Button>
        </div>
      </div>
      <DndContext
        sensors={sensors}
        collisionDetection={closestCenter}
        modifiers={[restrictToVerticalAxis, restrictToParentElement]}
        onDragEnd={onDragEnd}
      >
        <SortableContext items={entries.map((e) => e.key)} strategy={verticalListSortingStrategy}>
          <ul className="relative">
            {entries.map((entry) => (
              <SortableRow
                key={entry.key}
                entry={entry}
                onRemove={() => setEntries((list) => list.filter((e) => e.key !== entry.key))}
              />
            ))}
          </ul>
        </SortableContext>
      </DndContext>
      {entries.length === 0 ? (
        <p className="py-10 text-center text-sm text-muted-foreground">{t('playlist.editEmpty')}</p>
      ) : null}
    </div>
  )
}

function SortableRow({ entry, onRemove }: { entry: Entry; onRemove: () => void }) {
  const { t } = useTranslation('library')
  const { attributes, listeners, setNodeRef, setActivatorNodeRef, transform, transition, isDragging } = useSortable({
    id: entry.key,
  })
  const { track } = entry

  return (
    <li
      ref={setNodeRef}
      style={{ transform: CSS.Translate.toString(transform), transition }}
      className={cn(
        'relative flex h-[60px] items-center gap-3 rounded-lg bg-background md:h-12 md:px-2',
        isDragging && 'z-10 shadow-lg ring-1 ring-border',
      )}
    >
      <button
        type="button"
        onClick={onRemove}
        aria-label={t('playlist.removeSong', { title: track.title })}
        className="-ml-2 grid size-11 shrink-0 place-items-center text-destructive md:ml-0 md:size-8"
      >
        <MinusCircle className="size-[22px] md:size-5" fill="currentColor" stroke="var(--background)" strokeWidth={2} />
      </button>
      <CoverArt coverArt={track.coverArt} size={40} />
      <div className="min-w-0 flex-1">
        <p className="truncate text-[15px] font-medium md:text-sm">{track.title}</p>
        <p className="truncate text-[13px] text-muted-foreground">{track.artist}</p>
      </div>
      <span className="tnum hidden text-[13px] text-muted-foreground sm:block">{formatDuration(track.duration)}</span>
      <button
        type="button"
        ref={setActivatorNodeRef}
        {...attributes}
        {...listeners}
        aria-label={t('playlist.dragSong', { title: track.title })}
        className="-mr-2 grid size-11 shrink-0 cursor-grab touch-none place-items-center text-muted-foreground active:cursor-grabbing md:mr-0 md:size-8"
      >
        <GripVertical className="size-5" strokeWidth={1.75} aria-hidden />
      </button>
    </li>
  )
}
