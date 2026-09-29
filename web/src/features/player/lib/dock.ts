/**
 * Tablet / desktop player layouts (docs/architecture/contract.md §9.4). Pure helpers so they can
 * be unit tested without a DOM; the persisted store lives in `../dock.ts`.
 *
 * - `bar`: the full-width bottom player bar; `barAutoHide` slides it away (NetEase style) and
 *   leaves a handle that peeks it back on hover or pins it on click.
 * - `window`: a floating Now Playing window in the bottom-right corner.
 * - `compact`: a floating mini bar in the bottom-right corner.
 *
 * Full-screen Now Playing is an overlay on top of any of them (`usePlayer().nowPlayingOpen`).
 */

export const DOCK_MODES = ['bar', 'window', 'compact'] as const
export type DockMode = (typeof DOCK_MODES)[number]

export interface DockPrefs {
  mode: DockMode
  /** Bottom bar hides itself until hovered (only meaningful in `bar` mode). */
  barAutoHide: boolean
}

export const DEFAULT_DOCK_PREFS: DockPrefs = { mode: 'bar', barAutoHide: false }

export function sanitizeDockPrefs(input: Partial<DockPrefs> | null | undefined): DockPrefs {
  const p = input ?? {}
  return {
    mode: (DOCK_MODES as readonly string[]).includes(p.mode ?? '') ? (p.mode as DockMode) : DEFAULT_DOCK_PREFS.mode,
    barAutoHide: typeof p.barAutoHide === 'boolean' ? p.barAutoHide : DEFAULT_DOCK_PREFS.barAutoHide,
  }
}

/**
 * Value of `<html data-player-dock>`, which `index.css` maps to the space the player chrome
 * reserves (`--player-reserve`, `--player-clearance`, toast offsets). Floating modes render
 * nothing while idle, so they reserve nothing then.
 */
export type DockAttribute = 'bar' | 'collapsed' | 'window' | 'compact' | 'idle'

export function dockAttribute(prefs: DockPrefs, hasTrack: boolean): DockAttribute {
  if (prefs.mode === 'bar') return prefs.barAutoHide ? 'collapsed' : 'bar'
  return hasTrack ? prefs.mode : 'idle'
}

/** Pointer travel (px) before a press on the mini bar becomes a scrub instead of a tap. */
export const COMPACT_SCRUB_THRESHOLD = 7

/**
 * Relative scrub on the floating mini bar: a drag across its full width moves 20 % of the
 * track, bounded between 20 seconds and 10 minutes, clamped to the track.
 */
export function compactScrubTarget(originTime: number, deltaX: number, width: number, duration: number): number {
  if (!(duration > 0)) return 0
  const span = Math.min(600, Math.max(20, duration * 0.2))
  const target = originTime + (deltaX / Math.max(1, width)) * span
  return Math.min(duration, Math.max(0, target))
}
