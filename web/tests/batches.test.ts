// Run with `pnpm test` (node --test).
import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import { batches } from '../src/lib/batches.ts'

describe('batches', () => {
  it('splits a long id list so no request carries more than the batch size', () => {
    const ids = Array.from({ length: 250 }, (_, i) => `t${i}`)
    const out = batches(ids, 100)
    assert.deepEqual(
      out.map((b) => b.length),
      [100, 100, 50],
    )
    assert.deepEqual(out.flat(), ids)
  })

  it('makes no batch for no items', () => {
    assert.deepEqual(batches([], 100), [])
  })
})
