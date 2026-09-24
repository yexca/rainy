/** Library-manager filters, stored in the URL so other pages can deep-link (`/manage?albumId=…`). */
import type { SortOrder, TrackListParams, TrackSort } from '@/lib/api/endpoints'

export type MissingFilter = '' | 'include' | 'only'

export interface ManageFilters {
  q: string
  albumId: string
  artistId: string
  genre: string
  dirPrefix: string
  libraryId: number | undefined
  missing: MissingFilter
  sort: TrackSort
  order: SortOrder
}

export const DEFAULT_SORT: TrackSort = 'path'

const SORTS: ReadonlySet<string> = new Set<TrackSort>([
  'title', 'artist', 'album', 'albumArtist', 'year', 'duration', 'recent', 'updated', 'played', 'frequent',
  'rating', 'starred', 'random', 'path', 'track', 'size', 'bitrate', 'suffix',
])

export function readFilters(sp: URLSearchParams): ManageFilters {
  const lib = Number.parseInt(sp.get('libraryId') ?? '', 10)
  const missing = sp.get('missing')
  const sort = sp.get('sort') ?? ''
  return {
    q: sp.get('q') ?? '',
    albumId: sp.get('albumId') ?? '',
    artistId: sp.get('artistId') ?? '',
    genre: sp.get('genre') ?? '',
    dirPrefix: sp.get('dirPrefix') ?? '',
    libraryId: Number.isFinite(lib) && lib > 0 ? lib : undefined,
    missing: missing === 'include' || missing === 'only' ? missing : '',
    sort: SORTS.has(sort) ? (sort as TrackSort) : DEFAULT_SORT,
    order: sp.get('order') === 'desc' ? 'desc' : 'asc',
  }
}

/** Apply a patch to the URL search params (empty values are removed). */
export function writeFilters(sp: URLSearchParams, patch: Partial<ManageFilters>): URLSearchParams {
  const next = new URLSearchParams(sp)
  for (const [key, value] of Object.entries(patch)) {
    const empty =
      value === undefined || value === '' || (key === 'sort' && value === DEFAULT_SORT) || (key === 'order' && value === 'asc')
    if (empty) next.delete(key)
    else next.set(key, String(value))
  }
  return next
}

export function toTrackParams(f: ManageFilters): TrackListParams {
  return {
    q: f.q || undefined,
    albumId: f.albumId || undefined,
    artistId: f.artistId || undefined,
    genre: f.genre || undefined,
    dirPrefix: f.dirPrefix || undefined,
    libraryId: f.libraryId,
    missing: f.missing || undefined,
    sort: f.sort,
    order: f.order,
  }
}

export function activeFilterCount(f: ManageFilters): number {
  return [f.albumId, f.artistId, f.genre, f.dirPrefix, f.libraryId, f.missing].filter(Boolean).length
}
