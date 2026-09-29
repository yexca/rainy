/**
 * Bilingual (original + Chinese translation) lyrics helpers, shared by the lyrics editor and the
 * player (docs/architecture/contract.md §9.5).
 *
 * Two layouts are recognised:
 *
 * - **Paired lines** (the stored form): an original line and its translation carry the same LRC
 *   timestamp, original first. The server keeps both lines (and gives untimed lines after a timed
 *   one that line's time), so consecutive lines with an equal start form one bilingual line.
 *   Subsonic clients read the same file as two lines at one time.
 * - **Space separated** (common in Chinese lyric downloads): one line holds the original, a
 *   space, then the Chinese translation — `君の名前を 你的名字`. `splitBilingualLine` finds the
 *   split, `analyzeBilingual` decides whether a whole text uses this layout, and
 *   `toPairedLrc` rewrites timed lines into paired lines.
 *
 * Detection is deliberately conservative: a line only splits when a non-Chinese part (kana,
 * Hangul, Latin or another alphabet) is followed by a purely Chinese part, and a text only
 * counts as bilingual when most of its non-Chinese lines split that way.
 *
 * This module has no imports so `node --test` can run its tests directly (web/tests).
 */

const HAN = /\p{Script=Han}/u
const KANA = /[\p{Script=Hiragana}\p{Script=Katakana}ー]/u
const HANGUL = /\p{Script=Hangul}/u
/** Letters that are not Han, kana or Hangul (Latin, Cyrillic, Greek, …). */
const OTHER_LETTER = /(?![\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}\p{Script=Hangul}])\p{L}/u

/** Leading LRC time tags: `[mm:ss]`, `[mm:ss.xx]`, `[mm:ss:xx]`, `[mm:ss.xxx]` (several allowed). */
const LEADING_STAMPS = /^(?:\s*\[\s*\d{1,4}:\d{1,2}(?:[.:]\d{1,3})?\s*\])+/
/** An LRC ID tag line such as `[ar: Artist]` or `[offset:+250]`. */
const META_LINE = /^\s*\[\s*[A-Za-z#]+\s*:[^\]]*\]\s*$/
/** Minimum number of split lines before a text counts as bilingual. */
const MIN_SPLIT_LINES = 2
/** Share of the non-Chinese lines that must split. */
const MIN_SPLIT_RATIO = 0.6

const HAN_G = /\p{Script=Han}/gu
const OTHER_LETTER_G = /(?![\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}\p{Script=Hangul}])\p{L}/gu
/** Opening brackets that often wrap a translation written without a space (`once「…」`). */
const TRANSLATION_OPEN = /[\p{Script=Han}「『（【《〈]/u

type WordKind = 'zh' | 'neutral' | 'foreign'

function tokenKind(token: string): WordKind {
  if (KANA.test(token) || HANGUL.test(token)) return 'foreign'
  const { han, other } = letterCounts(token)
  // Chinese with a few Latin letters ("T恤", "卡拉OK") is still Chinese.
  if (han > 0 && other < han) return 'zh'
  if (other > 0) return 'foreign'
  return 'neutral'
}

/**
 * Whether text can be the original of a Chinese translation: it has kana or Hangul, or it has no
 * Han characters at all. Chinese mixed with English ("我爱你 baby") is not an original.
 */
function canBeOriginal(text: string): boolean {
  return KANA.test(text) || HANGUL.test(text) || (!HAN.test(text) && OTHER_LETTER.test(text))
}

/** Whether every token of text is Chinese or punctuation, with at least one Chinese token. */
function isChineseRun(tokens: readonly string[]): boolean {
  return tokens.every((t) => tokenKind(t) !== 'foreign') && tokens.some((t) => tokenKind(t) === 'zh')
}

/** Counts of Han characters and of letters from other alphabets (not kana or Hangul). */
function letterCounts(text: string): { han: number; other: number } {
  return { han: text.match(HAN_G)?.length ?? 0, other: text.match(OTHER_LETTER_G)?.length ?? 0 }
}

/**
 * Whether a line has non-Chinese text (it could be an original line): kana, Hangul, or more
 * letters of other alphabets than Han characters.
 */
export function hasForeignText(text: string): boolean {
  if (KANA.test(text) || HANGUL.test(text)) return true
  const { han, other } = letterCounts(text)
  return other > 0 && other >= han
}

/**
 * A BCP 47 language hint for a lyric line, so the browser picks matching CJK glyphs:
 * kana → `ja`, Hangul → `ko`, mostly Han → `zh`; `undefined` otherwise.
 */
export function guessLang(text: string): 'ja' | 'ko' | 'zh' | undefined {
  if (KANA.test(text)) return 'ja'
  if (HANGUL.test(text)) return 'ko'
  const { han, other } = letterCounts(text)
  if (han > 0 && other < han) return 'zh'
  return undefined
}

export interface BilingualSplit {
  original: string
  translation: string
}

/**
 * Split `original 中文翻译` at the last whitespace after which only Chinese (and punctuation or
 * digits) follows. Returns `null` unless the part before can be an original (kana, Hangul, or
 * letters without Han) and the part after contains Han characters. A Latin or Hangul original may
 * also be followed directly by the translation (`once「…」`). Text must not carry LRC timestamps.
 */
export function splitBilingualLine(text: string): BilingualSplit | null {
  const tokens = text.trim().split(/\s+/u).filter(Boolean)
  let start = tokens.length
  while (start > 0 && tokenKind(tokens[start - 1]) !== 'foreign') start--
  // Leading punctuation (`♪`, `—`) of the Chinese run stays with the original.
  while (start < tokens.length && tokenKind(tokens[start]) === 'neutral') start++
  if (start > 0 && start < tokens.length) {
    const original = tokens.slice(0, start).join(' ')
    const after = tokens.slice(start)
    // A translation never starts with a Latin letter ("once「…」" is the end of the original).
    const gluedToOriginal = OTHER_LETTER.test(after[0].charAt(0))
    if (!gluedToOriginal && canBeOriginal(original) && isChineseRun(after)) {
      return { original, translation: after.join(' ') }
    }
  }
  // No space: only for originals without Han or kana, where the script change is unambiguous.
  const at = text.search(TRANSLATION_OPEN)
  if (at <= 0) return null
  const original = text.slice(0, at).trim()
  const translation = text.slice(at).trim()
  if (KANA.test(original) || HAN.test(original) || !canBeOriginal(original)) return null
  if (!isChineseRun(translation.split(/\s+/u))) return null
  return { original, translation }
}

export interface LrcLineParts {
  /** The leading time tags exactly as written (`''` for untimed lines). */
  stamps: string
  /** The text after the time tags, trimmed. */
  body: string
  /** An `[ar:…]`-style ID tag line. */
  meta: boolean
}

export function lrcLineParts(line: string): LrcLineParts {
  if (META_LINE.test(line)) return { stamps: '', body: line.trim(), meta: true }
  const m = LEADING_STAMPS.exec(line)
  const stamps = m ? m[0].trim() : ''
  return { stamps, body: (m ? line.slice(m[0].length) : line).trim(), meta: false }
}

export interface BilingualLine extends BilingualSplit {
  /** 0-based line number in the analysed text. */
  line: number
  timed: boolean
}

export interface BilingualAnalysis {
  /** The text looks like space-separated bilingual lyrics. */
  detected: boolean
  /** Lines that split into original and translation. */
  lines: BilingualLine[]
  /** Lines with non-Chinese letters (candidates for an original line). */
  candidates: number
  /** How many of `lines` carry timestamps (the ones `toPairedLrc` rewrites). */
  timed: number
  /** Timestamps shared by two or more text lines (already paired). */
  paired: number
}

/** Analyse raw lyrics text (LRC or plain) for space-separated bilingual lines. */
export function analyzeBilingual(text: string): BilingualAnalysis {
  const lines: BilingualLine[] = []
  let candidates = 0
  const stampCounts = new Map<string, number>()
  text.split(/\r\n|\r|\n/).forEach((raw, index) => {
    const { stamps, body, meta } = lrcLineParts(raw)
    if (meta || !body) return
    if (stamps) {
      for (const stamp of stamps.match(/\[[^\]]*\]/g) ?? []) {
        const key = stamp.replace(/\s+/g, '')
        stampCounts.set(key, (stampCounts.get(key) ?? 0) + 1)
      }
    }
    if (!hasForeignText(body)) return
    candidates++
    const split = splitBilingualLine(body)
    if (split) lines.push({ ...split, line: index, timed: stamps !== '' })
  })
  let paired = 0
  for (const count of stampCounts.values()) if (count > 1) paired++
  const detected = lines.length >= MIN_SPLIT_LINES && lines.length / candidates >= MIN_SPLIT_RATIO
  return { detected, lines, candidates, timed: lines.filter((l) => l.timed).length, paired }
}

/**
 * Rewrite every timed space-separated bilingual line as two lines with the same timestamps:
 * the original, then the translation. Untimed lines, metadata and lines that do not split are
 * left exactly as they are.
 */
export function toPairedLrc(text: string, analysis: BilingualAnalysis = analyzeBilingual(text)): string {
  const byLine = new Map(analysis.lines.filter((l) => l.timed).map((l) => [l.line, l]))
  if (byLine.size === 0) return text
  const newline = text.includes('\r\n') ? '\r\n' : '\n'
  return text
    .split(/\r\n|\r|\n/)
    .map((raw, index) => {
      const split = byLine.get(index)
      if (!split) return raw
      const { stamps } = lrcLineParts(raw)
      return `${stamps}${split.original}${newline}${stamps}${split.translation}`
    })
    .join(newline)
}

export interface DisplayLine {
  /** Start in ms (-1 for plain lyrics). */
  start: number
  text: string
  /** Translations shown under the line (usually one). */
  translations: string[]
}

/**
 * Group lyric lines for display: consecutive lines with the same start become one line with
 * translations (synced lyrics only). When the text uses the space-separated layout, lines
 * without a paired translation are split as well.
 */
export function groupBilingual(lines: readonly { start: number; text: string }[], synced: boolean): DisplayLine[] {
  const out: DisplayLine[] = []
  for (const line of lines) {
    const text = line.text.trim()
    const prev = out.at(-1)
    if (synced && prev && prev.start === line.start && prev.text !== '') {
      if (text && text !== prev.text && !prev.translations.includes(text)) prev.translations.push(text)
      continue
    }
    out.push({ start: line.start, text, translations: [] })
  }
  const candidates = out.filter((l) => l.translations.length === 0 && hasForeignText(l.text))
  const splits = candidates.map((l) => splitBilingualLine(l.text))
  const splitCount = splits.filter(Boolean).length
  if (splitCount >= MIN_SPLIT_LINES && splitCount / candidates.length >= MIN_SPLIT_RATIO) {
    candidates.forEach((l, i) => {
      const split = splits[i]
      if (!split) return
      l.text = split.original
      l.translations = [split.translation]
    })
  }
  return out
}
