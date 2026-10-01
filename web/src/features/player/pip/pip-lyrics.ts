/**
 * Picture-in-picture lyrics (docs/architecture/contract.md §9.4): a small always-on-top window
 * with the line being sung, so lyrics stay visible over other apps.
 *
 * - `document`: Document Picture-in-Picture (Chromium desktop). The window is a real document;
 *   `<PipLyricsHost/>` renders `PipLyricsWindow` into it through a portal, with the app's styles
 *   copied over.
 * - `video`: elsewhere with standard video picture-in-picture (Safari on macOS, iPhone and iPad):
 *   the lyrics are drawn on a canvas that plays as a muted video (`CanvasLyricsVideo`).
 *
 * Opening must run inside the click that asked for it (browsers require a user gesture).
 */
import { toast } from 'sonner'
import { create } from 'zustand'

import i18n from '@/lib/i18n'

import { CanvasLyricsVideo } from './canvas-lyrics'

export type PipLyricsMode = 'document' | 'video'

interface PipLyricsState {
  mode: PipLyricsMode | null
  /** The Document Picture-in-Picture window (document mode). */
  window: Window | null
}

export const usePipLyrics = create<PipLyricsState>()(() => ({ mode: null, window: null }))

interface DocumentPictureInPicture {
  requestWindow(options?: { width?: number; height?: number }): Promise<Window>
}

function documentPip(): DocumentPictureInPicture | undefined {
  return (window as unknown as { documentPictureInPicture?: DocumentPictureInPicture }).documentPictureInPicture
}

interface WebKitVideo extends HTMLVideoElement {
  webkitSupportsPresentationMode?: (mode: string) => boolean
  webkitSetPresentationMode?: (mode: string) => void
}

/** Which picture-in-picture this browser can show lyrics in (`null`: none, e.g. Firefox). */
export function pipLyricsSupport(): PipLyricsMode | null {
  if (typeof window === 'undefined') return null
  if (documentPip() && !documentUnavailable) return 'document'
  return videoPipSupported() ? 'video' : null
}

function videoPipSupported(): boolean {
  if (typeof HTMLCanvasElement === 'undefined' || typeof HTMLCanvasElement.prototype.captureStream !== 'function') return false
  if (document.pictureInPictureEnabled) return true
  const video = document.createElement('video') as WebKitVideo
  return !!video.webkitSupportsPresentationMode?.('picture-in-picture')
}

let canvasVideo: CanvasLyricsVideo | null = null
/** Document Picture-in-Picture failed to open a window here (an embedded browser): use the video. */
let documentUnavailable = false
let stopThemeSync: (() => void) | null = null

/** Opens the lyrics window, or closes it when open. Call from a click handler. */
export function togglePipLyrics(): void {
  if (usePipLyrics.getState().mode) {
    closePipLyrics()
    return
  }
  const mode = pipLyricsSupport()
  const fail = () => {
    closePipLyrics()
    toast.error(i18n.t('player:pip.failed'), {
      description: documentUnavailable ? i18n.t('player:pip.tryAgain') : undefined,
    })
  }
  if (mode === 'document') {
    // Embedded browsers may offer the API without being able to open windows: use the video
    // from now on (this click's activation may be spent, so the next click opens it).
    openDocument().catch(() => {
      documentUnavailable = videoPipSupported()
      if (documentUnavailable) openVideo().catch(fail)
      else fail()
    })
  } else if (mode === 'video') {
    openVideo().catch(fail)
  }
}

export function closePipLyrics(): void {
  const { window: win } = usePipLyrics.getState()
  usePipLyrics.setState({ mode: null, window: null })
  stopThemeSync?.()
  stopThemeSync = null
  win?.close()
  canvasVideo?.stop()
  canvasVideo = null
}

async function openDocument(): Promise<void> {
  const pip = await documentPip()!.requestWindow({ width: 440, height: 220 })
  pip.document.title = i18n.t('player:pip.title')
  copyStyles(document, pip.document)
  stopThemeSync = syncRoot(document.documentElement, pip.document.documentElement)
  pip.document.body.style.overflow = 'hidden'
  pip.addEventListener(
    'pagehide',
    () => {
      if (usePipLyrics.getState().window === pip) closePipLyrics()
    },
    { once: true },
  )
  usePipLyrics.setState({ mode: 'document', window: pip })
}

async function openVideo(): Promise<void> {
  canvasVideo = new CanvasLyricsVideo(() => {
    if (usePipLyrics.getState().mode === 'video') closePipLyrics()
  })
  usePipLyrics.setState({ mode: 'video', window: null })
  await canvasVideo.start()
}

/** Copies the app's stylesheets (Tailwind, fonts) into the picture-in-picture document. */
function copyStyles(from: Document, to: Document): void {
  for (const sheet of Array.from(from.styleSheets)) {
    try {
      const style = to.createElement('style')
      style.textContent = Array.from(sheet.cssRules, (rule) => rule.cssText).join('\n')
      to.head.append(style)
    } catch {
      // A cross-origin sheet can't be read: link it instead.
      if (!sheet.href) continue
      const link = to.createElement('link')
      link.rel = 'stylesheet'
      link.href = sheet.href
      to.head.append(link)
    }
  }
}

/** Keeps the window's <html> theme (dark class, accent, colour scheme, language) in step. */
function syncRoot(from: HTMLElement, to: HTMLElement): () => void {
  const copy = () => {
    to.className = from.className
    to.lang = from.lang
    to.style.colorScheme = from.style.colorScheme
    for (const [key, value] of Object.entries(from.dataset)) to.dataset[key] = value
  }
  copy()
  const observer = new MutationObserver(copy)
  observer.observe(from, { attributes: true, attributeFilter: ['class', 'lang', 'style', 'data-accent'] })
  return () => observer.disconnect()
}
