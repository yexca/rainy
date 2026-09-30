// Run with `pnpm test` (node --test). Synthetic songs only.
import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import type { OnlineQualityType, OnlineSong } from '../src/lib/api/types.ts'
import { jobName } from '../src/features/manage/lib/downloads.ts'
import {
  bestQuality,
  chunk,
  expectedQuality,
  isPlatform,
  isQuality,
  songKey,
  uniqueSongs,
} from '../src/features/manage/lib/online-music.ts'

function song(id: string, qualities: OnlineQualityType[] = [], extra: Record<string, string> = {}): OnlineSong {
  return {
    platform: 'kg', id, title: `Synthetic ${id}`, artists: ['Synthetic Artist'], album: 'Synthetic Album', albumId: '',
    duration: 200, coverUrl: '', qualities: qualities.map((type) => ({ type, size: '' })), extra, pageUrl: '',
  }
}

describe('online music helpers', () => {
  it('validates platforms and qualities', () => {
    assert.ok(isPlatform('wy'))
    assert.ok(!isPlatform('xm'))
    assert.ok(isQuality('flac24bit'))
    assert.ok(!isQuality('999k'))
    assert.ok(!isQuality(null))
  })

  it('finds the best listed quality', () => {
    assert.equal(bestQuality(song('1', ['128k', 'flac', '320k'])), 'flac')
    assert.equal(bestQuality(song('1')), null)
  })

  it('predicts the quality a download asks for', () => {
    const s = song('1', ['128k', '320k', 'flac'])
    assert.equal(expectedQuality(s, 'flac24bit'), 'flac')
    assert.equal(expectedQuality(s, '320k'), '320k')
    assert.equal(expectedQuality(song('1', ['flac']), '320k'), 'flac', 'the lowest above when nothing is at or below')
    assert.equal(expectedQuality(song('1'), 'flac'), null)
  })

  it('keys Kugou results by hash and drops repeats', () => {
    const a = song('42', [], { hash: 'A' })
    const b = song('42', [], { hash: 'B' })
    assert.notEqual(songKey(a), songKey(b))
    assert.deepEqual(uniqueSongs([a, b, { ...a }]).map((s) => s.extra.hash), ['A', 'B'])
  })

  it('splits batches', () => {
    assert.deepEqual(chunk([1, 2, 3, 4, 5], 2), [[1, 2], [3, 4], [5]])
    assert.deepEqual(chunk([], 50), [])
  })

  it('names online download jobs after the song', () => {
    const s = song('7')
    const online = { song: s, quality: 'flac' as const, got: '' as const, source: '', lyrics: true, cover: true }
    assert.equal(jobName({ title: s.title, url: 'https://example.com/7', online }), 'Synthetic 7 — Synthetic Artist')
    assert.equal(jobName({ title: '', url: 'https://example.com/v', online: null }), 'https://example.com/v')
  })
})
