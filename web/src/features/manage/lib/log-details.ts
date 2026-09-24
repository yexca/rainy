/**
 * Readable views of `edit_log.details` (JSON written by the manage service, e.g.
 * `{"changes":{"TITLE":{"old":["a"],"new":["b"]}}}` or `{"from":"a.mp3","to":"b.mp3"}`).
 * Unknown shapes degrade to a key/value list.
 */

export interface TagChange {
  key: string
  old: string[]
  new: string[]
}

export interface ReadableDetails {
  changes: TagChange[]
  move?: { from: string; to: string }
  /** Any other scalar fields (`encoding: "gbk"`, `target: "lrc"`, …). */
  fields: { key: string; value: string }[]
}

/** Bookkeeping fields that mean nothing to people. */
const INTERNAL_KEYS: ReadonlySet<string> = new Set(['trashId', 'trashPath'])

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === 'object' && v !== null && !Array.isArray(v)
}

function toStrings(v: unknown): string[] {
  if (Array.isArray(v)) return v.map((x) => (typeof x === 'string' ? x : JSON.stringify(x)))
  if (v === undefined || v === null || v === '') return []
  return [typeof v === 'string' ? v : JSON.stringify(v)]
}

function scalar(v: unknown): string | null {
  if (typeof v === 'string') return v
  if (typeof v === 'number' || typeof v === 'boolean') return String(v)
  if (Array.isArray(v) && v.every((x) => typeof x === 'string' || typeof x === 'number')) return v.join(', ')
  return null
}

export function readDetails(details: unknown): ReadableDetails {
  const out: ReadableDetails = { changes: [], fields: [] }
  if (!isRecord(details)) return out
  for (const [key, value] of Object.entries(details)) {
    if (key === 'changes' && isRecord(value)) {
      for (const [tag, change] of Object.entries(value)) {
        if (isRecord(change)) out.changes.push({ key: tag, old: toStrings(change.old), new: toStrings(change.new) })
      }
      continue
    }
    if ((key === 'from' || key === 'to') && typeof value === 'string') continue
    if (INTERNAL_KEYS.has(key)) continue
    const s = scalar(value)
    if (s !== null && s !== '') out.fields.push({ key, value: s.length > 300 ? `${s.slice(0, 300)}…` : s })
    else if (isRecord(value) || Array.isArray(value)) {
      const json = JSON.stringify(value)
      out.fields.push({ key, value: json.length > 300 ? `${json.slice(0, 300)}…` : json })
    }
  }
  if (typeof details.from === 'string' && typeof details.to === 'string') out.move = { from: details.from, to: details.to }
  out.changes.sort((a, b) => a.key.localeCompare(b.key))
  return out
}
