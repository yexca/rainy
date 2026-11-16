// Run with `pnpm test` (node --test).
import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import {
  RECENTLY_ADDED_WINDOW_MS,
  availablePickModes,
  recentAlbums,
  resolvePickMode,
} from '../src/features/library/lib/home.ts'

describe('recentAlbums', () => {
  const now = Date.UTC(2026, 10, 14, 12, 0, 0)

  it('keeps albums added within the 14-day window and drops older or undated ones', () => {
    const albums = [
      { id: 'today', createdAt: now - 60_000 },
      { id: 'edge', createdAt: now - RECENTLY_ADDED_WINDOW_MS },
      { id: 'stale', createdAt: now - RECENTLY_ADDED_WINDOW_MS - 1 },
      { id: 'monthsAgo', createdAt: now - 90 * 24 * 60 * 60 * 1000 },
      { id: 'missingDate', createdAt: 0 },
    ]
    assert.deepEqual(
      recentAlbums(albums, now).map((a) => a.id),
      ['today', 'edge'],
    )
  })

  it('returns an empty list when nothing was imported recently', () => {
    const albums = [{ id: 'old', createdAt: now - 45 * 24 * 60 * 60 * 1000 }]
    assert.deepEqual(recentAlbums(albums, now), [])
  })
})

describe('song pick modes', () => {
  it('lists only modes with songs in display order', () => {
    assert.deepEqual(availablePickModes(undefined), [])
    assert.deepEqual(availablePickModes({ quick: [1], onRepeat: [], rediscover: [2] }), ['quick', 'rediscover'])
  })

  it('falls back to the first available mode when the chosen one has no songs', () => {
    assert.equal(resolvePickMode('onRepeat', ['quick', 'rediscover']), 'quick')
    assert.equal(resolvePickMode('rediscover', ['quick', 'rediscover']), 'rediscover')
    assert.equal(resolvePickMode('quick', []), null)
  })
})
