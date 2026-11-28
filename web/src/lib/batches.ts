/** Pure helper without imports so it can be unit-tested (web/tests/batches.test.ts). */

/** Splits items into consecutive batches of at most `size` (≥ 1) items; none for no items. */
export function batches<T>(items: readonly T[], size: number): T[][] {
  const step = Math.max(1, Math.floor(size))
  const out: T[][] = []
  for (let i = 0; i < items.length; i += step) out.push(items.slice(i, i + step))
  return out
}
