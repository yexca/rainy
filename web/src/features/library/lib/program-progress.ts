/**
 * Progress through a program show's episodes (docs/architecture/contract.md §9.5a). An episode
 * with a bookmark is partly played; one played before without a bookmark is played (the player
 * removes the bookmark once an episode finishes); anything else is unplayed. Pure functions
 * without imports so they can be unit-tested (web/tests/program-progress.test.ts).
 */

export type EpisodeState = 'played' | 'partial' | 'unplayed'

/** The fields of an episode these helpers read. */
export interface EpisodeItem {
  id: string
  playCount: number
  /** Seconds; 0 = unknown. */
  duration: number
}

export interface EpisodeProgress {
  state: EpisodeState
  /** 0–1 for a partly played episode with a known duration, else 0. */
  fraction: number
  /** Seconds left of a partly played episode (0 when unknown). */
  remaining: number
}

export function episodeProgress(episode: EpisodeItem, positionMs: number | undefined): EpisodeProgress {
  if (positionMs !== undefined) {
    const position = positionMs / 1000
    const known = episode.duration > 0
    return {
      state: 'partial',
      fraction: known ? Math.min(1, Math.max(0, position / episode.duration)) : 0,
      remaining: known ? Math.max(0, episode.duration - position) : 0,
    }
  }
  return { state: episode.playCount > 0 ? 'played' : 'unplayed', fraction: 0, remaining: 0 }
}

/**
 * Where "Continue" starts: the first episode in show order that is not played (partly played or
 * new), or the first episode when every one is played. -1 for a show without episodes.
 */
export function continueIndex(episodes: readonly EpisodeItem[], positions: ReadonlyMap<string, number>): number {
  if (episodes.length === 0) return -1
  const next = episodes.findIndex((e) => episodeProgress(e, positions.get(e.id)).state !== 'played')
  return next >= 0 ? next : 0
}

/** True once the listener started the show (a played or partly played episode). */
export function hasProgress(episodes: readonly EpisodeItem[], positions: ReadonlyMap<string, number>): boolean {
  return episodes.some((e) => episodeProgress(e, positions.get(e.id)).state !== 'unplayed')
}
