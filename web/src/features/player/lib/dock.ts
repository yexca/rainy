/**
 * Tablet / desktop player layouts (docs/architecture/contract.md §9.4). Pure helpers so they can
 * be unit tested without a DOM; the persisted store lives in `../dock.ts`.
 *
 * - `window`: a floating Now Playing window in the bottom-right corner.
 * - `bar`: the full-width bottom player bar (Apple Music web style: transport, a centred
 *   "LCD" with artwork, title and progress, then the toggles); `barAutoHide` (its handle's pin,
 *   off = pinned) slides it away (NetEase style) and leaves the handle, which peeks it back on
 *   hover.
 *
 * Full-screen Now Playing is an overlay on top of either (`usePlayer().nowPlayingOpen`); the
 * layout menu lists it first, then these in this order: three layouts in all.
 */

export const DOCK_MODES = ['window', 'bar'] as const
export type DockMode = (typeof DOCK_MODES)[number]

export interface DockPrefs {
  mode: DockMode
  /** Bottom bar is unpinned: it hides itself until hovered (only meaningful in `bar` mode). */
  barAutoHide: boolean
}

export const DEFAULT_DOCK_PREFS: DockPrefs = { mode: 'bar', barAutoHide: false }

/** Unknown layouts (including the retired floating mini bar, `compact`) fall back to the bar. */
export function sanitizeDockPrefs(input: Partial<DockPrefs> | null | undefined): DockPrefs {
  const p = input ?? {}
  return {
    mode: (DOCK_MODES as readonly string[]).includes(p.mode ?? '') ? (p.mode as DockMode) : DEFAULT_DOCK_PREFS.mode,
    barAutoHide: typeof p.barAutoHide === 'boolean' ? p.barAutoHide : DEFAULT_DOCK_PREFS.barAutoHide,
  }
}

/**
 * Value of `<html data-player-dock>`, which `index.css` maps to the space the player chrome
 * reserves (`--player-reserve`, `--player-clearance`, toast offsets). The floating window
 * renders nothing while idle, so it reserves nothing then.
 */
export type DockAttribute = 'bar' | 'collapsed' | 'window' | 'idle'

export function dockAttribute(prefs: DockPrefs, hasTrack: boolean): DockAttribute {
  if (prefs.mode === 'bar') return prefs.barAutoHide ? 'collapsed' : 'bar'
  return hasTrack ? prefs.mode : 'idle'
}

/**
 * Position (0..1) under the pointer on a horizontal progress track, for hover previews and
 * absolute seeks (the Apple Music bar seeks to where the pointer is).
 */
export function pointerRatio(clientX: number, left: number, width: number): number {
  if (!(width > 0)) return 0
  return Math.min(1, Math.max(0, (clientX - left) / width))
}
