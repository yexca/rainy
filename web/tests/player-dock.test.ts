// Run with `pnpm test` (node --test).
import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import { DOCK_MODES, dockAttribute, pointerRatio, sanitizeDockPrefs } from '../src/features/player/lib/dock.ts'

describe('DOCK_MODES', () => {
  it('lists the layouts in menu order, below full screen', () => {
    assert.deepEqual([...DOCK_MODES], ['window', 'bar'])
  })
})

describe('sanitizeDockPrefs', () => {
  it('keeps valid layouts and falls back to the bottom bar otherwise', () => {
    assert.deepEqual(sanitizeDockPrefs({ mode: 'window', barAutoHide: true }), { mode: 'window', barAutoHide: true })
    assert.deepEqual(sanitizeDockPrefs({ mode: 'mini' as never, barAutoHide: 'yes' as never }), {
      mode: 'bar',
      barAutoHide: false,
    })
    assert.deepEqual(sanitizeDockPrefs(null), { mode: 'bar', barAutoHide: false })
  })

  it('moves the retired floating mini bar to the bottom bar and keeps its pin', () => {
    assert.deepEqual(sanitizeDockPrefs({ mode: 'compact' as never, barAutoHide: true }), { mode: 'bar', barAutoHide: true })
  })
})

describe('dockAttribute', () => {
  it('reserves the bar unless it auto-hides, and nothing for an idle floating window', () => {
    assert.equal(dockAttribute({ mode: 'bar', barAutoHide: false }, false), 'bar')
    assert.equal(dockAttribute({ mode: 'bar', barAutoHide: true }, true), 'collapsed')
    assert.equal(dockAttribute({ mode: 'window', barAutoHide: false }, true), 'window')
    assert.equal(dockAttribute({ mode: 'window', barAutoHide: false }, false), 'idle')
  })
})

describe('pointerRatio', () => {
  it('maps the pointer to its position along the track', () => {
    assert.equal(pointerRatio(150, 100, 200), 0.25)
    assert.equal(pointerRatio(300, 100, 200), 1)
  })

  it('clamps outside the track and ignores an empty one', () => {
    assert.equal(pointerRatio(40, 100, 200), 0)
    assert.equal(pointerRatio(400, 100, 200), 1)
    assert.equal(pointerRatio(150, 100, 0), 0)
  })
})
