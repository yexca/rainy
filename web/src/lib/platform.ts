/**
 * Platform detection (evaluated once at startup). Prefer feature detection where possible;
 * these flags are for platform quirks (iOS audio volume, safe areas, PWA install hints, …).
 */

const nav: Navigator | undefined = typeof navigator === 'undefined' ? undefined : navigator
const ua = nav?.userAgent ?? ''

function matches(query: string): boolean {
  return typeof window !== 'undefined' && typeof window.matchMedia === 'function' && window.matchMedia(query).matches
}

/** iPhone / iPod / iPad — including iPadOS, which reports itself as a Mac with touch. */
export const isIOS: boolean = /iPad|iPhone|iPod/.test(ua) || (/Macintosh/.test(ua) && (nav?.maxTouchPoints ?? 0) > 1)

export const isAndroid: boolean = /Android/i.test(ua)

/** Safari engine on any Apple platform (not Chrome/Firefox/Edge wrappers on macOS). */
export const isSafari: boolean = /^((?!chrome|chromium|android|crios|fxios|edgios|edg\/).)*safari/i.test(ua)

export const isMac: boolean = /Macintosh|Mac OS X/.test(ua) && !isIOS

/** Primary input can touch (phones, tablets, touch laptops). */
export const isTouch: boolean =
  typeof window !== 'undefined' && ('ontouchstart' in window || (nav?.maxTouchPoints ?? 0) > 0)

/** Running as an installed PWA (home-screen app). */
export const isStandalone: boolean =
  matches('(display-mode: standalone)') ||
  matches('(display-mode: fullscreen)') ||
  (nav as (Navigator & { standalone?: boolean }) | undefined)?.standalone === true
