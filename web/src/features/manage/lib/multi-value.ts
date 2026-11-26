/**
 * Value list edits of the tag editor's multi-value fields (`MultiValueInput`): artists and
 * genres as chips. The order of the values is kept, since the first artist is the main one.
 */
// No imports, so node --test (tests/multi-value.test.ts) can load it.

/** Typed text as values: `;` (and `；`) separate them; with `splitComma` also `,` and `，`. */
export function parseValues(raw: string, splitComma: boolean): string[] {
  const normalized = (splitComma ? raw.replace(/[,，]/g, ';') : raw).replace(/；/g, ';')
  return normalized
    .split(';')
    .map((v) => v.trim())
    .filter(Boolean)
}

/**
 * `values` with `added` inserted at `at` (the end when `null`). Values already present
 * (case-insensitive) are not added twice.
 */
export function insertValues(values: readonly string[], added: readonly string[], at: number | null): string[] {
  const seen = new Set(values.map((v) => v.toLowerCase()))
  const fresh: string[] = []
  for (const v of added) {
    const key = v.toLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    fresh.push(v)
  }
  const pos = at === null ? values.length : Math.min(Math.max(at, 0), values.length)
  return [...values.slice(0, pos), ...fresh, ...values.slice(pos)]
}

export interface ValueEdit {
  /** The values without the one being edited. */
  values: string[]
  /** The value being edited, for the text field. */
  text: string
  /** Where the edited text goes back when it is committed. */
  at: number
}

/**
 * Takes `values[index]` out for editing. Text still being typed (`pending`, which belongs at
 * `pendingAt`) becomes a value first, so it is not lost.
 */
export function startEdit(values: readonly string[], index: number, pending: readonly string[], pendingAt: number | null): ValueEdit {
  const pos = pendingAt === null ? values.length : Math.min(Math.max(pendingAt, 0), values.length)
  const merged = insertValues(values, pending, pos)
  const target = index + (pos <= index ? merged.length - values.length : 0)
  return { values: merged.filter((_, i) => i !== target), text: merged[target] ?? '', at: target }
}
