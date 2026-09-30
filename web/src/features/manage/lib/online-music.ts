/** Helpers for Tracks → Online (online music search and downloads through lx-music sources). */
// Relative `.ts` import and no runtime imports, so node --test (tests/online-music.test.ts) can load it.
import type { OnlinePlatform, OnlineQualityType, OnlineSong } from '../../../lib/api/types.ts'

/** Platforms in display order (mirrors `lxmusic.Platforms`). */
export const PLATFORMS: readonly OnlinePlatform[] = ['kw', 'kg', 'tx', 'wy', 'mg']

/** Qualities, lowest first (mirrors `lxmusic.Qualities`). */
export const QUALITIES: readonly OnlineQualityType[] = ['128k', '320k', 'flac', 'flac24bit']

export function isPlatform(value: unknown): value is OnlinePlatform {
  return typeof value === 'string' && (PLATFORMS as readonly string[]).includes(value)
}

export function isQuality(value: unknown): value is OnlineQualityType {
  return typeof value === 'string' && (QUALITIES as readonly string[]).includes(value)
}

/** Identifies a search result (Kugou lists one song under several file hashes). */
export function songKey(song: Pick<OnlineSong, 'platform' | 'id' | 'extra'>): string {
  return `${song.platform}:${song.id}:${song.extra.hash ?? ''}`
}

/** The best quality a catalogue lists for a song (`null` when it lists none). */
export function bestQuality(song: Pick<OnlineSong, 'qualities'>): OnlineQualityType | null {
  let best: OnlineQualityType | null = null
  for (const q of song.qualities) {
    if (isQuality(q.type) && (best === null || QUALITIES.indexOf(q.type) > QUALITIES.indexOf(best))) best = q.type
  }
  return best
}

/**
 * The quality a download of `song` will ask for when `want` is the best wanted: the best one up
 * to `want` the song lists, else the lowest above it (like the server, before the sources' own
 * limits apply). `null` when the song lists no quality.
 */
export function expectedQuality(song: Pick<OnlineSong, 'qualities'>, want: OnlineQualityType): OnlineQualityType | null {
  const listed = QUALITIES.filter((q) => song.qualities.some((s) => s.type === q))
  const limit = QUALITIES.indexOf(want)
  const atOrBelow = listed.filter((q) => QUALITIES.indexOf(q) <= limit)
  return atOrBelow.at(-1) ?? listed[0] ?? null
}

/** Songs without repeats (a later page may repeat an earlier result). */
export function uniqueSongs(songs: readonly OnlineSong[]): OnlineSong[] {
  const seen = new Set<string>()
  const out: OnlineSong[] = []
  for (const song of songs) {
    const key = songKey(song)
    if (seen.has(key)) continue
    seen.add(key)
    out.push(song)
  }
  return out
}

/** Splits a list into parts of at most `size` items. */
export function chunk<T>(list: readonly T[], size: number): T[][] {
  const out: T[][] = []
  for (let i = 0; i < list.length; i += size) out.push(list.slice(i, i + size))
  return out
}
