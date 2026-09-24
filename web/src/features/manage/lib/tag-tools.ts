/**
 * Batch tools of the tag editor (auto-number, tags from filename, find & replace, case
 * transforms, copy field, clear field). Every tool is a pure function that proposes new logical
 * field values per track; the UI previews them before `setFieldValues` applies them.
 */
import type { TrackTags } from '@/lib/api/types'

import { FIELD_BY_ID, TEXT_FIELDS, type FieldId } from './tag-fields'
import { effectiveTags, type Edits } from './tag-state'

export type ToolId = 'autoNumber' | 'fromFilename' | 'replace' | 'case' | 'copy' | 'clear'

export const TOOL_IDS: readonly ToolId[] = ['autoNumber', 'fromFilename', 'replace', 'case', 'copy', 'clear']

export type FieldValues = Partial<Record<FieldId, string>>

export interface ToolChange {
  field: FieldId
  before: string
  after: string
}

export interface ToolPreviewRow {
  trackId: string
  /** File name shown in the preview. */
  name: string
  changes: ToolChange[]
  /** e.g. "filename does not match the pattern". */
  error?: 'noMatch'
}

export interface ToolResult {
  values: Record<string, FieldValues>
  rows: ToolPreviewRow[]
  /** Tracks that will change. */
  changed: number
  /** Invalid options (bad regex, empty pattern, …) as an i18n key suffix. */
  error?: 'invalidRegex' | 'emptyPattern' | 'noTokens' | 'sameField'
}

function fieldValue(item: TrackTags, edits: Edits, field: FieldId): string {
  return FIELD_BY_ID[field].get(effectiveTags(item, edits))
}

function buildResult(
  items: readonly TrackTags[],
  edits: Edits,
  proposals: ReadonlyMap<string, FieldValues | 'noMatch'>,
): ToolResult {
  const values: Record<string, FieldValues> = {}
  const rows: ToolPreviewRow[] = []
  let changed = 0
  for (const item of items) {
    const id = item.track.id
    const proposal = proposals.get(id)
    if (!proposal) continue
    const name = item.track.filename || item.track.path
    if (proposal === 'noMatch') {
      rows.push({ trackId: id, name, changes: [], error: 'noMatch' })
      continue
    }
    const changes: ToolChange[] = []
    const accepted: FieldValues = {}
    for (const [field, after] of Object.entries(proposal) as [FieldId, string][]) {
      const before = fieldValue(item, edits, field)
      if (before === after) continue
      changes.push({ field, before, after })
      accepted[field] = after
    }
    if (changes.length === 0) continue
    changed++
    values[id] = accepted
    rows.push({ trackId: id, name, changes })
  }
  return { values, rows, changed }
}

const empty = (error?: ToolResult['error']): ToolResult => ({ values: {}, rows: [], changed: 0, error })

// ---------------------------------------------------------------------------------------------
// Auto-number
// ---------------------------------------------------------------------------------------------

export interface AutoNumberOptions {
  start: number
  /** Also write the track total (size of the numbering group). */
  setTotal: boolean
  /** Restart numbering for each disc. */
  perDisc: boolean
  order: 'selection' | 'filename' | 'path'
}

export const defaultAutoNumber: AutoNumberOptions = { start: 1, setTotal: true, perDisc: true, order: 'selection' }

const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' })

export function autoNumber(items: readonly TrackTags[], edits: Edits, o: AutoNumberOptions): ToolResult {
  const ordered = [...items]
  if (o.order === 'filename') ordered.sort((a, b) => collator.compare(a.track.filename, b.track.filename))
  if (o.order === 'path') ordered.sort((a, b) => collator.compare(a.track.path, b.track.path))

  // Numbering restarts for every album (and, optionally, every disc of it).
  const group = (item: TrackTags) =>
    `${item.track.albumId}|${o.perDisc ? fieldValue(item, edits, 'discNumber') || '1' : ''}`
  const sizes = new Map<string, number>()
  for (const item of ordered) sizes.set(group(item), (sizes.get(group(item)) ?? 0) + 1)

  const counters = new Map<string, number>()
  const start = Number.isFinite(o.start) && o.start >= 0 ? Math.floor(o.start) : 1
  const proposals = new Map<string, FieldValues>()
  for (const item of ordered) {
    const g = group(item)
    const n = counters.get(g) ?? start
    counters.set(g, n + 1)
    const values: FieldValues = { trackNumber: String(n) }
    if (o.setTotal) values.trackTotal = String((sizes.get(g) ?? 0) + start - 1)
    proposals.set(item.track.id, values)
  }
  return buildResult(items, edits, proposals)
}

// ---------------------------------------------------------------------------------------------
// Tags from filename
// ---------------------------------------------------------------------------------------------

/** Pattern tokens → fields (`{ignore}` / `{*}` skips text). Same token names as rename patterns. */
export const FILENAME_TOKENS: Readonly<Record<string, FieldId | null>> = {
  title: 'title',
  artist: 'artist',
  album: 'album',
  albumartist: 'albumArtist',
  track: 'trackNumber',
  disc: 'discNumber',
  year: 'date',
  genre: 'genre',
  composer: 'composer',
  ignore: null,
  '*': null,
}

const NUMERIC_TOKENS = new Set(['track', 'disc', 'year'])

export interface CompiledPattern {
  regex: RegExp
  fields: (FieldId | null)[]
  /** Number of path segments the pattern spans (`{artist}/{album}/{track} {title}` → 3). */
  segments: number
}

function escapeRegex(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

export function compileFilenamePattern(pattern: string): CompiledPattern | 'emptyPattern' | 'noTokens' {
  const p = pattern.trim().replace(/^\/+|\/+$/g, '')
  if (!p) return 'emptyPattern'
  const fields: (FieldId | null)[] = []
  let source = '^'
  const re = /\{([a-z*]+)\}/gi
  let last = 0
  let m: RegExpExecArray | null
  while ((m = re.exec(p))) {
    const token = m[1].toLowerCase()
    source += escapeRegex(p.slice(last, m.index)).replace(/\s+/g, '\\s+')
    if (token in FILENAME_TOKENS) {
      fields.push(FILENAME_TOKENS[token])
      source += NUMERIC_TOKENS.has(token) ? '(\\d+)' : '(.+?)'
    } else {
      source += escapeRegex(m[0])
    }
    last = m.index + m[0].length
  }
  source += `${escapeRegex(p.slice(last)).replace(/\s+/g, '\\s+')}$`
  if (!fields.some((f) => f !== null)) return 'noTokens'
  return { regex: new RegExp(source, 'u'), fields, segments: p.split('/').length }
}

/** Path tail without extension spanning `segments` components. */
export function filenameSource(path: string, segments: number): string {
  const parts = path.split('/')
  const tail = parts.slice(Math.max(0, parts.length - segments))
  const lastPart = tail[tail.length - 1] ?? ''
  const dot = lastPart.lastIndexOf('.')
  tail[tail.length - 1] = dot > 0 ? lastPart.slice(0, dot) : lastPart
  return tail.join('/')
}

export function tagsFromFilename(items: readonly TrackTags[], edits: Edits, pattern: string): ToolResult {
  const compiled = compileFilenamePattern(pattern)
  if (typeof compiled === 'string') return empty(compiled)
  const proposals = new Map<string, FieldValues | 'noMatch'>()
  for (const item of items) {
    const m = compiled.regex.exec(filenameSource(item.track.path || item.track.filename, compiled.segments))
    if (!m) {
      proposals.set(item.track.id, 'noMatch')
      continue
    }
    const values: FieldValues = {}
    compiled.fields.forEach((field, i) => {
      if (!field) return
      const raw = (m[i + 1] ?? '').trim()
      values[field] = field === 'trackNumber' || field === 'discNumber' ? String(Number.parseInt(raw, 10)) : raw
    })
    proposals.set(item.track.id, values)
  }
  return buildResult(items, edits, proposals)
}

// ---------------------------------------------------------------------------------------------
// Find & replace
// ---------------------------------------------------------------------------------------------

export interface ReplaceOptions {
  field: FieldId | 'all'
  find: string
  replace: string
  regex: boolean
  caseSensitive: boolean
}

export const defaultReplace: ReplaceOptions = { field: 'title', find: '', replace: '', regex: false, caseSensitive: false }

function targetFields(field: FieldId | 'all'): FieldId[] {
  return field === 'all' ? TEXT_FIELDS.map((f) => f.id) : [field]
}

export function findReplace(items: readonly TrackTags[], edits: Edits, o: ReplaceOptions): ToolResult {
  if (!o.find) return empty()
  let re: RegExp
  try {
    re = new RegExp(o.regex ? o.find : escapeRegex(o.find), o.caseSensitive ? 'gu' : 'giu')
  } catch {
    return empty('invalidRegex')
  }
  // Literal mode: `$` in the replacement must not be interpreted.
  const replacement = o.regex ? o.replace : o.replace.replace(/\$/g, '$$$$')
  const proposals = new Map<string, FieldValues>()
  for (const item of items) {
    const values: FieldValues = {}
    for (const field of targetFields(o.field)) {
      const before = fieldValue(item, edits, field)
      if (!before) continue
      values[field] = before.replace(re, replacement)
    }
    proposals.set(item.track.id, values)
  }
  return buildResult(items, edits, proposals)
}

// ---------------------------------------------------------------------------------------------
// Case transforms
// ---------------------------------------------------------------------------------------------

export type CaseMode = 'title' | 'sentence' | 'upper' | 'lower'

export interface CaseOptions {
  field: FieldId | 'all'
  mode: CaseMode
}

export const defaultCase: CaseOptions = { field: 'title', mode: 'title' }

/** Words kept lower-case inside English titles (not at the start). */
const SMALL_WORDS = new Set(['a', 'an', 'and', 'as', 'at', 'but', 'by', 'for', 'in', 'of', 'on', 'or', 'the', 'to', 'vs', 'via'])

function capitalize(word: string): string {
  const chars = Array.from(word)
  const i = chars.findIndex((c) => /\p{L}/u.test(c))
  if (i < 0) return word
  chars[i] = chars[i].toLocaleUpperCase()
  return chars.join('')
}

export function transformCase(value: string, mode: CaseMode): string {
  switch (mode) {
    case 'upper':
      return value.toLocaleUpperCase()
    case 'lower':
      return value.toLocaleLowerCase()
    case 'sentence': {
      const lower = value.toLocaleLowerCase()
      return capitalize(lower)
    }
    case 'title': {
      let index = 0
      return value.replace(/[^\s/()[\]-]+/gu, (word) => {
        const position = index++
        // Keep acronyms (AC, DJ, USA) and words with inner capitals (iPhone, McCartney).
        if (/^\p{Lu}{2,}$/u.test(word) || /\p{Ll}\p{Lu}/u.test(word)) return word
        const lower = word.toLocaleLowerCase()
        if (position > 0 && SMALL_WORDS.has(lower)) return lower
        return capitalize(lower)
      })
    }
  }
}

export function changeCase(items: readonly TrackTags[], edits: Edits, o: CaseOptions): ToolResult {
  const proposals = new Map<string, FieldValues>()
  for (const item of items) {
    const values: FieldValues = {}
    for (const field of targetFields(o.field)) {
      const before = fieldValue(item, edits, field)
      if (before) values[field] = transformCase(before, o.mode)
    }
    proposals.set(item.track.id, values)
  }
  return buildResult(items, edits, proposals)
}

// ---------------------------------------------------------------------------------------------
// Copy / clear
// ---------------------------------------------------------------------------------------------

export interface CopyOptions {
  from: FieldId
  to: FieldId
  /** Only fill tracks where the target is empty. */
  onlyEmpty: boolean
}

export const defaultCopy: CopyOptions = { from: 'artist', to: 'albumArtist', onlyEmpty: false }

export function copyField(items: readonly TrackTags[], edits: Edits, o: CopyOptions): ToolResult {
  if (o.from === o.to) return empty('sameField')
  const proposals = new Map<string, FieldValues>()
  for (const item of items) {
    if (o.onlyEmpty && fieldValue(item, edits, o.to)) continue
    proposals.set(item.track.id, { [o.to]: fieldValue(item, edits, o.from) })
  }
  return buildResult(items, edits, proposals)
}

export interface ClearOptions {
  field: FieldId
}

export const defaultClear: ClearOptions = { field: 'comment' }

export function clearField(items: readonly TrackTags[], edits: Edits, o: ClearOptions): ToolResult {
  const proposals = new Map<string, FieldValues>()
  for (const item of items) proposals.set(item.track.id, { [o.field]: '' })
  return buildResult(items, edits, proposals)
}
