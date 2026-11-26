// Run with `pnpm test` (node --test). Synthetic tags only.
import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import type { StagedItem } from '../src/lib/api/types.ts'
import { conflictCount, formFromTags, stagedName, tagPatch } from '../src/features/manage/lib/review.ts'

const tags = {
  TITLE: ['Synthetic Song'],
  ARTIST: ['Artist A', 'Artist B'],
  ALBUM: ['Synthetic Album'],
  TRACKNUMBER: ['3/12'],
  YEAR: ['2024'],
}

describe('review form', () => {
  it('reads display values from tags', () => {
    const form = formFromTags(tags)
    assert.equal(form.title, 'Synthetic Song')
    assert.equal(form.artist, 'Artist A; Artist B')
    assert.equal(form.track, '3')
    assert.equal(form.disc, '')
    assert.equal(form.date, '2024')
  })

  it('patches only what changed', () => {
    assert.deepEqual(tagPatch(formFromTags(tags), tags), {})
    const form = { ...formFromTags(tags), title: ' Fixed ', artist: 'Artist A;; Artist C ', track: '04', albumArtist: 'Synthetic Artist' }
    assert.deepEqual(tagPatch(form, tags), {
      TITLE: ['Fixed'],
      ARTIST: ['Artist A', 'Artist C'],
      ALBUMARTIST: ['Synthetic Artist'],
      TRACKNUMBER: ['4/12'], // keeps the file's "n/total" style
    })
  })

  it('reads a typed "n/total" as a number and its total', () => {
    // Typing the file's own "3/12" is no change.
    assert.deepEqual(tagPatch({ ...formFromTags(tags), track: '3/12' }, tags), {})
    assert.deepEqual(tagPatch({ ...formFromTags(tags), track: '5 / 10', disc: '1/2' }, tags), {
      TRACKNUMBER: ['5/10'],
      DISCNUMBER: ['1/2'],
    })
    // A number with a trailing "/" keeps the file's total.
    assert.deepEqual(tagPatch({ ...formFromTags(tags), track: '4/' }, tags), { TRACKNUMBER: ['4/12'] })
  })

  it('leaves fields the user did not touch as the file stores them', () => {
    const stored = { GENRE: ['Ambient; Electronic'], DISCNUMBER: ['1/2'], DATE: ['2020-11-02'] }
    assert.deepEqual(tagPatch(formFromTags(stored), stored), {})
    assert.deepEqual(tagPatch({ ...formFromTags(stored), genre: 'Ambient; Jazz' }, stored), { GENRE: ['Ambient', 'Jazz'] })
  })

  it('writes a changed date to the date keys the file has', () => {
    // Only YEAR: clearing deletes it, a new date updates it (YEAR keeps the year).
    assert.deepEqual(tagPatch({ ...formFromTags(tags), date: '' }, tags), { YEAR: [] })
    assert.deepEqual(tagPatch({ ...formFromTags(tags), date: '2025-03-04' }, tags), { YEAR: ['2025'] })
    // Both: both change together.
    const both = { DATE: ['2020-11-02'], YEAR: ['2020'] }
    assert.deepEqual(tagPatch({ ...formFromTags(both), date: '2021-01-05' }, both), { DATE: ['2021-01-05'], YEAR: ['2021'] })
    assert.deepEqual(tagPatch({ ...formFromTags(both), date: '' }, both), { DATE: [], YEAR: [] })
    // Neither: DATE.
    assert.deepEqual(tagPatch({ ...formFromTags({}), date: '2024' }, {}), { DATE: ['2024'] })
  })

  it('deletes cleared fields', () => {
    const form = { ...formFromTags(tags), album: '  ', track: '' }
    assert.deepEqual(tagPatch(form, tags), { ALBUM: [], TRACKNUMBER: [] })
  })
})

function item(conflict: Partial<StagedItem['conflict']>): Pick<StagedItem, 'conflict'> {
  return { conflict: { exists: false, sameSource: [], similar: [], ...conflict } }
}

describe('staged items', () => {
  it('counts conflict kinds', () => {
    assert.equal(conflictCount(item({})), 0)
    const match = { trackId: 'synthetic', libraryId: 1, path: 'A/B/01 C.flac' }
    assert.equal(conflictCount(item({ exists: true, sameSource: [match], similar: [match, match] })), 3)
  })

  it('names an item by its tags, then by the reported title', () => {
    assert.equal(stagedName({ tags, title: 'Reported', name: 'x.m4a' }), 'Synthetic Song — Artist A / Artist B')
    assert.equal(stagedName({ tags: {}, title: 'Reported', name: 'x.m4a' }), 'Reported')
    assert.equal(stagedName({ tags: {}, title: '', name: 'Dir/x.m4a' }), 'x.m4a')
  })
})
