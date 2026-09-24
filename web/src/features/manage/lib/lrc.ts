/** Small LRC helpers for the lyrics editor (parsing proper lives on the server, §5.10). */

/** `[mm:ss.xx]` line timestamps (also `[m:ss]`, `[mm:ss:xx]`, `[mm:ss.xxx]`). */
const TIMESTAMP_RE = /\[\d{1,3}:\d{1,2}(?:[.:]\d{1,3})?\]/
const LEADING_STAMPS_RE = /^(?:\s*\[\d{1,3}:\d{1,2}(?:[.:]\d{1,3})?\])+/

/** Whether the text looks like synced LRC (at least one line timestamp). */
export function isSyncedLrc(text: string): boolean {
  return text.split('\n').some((line) => LEADING_STAMPS_RE.test(line))
}

/** `83.456` → `[01:23.45]`. */
export function formatLrcTimestamp(seconds: number): string {
  const total = Math.max(0, Number.isFinite(seconds) ? seconds : 0)
  const minutes = Math.floor(total / 60)
  const secs = total - minutes * 60
  const whole = Math.floor(secs)
  const hundredths = Math.min(99, Math.floor((secs - whole) * 100))
  return `[${String(minutes).padStart(2, '0')}:${String(whole).padStart(2, '0')}.${String(hundredths).padStart(2, '0')}]`
}

export interface InsertResult {
  text: string
  /** Caret position after the edit (start of the next line). */
  caret: number
}

/**
 * Stamp the line containing `caret` with `seconds`: replaces existing leading timestamps, then
 * moves the caret to the start of the next line so repeated presses sync line by line.
 */
export function stampLine(text: string, caret: number, seconds: number): InsertResult {
  const lineStart = text.lastIndexOf('\n', Math.max(0, caret - 1)) + 1
  let lineEnd = text.indexOf('\n', lineStart)
  if (lineEnd < 0) lineEnd = text.length
  const line = text.slice(lineStart, lineEnd)
  const body = line.replace(LEADING_STAMPS_RE, '').replace(/^\s+/, '')
  const stamped = `${formatLrcTimestamp(seconds)}${body}`
  const next = text.slice(0, lineStart) + stamped + text.slice(lineEnd)
  const newLineEnd = lineStart + stamped.length
  const caretAfter = newLineEnd < next.length ? newLineEnd + 1 : newLineEnd
  return { text: next, caret: caretAfter }
}

export type LrcSegment = { kind: 'time' | 'meta' | 'text'; text: string }

const META_RE = /^\[[a-z#]+:[^\]]*\]\s*$/i

/** Split one line into highlight segments (timestamps, `[ar:…]` metadata, text). */
export function highlightLine(line: string): LrcSegment[] {
  if (META_RE.test(line)) return [{ kind: 'meta', text: line }]
  const segments: LrcSegment[] = []
  let rest = line
  for (;;) {
    const m = TIMESTAMP_RE.exec(rest)
    if (!m || m.index !== 0) break
    segments.push({ kind: 'time', text: m[0] })
    rest = rest.slice(m[0].length)
  }
  if (rest) segments.push({ kind: 'text', text: rest })
  return segments
}
