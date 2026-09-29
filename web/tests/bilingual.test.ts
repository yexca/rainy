// Run with `pnpm test` (node --test). Synthetic lyrics only.
import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import {
  analyzeBilingual,
  groupBilingual,
  guessLang,
  splitBilingualLine,
  toPairedLrc,
} from '../src/lib/lyrics/bilingual.ts'

describe('splitBilingualLine', () => {
  it('splits Japanese, Korean and English originals from a Chinese translation', () => {
    assert.deepEqual(splitBilingualLine('君の名前を呼んだ 我呼唤了你的名字'), {
      original: '君の名前を呼んだ',
      translation: '我呼唤了你的名字',
    })
    assert.deepEqual(splitBilingualLine('사랑해 我爱你'), { original: '사랑해', translation: '我爱你' })
    assert.deepEqual(splitBilingualLine('Hello, my old friend 你好，老朋友'), {
      original: 'Hello, my old friend',
      translation: '你好，老朋友',
    })
  })

  it('keeps spaces inside the translation and kanji words inside the original', () => {
    assert.deepEqual(splitBilingualLine('夢 を見た 做了 一个梦'), { original: '夢 を見た', translation: '做了 一个梦' })
    assert.deepEqual(splitBilingualLine('Route 66 66号公路'), { original: 'Route 66', translation: '66号公路' })
    assert.deepEqual(splitBilingualLine('La la la ♪ 啦啦啦'), { original: 'La la la ♪', translation: '啦啦啦' })
  })

  it('accepts a few Latin letters inside the Chinese translation', () => {
    assert.deepEqual(splitBilingualLine('白いTシャツを着て　出かけよう 穿上白色T恤出发吧'), {
      original: '白いTシャツを着て 出かけよう',
      translation: '穿上白色T恤出发吧',
    })
  })

  it('splits a Latin original followed directly by a bracketed translation', () => {
    assert.deepEqual(splitBilingualLine('Deserts were oceans once「沙漠曾是海洋」'), {
      original: 'Deserts were oceans once',
      translation: '「沙漠曾是海洋」',
    })
    assert.deepEqual(splitBilingualLine('사랑해我爱你'), { original: '사랑해', translation: '我爱你' })
  })

  it('leaves monolingual and ambiguous lines alone', () => {
    for (const line of [
      '我们的 love story',
      '我爱你 baby 我的BB宝贝',
      '我却背着我的ABC',
      '君の声が聞こえた我听见了你的声音',
      'Song - 某某 (ソング)',
      '【Singer】',
      'Baby 我爱你 baby',
      '作词：某某',
      '夢の中へ 夢の中へ',
      'Just an English line',
      'Café au lait',
      '你好 世界',
      '',
      '♪',
    ]) {
      assert.equal(splitBilingualLine(line), null, line)
    }
  })
})

describe('analyzeBilingual', () => {
  it('detects space-separated LRC and counts timed lines', () => {
    const text = [
      '[ti:Synthetic]',
      '[00:01.00]作词：某某',
      '[00:10.00]Hold my hand 牵着我的手',
      '[00:15.50]Never let go 永不放开',
      '[00:20.00]Oh oh oh',
      '[00:25.00]Into the night 走进夜色',
    ].join('\n')
    const a = analyzeBilingual(text)
    assert.equal(a.detected, true)
    assert.equal(a.lines.length, 3)
    assert.equal(a.candidates, 4)
    assert.equal(a.timed, 3)
    assert.equal(a.paired, 0)
  })

  it('does not flag a Japanese song whose lines only sometimes end in a kanji word', () => {
    const text = [
      '[00:01.00]愛してる 永遠',
      '[00:05.00]君と歩いた道',
      '[00:09.00]忘れないで',
      '[00:13.00]星が綺麗だね',
      '[00:17.00]また明日',
    ].join('\n')
    assert.equal(analyzeBilingual(text).detected, false)
  })

  it('does not flag Chinese songs with English words or plain English songs', () => {
    assert.equal(analyzeBilingual('我们的 love story\n你是我的 baby\n一起 dancing tonight').detected, false)
    assert.equal(analyzeBilingual('One line\nAnother line\nThird line').detected, false)
  })

  it('reports lines that are already paired', () => {
    const a = analyzeBilingual('[00:10.00]Hold my hand\n[00:10.00]牵着我的手\n[00:15.00]Let go\n[00:15.00]放开')
    assert.equal(a.paired, 2)
    assert.equal(a.detected, false)
  })
})

describe('toPairedLrc', () => {
  it('rewrites timed bilingual lines into same-timestamp pairs and keeps everything else', () => {
    const text = [
      '[ar:Someone]',
      '[00:10.00][01:10.00]Hold my hand 牵着我的手',
      '[00:15.50]Oh oh oh',
      '[00:20.00]Into the night 走进夜色',
      'Untimed line 未计时',
    ].join('\r\n')
    assert.equal(
      toPairedLrc(text),
      [
        '[ar:Someone]',
        '[00:10.00][01:10.00]Hold my hand',
        '[00:10.00][01:10.00]牵着我的手',
        '[00:15.50]Oh oh oh',
        '[00:20.00]Into the night',
        '[00:20.00]走进夜色',
        'Untimed line 未计时',
      ].join('\r\n'),
    )
  })

  it('returns the text unchanged when nothing timed splits', () => {
    const text = 'Plain line 普通行\nAnother 另一行'
    assert.equal(toPairedLrc(text), text)
  })
})

describe('groupBilingual', () => {
  it('pairs consecutive lines with the same start', () => {
    const lines = [
      { start: 1000, text: 'Hold my hand' },
      { start: 1000, text: '牵着我的手' },
      { start: 2000, text: '' },
      { start: 3000, text: 'Chorus' },
      { start: 3000, text: 'Chorus' },
    ]
    assert.deepEqual(groupBilingual(lines, true), [
      { start: 1000, text: 'Hold my hand', translations: ['牵着我的手'] },
      { start: 2000, text: '', translations: [] },
      { start: 3000, text: 'Chorus', translations: [] },
    ])
  })

  it('splits space-separated lines when most lines are bilingual', () => {
    const lines = [
      { start: -1, text: 'Hold my hand 牵着我的手' },
      { start: -1, text: 'Never let go 永不放开' },
      { start: -1, text: '' },
      { start: -1, text: 'Oh oh oh' },
    ]
    assert.deepEqual(groupBilingual(lines, false), [
      { start: -1, text: 'Hold my hand', translations: ['牵着我的手'] },
      { start: -1, text: 'Never let go', translations: ['永不放开'] },
      { start: -1, text: '', translations: [] },
      { start: -1, text: 'Oh oh oh', translations: [] },
    ])
  })

  it('never pairs plain lines and never splits a mostly monolingual text', () => {
    const plain = [
      { start: -1, text: 'Same' },
      { start: -1, text: 'Same' },
    ]
    assert.equal(groupBilingual(plain, false).length, 2)
    const japanese = [
      { start: 0, text: '愛してる 永遠' },
      { start: 1000, text: '君と歩いた道' },
      { start: 2000, text: '忘れないで' },
    ]
    assert.deepEqual(
      groupBilingual(japanese, true).map((l) => l.translations),
      [[], [], []],
    )
  })
})

describe('guessLang', () => {
  it('maps scripts to language hints', () => {
    assert.equal(guessLang('君の名前'), 'ja')
    assert.equal(guessLang('사랑해'), 'ko')
    assert.equal(guessLang('你的名字'), 'zh')
    assert.equal(guessLang('Hello'), undefined)
    assert.equal(guessLang('我们的 love'), undefined)
  })
})
