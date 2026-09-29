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

/** The leading timestamps of a line, whitespace removed (`''` when untimed). */
function leadingStamps(line: string): string {
  return (LEADING_STAMPS_RE.exec(line)?.[0] ?? '').replace(/\s+/g, '')
}

/**
 * Stamp the line containing `caret` with `seconds`: replaces existing leading timestamps, then
 * moves the caret to the start of the next line so repeated presses sync line by line. Following
 * lines that shared the old timestamps (a bilingual translation paired with the line) get the new
 * time too, and the caret skips past them.
 */
export function stampLine(text: string, caret: number, seconds: number): InsertResult {
  const lineStart = text.lastIndexOf('\n', Math.max(0, caret - 1)) + 1
  let lineEnd = text.indexOf('\n', lineStart)
  if (lineEnd < 0) lineEnd = text.length
  const line = text.slice(lineStart, lineEnd)
  const stamp = formatLrcTimestamp(seconds)
  const restamp = (l: string) => `${stamp}${l.replace(LEADING_STAMPS_RE, '').replace(/^\s+/, '')}`
  let stamped = restamp(line)
  const old = leadingStamps(line)
  if (old) {
    while (lineEnd < text.length) {
      const nextStart = lineEnd + 1
      let nextEnd = text.indexOf('\n', nextStart)
      if (nextEnd < 0) nextEnd = text.length
      const next = text.slice(nextStart, nextEnd)
      if (leadingStamps(next) !== old) break
      stamped += `\n${restamp(next)}`
      lineEnd = nextEnd
    }
  }
  const result = text.slice(0, lineStart) + stamped + text.slice(lineEnd)
  const newLineEnd = lineStart + stamped.length
  const caretAfter = newLineEnd < result.length ? newLineEnd + 1 : newLineEnd
  return { text: result, caret: caretAfter }
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
