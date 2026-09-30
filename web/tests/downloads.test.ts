// Run with `pnpm test` (node --test). Synthetic links only.
import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import { detectSite, formatEta, isActiveJob } from '../src/features/manage/lib/downloads.ts'

/** A YouTube link changed by `edit` (keeps user info and ports out of the source text). */
function withUrl(edit: (u: URL) => void): string {
  const u = new URL('https://www.youtube.com/watch?v=synthetic01')
  edit(u)
  return u.toString()
}

describe('detectSite', () => {
  it('recognises the supported sites, share text and bare links', () => {
    assert.equal(detectSite('https://www.youtube.com/watch?v=synthetic01'), 'youtube')
    assert.equal(detectSite('youtu.be/synthetic01'), 'youtube')
    assert.equal(detectSite('https://music.youtube.com/watch?v=synthetic01'), 'youtube')
    assert.equal(detectSite('【Synthetic title-哔哩哔哩】 https://b23.tv/Synth01'), 'bilibili')
    assert.equal(detectSite('https://m.bilibili.com/video/BV1Synthetic'), 'bilibili')
  })

  it('rejects other hosts, look-alikes, credentials and ports', () => {
    for (const text of [
      '',
      'hello world',
      'https://evil.example.com/watch?v=synthetic01',
      'https://www.youtube.com.example.com/watch',
      'https://live.bilibili.com/1',
      withUrl((u) => {
        u.username = 'user'
      }),
      withUrl((u) => {
        u.port = '8443'
      }),
    ]) {
      assert.equal(detectSite(text), null, text)
    }
  })
})

describe('isActiveJob / formatEta', () => {
  it('treats queued, running and importing jobs as active', () => {
    assert.deepEqual(
      (['queued', 'running', 'importing', 'done', 'error', 'canceled'] as const).map((status) => isActiveJob({ status })),
      [true, true, true, false, false, false],
    )
  })

  it('formats remaining time', () => {
    assert.equal(formatEta(-1), '')
    assert.equal(formatEta(65), '1:05')
    assert.equal(formatEta(3725), '1:02:05')
  })
})
