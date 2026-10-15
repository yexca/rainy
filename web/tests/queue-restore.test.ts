import assert from 'node:assert/strict'
import { test } from 'node:test'

import { restoreQueue } from '../src/features/player/lib/queue-restore.ts'

test('restores the second occurrence after unavailable entries are filtered', () => {
  const restored = restoreQueue(['a', 'missing', 'unknown', 'b', 'a'], [
    { id: 'a' }, { id: 'missing', missing: true }, { id: 'b' },
  ], 'a', 4, 12.5)
  assert.deepEqual(restored.queue.map((track) => track.id), ['a', 'b', 'a'])
  assert.equal(restored.index, 2)
  assert.equal(restored.position, 12.5)
  assert.notEqual(restored.queue[0], restored.queue[2])
})

test('old servers and mismatched indexes fall back to the first occurrence', () => {
  for (const index of [undefined, -1, 1, 9, 0.5]) {
    const restored = restoreQueue(['a', 'b', 'a'], [{ id: 'a' }, { id: 'b' }], 'a', index, 7)
    assert.equal(restored.index, 0)
    assert.equal(restored.position, 7)
  }
})

test('an unavailable current song never transfers its position to another song', () => {
  assert.deepEqual(restoreQueue(['gone', 'b'], [{ id: 'b' }], 'gone', 0, 42), {
    queue: [{ id: 'b' }], index: 0, position: 0,
  })
  assert.deepEqual(restoreQueue(['gone'], [], 'gone', 0, 42), { queue: [], index: -1, position: 0 })
})
