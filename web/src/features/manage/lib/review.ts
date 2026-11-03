/**
 * Helpers for Tracks › Review: downloads held for review before they are imported
 * (`/api/manage/staging`).
 */
// Relative `.ts` import and no runtime imports, so node --test (tests/review.test.ts) can load it.
import type { StagedItem, TagMap } from '../../../lib/api/types.ts'

/** The tags the review form edits, as display strings (several values joined by "; "). */
export interface ReviewForm {
  title: string
  artist: string
  album: string
  albumArtist: string
  track: string
  disc: string
  date: string
  genre: string
}

export const REVIEW_FIELDS: readonly (keyof ReviewForm)[] = ['title', 'artist', 'album', 'albumArtist', 'track', 'disc', 'date', 'genre']

const SEPARATOR = '; '

/** Fields with several values ("Artist A; Artist B"). */
const MULTI: ReadonlySet<keyof ReviewForm> = new Set(['artist', 'genre'])

const KEYS: Record<keyof ReviewForm, string> = {
  title: 'TITLE',
  artist: 'ARTIST',
  album: 'ALBUM',
  albumArtist: 'ALBUMARTIST',
  track: 'TRACKNUMBER',
  disc: 'DISCNUMBER',
  date: 'DATE',
  genre: 'GENRE',
}

const NUMBERED: ReadonlySet<keyof ReviewForm> = new Set(['track', 'disc'])

function firstValue(tags: TagMap, key: string): string {
  return tags[key]?.[0] ?? ''
}

/** "3/12" → ["3", "12"]. */
function splitNumber(raw: string): [string, string] {
  const [n = '', total = ''] = raw.split('/', 2)
  return [n.trim(), total.trim()]
}

export function formFromTags(tags: TagMap): ReviewForm {
  const form = {} as ReviewForm
  for (const field of REVIEW_FIELDS) {
    const key = KEYS[field]
    if (NUMBERED.has(field)) form[field] = splitNumber(firstValue(tags, key))[0]
    else if (MULTI.has(field)) form[field] = (tags[key] ?? []).join(SEPARATOR)
    else form[field] = firstValue(tags, key)
  }
  if (!form.date) form.date = firstValue(tags, 'YEAR')
  return form
}

function values(field: keyof ReviewForm, value: string, tags: TagMap): string[] {
  const trimmed = value.trim()
  if (MULTI.has(field)) {
    return trimmed
      .split(';')
      .map((v) => v.trim())
      .filter(Boolean)
  }
  if (NUMBERED.has(field)) {
    const digits = trimmed.replace(/[^\d]/g, '')
    if (!digits) return []
    const n = String(Number.parseInt(digits, 10))
    const total = splitNumber(firstValue(tags, KEYS[field]))[1]
    return [total ? `${n}/${total}` : n] // keep the file's "n/total" style
  }
  return trimmed ? [trimmed] : []
}

function sameValues(a: readonly string[] | undefined, b: readonly string[]): boolean {
  const left = a ?? []
  return left.length === b.length && left.every((v, i) => v === b[i])
}

/**
 * The tag changes (`key → values`, `[]` deletes) that turn `tags` into `form`; empty when nothing
 * changed. A field the user left as it was is never rewritten, even when the file stores it in
 * another shape ("A; B" in one value, a YEAR without DATE, "1/12").
 */
export function tagPatch(form: ReviewForm, tags: TagMap): TagMap {
  const original = formFromTags(tags)
  const patch: TagMap = {}
  for (const field of REVIEW_FIELDS) {
    const next = values(field, form[field], tags)
    if (sameValues(values(field, original[field], tags), next)) continue
    patch[KEYS[field]] = next
  }
  return patch
}

/** How many kinds of conflict an item has (existing file, earlier download, similar tracks). */
export function conflictCount(item: Pick<StagedItem, 'conflict'>): number {
  const c = item.conflict
  return (c.exists ? 1 : 0) + (c.sameSource.length > 0 ? 1 : 0) + (c.similar.length > 0 ? 1 : 0)
}

/** "Title — Artist A / Artist B", falling back to the title the site reported. */
export function stagedName(item: Pick<StagedItem, 'tags' | 'title' | 'name'>): string {
  const title = firstValue(item.tags, 'TITLE') || item.title || item.name.split('/').pop() || ''
  const artists = (item.tags.ARTIST ?? []).join(' / ')
  return artists ? `${title} — ${artists}` : title
}
