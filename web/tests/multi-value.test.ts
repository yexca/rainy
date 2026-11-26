// Run with `pnpm test` (node --test). Synthetic values only.
import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import { insertValues, parseValues, startEdit } from '../src/features/manage/lib/multi-value.ts'

describe('multi-value edits', () => {
  it('splits typed text on semicolons, and on commas when asked', () => {
    assert.deepEqual(parseValues(' Artist A ；Artist B; ', false), ['Artist A', 'Artist B'])
    assert.deepEqual(parseValues('Artist, With Comma', false), ['Artist, With Comma'])
    assert.deepEqual(parseValues('Rock，Pop, Jazz', true), ['Rock', 'Pop', 'Jazz'])
  })

  it('adds new values at the end and skips ones already there', () => {
    assert.deepEqual(insertValues(['Main', 'Guest'], ['guest', 'Other', 'other'], null), ['Main', 'Guest', 'Other'])
  })

  it('puts an edited chip back in its place, so the main artist stays first', () => {
    const edit = startEdit(['Main', 'Guest'], 0, [], null)
    assert.deepEqual(edit, { values: ['Guest'], text: 'Main', at: 0 })
    // Committing the unchanged text restores the original order.
    assert.deepEqual(insertValues(edit.values, ['Main'], edit.at), ['Main', 'Guest'])
    // A corrected value keeps the position too.
    assert.deepEqual(insertValues(edit.values, ['Main Artist'], edit.at), ['Main Artist', 'Guest'])
  })

  it('keeps text still being typed before taking a chip out', () => {
    // "New" belongs at the end; editing "Guest" (index 1) is unaffected.
    assert.deepEqual(startEdit(['Main', 'Guest'], 1, ['New'], null), { values: ['Main', 'New'], text: 'Guest', at: 1 })
    // "New" was an edited first chip: the chip clicked after it moves one place down.
    assert.deepEqual(startEdit(['Main', 'Guest'], 1, ['New'], 0), { values: ['New', 'Main'], text: 'Guest', at: 2 })
  })
})
