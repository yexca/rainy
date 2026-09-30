// Run with `pnpm test` (node --test). Synthetic tracks and lyrics only.
import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import type { MetadataResult, TrackTags } from '../src/lib/api/types.ts'
import {
  defaultQuery,
  durationMatch,
  lyricsText,
  mergeTranslation,
  onlineFieldValues,
  resultFields,
  type OnlineFieldId,
} from '../src/features/manage/lib/online.ts'

const result: MetadataResult = {
  provider: 'qq',
  id: '000aaaaaaaaaaa',
  title: 'Synthetic Song',
  artists: ['Artist A', 'Artist B'],
  album: 'Test Album',
  albumArtist: '',
  trackNumber: 3,
  trackTotal: 0,
  discNumber: 1,
  discTotal: 0,
  date: '2020-01-02',
  genre: '',
  duration: 200,
  coverUrl: '',
  thumbUrl: '',
}

function item(id: string, tags: Record<string, string[]>, track: Partial<TrackTags['track']> = {}): TrackTags {
  return { track: { id, title: '', artist: '', album: '', ...track }, tags } as unknown as TrackTags
}

describe('resultFields', () => {
  it('offers every known field for one track and drops unknown ones', () => {
    assert.deepEqual(resultFields(result, true), {
      title: 'Synthetic Song',
      artist: 'Artist A; Artist B',
      album: 'Test Album',
      trackNumber: '3',
      discNumber: '1',
      date: '2020-01-02',
    })
  })

  it('offers only album-level fields for several tracks', () => {
    assert.deepEqual(resultFields(result, false), { album: 'Test Album', date: '2020-01-02' })
  })
})

describe('onlineFieldValues', () => {
  const values = resultFields(result, true)
  const all = new Set(Object.keys(values) as OnlineFieldId[])

  it('fills the chosen fields of the single track, in field order', () => {
    const out = onlineFieldValues([item('t1', {})], values, new Set<OnlineFieldId>(['date', 'title']))
    assert.deepEqual(out, { t1: { title: 'Synthetic Song', date: '2020-01-02' } })
    assert.deepEqual(Object.keys(out.t1), ['title', 'date'])
  })

  it('never copies song-level fields onto several tracks', () => {
    const out = onlineFieldValues([item('t1', {}), item('t2', {})], values, all)
    assert.deepEqual(out, {
      t1: { album: 'Test Album', date: '2020-01-02' },
      t2: { album: 'Test Album', date: '2020-01-02' },
    })
  })

  it('returns nothing when no field is chosen', () => {
    assert.deepEqual(onlineFieldValues([item('t1', {})], values, new Set()), {})
  })
})

describe('defaultQuery', () => {
  it('uses title and artist for one track, album and album artist for several', () => {
    assert.equal(defaultQuery([item('t1', { TITLE: ['Song'], ARTIST: ['Singer'] })]), 'Song Singer')
    assert.equal(
      defaultQuery([item('t1', { ALBUM: ['Record'], ALBUMARTIST: ['Band'] }), item('t2', {})]),
      'Record Band',
    )
    assert.equal(defaultQuery([item('t1', {}, { title: 'Indexed', artist: '' })]), 'Indexed')
    assert.equal(defaultQuery([]), '')
  })
})

describe('durationMatch', () => {
  it('grades the length difference', () => {
    assert.equal(durationMatch(200, 202), 'match')
    assert.equal(durationMatch(200, 208), 'near')
    assert.equal(durationMatch(200, 240), 'mismatch')
    assert.equal(durationMatch(0, 200), 'unknown')
  })
})

describe('mergeTranslation', () => {
  it('pairs translated lines after their originals with the same time tags', () => {
    const text = '[ti:Song]\n[00:01.00]Hello\n[00:02.50]World\n[00:04.00]Instrumental'
    const trans = '[by:someone]\n[00:01.000]你好\n[00:02.5]世界\n[00:04.00]//\n[00:09.00]orphan'
    assert.equal(
      mergeTranslation(text, trans),
      '[ti:Song]\n[00:01.00]Hello\n[00:01.00]你好\n[00:02.50]World\n[00:02.50]世界\n[00:04.00]Instrumental',
    )
  })

  it('leaves the text alone when nothing pairs', () => {
    assert.equal(mergeTranslation('[00:01.00]Same', '[00:01.00]Same'), '[00:01.00]Same')
    assert.equal(mergeTranslation('plain text', '[00:01.00]翻译'), 'plain text')
    assert.equal(mergeTranslation('[00:01.00]A', ''), '[00:01.00]A')
  })

  it('only merges when asked and a translation exists', () => {
    const l = { text: '[00:01.00]A', translation: '[00:01.00]甲' }
    assert.equal(lyricsText(l, false), '[00:01.00]A')
    assert.equal(lyricsText(l, true), '[00:01.00]A\n[00:01.00]甲')
    assert.equal(lyricsText({ text: 'x', translation: '' }, true), 'x')
  })
})
