// Run with `pnpm test` (node --test).
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { describe, it } from 'node:test'

import {
  ACCENT_SWATCHES,
  CHROMES,
  DEFAULT_ACCENT,
  DEFAULT_STYLE,
  IMMERSIVE_SWATCH,
  STYLES,
  STYLE_CHROME,
  STYLE_IMMERSIVE,
  STYLE_PREVIEWS,
  accentFill,
  accentSwatch,
  immersiveForStyle,
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
    assert.equal(isAccent('immersive'), false)
    assert.equal(isAccent('cobalt'), false)
  })
})

describe('style presets', () => {
  it('gives the apple style the sidebar chrome and immersive mode', () => {
    assert.equal(STYLE_CHROME.apple, 'sidebar')
    assert.equal(STYLE_IMMERSIVE.apple, true)
    for (const style of ['anthropic', 'openai', 'google-md'] as const) {
      assert.equal(STYLE_CHROME[style], 'header')
      assert.equal(STYLE_IMMERSIVE[style], false)
    }
  })

  it('turns immersive mode on entering the apple style and off leaving it', () => {
    assert.equal(immersiveForStyle(false, 'anthropic', 'apple'), true)
    assert.equal(immersiveForStyle(true, 'apple', 'anthropic'), false)
    assert.equal(immersiveForStyle(false, 'anthropic', 'openai'), false)
  })

  it('keeps immersive mode as the user set it against the style', () => {
    assert.equal(immersiveForStyle(true, 'anthropic', 'openai'), true)
    assert.equal(immersiveForStyle(true, 'anthropic', 'apple'), true)
    assert.equal(immersiveForStyle(false, 'apple', 'anthropic'), false)
  })
})

describe('style colours', () => {
  it('uses the style primary for the original accent and the preset otherwise', () => {
    assert.equal(accentSwatch('original', 'openai'), STYLE_PREVIEWS.openai.primary)
    assert.equal(accentSwatch('rose', 'openai'), ACCENT_SWATCHES.rose)
  })

  it('shows the original accent as a colour wheel in immersive mode and leaves the presets solid', () => {
    assert.equal(accentFill('original', 'apple', true), IMMERSIVE_SWATCH)
    assert.equal(accentFill('original', 'apple', false), STYLE_PREVIEWS.apple.primary)
    assert.equal(accentFill('rose', 'apple', true), ACCENT_SWATCHES.rose)
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

  it('has a stylesheet rule for every chrome layout but the default', () => {
    const css = readFileSync(new URL('../src/index.css', import.meta.url), 'utf8')
    for (const chrome of CHROMES) {
      if (chrome === STYLE_CHROME[DEFAULT_STYLE]) continue
      assert.ok(css.includes(`:root[data-chrome="${chrome}"] {`), `${chrome} chrome rule`)
    }
  })
})
