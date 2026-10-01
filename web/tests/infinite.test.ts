// Run with `pnpm test` (node --test).
import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import {
  autoEntries,
  manualInsertAt,
  MAX_SEEDS,
  mixExclude,
  mixSeeds,
  needsRefill,
  refillKey,
  type QueueItem,
} from '../src/features/player/lib/infinite.ts'

function track(id: string, extra: Partial<QueueItem> = {}): QueueItem {
  return { id, ...extra }
}

const q = (...ids: string[]) => ids.map((id) => track(id))

describe('needsRefill', () => {
  it('asks for more when two or fewer songs are left, only with infinite on and repeat off', () => {
    const queue = q('a', 'b', 'c', 'd')
    assert.equal(needsRefill({ infinite: true, repeat: 'off', queue, index: 0 }), false)
    assert.equal(needsRefill({ infinite: true, repeat: 'off', queue, index: 1 }), true)
    assert.equal(needsRefill({ infinite: true, repeat: 'off', queue, index: 3 }), true)
    assert.equal(needsRefill({ infinite: false, repeat: 'off', queue, index: 3 }), false)
    assert.equal(needsRefill({ infinite: true, repeat: 'all', queue, index: 3 }), false)
    assert.equal(needsRefill({ infinite: true, repeat: 'one', queue, index: 3 }), false)
    assert.equal(needsRefill({ infinite: true, repeat: 'off', queue: [], index: -1 }), false)
  })

  it('never refills behind live radio', () => {
    const queue = [track('live', { isRadio: true })]
    assert.equal(needsRefill({ infinite: true, repeat: 'off', queue, index: 0 }), false)
  })
})

describe('mixSeeds', () => {
  it('sends the latest songs up to the current one plus songs the user queued, newest last', () => {
    const queue = [...q('a', 'b', 'c', 'd', 'e', 'f', 'g'), track('auto', { autoAdded: true })]
    assert.deepEqual(mixSeeds(queue, 6), ['c', 'd', 'e', 'f', 'g'])
    assert.equal(mixSeeds(queue, 6).length, MAX_SEEDS)
    // The user's own upcoming song counts; suggestions don't.
    assert.deepEqual(mixSeeds([...q('a', 'b'), track('x', { autoAdded: true }), track('mine')], 1), ['a', 'b', 'mine'])
  })

  it('leaves out radio and repeats', () => {
    const queue = [track('a'), track('live', { isRadio: true }), track('a'), track('b')]
    assert.deepEqual(mixSeeds(queue, 3), ['a', 'b'])
  })
})

describe('mixExclude', () => {
  it('lists every queued song once', () => {
    assert.deepEqual(mixExclude([...q('a', 'b', 'a'), track('live', { isRadio: true })]), ['a', 'b'])
  })
})

describe('refillKey', () => {
  it('changes when the queue moves on, so a failed request is retried only after a change', () => {
    const queue = q('a', 'b', 'c')
    assert.equal(refillKey(queue, 2), refillKey(q('a', 'b', 'c'), 2))
    assert.notEqual(refillKey(queue, 2), refillKey(queue, 1))
    assert.notEqual(refillKey(queue, 2), refillKey(q('a', 'b', 'c', 'd'), 2))
  })
})

describe('manualInsertAt', () => {
  it('puts songs the user adds before the suggestions', () => {
    const queue = [...q('a', 'b'), track('s1', { autoAdded: true }), track('s2', { autoAdded: true })]
    assert.equal(manualInsertAt(queue, 0), 2)
    assert.equal(manualInsertAt(queue, 2), 3)
    assert.equal(manualInsertAt(q('a', 'b'), 0), 2)
  })

  it('ignores suggestions that already played', () => {
    const queue = [track('s1', { autoAdded: true }), ...q('a', 'b')]
    assert.equal(manualInsertAt(queue, 1), 3)
  })
})

describe('autoEntries', () => {
  it('marks new entries and drops songs already queued or repeated', () => {
    const got = autoEntries([track('a'), track('x'), track('x'), track('y')], q('a', 'b'))
    assert.deepEqual(
      got.map((t) => [t.id, t.autoAdded]),
      [
        ['x', true],
        ['y', true],
      ],
    )
  })
})
