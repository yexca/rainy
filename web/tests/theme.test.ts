// Run with `pnpm test` (node --test).
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { describe, it } from 'node:test'

import {
  ACCENTS,
  CHROMES,
  DEFAULT_ACCENT,
  DEFAULT_STYLE,
  IMMERSIVE_SWATCH,
  RETIRED_ACCENTS,
  STYLES,
  STYLE_ACCENTS,
  STYLE_CHROME,
  STYLE_IMMERSIVE,
  STYLE_QUICK_ACTIONS,
  STYLE_RAIL,
  accentSwatch,
  immersiveForStyle,
  isAccent,
  isStyle,
  parseAccent,
  resolveAccent,
  showsCoverWheel,
  styleAccents,
  themeColor,
} from '../src/lib/theme-styles.ts'

/** The style whose tokens and chrome layout are the base of index.css; the others override it. */
const CSS_BASE_STYLE = 'anthropic'

describe('style preference', () => {
  it('defaults to the apple style, in immersive mode, with its original accent', () => {
    assert.equal(DEFAULT_STYLE, 'apple')
    assert.equal(STYLE_IMMERSIVE[DEFAULT_STYLE], true)
    assert.equal(DEFAULT_ACCENT, 'original')
  })

  it('accepts only the four styles and the accent presets', () => {
    assert.deepEqual(STYLES, ['anthropic', 'openai', 'apple', 'google-md'])
    for (const value of ['claude', '', null, 'OpenAI']) assert.equal(isStyle(value), false)
    assert.equal(isAccent('original'), true)
    assert.equal(isAccent('blue'), true)
    assert.equal(isAccent('rain'), false)
    assert.equal(isAccent('immersive'), false)
    assert.equal(isAccent('cobalt'), false)
  })
})

describe('style presets', () => {
  it('gives every style its own layout', () => {
    const layout = (style: (typeof STYLES)[number]) => [STYLE_CHROME[style], STYLE_QUICK_ACTIONS[style], STYLE_RAIL[style]]
    assert.deepEqual(layout('anthropic'), ['header', 'icon', 'icons'])
    assert.deepEqual(layout('openai'), ['sidebar', 'field', 'icons'])
    assert.deepEqual(layout('apple'), ['sidebar', 'none', 'icons'])
    assert.deepEqual(layout('google-md'), ['header', 'field', 'labels'])
  })

  it('starts only the apple style in immersive mode', () => {
    for (const style of STYLES) assert.equal(STYLE_IMMERSIVE[style], style === 'apple')
  })

  it('turns immersive mode on entering the apple style and off leaving it', () => {
    assert.equal(immersiveForStyle(false, 'anthropic', 'apple'), true)
    assert.equal(immersiveForStyle(true, 'apple', 'anthropic'), false)
    assert.equal(immersiveForStyle(false, 'anthropic', 'openai'), false)
  })

  it('keeps immersive mode as the user set it against the style', () => {
    assert.equal(immersiveForStyle(true, 'anthropic', 'google-md'), true)
    assert.equal(immersiveForStyle(true, 'anthropic', 'apple'), true)
    assert.equal(immersiveForStyle(false, 'apple', 'anthropic'), false)
  })
})

/** index.css as `selector → declarations`, outside `@media` blocks or inside the one whose condition is `media`. */
function cssRules(css: string, media?: string): Map<string, Record<string, string>> {
  const blocks: string[] = []
  let outside = ''
  // Only @media blocks that start a line (`@custom-variant … (@media …)` is a one-line statement).
  const mediaAt = /^[ \t]*@media/gm
  for (let i = 0; i < css.length; ) {
    mediaAt.lastIndex = i
    const found = mediaAt.exec(css)
    const at = found ? found.index + found[0].length - 6 : -1
    if (at < 0) {
      outside += css.slice(i)
      break
    }
    outside += css.slice(i, at)
    const open = css.indexOf('{', at)
    let depth = 1
    let end = open + 1
    for (; depth > 0; end++) depth += css[end] === '{' ? 1 : css[end] === '}' ? -1 : 0
    if (css.slice(at + 6, open).trim() === `(${media})`) blocks.push(css.slice(open + 1, end - 1))
    i = end
  }
  const rules = new Map<string, Record<string, string>>()
  for (const [, selectors, body] of (media ? blocks.join('\n') : outside).replace(/\/\*[\s\S]*?\*\//g, '').matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    const decls = Object.fromEntries([...body.matchAll(/(--[\w-]+):\s*([^;]+);/g)].map(([, name, value]) => [name, value.trim()]))
    // Statements before a rule (`@import …;`) are not part of its selector.
    const list = selectors.slice(selectors.lastIndexOf(';') + 1).split(',')
    for (const selector of list.map((s) => s.trim())) rules.set(selector, { ...rules.get(selector), ...decls })
  }
  return rules
}

/** sRGB channels 0–1 of a `#rrggbb` or `hsl(h s% l%)` value. */
function rgb(color: string): [number, number, number] {
  const hex = /^#([\da-f]{6})$/i.exec(color)
  if (hex) return [0, 2, 4].map((i) => parseInt(hex[1].slice(i, i + 2), 16) / 255) as [number, number, number]
  const hsl = /^hsl\(([\d.]+) ([\d.]+)% ([\d.]+)%\)$/.exec(color)
  assert.ok(hsl, `parsable colour: ${color}`)
  const [h, s, l] = [Number(hsl[1]), Number(hsl[2]) / 100, Number(hsl[3]) / 100]
  const f = (n: number) => {
    const k = (n + h / 30) % 12
    return l - s * Math.min(l, 1 - l) * Math.max(-1, Math.min(k - 3, 9 - k, 1))
  }
  return [f(0), f(8), f(4)]
}

/** WCAG 2 contrast ratio. */
function contrast(a: string, b: string): number {
  const luminance = (color: string) => {
    const [r, g, bl] = rgb(color).map((c) => (c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4))
    return 0.2126 * r + 0.7152 * g + 0.0722 * bl
  }
  const [x, y] = [luminance(a), luminance(b)].sort((p, q) => q - p)
  return (x + 0.05) / (y + 0.05)
}

const styleSelector = (style: string, dark: boolean) =>
  style === CSS_BASE_STYLE ? (dark ? '.dark' : ':root') : `:root${dark ? '.dark' : ''}[data-style="${style}"]`

describe('accents', () => {
  it('reads retired accents as their hue and anything unknown as original', () => {
    assert.deepEqual(ACCENTS, ['original', 'blue', 'green', 'purple', 'pink', 'orange'])
    assert.equal(parseAccent('purple'), 'purple')
    assert.equal(parseAccent('rain'), 'blue')
    assert.equal(parseAccent('rose'), 'pink')
    assert.equal(parseAccent('violet'), 'purple')
    assert.equal(parseAccent('emerald'), 'green')
    assert.equal(parseAccent('amber'), 'orange')
    assert.equal(parseAccent('graphite'), 'original')
    for (const value of [null, '', 'cobalt', 'toString']) assert.equal(parseAccent(value), 'original')
  })

  it('offers per style the hues of its design source, original first', () => {
    assert.deepEqual(styleAccents('anthropic'), ['original', 'blue', 'green'])
    assert.deepEqual(styleAccents('openai'), ['original', 'blue', 'purple', 'orange'])
    assert.deepEqual(styleAccents('apple'), ['original', 'purple', 'pink', 'orange'])
    assert.deepEqual(styleAccents('google-md'), ['original', 'green', 'purple', 'orange'])
  })

  it('shows original for a hue the style does not offer', () => {
    assert.equal(resolveAccent('pink', 'apple'), 'pink')
    assert.equal(resolveAccent('pink', 'anthropic'), 'original')
    assert.equal(resolveAccent('blue', 'apple'), 'original')
    assert.equal(accentSwatch('pink', 'google-md'), STYLE_ACCENTS['google-md'].original)
  })

  it('draws the original accent as the cover wheel only in immersive mode', () => {
    assert.equal(showsCoverWheel('original', 'apple', true), true)
    assert.equal(showsCoverWheel('original', 'apple', false), false)
    assert.equal(showsCoverWheel('pink', 'apple', true), false)
    // A hue the style lacks shows original, so it follows the cover too.
    assert.equal(showsCoverWheel('green', 'apple', true), true)
    assert.match(IMMERSIVE_SWATCH, /^conic-gradient\(/)
  })

  it('gives every swatch the colours the stylesheet sets, light and dark', () => {
    const rules = cssRules(readFileSync(new URL('../src/index.css', import.meta.url), 'utf8'))
    for (const style of STYLES) {
      for (const accent of styleAccents(style)) {
        for (const dark of [false, true]) {
          const selector =
            accent === 'original' ? styleSelector(style, dark) : `:root${dark ? '.dark' : ''}[data-style="${style}"][data-accent="${accent}"]`
          const colors = STYLE_ACCENTS[style][accent]![dark ? 'dark' : 'light']
          const decls = rules.get(selector)
          assert.ok(decls, `${selector} rule`)
          assert.equal(decls['--primary']?.toLowerCase(), colors.primary, `${selector} --primary`)
          assert.equal(decls['--primary-foreground']?.toLowerCase(), colors.foreground, `${selector} --primary-foreground`)
          assert.ok(decls['--ring'], `${selector} --ring`)
        }
      }
    }
  })

  it('keeps every focus ring at 3:1 or more on the page', () => {
    const rules = cssRules(readFileSync(new URL('../src/index.css', import.meta.url), 'utf8'))
    for (const style of STYLES) {
      for (const dark of [false, true]) {
        const background = rules.get(styleSelector(style, dark))?.['--background']
        assert.ok(background, `${style} ${dark ? 'dark' : 'light'} --background`)
        for (const accent of styleAccents(style)) {
          const selector =
            accent === 'original' ? styleSelector(style, dark) : `:root${dark ? '.dark' : ''}[data-style="${style}"][data-accent="${accent}"]`
          const ring = rules.get(selector)?.['--ring']
          assert.ok(ring, `${selector} --ring`)
          const ratio = contrast(ring, background)
          assert.ok(ratio >= 3, `${selector} ring ${ring} on ${background}: ${ratio.toFixed(2)}`)
        }
      }
    }
  })

  it('gives the browser chrome the page colour and keeps text readable on every surface', () => {
    const rules = cssRules(readFileSync(new URL('../src/index.css', import.meta.url), 'utf8'))
    const hex = (color: string) => `#${rgb(color).map((c) => Math.round(c * 255).toString(16).padStart(2, '0')).join('')}`
    for (const style of STYLES) {
      for (const dark of [false, true]) {
        const name = `${style} ${dark ? 'dark' : 'light'}`
        const tokens = { ...rules.get(':root'), ...(dark ? rules.get('.dark') : {}), ...rules.get(styleSelector(style, dark)) }
        assert.equal(hex(tokens['--background']), themeColor(style, dark ? 'dark' : 'light'), `${name} theme-color`)
        for (const [text, surfaces, min] of [
          ['--foreground', ['--background', '--card', '--popover'], 7],
          ['--muted-foreground', ['--background', '--card', '--muted'], 4.5],
          ['--secondary-foreground', ['--secondary'], 4.5],
          ['--accent-foreground', ['--accent'], 4.5],
        ] as const) {
          for (const surface of surfaces) {
            const ratio = contrast(tokens[text], tokens[surface])
            assert.ok(ratio >= min, `${name} ${text} on ${surface}: ${ratio.toFixed(2)}`)
          }
        }
      }
    }
  })

  it('has accent rules, normal and high-contrast, only for the hues a style offers', () => {
    const css = readFileSync(new URL('../src/index.css', import.meta.url), 'utf8')
    for (const rules of [cssRules(css), cssRules(css, 'prefers-contrast: more')]) {
      for (const selector of rules.keys()) {
        const match = /\[data-style="([^"]+)"\]\[data-accent="([^"]+)"\]/.exec(selector)
        if (!match) continue
        const [, style, accent] = match
        assert.ok(isStyle(style) && styleAccents(style).includes(accent as never), `${selector} names an offered accent`)
      }
    }
    // Inside the high-contrast block a light rule must not reach dark mode.
    for (const selector of cssRules(css, 'prefers-contrast: more').keys()) {
      assert.match(selector, /^:root(:not\(\.dark\)|\.dark)\[data-style="[^"]+"\]\[data-accent="[^"]+"\]$/, selector)
    }
  })

  it('resolves the accent the same way in the pre-paint script', () => {
    const html = readFileSync(new URL('../index.html', import.meta.url), 'utf8')
    const object = (name: string) => {
      const match = new RegExp(`var ${name} = (\\{[\\s\\S]*?\\})\\n`).exec(html)
      assert.ok(match, `${name} in the pre-paint script`)
      return new Function(`return ${match[1]}`)() as unknown
    }
    assert.deepEqual(object('accents'), Object.fromEntries(STYLES.map((style) => [style, styleAccents(style).slice(1)])))
    assert.deepEqual(object('retired'), RETIRED_ACCENTS)
  })
})

describe('style colours', () => {
  it('matches the pre-paint script and the stylesheet', () => {
    const html = readFileSync(new URL('../index.html', import.meta.url), 'utf8')
    const css = readFileSync(new URL('../src/index.css', import.meta.url), 'utf8')
    assert.ok(html.includes(`style = '${DEFAULT_STYLE}'`), 'the pre-paint script falls back to the default style')
    assert.ok(html.includes(`html { background-color: ${themeColor(DEFAULT_STYLE, 'light')}; }`), 'default light background')
    assert.ok(html.includes(`html.dark { background-color: ${themeColor(DEFAULT_STYLE, 'dark')}; }`), 'default dark background')
    for (const style of STYLES) {
      const light = themeColor(style, 'light')
      const dark = themeColor(style, 'dark')
      assert.match(html, new RegExp(`'?${style}'?: \\['${light}', '${dark}'\\]`), `${style} in the pre-paint script`)
      if (style !== DEFAULT_STYLE) {
        assert.ok(html.includes(`html[data-style="${style}"] { background-color: ${light}; }`), `${style} light background`)
        assert.ok(html.includes(`html.dark[data-style="${style}"] { background-color: ${dark}; }`), `${style} dark background`)
      }
      if (style !== CSS_BASE_STYLE) {
        assert.ok(css.includes(`:root[data-style="${style}"] {`), `${style} light tokens`)
        assert.ok(css.includes(`:root.dark[data-style="${style}"] {`), `${style} dark tokens`)
      }
    }
  })

  it('has a stylesheet rule for every chrome layout but the base', () => {
    const css = readFileSync(new URL('../src/index.css', import.meta.url), 'utf8')
    for (const chrome of CHROMES) {
      if (chrome === STYLE_CHROME[CSS_BASE_STYLE]) continue
      assert.ok(css.includes(`:root[data-chrome="${chrome}"] {`), `${chrome} chrome rule`)
    }
  })
})
