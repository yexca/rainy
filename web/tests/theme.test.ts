// Run with `pnpm test` (node --test).
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { describe, it } from 'node:test'

import {
  ACCENT_SWATCHES,
  DEFAULT_ACCENT,
  DEFAULT_STYLE,
  STYLES,
  STYLE_PREVIEWS,
  accentSwatch,
  isAccent,
  isStyle,
  themeColor,
} from '../src/lib/theme-styles.ts'

describe('style preference', () => {
  it('defaults to the anthropic style with its original accent', () => {
    assert.equal(DEFAULT_STYLE, 'anthropic')
    assert.equal(STYLES[0], 'anthropic')
    assert.equal(DEFAULT_ACCENT, 'original')
  })

  it('accepts only the four styles and the accent presets', () => {
    assert.deepEqual(STYLES, ['anthropic', 'openai', 'apple', 'google-md'])
    for (const value of ['claude', '', null, 'OpenAI']) assert.equal(isStyle(value), false)
    assert.equal(isAccent('original'), true)
    assert.equal(isAccent('rain'), true)
    assert.equal(isAccent('cobalt'), false)
  })
})

describe('style colours', () => {
  it('uses the style primary for the original accent and the preset otherwise', () => {
    assert.equal(accentSwatch('original', 'openai'), STYLE_PREVIEWS.openai.primary)
    assert.equal(accentSwatch('rose', 'openai'), ACCENT_SWATCHES.rose)
  })

  it('matches the pre-paint script and the stylesheet', () => {
    const html = readFileSync(new URL('../index.html', import.meta.url), 'utf8')
    const css = readFileSync(new URL('../src/index.css', import.meta.url), 'utf8')
    for (const style of STYLES) {
      const light = themeColor(style, 'light')
      const dark = themeColor(style, 'dark')
      assert.match(html, new RegExp(`'?${style}'?: \\['${light}', '${dark}'\\]`), `${style} in the pre-paint script`)
      if (style === DEFAULT_STYLE) continue
      assert.ok(html.includes(`html[data-style="${style}"] { background-color: ${light}; }`), `${style} light background`)
      assert.ok(html.includes(`html.dark[data-style="${style}"] { background-color: ${dark}; }`), `${style} dark background`)
      assert.ok(css.includes(`:root[data-style="${style}"] {`), `${style} light tokens`)
      assert.ok(css.includes(`:root.dark[data-style="${style}"] {`), `${style} dark tokens`)
    }
  })
})
