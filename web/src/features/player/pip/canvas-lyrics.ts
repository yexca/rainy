/**
 * Video picture-in-picture lyrics (Safari on macOS, iPhone and iPad, and other browsers without
 * Document Picture-in-Picture): the current lyric line is drawn on a canvas whose stream plays in
 * a muted, nearly invisible <video>, and that video goes picture-in-picture. The video's play /
 * pause buttons control the player. Redraws when the line, the track, the translation setting or
 * the cover colours change, and once a second so the stream keeps producing frames.
 */
import { coverUrlForPixels } from '@/lib/cover'
import i18n from '@/lib/i18n'
import { groupBilingual, type DisplayLine } from '@/lib/lyrics/bilingual'
import { queryClient } from '@/lib/query-client'

import { coverColorQuery, DEFAULT_PALETTE, type CoverPalette } from '../hooks/use-cover-color'
import { activeLineIndex, lyricsQuery } from '../hooks/use-lyrics'
import { pipLyricWindow, wrapText } from '../lib/pip-lyrics'
import { usePlaybackPrefs } from '../prefs'
import { usePlayback, usePlayer } from '../store'
import type { PlayableTrack } from '../types'

const WIDTH = 640
const HEIGHT = 320
const SCALE = 2
const PAD = 28

type LyricsState =
  | { kind: 'loading' }
  | { kind: 'none' | 'radio' | 'plain' | 'error' }
  | { kind: 'synced'; lines: DisplayLine[] }

interface WebKitVideo extends HTMLVideoElement {
  webkitSetPresentationMode?: (mode: string) => void
  webkitPresentationMode?: string
}

export class CanvasLyricsVideo {
  private readonly canvas = document.createElement('canvas')
  private readonly ctx: CanvasRenderingContext2D
  private readonly video = document.createElement('video') as WebKitVideo
  private readonly cleanups: (() => void)[] = []
  private track: PlayableTrack | undefined
  private lyrics: LyricsState = { kind: 'loading' }
  private palette: CoverPalette = DEFAULT_PALETTE
  private cover: HTMLImageElement | null = null
  private active = -2
  private syncing = false
  private stopped = false
  private readonly font = getComputedStyle(document.body).fontFamily || 'sans-serif'

  private readonly onClose: () => void

  constructor(onClose: () => void) {
    this.onClose = onClose
    this.canvas.width = WIDTH * SCALE
    this.canvas.height = HEIGHT * SCALE
    this.ctx = this.canvas.getContext('2d')!
    this.ctx.scale(SCALE, SCALE)
    const v = this.video
    v.muted = true
    v.playsInline = true
    v.setAttribute('aria-hidden', 'true')
    // In the page but invisible: Safari only puts a rendered video into picture-in-picture.
    Object.assign(v.style, { position: 'fixed', left: '0', bottom: '0', width: '2px', height: '1px', opacity: '0.01', pointerEvents: 'none' })
  }

  /** Draws the first frame and enters picture-in-picture (call from a click). */
  async start(): Promise<void> {
    this.setTrack(currentTrack())
    this.draw()
    this.video.srcObject = this.canvas.captureStream()
    document.body.append(this.video)
    this.listen()
    await this.video.play()
    if (this.video.requestPictureInPicture) await this.video.requestPictureInPicture()
    else this.video.webkitSetPresentationMode?.('picture-in-picture')
    this.syncVideo(usePlayer.getState().isPlaying)
  }

  stop(): void {
    if (this.stopped) return
    this.stopped = true
    for (const fn of this.cleanups) fn()
    if (document.pictureInPictureElement === this.video) void document.exitPictureInPicture().catch(() => {})
    else if (this.video.webkitPresentationMode === 'picture-in-picture') this.video.webkitSetPresentationMode?.('inline')
    this.video.pause()
    this.video.srcObject = null
    this.video.remove()
  }

  private listen(): void {
    const v = this.video
    const on = <K extends keyof HTMLVideoElementEventMap>(type: K, fn: () => void) => {
      v.addEventListener(type, fn)
      this.cleanups.push(() => v.removeEventListener(type, fn))
    }
    on('leavepictureinpicture', () => this.close())
    v.addEventListener('webkitpresentationmodechanged', () => {
      if (v.webkitPresentationMode === 'inline') this.close()
    })
    // The window's play / pause buttons drive the player.
    on('play', () => {
      if (!this.syncing && !usePlayer.getState().isPlaying) usePlayer.getState().play()
    })
    on('pause', () => {
      if (!this.syncing && usePlayer.getState().isPlaying) usePlayer.getState().pause()
    })

    this.cleanups.push(
      usePlayer.subscribe((s, prev) => {
        const track = s.index >= 0 ? s.queue[s.index] : undefined
        if (track?.id !== this.track?.id || track?.coverArt !== this.track?.coverArt) {
          this.setTrack(track)
          this.draw()
        }
        if (s.isPlaying !== prev.isPlaying) this.syncVideo(s.isPlaying)
      }),
      usePlayback.subscribe((s) => {
        if (this.lyrics.kind !== 'synced') return
        const next = activeLineIndex(this.lyrics.lines, s.currentTime * 1000)
        if (next !== this.active) {
          this.active = next
          this.draw()
        }
      }),
      usePlaybackPrefs.subscribe((s, prev) => {
        if (s.lyricsTranslation !== prev.lyricsTranslation) this.draw()
      }),
    )
    const heartbeat = setInterval(() => this.draw(), 1000)
    this.cleanups.push(() => clearInterval(heartbeat))
  }

  private close(): void {
    if (this.stopped) return
    this.stop()
    this.onClose()
  }

  /** Mirrors the player in the video so the window shows the right play / pause button. */
  private syncVideo(playing: boolean): void {
    if (this.stopped || playing !== this.video.paused) return
    this.syncing = true
    const done = () => {
      this.syncing = false
    }
    if (playing) void this.video.play().then(done, done)
    else {
      this.video.pause()
      done()
    }
  }

  private setTrack(track: PlayableTrack | undefined): void {
    this.track = track
    this.active = -2
    this.palette = DEFAULT_PALETTE
    this.cover = null
    if (!track) {
      this.lyrics = { kind: 'none' }
      return
    }
    if (track.isRadio) {
      this.lyrics = { kind: 'radio' }
    } else {
      this.lyrics = { kind: 'loading' }
      void queryClient.fetchQuery(lyricsQuery(track)).then(
        (data) => {
          if (this.track?.id !== track.id) return
          this.lyrics =
            data.lines.length === 0
              ? { kind: 'none' }
              : data.synced
                ? { kind: 'synced', lines: groupBilingual(data.lines, true) }
                : { kind: 'plain' }
          this.active = -2
          this.draw()
        },
        () => {
          if (this.track?.id !== track.id) return
          this.lyrics = { kind: 'error' }
          this.draw()
        },
      )
    }
    if (track.coverArt) {
      const coverArt = track.coverArt
      void queryClient.fetchQuery(coverColorQuery(coverArt)).then(
        (palette) => {
          if (this.track?.coverArt !== coverArt) return
          this.palette = palette
          this.draw()
        },
        () => {},
      )
      const img = new Image()
      img.onload = () => {
        if (this.track?.coverArt !== coverArt) return
        this.cover = img
        this.draw()
      }
      img.src = coverUrlForPixels(coverArt, 96)
    }
  }

  private draw(): void {
    if (this.stopped) return
    const { ctx } = this
    const bg = ctx.createLinearGradient(0, 0, 0, HEIGHT)
    bg.addColorStop(0, this.palette.top)
    bg.addColorStop(1, this.palette.bottom)
    ctx.fillStyle = bg
    ctx.fillRect(0, 0, WIDTH, HEIGHT)
    ctx.fillStyle = 'rgba(0,0,0,0.18)'
    ctx.fillRect(0, 0, WIDTH, HEIGHT)
    this.drawHeader()

    const t = i18n.t.bind(i18n)
    const maxWidth = WIDTH - PAD * 2
    if (this.lyrics.kind !== 'synced') {
      const message = {
        loading: '…',
        none: t('player:lyrics.none'),
        radio: t('player:lyrics.radio'),
        plain: t('player:pip.notSynced'),
        error: t('player:lyrics.error'),
      }[this.lyrics.kind]
      this.text([message], 22, 600, 'rgba(255,255,255,0.75)', HEIGHT / 2 + 20, 30)
      return
    }
    if (this.active === -2) this.active = activeLineIndex(this.lyrics.lines, usePlayback.getState().currentTime * 1000)
    const { current, next } = pipLyricWindow(this.lyrics.lines, this.active)
    const translate = usePlaybackPrefs.getState().lyricsTranslation

    ctx.font = this.fontSpec(36, 700)
    const main = current ? wrapText(current.text || '♪', maxWidth, (s) => ctx.measureText(s).width, 2) : ['♪']
    ctx.font = this.fontSpec(22, 600)
    const translation =
      translate && current?.translations[0] ? wrapText(current.translations[0], maxWidth, (s) => ctx.measureText(s).width, 1) : []
    const following = next ? wrapText(next.text, maxWidth, (s) => ctx.measureText(s).width, 1) : []

    const blockHeight = main.length * 44 + translation.length * 36
    let y = 96 + (HEIGHT - 96 - 52 - blockHeight) / 2 + 36
    y = this.text(main, 36, 700, '#fff', y, 44)
    this.text(translation, 22, 600, 'rgba(255,255,255,0.72)', y - 6, 30)
    this.text(following, 22, 600, 'rgba(255,255,255,0.42)', HEIGHT - PAD, 30)
  }

  private drawHeader(): void {
    const { ctx } = this
    const size = 44
    if (this.cover) {
      ctx.save()
      ctx.beginPath()
      ctx.roundRect(PAD, 22, size, size, 8)
      ctx.clip()
      ctx.drawImage(this.cover, PAD, 22, size, size)
      ctx.restore()
    }
    const x = this.cover ? PAD + size + 14 : PAD
    const width = WIDTH - x - PAD
    const track = this.track
    ctx.textAlign = 'left'
    ctx.fillStyle = 'rgba(255,255,255,0.92)'
    ctx.font = this.fontSpec(18, 600)
    ctx.fillText(wrapText(track?.title ?? 'Rainy', width, (s) => ctx.measureText(s).width, 1)[0] ?? '', x, 41)
    ctx.fillStyle = 'rgba(255,255,255,0.6)'
    ctx.font = this.fontSpec(15, 500)
    ctx.fillText(wrapText(track?.artist ?? '', width, (s) => ctx.measureText(s).width, 1)[0] ?? '', x, 63)
  }

  /** Centred lines from baseline `y`; returns the baseline after the last line. */
  private text(lines: string[], size: number, weight: number, color: string, y: number, lineHeight: number): number {
    const { ctx } = this
    ctx.font = this.fontSpec(size, weight)
    ctx.fillStyle = color
    ctx.textAlign = 'center'
    for (const line of lines) {
      ctx.fillText(line, WIDTH / 2, y)
      y += lineHeight
    }
    return y
  }

  private fontSpec(size: number, weight: number): string {
    return `${weight} ${size}px ${this.font}`
  }
}

function currentTrack(): PlayableTrack | undefined {
  const s = usePlayer.getState()
  return s.index >= 0 ? s.queue[s.index] : undefined
}
