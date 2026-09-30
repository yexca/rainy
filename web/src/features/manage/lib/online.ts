/**
 * Pure helpers for the tag editor's online lookup (contract §7.6 `/manage/metadata/*`).
 *
 * A search result never touches files: the chosen fields become ordinary pending edits in the
 * editor (via `setFieldValues`), and the cover / lyrics become the editor's pending cover and
 * lyrics draft. The user reviews them and saves as usual.
 */
// Relative `.ts` import and no runtime imports, so node --test (tests/online.test.ts) can load it.
import type { MetadataResult, TrackTags } from '../../../lib/api/types.ts'

const MULTI_SEPARATOR = '; ' // as in tag-fields
const first = (tags: Record<string, string[]>, key: string): string => tags[key]?.[0] ?? ''

/** Tag fields (`FieldId`s of tag-fields) a result can fill, in the editor's field order. */
export const ONLINE_FIELDS = [
  'title',
  'artist',
  'album',
  'albumArtist',
  'trackNumber',
  'trackTotal',
  'discNumber',
  'discTotal',
  'date',
  'genre',
] as const

export type OnlineFieldId = (typeof ONLINE_FIELDS)[number]

/** Fields that describe one song, not the album: only offered when editing a single track. */
export const TRACK_ONLY_FIELDS: ReadonlySet<OnlineFieldId> = new Set(['title', 'artist', 'trackNumber', 'discNumber'])

/** The fields a result offers for this selection size, with their display values. */
export function resultFields(result: MetadataResult, single: boolean): Partial<Record<OnlineFieldId, string>> {
  const n = (v: number) => (v > 0 ? String(v) : '')
  const all: Record<OnlineFieldId, string> = {
    title: result.title,
    artist: result.artists.join(MULTI_SEPARATOR),
    album: result.album,
    albumArtist: result.albumArtist,
    trackNumber: n(result.trackNumber),
    trackTotal: n(result.trackTotal),
    discNumber: n(result.discNumber),
    discTotal: n(result.discTotal),
    date: result.date,
    genre: result.genre,
  }
  const out: Partial<Record<OnlineFieldId, string>> = {}
  for (const id of ONLINE_FIELDS) {
    if (!single && TRACK_ONLY_FIELDS.has(id)) continue
    const value = all[id].trim()
    if (value) out[id] = value
  }
  return out
}

/** Per-track field values for `setFieldValues`, in field order. */
export function onlineFieldValues(
  items: readonly TrackTags[],
  values: Partial<Record<OnlineFieldId, string>>,
  chosen: ReadonlySet<OnlineFieldId>,
): Record<string, Partial<Record<OnlineFieldId, string>>> {
  const perTrack: Partial<Record<OnlineFieldId, string>> = {}
  for (const id of ONLINE_FIELDS) {
    const value = values[id]
    if (value !== undefined && chosen.has(id) && (items.length === 1 || !TRACK_ONLY_FIELDS.has(id))) perTrack[id] = value
  }
  const out: Record<string, Partial<Record<OnlineFieldId, string>>> = {}
  if (Object.keys(perTrack).length === 0) return out
  for (const item of items) out[item.track.id] = { ...perTrack }
  return out
}

/** The search box's starting text: "title artist" for one track, "album album-artist" for several. */
export function defaultQuery(items: readonly TrackTags[]): string {
  const firstItem = items[0]
  if (!firstItem) return ''
  const tags = firstItem.tags
  const track = firstItem.track
  const parts =
    items.length === 1
      ? [first(tags, 'TITLE') || track.title, first(tags, 'ARTIST') || track.artist]
      : [first(tags, 'ALBUM') || track.album, first(tags, 'ALBUMARTIST') || first(tags, 'ARTIST') || track.artist]
  return parts
    .map((p) => p.trim())
    .filter(Boolean)
    .join(' ')
}

/** How well a result's length matches the track: within 3 s, off by more than 10 s, or in between. */
export function durationMatch(trackSeconds: number, resultSeconds: number): 'match' | 'near' | 'mismatch' | 'unknown' {
  if (!(trackSeconds > 0) || !(resultSeconds > 0)) return 'unknown'
  const diff = Math.abs(trackSeconds - resultSeconds)
  if (diff <= 3) return 'match'
  return diff > 10 ? 'mismatch' : 'near'
}

const STAMP_RE = /\[(\d{1,3}):(\d{1,2})(?:[.:](\d{1,3}))?\]/g
const LEADING_STAMPS_RE = /^(?:\s*\[\d{1,3}:\d{1,2}(?:[.:]\d{1,3})?\])+/

/** Leading LRC time tags of a line as milliseconds (empty for metadata and untimed lines). */
function lineTimes(line: string): number[] {
  const lead = LEADING_STAMPS_RE.exec(line)?.[0]
  if (!lead) return []
  const out: number[] = []
  for (const m of lead.matchAll(STAMP_RE)) {
    const frac = m[3] ?? ''
    const ms = frac.length === 1 ? Number(frac) * 100 : frac.length === 2 ? Number(frac) * 10 : Number(frac || 0)
    out.push(Number(m[1]) * 60000 + Number(m[2]) * 1000 + ms)
  }
  return out
}

function lineText(line: string): string {
  return line.replace(LEADING_STAMPS_RE, '').trim()
}

/**
 * Pair a translation with synced lyrics: each translated line is inserted right after the
 * original line with the same time tag, with that line's time tags — Rainy's bilingual LRC
 * layout. Empty and "//" translations, lines identical to the original, and translated lines
 * whose time has no original are dropped. Returns `text` unchanged when nothing pairs.
 */
export function mergeTranslation(text: string, translation: string): string {
  const byTime = new Map<number, string>()
  for (const line of translation.split('\n')) {
    const value = lineText(line)
    if (!value || value === '//') continue
    for (const time of lineTimes(line)) if (!byTime.has(time)) byTime.set(time, value)
  }
  if (byTime.size === 0) return text
  const out: string[] = []
  let paired = 0
  for (const line of text.split('\n')) {
    out.push(line)
    const times = lineTimes(line)
    if (times.length === 0) continue
    const value = byTime.get(times[0])
    if (value && value !== lineText(line)) {
      out.push(`${LEADING_STAMPS_RE.exec(line)?.[0].trim() ?? ''}${value}`)
      paired++
    }
  }
  return paired > 0 ? out.join('\n') : text
}

/** Lyrics as they would go into the draft. */
export function lyricsText(lyrics: { text: string; translation: string }, withTranslation: boolean): string {
  return withTranslation && lyrics.translation ? mergeTranslation(lyrics.text, lyrics.translation) : lyrics.text
}

/** File name for a downloaded cover, by content type. */
export function coverFileName(type: string): string {
  const ext = { 'image/png': 'png', 'image/webp': 'webp', 'image/gif': 'gif' }[type] ?? 'jpg'
  return `cover.${ext}`
}
