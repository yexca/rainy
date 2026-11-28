// Run with `pnpm test` (node --test).
import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import { continueIndex, episodeProgress, hasProgress, type EpisodeItem } from '../src/features/library/lib/program-progress.ts'

const episode = (id: string, playCount = 0, duration = 1680): EpisodeItem => ({ id, playCount, duration })

describe('episodeProgress', () => {
  it('tells played, partly played and new episodes apart', () => {
    assert.equal(episodeProgress(episode('a', 1), undefined).state, 'played')
    assert.equal(episodeProgress(episode('a'), undefined).state, 'unplayed')
    const partial = episodeProgress(episode('a', 1), 420_000)
    assert.equal(partial.state, 'partial')
    assert.equal(partial.fraction, 0.25)
    assert.equal(partial.remaining, 1260)
    assert.deepEqual(episodeProgress(episode('a', 0, 0), 60_000), { state: 'partial', fraction: 0, remaining: 0 })
  })
})

describe('continueIndex', () => {
  it('starts at the first unfinished episode, or the first one when all are played', () => {
    const show = [episode('e1', 2), episode('e2', 1), episode('e3'), episode('e4')]
    assert.equal(continueIndex(show, new Map()), 2)
    assert.equal(continueIndex(show, new Map([['e2', 90_000]])), 1)
    assert.equal(continueIndex([episode('e1', 1), episode('e2', 1)], new Map()), 0)
    assert.equal(continueIndex([], new Map()), -1)
  })

  it('knows whether the listener started the show', () => {
    assert.equal(hasProgress([episode('e1'), episode('e2')], new Map()), false)
    assert.equal(hasProgress([episode('e1'), episode('e2')], new Map([['e2', 10_000]])), true)
    assert.equal(hasProgress([episode('e1', 1)], new Map()), true)
  })
})
