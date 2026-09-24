/**
 * Helpers for editing playlists.
 *
 * The server's playlist views skip tracks whose files are missing. `PUT /api/playlists/{id}/tracks`
 * replaces the *visible* order and the server keeps those hidden entries next to the rows they
 * followed, so an edit is a single atomic request with the full new order.
 */
import type { PlaylistDetail, Track } from '@/lib/api/types'

export interface PlaylistEditPlan {
  /** The complete new visible order. */
  order: string[]
}

/**
 * Plan an edit. `kept` lists, in the new order, the original (visible) index of every row that is
 * still in the playlist; `trackIds` are the matching ids. Returns `null` when nothing changed.
 */
export function planPlaylistEdit(
  originalLength: number,
  kept: readonly number[],
  trackIds: readonly string[],
): PlaylistEditPlan | null {
  const unchanged = kept.length === originalLength && kept.every((index, i) => index === i)
  return unchanged ? null : { order: [...trackIds] }
}

/** Run a plan against the API. */
export async function applyPlaylistEdit(
  plan: PlaylistEditPlan,
  ops: { replace: (trackIds: string[]) => Promise<unknown> },
): Promise<void> {
  await ops.replace(plan.order)
}

/** The playlist detail showing `tracks` (the new visible list) with count + duration adjusted. */
export function withTracks(detail: PlaylistDetail, tracks: Track[]): PlaylistDetail {
  const sum = (list: readonly Track[]) => list.reduce((total, t) => total + (t.duration || 0), 0)
  const duration = Math.max(0, detail.duration - sum(detail.tracks) + sum(tracks))
  const songCount = Math.max(0, detail.songCount - detail.tracks.length + tracks.length)
  return { ...detail, tracks, songCount, duration }
}

/** The playlist detail without the rows at `positions` (visible-list indexes). */
export function withoutPositions(detail: PlaylistDetail, positions: readonly number[]): PlaylistDetail {
  const drop = new Set(positions)
  const tracks = detail.tracks.filter((_, i) => !drop.has(i))
  if (tracks.length === detail.tracks.length) return detail
  return withTracks(detail, tracks)
}
