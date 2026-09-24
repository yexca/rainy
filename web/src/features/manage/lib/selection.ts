/**
 * Selection model of the library manager: either an explicit id set, or "everything matching
 * the current filters" minus exclusions (so selecting 20 000 tracks costs nothing until an
 * action actually needs the ids).
 */
import { useCallback, useMemo, useState } from 'react'

export type Selection =
  | { kind: 'ids'; ids: ReadonlySet<string> }
  | { kind: 'all'; excluded: ReadonlySet<string> }

export const EMPTY_SELECTION: Selection = { kind: 'ids', ids: new Set() }

export function isSelected(sel: Selection, id: string): boolean {
  return sel.kind === 'ids' ? sel.ids.has(id) : !sel.excluded.has(id)
}

export function selectionCount(sel: Selection, total: number): number {
  return sel.kind === 'ids' ? sel.ids.size : Math.max(0, total - sel.excluded.size)
}

export function toggleId(sel: Selection, id: string): Selection {
  if (sel.kind === 'ids') {
    const ids = new Set(sel.ids)
    if (ids.has(id)) ids.delete(id)
    else ids.add(id)
    return { kind: 'ids', ids }
  }
  const excluded = new Set(sel.excluded)
  if (excluded.has(id)) excluded.delete(id)
  else excluded.add(id)
  return { kind: 'all', excluded }
}

/** Add (or, with `select = false`, remove) several ids. */
export function setIds(sel: Selection, ids: readonly string[], select: boolean): Selection {
  if (sel.kind === 'ids') {
    const next = new Set(sel.ids)
    for (const id of ids) {
      if (select) next.add(id)
      else next.delete(id)
    }
    return { kind: 'ids', ids: next }
  }
  const excluded = new Set(sel.excluded)
  for (const id of ids) {
    if (select) excluded.delete(id)
    else excluded.add(id)
  }
  return { kind: 'all', excluded }
}

export interface SelectionApi {
  selection: Selection
  /** Row index of the last plain / ctrl click (shift-range anchor). */
  anchor: number | null
  setAnchor: (index: number | null) => void
  isSelected: (id: string) => boolean
  count: (total: number) => number
  toggle: (id: string, index?: number) => void
  only: (id: string, index?: number) => void
  add: (ids: readonly string[], select?: boolean) => void
  selectAll: () => void
  clear: () => void
}

export function useSelection(): SelectionApi {
  const [selection, setSelection] = useState<Selection>(EMPTY_SELECTION)
  const [anchor, setAnchor] = useState<number | null>(null)

  const toggle = useCallback((id: string, index?: number) => {
    setSelection((s) => toggleId(s, id))
    if (index !== undefined) setAnchor(index)
  }, [])
  const only = useCallback((id: string, index?: number) => {
    setSelection({ kind: 'ids', ids: new Set([id]) })
    if (index !== undefined) setAnchor(index)
  }, [])
  const add = useCallback((ids: readonly string[], select = true) => setSelection((s) => setIds(s, ids, select)), [])
  const selectAll = useCallback(() => setSelection({ kind: 'all', excluded: new Set() }), [])
  const clear = useCallback(() => {
    setSelection(EMPTY_SELECTION)
    setAnchor(null)
  }, [])

  return useMemo(
    () => ({
      selection,
      anchor,
      setAnchor,
      isSelected: (id: string) => isSelected(selection, id),
      count: (total: number) => selectionCount(selection, total),
      toggle,
      only,
      add,
      selectAll,
      clear,
    }),
    [selection, anchor, toggle, only, add, selectAll, clear],
  )
}
