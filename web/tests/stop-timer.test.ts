// Run with `pnpm test` (node --test).
import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import {
  expireTimer,
  parseMinutes,
  remainingSeconds,
  sanitizeTimer,
  startTimer,
  TIMER_OFF,
  touchTimer,
} from '../src/features/player/lib/stop-timer.ts'

const MIN = 60_000

describe('startTimer', () => {
  it('stops the picked time from now', () => {
    assert.deepEqual(startTimer('sleep', 30, 1000), { mode: 'sleep', minutes: 30, endsAt: 1000 + 30 * MIN })
  })
})

describe('touchTimer', () => {
  it('restarts the idle count from the activity', () => {
    const timer = startTimer('idle', 15, 0)
    assert.equal(touchTimer(timer, 10 * MIN).endsAt, 25 * MIN)
  })

  it('leaves a sleep timer and a timer that is off alone', () => {
    const sleep = startTimer('sleep', 15, 0)
    assert.equal(touchTimer(sleep, 10 * MIN), sleep)
    assert.equal(touchTimer(TIMER_OFF, 10 * MIN), TIMER_OFF)
  })

  it('keeps the deadline for activity in quick succession', () => {
    const timer = startTimer('idle', 15, 0)
    assert.equal(touchTimer(timer, 4000), timer)
    assert.equal(touchTimer(timer, 6000).endsAt, 6000 + 15 * MIN)
  })

  it('counts again after idle mode stopped playback', () => {
    const waiting = expireTimer(startTimer('idle', 15, 0))
    assert.equal(touchTimer(waiting, 60 * MIN).endsAt, 75 * MIN)
  })
})

describe('expireTimer', () => {
  it('turns a sleep timer off and keeps idle mode on without a deadline', () => {
    assert.deepEqual(expireTimer(startTimer('sleep', 30, 0)), TIMER_OFF)
    assert.deepEqual(expireTimer(startTimer('idle', 45, 0)), { mode: 'idle', minutes: 45, endsAt: null })
  })
})

describe('sanitizeTimer', () => {
  it('restores idle mode without a deadline', () => {
    assert.deepEqual(sanitizeTimer({ mode: 'idle', minutes: 60, endsAt: 123 }), { mode: 'idle', minutes: 60, endsAt: null })
  })

  it('drops a sleep timer and anything invalid', () => {
    assert.deepEqual(sanitizeTimer({ mode: 'sleep', minutes: 30 }), TIMER_OFF)
    assert.deepEqual(sanitizeTimer({ mode: 'idle', minutes: 0 }), TIMER_OFF)
    assert.deepEqual(sanitizeTimer({ mode: 'idle', minutes: 7.5 }), TIMER_OFF)
    assert.deepEqual(sanitizeTimer({ mode: 'idle', minutes: 5000 }), TIMER_OFF)
    assert.deepEqual(sanitizeTimer({ mode: 'idle', minutes: '30' as never }), TIMER_OFF)
    assert.deepEqual(sanitizeTimer(null), TIMER_OFF)
  })

  it('keeps a custom time', () => {
    assert.deepEqual(sanitizeTimer({ mode: 'idle', minutes: 7 }), { mode: 'idle', minutes: 7, endsAt: null })
  })
})

describe('parseMinutes', () => {
  it('accepts whole minutes from one minute to a day', () => {
    assert.equal(parseMinutes('1'), 1)
    assert.equal(parseMinutes(' 25 '), 25)
    assert.equal(parseMinutes('1440'), 1440)
  })

  it('rejects everything else', () => {
    for (const text of ['', '0', '1441', '-5', '2.5', '1e2', 'ten']) assert.equal(parseMinutes(text), null, text)
  })
})

describe('remainingSeconds', () => {
  it('rounds up and never goes below zero', () => {
    assert.equal(remainingSeconds(10_000, 0), 10)
    assert.equal(remainingSeconds(10_000, 9_001), 1)
    assert.equal(remainingSeconds(10_000, 12_000), 0)
  })
})
