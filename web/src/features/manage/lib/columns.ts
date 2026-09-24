/** Column metadata of the library manager table (rendering lives in `track-table.tsx`). */
import type { TrackSort } from '@/lib/api/endpoints'

export type ColumnId =
  | 'cover'
  | 'title'
  | 'artist'
  | 'album'
  | 'albumArtist'
  | 'track'
  | 'disc'
  | 'year'
  | 'genre'
  | 'format'
  | 'bitrate'
  | 'duration'
  | 'size'
  | 'path'
  | 'updated'

export interface ColumnDef {
  id: ColumnId
  /** Sort key sent to `/api/tracks` (unsortable columns omit it). */
  sort?: TrackSort
  /** CSS grid track. */
  width: string
  align?: 'right'
  defaultVisible: boolean
  /** Cannot be hidden. */
  fixed?: boolean
}

export const COLUMNS: readonly ColumnDef[] = [
  { id: 'cover', width: '36px', defaultVisible: true },
  { id: 'title', sort: 'title', width: 'minmax(10rem,2.2fr)', defaultVisible: true, fixed: true },
  { id: 'artist', sort: 'artist', width: 'minmax(7rem,1.3fr)', defaultVisible: true },
  { id: 'album', sort: 'album', width: 'minmax(7rem,1.3fr)', defaultVisible: true },
  { id: 'albumArtist', sort: 'albumArtist', width: 'minmax(7rem,1.1fr)', defaultVisible: false },
  { id: 'track', sort: 'track', width: '3rem', align: 'right', defaultVisible: true },
  { id: 'disc', width: '3rem', align: 'right', defaultVisible: false },
  { id: 'year', sort: 'year', width: '3.5rem', align: 'right', defaultVisible: true },
  { id: 'genre', width: 'minmax(5rem,0.8fr)', defaultVisible: false },
  { id: 'format', sort: 'suffix', width: '4rem', defaultVisible: true },
  { id: 'bitrate', sort: 'bitrate', width: '5.5rem', align: 'right', defaultVisible: false },
  { id: 'duration', sort: 'duration', width: '4rem', align: 'right', defaultVisible: true },
  { id: 'size', sort: 'size', width: '5rem', align: 'right', defaultVisible: false },
  { id: 'path', sort: 'path', width: 'minmax(10rem,2fr)', defaultVisible: false },
  { id: 'updated', sort: 'updated', width: '7rem', align: 'right', defaultVisible: false },
]

const STORAGE_KEY = 'rainy.manage.columns'

export function loadVisibleColumns(): ReadonlySet<ColumnId> {
  const defaults = new Set(COLUMNS.filter((c) => c.defaultVisible).map((c) => c.id))
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return defaults
    const parsed: unknown = JSON.parse(raw)
    if (!Array.isArray(parsed)) return defaults
    const known = new Set<string>(COLUMNS.map((c) => c.id))
    const ids = parsed.filter((v): v is ColumnId => typeof v === 'string' && known.has(v))
    const set = new Set(ids)
    for (const c of COLUMNS) if (c.fixed) set.add(c.id)
    return set
  } catch {
    return defaults
  }
}

export function saveVisibleColumns(columns: ReadonlySet<ColumnId>): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify([...columns]))
  } catch {
    // storage unavailable (private mode) — the choice lasts for this session only
  }
}
