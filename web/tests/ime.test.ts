// Run with `pnpm test` (node --test).
import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import { IME_IDLE, isImeKeyEvent, stepIme, type ImeEvent, type ImeState } from '../src/lib/ime.ts'

/** Replays events and returns what the consumer received and what the field shows. */
function replay(events: ImeEvent[], start: ImeState = IME_IDLE) {
  let state = start
  const commits: string[] = []
  const shown: (string | null)[] = []
  for (const event of events) {
    const step = stepIme(state, event)
    state = step.state
    if (step.commit !== undefined) commits.push(step.commit)
    shown.push(state.draft)
  }
  return { state, commits, shown }
}

describe('stepIme', () => {
  it('passes plain typing straight through', () => {
    const { commits, state } = replay([
      { type: 'input', value: 'b', isComposing: false },
      { type: 'input', value: 'be', isComposing: false },
    ])
    assert.deepEqual(commits, ['b', 'be'])
    assert.deepEqual(state, IME_IDLE)
  })

  it('keeps a pinyin reading out of the consumer until it commits (Chromium order)', () => {
    const { commits, shown, state } = replay([
      { type: 'start', value: '' },
      { type: 'input', value: 'n', isComposing: true },
      { type: 'input', value: "ni'h", isComposing: true },
      { type: 'input', value: '你好', isComposing: true },
      { type: 'end', value: '你好' },
    ])
    assert.deepEqual(commits, ['你好'])
    assert.deepEqual(shown.slice(1, 4), ['n', "ni'h", '你好'])
    assert.deepEqual(state, IME_IDLE)
  })

  it('commits kana conversion once Safari ends composition before the last input', () => {
    const { commits, state } = replay([
      { type: 'start', value: 'ab' },
      { type: 'input', value: 'abにほん', isComposing: true },
      { type: 'end', value: 'ab日本' },
      { type: 'input', value: 'ab日本', isComposing: false },
    ])
    assert.deepEqual(commits, ['ab日本', 'ab日本'])
    assert.deepEqual(state, IME_IDLE)
  })

  it('treats an input that reports composition as a draft even without compositionstart', () => {
    const { commits, state } = replay([{ type: 'input', value: 'ㅎ', isComposing: true }])
    assert.deepEqual(commits, [])
    assert.equal(state.draft, 'ㅎ')
  })

  it('restores the committed text when a reading is cancelled', () => {
    const { commits, state } = replay([
      { type: 'start', value: 'rock' },
      { type: 'input', value: 'rockに', isComposing: true },
      { type: 'end', value: 'rock' },
    ])
    assert.deepEqual(commits, ['rock'])
    assert.deepEqual(state, IME_IDLE)
  })
})

describe('isImeKeyEvent', () => {
  it('recognises composing keys, including Safari’s post-composition keyCode 229', () => {
    assert.equal(isImeKeyEvent({ isComposing: true, keyCode: 13 }), true)
    assert.equal(isImeKeyEvent({ isComposing: false, keyCode: 229 }), true)
    assert.equal(isImeKeyEvent({ isComposing: false, keyCode: 27 }), false)
  })
})
