/**
 * Pure helpers for the Home page: filtering "Recently added" to genuinely recent imports and
 * picking the active song-pick mode. Kept free of path aliases so `node --test` imports it directly.
 */

/** How long newly added albums stay on Home (14 days). */
export const RECENTLY_ADDED_WINDOW_MS = 14 * 24 * 60 * 60 * 1000

export const PICK_MODES = ['quick', 'onRepeat', 'rediscover'] as const
export type PickMode = (typeof PICK_MODES)[number]

/** Keep only albums added within `windowMs` up to `now`. */
export function recentAlbums<T extends { createdAt: number }>(
  albums: readonly T[],
  now: number = Date.now(),
  windowMs: number = RECENTLY_ADDED_WINDOW_MS,
): T[] {
  const cutoff = now - windowMs
  return albums.filter((album) => album.createdAt > 0 && album.createdAt >= cutoff)
}

/** Non-empty song-pick modes in display order. */
export function availablePickModes<T>(picks: Record<PickMode, readonly T[]> | undefined): PickMode[] {
  if (!picks) return []
  return PICK_MODES.filter((mode) => picks[mode].length > 0)
}

/** Active song-pick mode: `selected` while it has songs, else the first non-empty mode. */
export function resolvePickMode(selected: PickMode, available: readonly PickMode[]): PickMode | null {
  if (available.length === 0) return null
  return available.includes(selected) ? selected : available[0]
}
