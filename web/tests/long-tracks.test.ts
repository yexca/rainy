// Run with `pnpm test` (node --test).
import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import {
  clampLongTrackMinutes,
  isLongTrack,
  MAX_LONG_TRACK_MINUTES,
  withoutLongTracks,
  type LongTrackItem,
} from '../src/features/player/lib/long-tracks.ts'

type Item = LongTrackItem & { id: string }

const LIMIT = 600

function track(id: string, duration: number, extra: Partial<Item> = {}): Item {
  return { id, duration, ...extra }
}

const ids = (items: readonly Item[]) => items.map((t) => t.id)

describe('isLongTrack', () => {
  it('counts songs at or over the limit, never radio, unknown durations, or a disabled filter', () => {
    assert.equal(isLongTrack(track('a', 599), LIMIT), false)
    assert.equal(isLongTrack(track('a', 600), LIMIT), true)
    assert.equal(isLongTrack(track('a', 0), LIMIT), false)
    assert.equal(isLongTrack(track('a', 3600, { isRadio: true }), LIMIT), false)
    assert.equal(isLongTrack(track('a', 3600), 0), false)
  })
})

describe('withoutLongTracks', () => {
  const list = [track('a', 200), track('mix', 3600), track('b', 240), track('live', 1200)]

  it('leaves long songs out of a list', () => {
    const r = withoutLongTracks(list, LIMIT)
    assert.deepEqual(ids(r.tracks), ['a', 'b'])
    assert.equal(r.skipped, 2)
    assert.equal(r.index, -1)
  })

  it('keeps the picked song even when it is long and moves its index', () => {
    const r = withoutLongTracks(list, LIMIT, 3)
    assert.deepEqual(ids(r.tracks), ['a', 'b', 'live'])
    assert.equal(r.index, 2)
    assert.equal(r.skipped, 1)
    assert.equal(r.tracks[r.index].id, 'live')
  })

  it('maps a short picked song to its new position', () => {
    const r = withoutLongTracks(list, LIMIT, 2)
    assert.deepEqual(ids(r.tracks), ['a', 'b'])
    assert.equal(r.index, 1)
  })

  it('leaves nothing out when every song is long', () => {
    const longs = [track('x', 900), track('y', 1800)]
    assert.deepEqual(withoutLongTracks(longs, LIMIT), { tracks: longs, index: -1, skipped: 0 })
    assert.deepEqual(withoutLongTracks(longs, LIMIT, 1), { tracks: longs, index: 1, skipped: 0 })
  })

  it('changes nothing while the filter is off', () => {
    const r = withoutLongTracks(list, 0, 1)
    assert.deepEqual(ids(r.tracks), ids(list))
    assert.equal(r.index, 1)
    assert.equal(r.skipped, 0)
  })
})

describe('clampLongTrackMinutes', () => {
  it('rounds to whole minutes within range and falls back for non-numbers', () => {
    assert.equal(clampLongTrackMinutes(12.4), 12)
    assert.equal(clampLongTrackMinutes(0), 1)
    assert.equal(clampLongTrackMinutes(100000), MAX_LONG_TRACK_MINUTES)
    assert.equal(clampLongTrackMinutes('15'), 10)
    assert.equal(clampLongTrackMinutes(Number.NaN, 7), 7)
  })
})
