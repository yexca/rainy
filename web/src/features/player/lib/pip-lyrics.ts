/**
 * Picture-in-picture lyrics helpers (docs/architecture/contract.md §9.4): which lines the small
 * window shows, and line wrapping for the canvas fallback. No imports, so `node --test` can run
 * them (web/tests/pip-lyrics.test.ts).
 */

/** A display line (same shape as `DisplayLine` in lib/lyrics/bilingual). */
export interface PipLine {
  start: number
  text: string
  translations: string[]
}

export interface PipLyricWindow {
  /** The line being sung; `null` before the first line. */
  current: PipLine | null
  /** The next line with text (instrumental gaps are skipped); `null` at the end. */
  next: PipLine | null
  /** Index of `current` (-1 before the first line), for animations keyed on the line. */
  index: number
}

/** The current and next line of synced lyrics for the active line index. */
export function pipLyricWindow(lines: readonly PipLine[], active: number): PipLyricWindow {
  const current = active >= 0 && active < lines.length ? lines[active] : null
  let next: PipLine | null = null
  for (let i = Math.max(active + 1, 0); i < lines.length; i++) {
    if (lines[i].text !== '') {
      next = lines[i]
      break
    }
  }
  return { current, next, index: current ? active : -1 }
}

const SPACE = /\s/
/** Characters a line may break after without a space (CJK, kana, Hangul, full-width punctuation). */
const BREAK_ANYWHERE = /[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}\p{Script=Hangul}\u3000-\u303f\uff00-\uffef]/u

/**
 * Wraps text to lines no wider than `maxWidth` (as measured by `measure`), breaking at spaces
 * and between CJK characters; a word longer than a line is broken anywhere. At most `maxLines`
 * lines; the last one ends with "…" when text was cut.
 */
export function wrapText(text: string, maxWidth: number, measure: (s: string) => number, maxLines = 2): string[] {
  const chars = [...text.trim()]
  const lines: string[] = []
  let line = ''
  /** Position in `line` after which it may break (exclusive end), -1 = none. */
  let breakAt = -1
  for (let i = 0; i < chars.length; i++) {
    const ch = chars[i]
    const candidate = line + ch
    if (line !== '' && !SPACE.test(ch) && measure(candidate) > maxWidth) {
      const cut = breakAt > 0 ? breakAt : [...line].length
      const lineChars = [...line]
      lines.push(lineChars.slice(0, cut).join('').trimEnd())
      line = lineChars.slice(cut).join('').trimStart() + ch
      breakAt = -1
      if (lines.length === maxLines) {
        return ellipsize(lines, chars.slice(i + 1).join('') !== '' || line !== '', maxWidth, measure)
      }
    } else {
      line = candidate
    }
    const n = [...line].length
    if (SPACE.test(ch)) breakAt = n
    else if (BREAK_ANYWHERE.test(ch)) breakAt = n
  }
  if (line.trim() !== '') lines.push(line.trim())
  return lines
}

function ellipsize(lines: string[], cut: boolean, maxWidth: number, measure: (s: string) => number): string[] {
  if (!cut) return lines
  const last = [...lines[lines.length - 1]]
  while (last.length > 0 && measure(last.join('') + '…') > maxWidth) last.pop()
  lines[lines.length - 1] = last.join('').trimEnd() + '…'
  return lines
}
