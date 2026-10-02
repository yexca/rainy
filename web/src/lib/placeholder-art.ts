import artist from '@/assets/mascot/placeholder-artist.webp'
import cover1 from '@/assets/mascot/placeholder-cover-1.webp'
import cover2 from '@/assets/mascot/placeholder-cover-2.webp'
import cover3 from '@/assets/mascot/placeholder-cover-3.webp'
import playlist from '@/assets/mascot/placeholder-playlist.webp'
import radio from '@/assets/mascot/placeholder-radio.webp'

/** What the missing artwork stands for. */
export type PlaceholderKind = 'cover' | 'artist' | 'playlist' | 'radio'

const COVERS = [cover1, cover2, cover3]
const SINGLE: Record<Exclude<PlaceholderKind, 'cover'>, string> = { artist, playlist, radio }

/**
 * Mascot artwork (384 px square WebP) for a song, album, artist, playlist or station without
 * its own (docs/development/design.md#mascot). Covers come in three variants picked by `seed`
 * (e.g. the title), so a grid of missing covers doesn't repeat one picture.
 */
export function placeholderArt(kind: PlaceholderKind, seed = ''): string {
  if (kind !== 'cover') return SINGLE[kind]
  let hash = 0
  for (const ch of seed) hash = (hash * 31 + (ch.codePointAt(0) ?? 0)) >>> 0
  return COVERS[hash % COVERS.length]
}
