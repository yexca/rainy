// Run with `pnpm test` (node --test).
import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import {
  busiestTimes,
  dayKey,
  groupByDay,
  heatLevel,
  isCalendarRange,
  isListeningRange,
  monthOf,
  monthsSince,
  niceMax,
  percentChange,
  rangeBounds,
  recapMonth,
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
  it('covers a whole calendar month', () => {
    assert.deepEqual(rangeBounds('m2026-09', now), { from: new Date(2026, 8, 1).getTime(), to: new Date(2026, 9, 1).getTime() })
    assert.deepEqual(rangeBounds('m2025-12', now), { from: new Date(2025, 11, 1).getTime(), to: new Date(2026, 0, 1).getTime() })
    assert.deepEqual(monthOf('m2026-02'), { year: 2026, month: 2 })
    assert.equal(monthOf('m2026-13'), null)
    assert.ok(isCalendarRange('m2026-09') && isCalendarRange('y2025') && !isCalendarRange('30d'))
  })
  it('accepts only known ranges', () => {
    assert.ok(isListeningRange('m2026-09'))
    assert.ok(!isListeningRange('m2026-9'))
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

describe('monthsSince', () => {
  it('lists the months with plays, newest first, at most a year', () => {
    assert.deepEqual(monthsSince(new Date(2026, 6, 20).getTime(), now), ['m2026-09', 'm2026-08', 'm2026-07'])
    assert.equal(monthsSince(new Date(2020, 0, 1).getTime(), now).length, 12)
    assert.deepEqual(monthsSince(new Date(2025, 11, 31).getTime(), new Date(2026, 0, 5)), ['m2026-01', 'm2025-12'])
    assert.deepEqual(monthsSince(0, now), [])
  })
})

describe('recapMonth', () => {
  it('offers last month during the first week of a month', () => {
    assert.equal(recapMonth(new Date(2026, 9, 1)), 'm2026-09')
    assert.equal(recapMonth(new Date(2026, 0, 7)), 'm2025-12')
    assert.equal(recapMonth(new Date(2026, 9, 8)), null)
  })
})

describe('busiestTimes', () => {
  it('finds the weekday and hour with the most plays', () => {
    const clock = Array.from({ length: 7 }, () => Array.from({ length: 24 }, () => 0))
    assert.equal(busiestTimes(clock), null)
    clock[5][22] = 4
    clock[5][9] = 1
    clock[0][22] = 2
    clock[1][8] = 3
    assert.deepEqual(busiestTimes(clock), { weekday: 5, hour: 22 })
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
