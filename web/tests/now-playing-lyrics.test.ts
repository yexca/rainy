// Run with `pnpm test` (node --test).
import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import {
  DEFAULT_NOW_PLAYING_LYRICS,
  openingPanel,
  rememberLyrics,
  sanitizeNowPlayingLyrics,
} from '../src/features/player/lib/now-playing-lyrics.ts'

describe('sanitizeNowPlayingLyrics', () => {
  it('opens on the artwork on phones and on the lyrics on larger screens by default', () => {
    assert.deepEqual(DEFAULT_NOW_PLAYING_LYRICS, { sheet: false, stage: true })
    assert.deepEqual(sanitizeNowPlayingLyrics(undefined), DEFAULT_NOW_PLAYING_LYRICS)
  })

  it('keeps stored choices and replaces invalid ones with the default', () => {
    assert.deepEqual(sanitizeNowPlayingLyrics({ sheet: true, stage: false }), { sheet: true, stage: false })
    assert.deepEqual(sanitizeNowPlayingLyrics({ sheet: 'yes', stage: false }), { sheet: false, stage: false })
  })
})

describe('openingPanel', () => {
  it('opens on the lyrics when the surface remembers them', () => {
    assert.equal(openingPanel('none', true, false), 'lyrics')
    assert.equal(openingPanel('queue', true, false), 'lyrics')
  })

  it('leaves the lyrics closed when the surface does not show them', () => {
    assert.equal(openingPanel('lyrics', false, false), 'none')
    assert.equal(openingPanel('none', false, false), 'none')
    assert.equal(openingPanel('queue', false, false), 'queue')
  })

  it('keeps the panel for radio', () => {
    assert.equal(openingPanel('none', true, true), 'none')
    assert.equal(openingPanel('queue', true, true), 'queue')
  })
})

describe('rememberLyrics', () => {
  it('records closing and opening the lyrics for that surface only', () => {
    const closed = rememberLyrics(DEFAULT_NOW_PLAYING_LYRICS, 'stage', 'none')
    assert.deepEqual(closed, { sheet: false, stage: false })
    assert.deepEqual(rememberLyrics(closed, 'sheet', 'lyrics'), { sheet: true, stage: false })
    assert.deepEqual(rememberLyrics({ sheet: true, stage: true }, 'sheet', 'queue'), { sheet: false, stage: true })
  })

  it('returns the same object when nothing changes', () => {
    assert.equal(rememberLyrics(DEFAULT_NOW_PLAYING_LYRICS, 'stage', 'lyrics'), DEFAULT_NOW_PLAYING_LYRICS)
  })
})
