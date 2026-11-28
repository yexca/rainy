// Run with `pnpm test` (node --test).
import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import {
  bookmarkAction,
  isFinished,
  resumeAt,
  sameAction,
  saveDue,
  SAVE_INTERVAL_MS,
} from '../src/features/player/lib/program-resume.ts'

const EPISODE = 28 * 60 // seconds

describe('resumeAt', () => {
  it('resumes a bookmark past the first seconds and before the end', () => {
    assert.equal(resumeAt(600_000, EPISODE), 600)
    assert.equal(resumeAt(5_500, EPISODE), 5.5)
    assert.equal(resumeAt(600_000, 0), 600) // unknown duration
  })

  it('starts from the beginning without a useful bookmark', () => {
    assert.equal(resumeAt(undefined, EPISODE), 0)
    assert.equal(resumeAt(5_000, EPISODE), 0)
    assert.equal(resumeAt((EPISODE - 30) * 1000, EPISODE), 0)
    assert.equal(resumeAt((EPISODE - 10) * 1000, EPISODE), 0)
    assert.equal(resumeAt(Number.NaN, EPISODE), 0)
  })
})

describe('bookmarkAction', () => {
  it('saves a position in the middle, rounded to whole milliseconds', () => {
    assert.deepEqual(bookmarkAction(612.3456, EPISODE), { kind: 'save', positionMs: 612_346 })
  })

  it('clears the bookmark at the start and within the last 30 seconds', () => {
    assert.deepEqual(bookmarkAction(3, EPISODE), { kind: 'clear' })
    assert.deepEqual(bookmarkAction(EPISODE - 29, EPISODE), { kind: 'clear' })
    assert.deepEqual(bookmarkAction(EPISODE, EPISODE), { kind: 'clear' })
    assert.equal(isFinished(EPISODE - 31, EPISODE), false)
    assert.equal(isFinished(100, 0), false)
  })
})

describe('sameAction and saveDue', () => {
  it('skips sending what is already stored', () => {
    assert.equal(sameAction(null, { kind: 'clear' }), false)
    assert.equal(sameAction({ kind: 'clear' }, { kind: 'clear' }), true)
    assert.equal(sameAction({ kind: 'save', positionMs: 60_000 }, { kind: 'save', positionMs: 60_400 }), true)
    assert.equal(sameAction({ kind: 'save', positionMs: 60_000 }, { kind: 'save', positionMs: 75_000 }), false)
    assert.equal(sameAction({ kind: 'save', positionMs: 60_000 }, { kind: 'clear' }), false)
  })

  it('saves about every 15 seconds while playing', () => {
    assert.equal(saveDue(SAVE_INTERVAL_MS - 1, 0), false)
    assert.equal(saveDue(SAVE_INTERVAL_MS, 0), true)
  })
})
