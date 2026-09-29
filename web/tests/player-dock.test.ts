// Run with `pnpm test` (node --test).
import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import { compactScrubTarget, dockAttribute, sanitizeDockPrefs } from '../src/features/player/lib/dock.ts'

describe('sanitizeDockPrefs', () => {
  it('keeps valid layouts and falls back to the bottom bar otherwise', () => {
    assert.deepEqual(sanitizeDockPrefs({ mode: 'window', barAutoHide: true }), { mode: 'window', barAutoHide: true })
    assert.deepEqual(sanitizeDockPrefs({ mode: 'mini' as never, barAutoHide: 'yes' as never }), {
      mode: 'bar',
      barAutoHide: false,
    })
    assert.deepEqual(sanitizeDockPrefs(null), { mode: 'bar', barAutoHide: false })
  })
})

describe('dockAttribute', () => {
  it('reserves the bar unless it auto-hides, and nothing for an idle floating player', () => {
    assert.equal(dockAttribute({ mode: 'bar', barAutoHide: false }, false), 'bar')
    assert.equal(dockAttribute({ mode: 'bar', barAutoHide: true }, true), 'collapsed')
    assert.equal(dockAttribute({ mode: 'compact', barAutoHide: true }, true), 'compact')
    assert.equal(dockAttribute({ mode: 'window', barAutoHide: false }, true), 'window')
    assert.equal(dockAttribute({ mode: 'window', barAutoHide: false }, false), 'idle')
  })
})

describe('compactScrubTarget', () => {
  it('moves 20 % of the track per full-width drag', () => {
    // 300 s track → 60 s per 390 px.
    assert.equal(compactScrubTarget(100, 195, 390, 300), 130)
    assert.equal(compactScrubTarget(100, -390, 390, 300), 40)
  })

  it('spans at least 20 seconds and at most 10 minutes', () => {
    assert.equal(compactScrubTarget(10, 100, 100, 30), 30)
    assert.equal(compactScrubTarget(0, 100, 100, 7200), 600)
  })

  it('clamps to the track and ignores unknown durations', () => {
    assert.equal(compactScrubTarget(290, 390, 390, 300), 300)
    assert.equal(compactScrubTarget(5, -390, 390, 300), 0)
    assert.equal(compactScrubTarget(5, 50, 390, 0), 0)
  })
})
