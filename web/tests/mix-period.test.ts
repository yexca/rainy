// Run with `pnpm test` (node --test).
import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import { localDay, localMonday, weekDays } from '../src/features/library/lib/mix-period.ts'

describe('localMonday', () => {
  it('keeps one week from Monday to Sunday', () => {
    assert.equal(localMonday(new Date(2026, 8, 28, 0, 0)), '2026-09-28') // Monday, midnight
    assert.equal(localMonday(new Date(2026, 9, 1, 12, 0)), '2026-09-28') // Thursday, across the month
    assert.equal(localMonday(new Date(2026, 9, 4, 23, 59)), '2026-09-28') // Sunday, last minute
  })

  it('changes on Monday', () => {
    assert.equal(localMonday(new Date(2026, 9, 5, 0, 0)), '2026-10-05')
    assert.equal(localMonday(new Date(2027, 0, 1, 9, 0)), '2026-12-28') // Friday, across the year
  })
})

describe('localDay', () => {
  it('pads the month and day', () => {
    assert.equal(localDay(new Date(2026, 0, 5, 23, 59)), '2026-01-05')
  })
})

describe('weekDays', () => {
  it('returns the Monday and the Sunday after it', () => {
    const days = weekDays('2026-09-28')
    assert.ok(days)
    assert.deepEqual(days.map(localDay), ['2026-09-28', '2026-10-04'])
  })

  it('rejects a malformed date', () => {
    assert.equal(weekDays('this week'), null)
  })
})
