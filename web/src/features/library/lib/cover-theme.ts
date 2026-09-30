/**
 * Album colours (no imports, unit-tested in web/tests): pick a background and an accent colour
 * from a cover's pixels, then derive a scoped set of theme tokens (`--background`,
 * `--foreground`, `--primary`, …) so a detail page takes on the album's colours, like the
 * Apple Music album view. Colour math runs in OKLab/OKLCH; every token is gamut-mapped to sRGB
 * and text colours are checked against WCAG contrast.
 */

/** OKLCH colour: lightness 0–1, chroma ≥ 0, hue in degrees. */
export interface Oklch {
  l: number
  c: number
  h: number
}

/** The two colours a cover contributes. */
export interface CoverSeeds {
  /** The cover's dominant colour (edges weigh double, so the page continues the artwork). */
  background: Oklch
  /** The most vivid well-represented colour, or `null` for a greyscale cover. */
  accent: Oklch | null
}

export interface CoverTheme {
  /** The page renders dark (adds `.dark` so `dark:` variants follow). */
  dark: boolean
  /** Custom properties to set on the page element. */
  vars: Record<string, string>
  /** Page background as hex, for `<meta name="theme-color">`. */
  themeColor: string
}

type Rgb = [number, number, number]

function clamp(n: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, n))
}

function toLinear(c: number): number {
  return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
}

function fromLinear(c: number): number {
  return c <= 0.0031308 ? 12.92 * c : 1.055 * c ** (1 / 2.4) - 0.055
}

/** sRGB channels 0–255 → OKLab. */
function rgbToOklab(r: number, g: number, b: number): [number, number, number] {
  const lr = toLinear(r / 255)
  const lg = toLinear(g / 255)
  const lb = toLinear(b / 255)
  const l = Math.cbrt(0.4122214708 * lr + 0.5363325363 * lg + 0.0514459929 * lb)
  const m = Math.cbrt(0.2119034982 * lr + 0.6806995451 * lg + 0.1073969566 * lb)
  const s = Math.cbrt(0.0883024619 * lr + 0.2817188376 * lg + 0.6299787005 * lb)
  return [
    0.2104542553 * l + 0.793617785 * m - 0.0040720468 * s,
    1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s,
    0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s,
  ]
}

/** OKLab → linear sRGB (may fall outside 0–1). */
function oklabToLinear(L: number, a: number, b: number): Rgb {
  const l = (L + 0.3963377774 * a + 0.2158037573 * b) ** 3
  const m = (L - 0.1055613458 * a - 0.0638541728 * b) ** 3
  const s = (L - 0.0894841775 * a - 1.291485548 * b) ** 3
  return [
    4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s,
    -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s,
    -0.0041960863 * l - 0.7034186147 * m + 1.707612701 * s,
  ]
}

function labToLch(L: number, a: number, b: number): Oklch {
  const h = (Math.atan2(b, a) * 180) / Math.PI
  return { l: L, c: Math.hypot(a, b), h: h < 0 ? h + 360 : h }
}

export function rgbToOklch(r: number, g: number, b: number): Oklch {
  const [L, a, bb] = rgbToOklab(r, g, b)
  return labToLch(L, a, bb)
}

function lchToLinear({ l, c, h }: Oklch): Rgb {
  const rad = (h * Math.PI) / 180
  return oklabToLinear(l, c * Math.cos(rad), c * Math.sin(rad))
}

function inGamut(rgb: Rgb): boolean {
  return rgb.every((v) => v >= -1e-4 && v <= 1 + 1e-4)
}

/** Linear sRGB of `color`, reducing chroma (keeping lightness and hue) until it fits sRGB. */
function toGamutLinear(color: Oklch): Rgb {
  const l = clamp(color.l, 0, 1)
  let rgb = lchToLinear({ ...color, l })
  if (inGamut(rgb)) return rgb
  let lo = 0
  let hi = color.c
  for (let i = 0; i < 20; i++) {
    const mid = (lo + hi) / 2
    const candidate = lchToLinear({ l, c: mid, h: color.h })
    if (inGamut(candidate)) {
      lo = mid
      rgb = candidate
    } else {
      hi = mid
    }
  }
  return lo === 0 ? lchToLinear({ l, c: 0, h: color.h }) : rgb
}

/** Relative luminance (WCAG) of an OKLCH colour after gamut mapping. */
export function luminance(color: Oklch): number {
  const [r, g, b] = toGamutLinear(color).map((v) => clamp(v, 0, 1))
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

/** WCAG contrast ratio of two colours (1–21). */
export function contrast(a: Oklch, b: Oklch): number {
  const la = luminance(a)
  const lb = luminance(b)
  return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05)
}

/** `#rrggbb` of an OKLCH colour (gamut-mapped). */
export function toHex(color: Oklch): string {
  return `#${toGamutLinear(color)
    .map((v) => Math.round(clamp(fromLinear(clamp(v, 0, 1)), 0, 1) * 255).toString(16).padStart(2, '0'))
    .join('')}`
}

// ------------------------------------------------------------------------------------------ //
// Extraction                                                                                 //
// ------------------------------------------------------------------------------------------ //

interface Cluster {
  weight: number
  L: number
  a: number
  b: number
}

/** Colours closer than this (OKLab distance) count as one. */
const MERGE_DISTANCE = 0.09
/** An accent must cover at least this share of the cover. */
const MIN_ACCENT_SHARE = 0.02
/** Below this chroma a colour is grey. */
const MIN_ACCENT_CHROMA = 0.05

/**
 * Pick the background and accent colours of an RGBA pixel buffer (`ImageData.data` of a small,
 * `width`-wide rendering of the cover). Transparent pixels are ignored; returns `null` when
 * nothing is left.
 */
export function extractCoverSeeds(data: ArrayLike<number>, width: number): CoverSeeds | null {
  const pixels = Math.floor(data.length / 4)
  const height = width > 0 ? Math.floor(pixels / width) : 0
  if (width <= 0 || height <= 0) return null
  const edge = Math.max(1, Math.round(Math.min(width, height) / 8))

  // 4-bit-per-channel histogram with running sums for the bucket's mean colour.
  const buckets = new Map<number, { weight: number; r: number; g: number; b: number }>()
  let total = 0
  for (let y = 0; y < height; y++) {
    for (let x = 0; x < width; x++) {
      const i = (y * width + x) * 4
      if (data[i + 3] < 128) continue
      const r = data[i]
      const g = data[i + 1]
      const b = data[i + 2]
      const onEdge = x < edge || y < edge || x >= width - edge || y >= height - edge
      const w = onEdge ? 2 : 1
      const key = ((r >> 4) << 8) | ((g >> 4) << 4) | (b >> 4)
      const bucket = buckets.get(key)
      if (bucket) {
        bucket.weight += w
        bucket.r += r * w
        bucket.g += g * w
        bucket.b += b * w
      } else {
        buckets.set(key, { weight: w, r: r * w, g: g * w, b: b * w })
      }
      total += w
    }
  }
  if (total === 0) return null

  // Greedily merge buckets (heaviest first) into perceptual clusters.
  const clusters: Cluster[] = []
  const sorted = [...buckets.values()].sort((x, y) => y.weight - x.weight)
  for (const bucket of sorted) {
    const [L, a, b] = rgbToOklab(bucket.r / bucket.weight, bucket.g / bucket.weight, bucket.b / bucket.weight)
    const near = clusters.find((c) => Math.hypot(c.L - L, c.a - a, c.b - b) < MERGE_DISTANCE)
    if (near) {
      const w = near.weight + bucket.weight
      near.L = (near.L * near.weight + L * bucket.weight) / w
      near.a = (near.a * near.weight + a * bucket.weight) / w
      near.b = (near.b * near.weight + b * bucket.weight) / w
      near.weight = w
    } else {
      clusters.push({ weight: bucket.weight, L, a, b })
    }
  }
  clusters.sort((x, y) => y.weight - x.weight)

  const base = clusters[0]
  const background = labToLch(base.L, base.a, base.b)

  // Vivid beats big (chroma² · √share), and a colour that stands out from the page background
  // beats the background itself; specks are ignored.
  let accent: Oklch | null = null
  let best = 0
  for (const cluster of clusters) {
    const share = cluster.weight / total
    if (share < MIN_ACCENT_SHARE) continue
    const lch = labToLch(cluster.L, cluster.a, cluster.b)
    if (lch.c < MIN_ACCENT_CHROMA) continue
    const distance = Math.hypot(cluster.L - base.L, cluster.a - base.a, cluster.b - base.b)
    const score = lch.c * lch.c * Math.sqrt(share) * (1 + 4 * distance)
    if (score > best) {
      best = score
      accent = lch
    }
  }
  return { background, accent }
}

// ------------------------------------------------------------------------------------------ //
// Theme tokens                                                                               //
// ------------------------------------------------------------------------------------------ //

/** Contrast every text token keeps against the page background (WCAG AA). */
export const TEXT_CONTRAST = 4.5

/** Step `color`'s lightness towards `direction` until it reaches `ratio` against `bg`. */
function withContrast(color: Oklch, bg: Oklch, ratio: number, direction: 1 | -1): Oklch {
  let current = color
  for (let i = 0; i < 60 && contrast(current, bg) < ratio; i++) {
    const l = current.l + direction * 0.01
    if (l < 0 || l > 1) break
    current = { ...current, l }
  }
  return current
}

/** Lightness below which a light-themed viewer still gets a dark album page. */
const DARK_COVER_LIGHTNESS = 0.6

/**
 * Theme tokens for a page coloured by `seeds`. `siteDark` is the resolved app theme: a dark
 * app always gets a deep version of the cover colour; a light app gets a pale tint, or a deep
 * page when the cover itself is dark. The accent becomes `--primary` (kept readable against the
 * page); a greyscale cover keeps the user's accent.
 */
export function coverTheme(seeds: CoverSeeds, siteDark: boolean): CoverTheme {
  const seed = seeds.background
  const hue = seed.h
  const dark = siteDark || seed.l < DARK_COVER_LIGHTNESS

  // Deep page: darker covers keep their depth; light covers (only in the dark app theme) fall
  // back towards the app's own dark background instead of a washed-out grey.
  const deep = seed.l <= 0.45 ? seed.l * 0.62 : 0.28 - (seed.l - 0.45) * 0.2
  const bg: Oklch = dark
    ? { l: clamp(deep, 0.18, 0.28), c: Math.min(seed.c * 0.85, 0.1), h: hue }
    : { l: clamp(seed.l + 0.25, 0.92, 0.965), c: Math.min(seed.c * 0.35, 0.045), h: hue }

  const fg: Oklch = dark ? { l: 0.98, c: Math.min(bg.c, 0.015), h: hue } : { l: 0.2, c: Math.min(bg.c, 0.02), h: hue }
  const muted = withContrast(
    dark ? { l: 0.8, c: Math.min(bg.c * 0.5, 0.03), h: hue } : { l: 0.48, c: Math.min(bg.c, 0.04), h: hue },
    bg,
    TEXT_CONTRAST,
    dark ? 1 : -1,
  )
  const card: Oklch = dark ? { ...bg, l: bg.l + 0.035 } : { l: Math.min(bg.l + 0.025, 0.99), c: bg.c * 0.6, h: hue }
  const fill: Oklch = dark ? { l: bg.l + 0.075, c: bg.c * 0.9, h: hue } : { l: bg.l - 0.045, c: bg.c * 1.2, h: hue }

  const vars: Record<string, string> = {
    '--background': toHex(bg),
    '--foreground': toHex(fg),
    '--card': toHex(card),
    '--card-foreground': toHex(fg),
    '--secondary': toHex(fill),
    '--secondary-foreground': toHex(fg),
    '--muted': toHex(fill),
    '--muted-foreground': toHex(muted),
    '--accent': toHex(fill),
    '--accent-foreground': toHex(fg),
    '--border': dark ? 'rgb(255 255 255 / 12%)' : 'rgb(0 0 0 / 10%)',
    '--input': dark ? 'rgb(255 255 255 / 16%)' : 'rgb(0 0 0 / 12%)',
  }

  const accentSeed = seeds.accent
  if (accentSeed) {
    const start: Oklch = dark
      ? { l: Math.max(accentSeed.l, 0.72), c: clamp(accentSeed.c, 0.08, 0.17), h: accentSeed.h }
      : { l: Math.min(accentSeed.l, 0.56), c: clamp(accentSeed.c, 0.08, 0.19), h: accentSeed.h }
    const primary = withContrast(start, bg, TEXT_CONTRAST, dark ? 1 : -1)
    const white: Oklch = { l: 1, c: 0, h: 0 }
    const ink: Oklch = { l: 0.2, c: 0.02, h: accentSeed.h }
    const onPrimary = contrast(white, primary) >= contrast(ink, primary) ? white : ink
    vars['--primary'] = toHex(primary)
    vars['--primary-foreground'] = toHex(onPrimary)
    vars['--ring'] = toHex(primary)
  }

  return { dark, vars, themeColor: toHex(bg) }
}
