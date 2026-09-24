/**
 * Logical tag fields shown in the tag editor's "Details" tab and used by the batch tools.
 *
 * Every field maps onto one or more TagLib property keys (docs/architecture/contract.md §7.6, upper-case). A
 * field reads a display string from a track's (effective) tag map and turns an edited string
 * back into a tag patch (`key → values`, `[]` deletes the key). Composite fields (track / disc
 * number + total) keep the file's existing style: `3/12` in `TRACKNUMBER` for ID3 / MP4 / APE
 * files, separate `TRACKTOTAL` for Vorbis-comment formats (FLAC, Ogg, Opus).
 */
import type { TagMap } from '@/lib/api/types'

export type FieldId =
  | 'title'
  | 'artist'
  | 'album'
  | 'albumArtist'
  | 'trackNumber'
  | 'trackTotal'
  | 'discNumber'
  | 'discTotal'
  | 'date'
  | 'genre'
  | 'composer'
  | 'comment'
  | 'bpm'
  | 'compilation'
  | 'discSubtitle'

export type FieldKind = 'text' | 'multi' | 'number' | 'textarea' | 'bool' | 'genre' | 'date'

/** Per-track context a field needs to decide how to write itself. */
export interface FieldContext {
  /** Lower-case file suffix without dot (`flac`, `mp3`, …). */
  suffix: string
}

export interface FieldDef {
  id: FieldId
  kind: FieldKind
  /** Tag keys this field reads / writes (used for revert and dirty tracking). */
  keys: readonly string[]
  /** Whether batch text tools (find & replace, case, copy) may target this field. */
  textual: boolean
  get(tags: TagMap): string
  set(value: string, tags: TagMap, ctx: FieldContext): TagMap
}

/** Separator shown between multiple values of one key (`Artist A; Artist B`). */
export const MULTI_SEPARATOR = '; '

/** Formats that store track/disc totals in separate Vorbis comments. */
const VORBIS_SUFFIXES: ReadonlySet<string> = new Set(['flac', 'ogg', 'oga', 'opus', 'spx'])

export function first(tags: TagMap, key: string): string {
  return tags[key]?.[0] ?? ''
}

export function splitMulti(value: string): string[] {
  return value
    .split(';')
    .map((v) => v.trim())
    .filter(Boolean)
}

function single(value: string): string[] {
  const trimmed = value.trim()
  return trimmed ? [trimmed] : []
}

function textField(id: FieldId, key: string, kind: 'text' | 'textarea' = 'text'): FieldDef {
  return {
    id,
    kind,
    keys: [key],
    textual: true,
    get: (tags) => (tags[key] ?? []).join(MULTI_SEPARATOR),
    set: (value) => ({ [key]: kind === 'textarea' ? (value.trim() ? [value.replace(/\s+$/, '')] : []) : single(value) }),
  }
}

function multiField(id: FieldId, key: string, kind: 'multi' | 'genre' = 'multi'): FieldDef {
  return {
    id,
    kind,
    keys: [key],
    textual: true,
    get: (tags) => (tags[key] ?? []).join(MULTI_SEPARATOR),
    set: (value) => ({ [key]: splitMulti(value) }),
  }
}

/** `"3/12"` → `["3", "12"]`, `"3"` → `["3", ""]`. */
export function splitPair(raw: string): [string, string] {
  const [a = '', b = ''] = raw.split('/', 2)
  return [a.trim(), b.trim()]
}

/** Normalise a number-ish input: digits only, leading zeros kept out (`"03"` → `"3"`). */
export function cleanNumber(value: string): string {
  const digits = value.trim().replace(/[^\d]/g, '')
  if (!digits) return ''
  return String(Number.parseInt(digits, 10))
}

interface PairKeys {
  number: string
  totals: readonly string[] // primary first
}

const TRACK_KEYS: PairKeys = { number: 'TRACKNUMBER', totals: ['TRACKTOTAL', 'TOTALTRACKS'] }
const DISC_KEYS: PairKeys = { number: 'DISCNUMBER', totals: ['DISCTOTAL', 'TOTALDISCS'] }

function pairTotal(tags: TagMap, keys: PairKeys): string {
  for (const key of keys.totals) {
    const v = first(tags, key)
    if (v) return v.trim()
  }
  return splitPair(first(tags, keys.number))[1]
}

/** Whether this file keeps `n/total` in the number key (instead of a separate total key). */
function usesCombinedStyle(tags: TagMap, keys: PairKeys, ctx: FieldContext): boolean {
  if (first(tags, keys.number).includes('/')) return true
  if (keys.totals.some((k) => (tags[k]?.length ?? 0) > 0)) return false
  return !VORBIS_SUFFIXES.has(ctx.suffix.toLowerCase())
}

function writePair(tags: TagMap, keys: PairKeys, ctx: FieldContext, num: string, total: string): TagMap {
  const patch: TagMap = {}
  if (usesCombinedStyle(tags, keys, ctx)) {
    patch[keys.number] = num ? [total ? `${num}/${total}` : num] : total ? [`0/${total}`] : []
    for (const k of keys.totals) if (tags[k]?.length) patch[k] = []
    return patch
  }
  patch[keys.number] = num ? [num] : []
  const [primary, ...others] = keys.totals
  patch[primary] = total ? [total] : []
  for (const k of others) if (tags[k]?.length) patch[k] = []
  return patch
}

function pairNumberField(id: FieldId, keys: PairKeys): FieldDef {
  return {
    id,
    kind: 'number',
    keys: [keys.number, ...keys.totals],
    textual: false,
    get: (tags) => {
      const n = splitPair(first(tags, keys.number))[0]
      return n === '0' ? '' : n
    },
    set: (value, tags, ctx) => writePair(tags, keys, ctx, cleanNumber(value), cleanNumber(pairTotal(tags, keys))),
  }
}

function pairTotalField(id: FieldId, keys: PairKeys): FieldDef {
  return {
    id,
    kind: 'number',
    keys: [keys.number, ...keys.totals],
    textual: false,
    get: (tags) => pairTotal(tags, keys),
    set: (value, tags, ctx) =>
      writePair(tags, keys, ctx, cleanNumber(splitPair(first(tags, keys.number))[0]), cleanNumber(value)),
  }
}

const dateField: FieldDef = {
  id: 'date',
  kind: 'date',
  keys: ['DATE', 'YEAR'],
  textual: false,
  get: (tags) => first(tags, 'DATE') || first(tags, 'YEAR'),
  set: (value, tags) => {
    const v = value.trim()
    const patch: TagMap = { DATE: v ? [v] : [] }
    // Keep a legacy YEAR key consistent instead of leaving a stale year behind.
    if (tags.YEAR?.length) patch.YEAR = v ? [v.slice(0, 4)] : []
    return patch
  },
}

const bpmField: FieldDef = {
  id: 'bpm',
  kind: 'number',
  keys: ['BPM'],
  textual: false,
  get: (tags) => first(tags, 'BPM'),
  set: (value) => {
    const n = cleanNumber(value)
    return { BPM: n && n !== '0' ? [n] : [] }
  },
}

const compilationField: FieldDef = {
  id: 'compilation',
  kind: 'bool',
  keys: ['COMPILATION'],
  textual: false,
  get: (tags) => (['1', 'true', 'yes'].includes(first(tags, 'COMPILATION').trim().toLowerCase()) ? '1' : ''),
  set: (value) => ({ COMPILATION: value === '1' ? ['1'] : [] }),
}

export const FIELDS: readonly FieldDef[] = [
  textField('title', 'TITLE'),
  multiField('artist', 'ARTIST'),
  textField('album', 'ALBUM'),
  multiField('albumArtist', 'ALBUMARTIST'),
  pairNumberField('trackNumber', TRACK_KEYS),
  pairTotalField('trackTotal', TRACK_KEYS),
  pairNumberField('discNumber', DISC_KEYS),
  pairTotalField('discTotal', DISC_KEYS),
  textField('discSubtitle', 'DISCSUBTITLE'),
  dateField,
  multiField('genre', 'GENRE', 'genre'),
  multiField('composer', 'COMPOSER'),
  bpmField,
  compilationField,
  textField('comment', 'COMMENT', 'textarea'),
]

export const FIELD_BY_ID: Readonly<Record<FieldId, FieldDef>> = Object.fromEntries(
  FIELDS.map((f) => [f.id, f]),
) as Record<FieldId, FieldDef>

/** Fields text tools (find & replace, case, copy) can target. */
export const TEXT_FIELDS: readonly FieldDef[] = FIELDS.filter((f) => f.textual)

/** Keys edited elsewhere (Lyrics tab, Cover tab) and hidden from the raw "All tags" table. */
export const HIDDEN_RAW_KEYS: ReadonlySet<string> = new Set(['LYRICS', 'UNSYNCEDLYRICS'])

/** Valid custom tag key: upper-case letters, digits and a few separators. */
export const TAG_KEY_PATTERN = /^[A-Z0-9][A-Z0-9 _:.\-/]{0,63}$/

export function normalizeTagKey(key: string): string {
  return key.trim().toUpperCase()
}
