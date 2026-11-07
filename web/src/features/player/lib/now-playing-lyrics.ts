/**
 * Whether full-screen Now Playing opens on the lyrics, remembered per surface: the phone sheet
 * (default: artwork) and the tablet / desktop stage (default: lyrics). Toggling the lyrics inside
 * Now Playing updates the surface's choice (`usePlayer().nowPlayingLyrics`, persisted).
 * Without imports so it can be unit-tested (web/tests/now-playing-lyrics.test.ts).
 */

/** `PlayerPanel` from `../types`. */
type PlayerPanel = 'none' | 'queue' | 'lyrics'

export type NowPlayingSurface = 'sheet' | 'stage'

export type NowPlayingLyrics = Record<NowPlayingSurface, boolean>

export const DEFAULT_NOW_PLAYING_LYRICS: NowPlayingLyrics = { sheet: false, stage: true }

/** Phones in landscape are wider than the mobile breakpoint but still get the sheet. */
export const LANDSCAPE_PHONE_QUERY = '(orientation: landscape) and (max-height: 540px)'

export function sanitizeNowPlayingLyrics(input: unknown): NowPlayingLyrics {
  const p = (input && typeof input === 'object' ? input : {}) as Partial<Record<NowPlayingSurface, unknown>>
  return {
    sheet: typeof p.sheet === 'boolean' ? p.sheet : DEFAULT_NOW_PLAYING_LYRICS.sheet,
    stage: typeof p.stage === 'boolean' ? p.stage : DEFAULT_NOW_PLAYING_LYRICS.stage,
  }
}

/**
 * The panel Now Playing opens on: the lyrics when the surface remembers them, otherwise the
 * current panel minus the lyrics. Radio has no lyrics and keeps its panel.
 */
export function openingPanel(current: PlayerPanel, showLyrics: boolean, isRadio: boolean): PlayerPanel {
  if (isRadio) return current
  if (showLyrics) return 'lyrics'
  return current === 'lyrics' ? 'none' : current
}

/** The surface's choice after the user picked `panel` inside Now Playing. */
export function rememberLyrics(prefs: NowPlayingLyrics, surface: NowPlayingSurface, panel: PlayerPanel): NowPlayingLyrics {
  const show = panel === 'lyrics'
  return prefs[surface] === show ? prefs : { ...prefs, [surface]: show }
}
