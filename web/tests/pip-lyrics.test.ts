// Run with `pnpm test` (node --test).
import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import { pipLyricWindow, wrapText, type PipLine } from '../src/features/player/lib/pip-lyrics.ts'

const line = (start: number, text: string, translations: string[] = []): PipLine => ({ start, text, translations })
const lines = [line(0, 'first'), line(1000, ''), line(2000, 'second', ['第二']), line(3000, 'third')]

describe('pipLyricWindow', () => {
  it('shows nothing current before the first line, and the first line next', () => {
    assert.deepEqual(pipLyricWindow(lines, -1), { current: null, next: lines[0], index: -1 })
  })
  it('skips instrumental gaps for the next line', () => {
    assert.deepEqual(pipLyricWindow(lines, 0), { current: lines[0], next: lines[2], index: 0 })
    assert.deepEqual(pipLyricWindow(lines, 1), { current: lines[1], next: lines[2], index: 1 })
  })
  it('has no next line at the end', () => {
    assert.deepEqual(pipLyricWindow(lines, 3), { current: lines[3], next: null, index: 3 })
    assert.deepEqual(pipLyricWindow([], 0), { current: null, next: null, index: -1 })
  })
})

describe('wrapText', () => {
  const measure = (s: string) => [...s].length // one unit per character
  it('breaks Latin text at spaces', () => {
    assert.deepEqual(wrapText('the quick brown fox', 10, measure), ['the quick', 'brown fox'])
  })
  it('breaks CJK text between characters', () => {
    assert.deepEqual(wrapText('君の名前を呼んでいる', 4, measure, 3), ['君の名前', 'を呼んで', 'いる'])
  })
  it('cuts long text with an ellipsis', () => {
    assert.deepEqual(wrapText('one two three four five six', 9, measure, 2), ['one two', 'three…'])
  })
  it('breaks a word longer than a line anywhere', () => {
    assert.deepEqual(wrapText('abcdefghij', 4, measure, 3), ['abcd', 'efgh', 'ij'])
  })
  it('keeps short text on one line', () => {
    assert.deepEqual(wrapText('  hello  ', 20, measure), ['hello'])
    assert.deepEqual(wrapText('', 20, measure), [])
  })
})
