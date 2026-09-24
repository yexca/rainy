/**
 * Pure state helpers for the tag editor.
 *
 * The editor keeps the loaded originals (`TrackTags[]`, in selection order) untouched and records
 * pending changes per track as a sparse tag patch (`edits[trackId][KEY] = values`, `[]` deletes).
 * A patch entry equal to the original value is dropped, so `edits` always is the exact diff that
 * `POST /api/manage/tags` needs.
 */
import type { TagEdit, TagMap, TrackTags } from '@/lib/api/types'

import { FIELD_BY_ID, type FieldContext, type FieldDef, type FieldId } from './tag-fields'

export type Edits = Readonly<Record<string, TagMap>>

export function sameValues(a: readonly string[] | undefined, b: readonly string[] | undefined): boolean {
  const x = a ?? []
  const y = b ?? []
  if (x.length !== y.length) return false
  for (let i = 0; i < x.length; i++) if (x[i] !== y[i]) return false
  return true
}

export function fieldContext(item: TrackTags): FieldContext {
  return { suffix: item.track.suffix || item.file.format || '' }
}

/** Original tags merged with the pending patch (deleted keys removed). */
export function effectiveTags(item: TrackTags, edits: Edits): TagMap {
  const patch = edits[item.track.id]
  if (!patch) return item.tags
  const out: TagMap = { ...item.tags }
  for (const [key, values] of Object.entries(patch)) {
    if (values.length === 0) delete out[key]
    else out[key] = values
  }
  return out
}

/** Apply `patch` to one track's pending edits, dropping entries equal to the original. */
function mergePatch(item: TrackTags, current: TagMap | undefined, patch: TagMap): TagMap | undefined {
  const next: TagMap = { ...(current ?? {}) }
  for (const [key, values] of Object.entries(patch)) {
    const original = item.tags[key] ?? []
    if (sameValues(original, values)) delete next[key]
    else next[key] = values
  }
  return Object.keys(next).length > 0 ? next : undefined
}

/** Apply per-track patches (`trackId → TagMap`). */
export function applyPatches(items: readonly TrackTags[], edits: Edits, patches: Readonly<Record<string, TagMap>>): Edits {
  const next: Record<string, TagMap> = { ...edits }
  for (const item of items) {
    const patch = patches[item.track.id]
    if (!patch) continue
    const merged = mergePatch(item, next[item.track.id], patch)
    if (merged) next[item.track.id] = merged
    else delete next[item.track.id]
  }
  return next
}

/** Set a logical field on every item to `value`. */
export function setField(items: readonly TrackTags[], edits: Edits, field: FieldDef, value: string): Edits {
  const patches: Record<string, TagMap> = {}
  for (const item of items) {
    patches[item.track.id] = field.set(value, effectiveTags(item, edits), fieldContext(item))
  }
  return applyPatches(items, edits, patches)
}

/**
 * Apply an edit from a Details input. Emptying a field whose tracks originally disagreed means
 * "leave them alone" (the input just shows the "Multiple values" placeholder again), not "delete
 * the tag on every track" — clearing is explicit via the Clear-field tool or the All-tags tab.
 */
export function changeField(items: readonly TrackTags[], edits: Edits, def: FieldDef, value: string): Edits {
  if (value.trim() === '' && def.kind !== 'bool' && summarizeField(items, {}, def).mixed) {
    return revertKeys(edits, def.keys)
  }
  return setField(items, edits, def, value)
}

/** Set logical field values per track (`trackId → fieldId → value`), in field order. */
export function setFieldValues(
  items: readonly TrackTags[],
  edits: Edits,
  values: Readonly<Record<string, Partial<Record<FieldId, string>>>>,
): Edits {
  let next = edits
  for (const item of items) {
    const perField = values[item.track.id]
    if (!perField) continue
    for (const [fieldId, value] of Object.entries(perField) as [FieldId, string][]) {
      const field = FIELD_BY_ID[fieldId]
      const patch = field.set(value, effectiveTags(item, next), fieldContext(item))
      next = applyPatches([item], next, { [item.track.id]: patch })
    }
  }
  return next
}

/** Drop pending changes to the given keys on every track. */
export function revertKeys(edits: Edits, keys: readonly string[]): Edits {
  const next: Record<string, TagMap> = {}
  for (const [id, patch] of Object.entries(edits)) {
    const rest: TagMap = { ...patch }
    for (const key of keys) delete rest[key]
    if (Object.keys(rest).length > 0) next[id] = rest
  }
  return next
}

export interface FieldSummary {
  /** Common value, or `''` when mixed. */
  value: string
  /** Tracks disagree on this field. */
  mixed: boolean
  /** The field has pending changes on at least one track. */
  dirty: boolean
}

export function summarizeField(items: readonly TrackTags[], edits: Edits, field: FieldDef): FieldSummary {
  let value: string | undefined
  let mixed = false
  let dirty = false
  for (const item of items) {
    const v = field.get(effectiveTags(item, edits))
    if (value === undefined) value = v
    else if (v !== value) mixed = true
    const patch = edits[item.track.id]
    if (patch && field.keys.some((k) => k in patch)) dirty = true
  }
  return { value: mixed ? '' : (value ?? ''), mixed, dirty }
}

export interface RawKeySummary {
  key: string
  /** Common values, or `[]` when mixed. */
  values: string[]
  mixed: boolean
  /** Present on only some of the tracks. */
  partial: boolean
  dirty: boolean
  /** Added in this session (not present in any original). */
  added: boolean
}

/** Union of all raw keys across tracks (sorted), with common values. */
export function summarizeRawKeys(items: readonly TrackTags[], edits: Edits, hidden: ReadonlySet<string>): RawKeySummary[] {
  const effective = items.map((item) => effectiveTags(item, edits))
  const keys = new Set<string>()
  for (const tags of effective) for (const key of Object.keys(tags)) if (!hidden.has(key)) keys.add(key)
  // Keys deleted on every track still show (struck through) so the deletion can be reverted.
  for (const patch of Object.values(edits)) for (const key of Object.keys(patch)) if (!hidden.has(key)) keys.add(key)

  const originalKeys = new Set<string>()
  for (const item of items) for (const key of Object.keys(item.tags)) originalKeys.add(key)

  return [...keys].sort().map((key) => {
    let values: string[] | undefined
    let mixed = false
    let present = 0
    for (const tags of effective) {
      const v = tags[key] ?? []
      if (v.length) present++
      if (values === undefined) values = v
      else if (!sameValues(values, v)) mixed = true
    }
    const dirty = Object.values(edits).some((patch) => key in patch)
    return {
      key,
      values: mixed ? [] : (values ?? []),
      mixed,
      partial: present > 0 && present < effective.length,
      dirty,
      added: !originalKeys.has(key),
    }
  })
}

/** Set a raw key to the same values on every track. */
export function setRawKey(items: readonly TrackTags[], edits: Edits, key: string, values: string[]): Edits {
  const patches: Record<string, TagMap> = {}
  for (const item of items) patches[item.track.id] = { [key]: values }
  return applyPatches(items, edits, patches)
}

/** The request body for `POST /api/manage/tags`: only tracks with changes. */
export function buildTagEdits(items: readonly TrackTags[], edits: Edits): TagEdit[] {
  const out: TagEdit[] = []
  for (const item of items) {
    const patch = edits[item.track.id]
    if (!patch) continue
    const tags: TagMap = {}
    for (const [key, values] of Object.entries(patch)) {
      // The raw editor keeps empty inputs while typing; they never reach the files.
      const clean = values.filter((v) => v !== '')
      if (!sameValues(clean, item.tags[key])) tags[key] = clean
    }
    if (Object.keys(tags).length > 0) out.push({ trackId: item.track.id, tags })
  }
  return out
}

export function changedTrackCount(items: readonly TrackTags[], edits: Edits): number {
  return buildTagEdits(items, edits).length
}

/** Keep only edits for the given track ids (e.g. the ones that failed to save). */
export function keepEdits(edits: Edits, ids: ReadonlySet<string>): Edits {
  const next: Record<string, TagMap> = {}
  for (const [id, patch] of Object.entries(edits)) if (ids.has(id)) next[id] = patch
  return next
}
