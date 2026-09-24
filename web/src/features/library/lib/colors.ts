/** Deterministic, pleasant colours for items without artwork (genre tiles, radio stations). */

/** FNV-1a 32-bit hash of a string. */
export function hashString(value: string): number {
  let hash = 0x811c9dc5
  for (let i = 0; i < value.length; i++) {
    hash ^= value.charCodeAt(i)
    hash = Math.imul(hash, 0x01000193)
  }
  return hash >>> 0
}

/**
 * Hand-picked two-stop gradients (Apple Music category tiles): saturated, never muddy, and dark
 * enough at the top-left for white bold text in light and dark mode. Arbitrary hue pairs tend to
 * produce olive / brown blends, hence a curated list.
 */
const GRADIENTS: readonly (readonly [string, string])[] = [
  ['oklch(0.64 0.17 252)', 'oklch(0.5 0.2 272)'], // azure → indigo
  ['oklch(0.6 0.2 295)', 'oklch(0.48 0.21 280)'], // violet
  ['oklch(0.64 0.21 350)', 'oklch(0.52 0.21 12)'], // pink → raspberry
  ['oklch(0.68 0.18 38)', 'oklch(0.57 0.21 24)'], // coral → red
  ['oklch(0.72 0.16 62)', 'oklch(0.6 0.19 38)'], // amber → tangerine
  ['oklch(0.66 0.14 170)', 'oklch(0.52 0.12 200)'], // mint → teal
  ['oklch(0.64 0.16 148)', 'oklch(0.5 0.13 172)'], // leaf → emerald
  ['oklch(0.66 0.13 220)', 'oklch(0.5 0.16 252)'], // sky → blue
  ['oklch(0.6 0.22 325)', 'oklch(0.47 0.2 300)'], // magenta → purple
  ['oklch(0.58 0.09 255)', 'oklch(0.42 0.08 268)'], // slate
  ['oklch(0.66 0.19 18)', 'oklch(0.54 0.2 355)'], // salmon → rose
  ['oklch(0.64 0.12 195)', 'oklch(0.48 0.15 238)'], // cyan → ocean
]

/**
 * Two-stop gradient (`background-image` value) derived from `seed`, readable with white text in
 * light and dark mode.
 */
export function seedGradient(seed: string): string {
  const hash = hashString(seed.toLowerCase())
  const [from, to] = GRADIENTS[hash % GRADIENTS.length]
  const angle = 125 + ((hash >>> 8) % 60)
  return `linear-gradient(${angle}deg, ${from}, ${to})`
}
