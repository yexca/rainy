import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { Virtualizer } from '@tanstack/react-virtual'

import { createTrackRows } from '../src/features/library/lib/track-rows.ts'

describe('virtual track rows', () => {
  it('keeps a large unloaded library implicit while preserving its scroll positions', () => {
    const rows = createTrackRows([{ id: 'synthetic-a' }, { id: 'synthetic-b' }], 1_000_000)
    assert.equal(rows.loaded.length, 2)
    assert.equal(rows.count, 1_000_000)
    assert.deepEqual(rows.getRow(999_999), { type: 'skeleton', index: 999_999 })
    assert.equal(rows.getItemKey(999_999), 's:999999')
    assert.equal(rows.getRow(1_000_000), undefined)
    assert.equal(rows.getRow(-1), undefined)
  })

  it('keeps disc headers, repeated playlist entries and unloaded tracks distinct', () => {
    const tracks = [
      { id: 'synthetic-a', disc: '1' },
      { id: 'synthetic-a', disc: '2' },
      { id: 'synthetic-b', disc: '1' },
    ]
    const rows = createTrackRows(tracks, 5, (track) => ({ key: track.disc, label: track.disc }))
    assert.equal(rows.count, 8)
    assert.deepEqual(rows.getRow(1), { type: 'track', key: 't:0:synthetic-a', index: 0 })
    assert.deepEqual(rows.getRow(6), { type: 'skeleton', index: 3 })
    const keys = Array.from({ length: rows.count }, (_, i) => rows.getItemKey(i))
    assert.equal(new Set(keys).size, rows.count)
  })

  it('replaces loaded placeholders without losing the positions of later unloaded tracks', () => {
    const first = createTrackRows([{ id: 'synthetic-a' }], 3)
    const next = createTrackRows([{ id: 'synthetic-a' }, { id: 'synthetic-b' }], 3)
    assert.equal(first.getItemKey(0), next.getItemKey(0))
    assert.equal(first.getItemKey(1), 's:1')
    assert.equal(next.getItemKey(1), 't:1:synthetic-b')
    assert.equal(first.getItemKey(2), next.getItemKey(2))
    assert.equal(createTrackRows([{ id: 'synthetic-a' }], 0).count, 1)
  })

  it('reuses virtualizer measurements across scrolling updates instead of traversing the library', () => {
    const rows = createTrackRows([{ id: 'synthetic-a' }], 10_000)
    let estimates = 0
    const options = {
      count: rows.count,
      getItemKey: rows.getItemKey,
      getScrollElement: () => null,
      estimateSize: () => { estimates++; return 60 },
      scrollToFn: () => {},
      observeElementRect: () => {},
      observeElementOffset: () => {},
      initialRect: { width: 375, height: 812 },
    }
    const virtualizer = new Virtualizer(options)
    virtualizer.getVirtualItems()
    assert.equal(estimates, rows.count)
    estimates = 0
    for (let i = 0; i < 120; i++) {
      virtualizer.setOptions({ ...options })
      virtualizer.scrollOffset = i * 60
      assert.ok(virtualizer.getVirtualItems().length > 0)
      assert.equal(virtualizer.getTotalSize(), 600_000)
    }
    assert.equal(estimates, 0)
  })
})
