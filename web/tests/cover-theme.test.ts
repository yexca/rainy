// Run with `pnpm test` (node --test). Synthetic pixel buffers only.
import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import {
  TEXT_CONTRAST,
  contrast,
  coverTheme,
  extractCoverSeeds,
  rgbToOklch,
  type CoverSeeds,
  type Oklch,
} from '../src/features/library/lib/cover-theme.ts'

type Rgba = [number, number, number, number]

/** `size`×`size` RGBA buffer; `paint(x, y)` returns each pixel. */
function image(size: number, paint: (x: number, y: number) => Rgba): Uint8ClampedArray {
  const data = new Uint8ClampedArray(size * size * 4)
  for (let y = 0; y < size; y++) {
    for (let x = 0; x < size; x++) data.set(paint(x, y), (y * size + x) * 4)
  }
  return data
}

function hueDistance(a: number, b: number): number {
  const d = Math.abs(a - b) % 360
  return d > 180 ? 360 - d : d
}

const NAVY: Rgba = [20, 40, 110, 255]
const RED: Rgba = [230, 40, 40, 255]
const BLUE_HUE = rgbToOklch(20, 40, 110).h
const RED_HUE = rgbToOklch(230, 40, 40).h

/** Parse `#rrggbb` back to OKLCH. */
function fromHex(hex: string): Oklch {
  const n = Number.parseInt(hex.slice(1), 16)
  return rgbToOklch((n >> 16) & 255, (n >> 8) & 255, n & 255)
}

describe('extractCoverSeeds', () => {
  it('takes the dominant colour as background and a vivid minority colour as accent', () => {
    // Navy cover with a red square in the middle (~10%).
    const seeds = extractCoverSeeds(
      image(40, (x, y) => (x >= 14 && x < 26 && y >= 14 && y < 26 ? RED : NAVY)),
      40,
    )
    assert.ok(seeds)
    assert.ok(hueDistance(seeds.background.h, BLUE_HUE) < 10, `background hue ${seeds.background.h}`)
    assert.ok(seeds.accent)
    assert.ok(hueDistance(seeds.accent.h, RED_HUE) < 10, `accent hue ${seeds.accent.h}`)
  })

  it('ignores specks smaller than the accent threshold', () => {
    const seeds = extractCoverSeeds(image(40, (x, y) => (x === 20 && y === 20 ? RED : NAVY)), 40)
    assert.ok(seeds?.accent)
    assert.ok(hueDistance(seeds.accent.h, BLUE_HUE) < 10)
  })

  it('has no accent for a greyscale cover', () => {
    const seeds = extractCoverSeeds(image(32, (x) => (x < 16 ? [30, 30, 30, 255] : [200, 200, 200, 255])), 32)
    assert.ok(seeds)
    assert.equal(seeds.accent, null)
    assert.ok(seeds.background.c < 0.01)
  })

  it('weighs the edges double, so the page continues the artwork border', () => {
    // A 12-px frame of red around navy: navy has more pixels, the frame more weight.
    const seeds = extractCoverSeeds(image(48, (x, y) => (x < 6 || y < 6 || x >= 42 || y >= 42 ? RED : NAVY)), 48)
    assert.ok(seeds)
    assert.ok(hueDistance(seeds.background.h, RED_HUE) < 10)
  })

  it('ignores transparent pixels and returns null for empty input', () => {
    assert.equal(extractCoverSeeds(image(8, () => [255, 0, 0, 0]), 8), null)
    assert.equal(extractCoverSeeds(new Uint8ClampedArray(0), 0), null)
    const seeds = extractCoverSeeds(image(8, (x) => (x < 4 ? [255, 0, 0, 0] : NAVY)), 8)
    assert.ok(seeds && hueDistance(seeds.background.h, BLUE_HUE) < 10)
  })
})

describe('coverTheme', () => {
  const seeds = (background: Oklch, accent: Oklch | null = null): CoverSeeds => ({ background, accent })

  it('follows the app theme, but a dark cover makes a deep page in the light theme', () => {
    const light = { l: 0.85, c: 0.08, h: 90 }
    const dark = { l: 0.3, c: 0.1, h: 260 }
    assert.equal(coverTheme(seeds(light), false).dark, false)
    assert.equal(coverTheme(seeds(light), true).dark, true)
    assert.equal(coverTheme(seeds(dark), false).dark, true)
    assert.equal(coverTheme(seeds(dark), true).dark, true)
  })

  it('keeps the user accent for a greyscale cover', () => {
    const theme = coverTheme(seeds({ l: 0.5, c: 0, h: 0 }), false)
    assert.equal(theme.vars['--primary'], undefined)
    assert.equal(theme.themeColor, theme.vars['--background'])
  })

  it('keeps text and accent readable for any cover colour (WCAG AA)', () => {
    for (const siteDark of [false, true]) {
      for (let l = 0.05; l <= 1; l += 0.15) {
        for (let c = 0; c <= 0.3; c += 0.1) {
          for (let h = 0; h < 360; h += 30) {
            const color = { l, c, h }
            const theme = coverTheme(seeds(color, c > 0 ? color : null), siteDark)
            const bg = fromHex(theme.vars['--background'])
            const label = `l=${l.toFixed(2)} c=${c.toFixed(1)} h=${h} dark=${siteDark}`
            assert.ok(contrast(fromHex(theme.vars['--foreground']), bg) >= 7, `foreground ${label}`)
            // Hex rounding may cost a hair of contrast.
            assert.ok(contrast(fromHex(theme.vars['--muted-foreground']), bg) >= TEXT_CONTRAST - 0.1, `muted ${label}`)
            if (theme.vars['--primary']) {
              const primary = fromHex(theme.vars['--primary'])
              assert.ok(contrast(primary, bg) >= TEXT_CONTRAST - 0.1, `primary ${label}`)
              assert.ok(contrast(fromHex(theme.vars['--primary-foreground']), primary) >= 3, `on primary ${label}`)
            }
          }
        }
      }
    }
  })

  it('emits valid hex colours', () => {
    const theme = coverTheme(seeds({ l: 0.6, c: 0.35, h: 145 }, { l: 0.7, c: 0.4, h: 330 }), false)
    for (const key of ['--background', '--foreground', '--primary', '--muted-foreground', '--card', '--secondary']) {
      assert.match(theme.vars[key], /^#[0-9a-f]{6}$/, key)
    }
  })
})
