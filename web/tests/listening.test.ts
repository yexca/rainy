// Run with `pnpm test` (node --test).
import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import {
  dayKey,
  groupByDay,
  heatLevel,
  isListeningRange,
  niceMax,
  percentChange,
  rangeBounds,
  relativeDay,
  weekdayOrder,
  yearsSince,
} from '../src/features/library/lib/listening.ts'

const now = new Date(2026, 8, 30, 15, 30) // 30 Sep 2026, 15:30 local

describe('rangeBounds', () => {
  it('starts rolling ranges at local midnight and ends them now', () => {
    assert.deepEqual(rangeBounds('7d', now), { from: new Date(2026, 8, 24).getTime(), to: 0 })
    assert.deepEqual(rangeBounds('30d', now), { from: new Date(2026, 8, 1).getTime(), to: 0 })
    assert.deepEqual(rangeBounds('12m', now), { from: new Date(2025, 9, 1).getTime(), to: 0 })
    assert.deepEqual(rangeBounds('all', now), { from: 0, to: 0 })
  })
  it('covers a whole calendar year', () => {
    assert.deepEqual(rangeBounds('y2025', now), { from: new Date(2025, 0, 1).getTime(), to: new Date(2026, 0, 1).getTime() })
  })
  it('accepts only known ranges', () => {
    assert.ok(isListeningRange('90d'))
    assert.ok(isListeningRange('y2024'))
    assert.ok(!isListeningRange('y24'))
    assert.ok(!isListeningRange('1d'))
    assert.ok(!isListeningRange(null))
  })
})

describe('yearsSince', () => {
  it('lists the years with plays, newest first', () => {
    assert.deepEqual(yearsSince(new Date(2024, 5, 1).getTime(), now), [2026, 2025, 2024])
    assert.deepEqual(yearsSince(0, now), [])
  })
})

describe('percentChange', () => {
  it('compares with the previous period', () => {
    assert.equal(percentChange(150, 100), 50)
    assert.equal(percentChange(50, 100), -50)
    assert.equal(percentChange(10, 0), null)
    assert.equal(percentChange(10, null), null)
  })
})

describe('chart scales', () => {
  it('rounds axis maxima up to 1, 2 or 5 × 10ⁿ', () => {
    assert.deepEqual([0, 1, 3, 7, 12, 20, 47, 101].map(niceMax), [1, 1, 5, 10, 20, 20, 50, 200])
  })
  it('shades the heatmap on a square-root scale', () => {
    assert.deepEqual([0, 1, 4, 16, 64].map((v) => heatLevel(v, 64)), [0, 1, 1, 2, 4])
    assert.equal(heatLevel(3, 0), 0)
  })
  it('orders weekdays from the locale’s first day', () => {
    assert.deepEqual(weekdayOrder(1), [0, 1, 2, 3, 4, 5, 6]) // Monday first
    assert.deepEqual(weekdayOrder(0), [6, 0, 1, 2, 3, 4, 5]) // Sunday first
    assert.deepEqual(weekdayOrder(6), [5, 6, 0, 1, 2, 3, 4]) // Saturday first
  })
})

describe('history grouping', () => {
  it('groups plays by local day and names today and yesterday', () => {
    const plays = [new Date(2026, 8, 30, 9), new Date(2026, 8, 30, 1), new Date(2026, 8, 29, 23), new Date(2026, 8, 20)].map((d) => ({
      at: d.getTime(),
    }))
    const groups = groupByDay(plays, (p) => p.at)
    assert.deepEqual(
      groups.map((g) => [g.key, g.items.length]),
      [
        ['2026-09-30', 2],
        ['2026-09-29', 1],
        ['2026-09-20', 1],
      ],
    )
    assert.equal(relativeDay(groups[0].key, now), 'today')
    assert.equal(relativeDay(groups[1].key, now), 'yesterday')
    assert.equal(relativeDay(groups[2].key, now), null)
    assert.equal(dayKey(new Date(2026, 0, 5).getTime()), '2026-01-05')
  })
})
