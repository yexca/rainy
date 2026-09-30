/**
 * Shared React Query keys.
 *
 * Every query that shows library data MUST start with one of {@link LIBRARY_QUERY_ROOTS} so that
 * `library` server events (scan finished, tags edited, files moved, …) refresh it. Feature code
 * may append more segments freely, e.g. `['albums', 'list', params]` or `['albums', id]`.
 */
export const queryKeys = {
  /** `/api/auth/status` — owned by `useAuth`. */
  auth: ['auth'] as const,
  /** `/api/playlists` — shared by the sidebar and the add-to-playlist dialog. */
  playlists: ['playlists'] as const,
  playlist: (id: string) => ['playlists', id] as const,
  home: ['home'] as const,
  albums: ['albums'] as const,
  album: (id: string) => ['albums', id] as const,
  artists: ['artists'] as const,
  artist: (id: string) => ['artists', id] as const,
  tracks: ['tracks'] as const,
  track: (id: string) => ['tracks', id] as const,
  genres: ['genres'] as const,
  search: (q: string) => ['search', q] as const,
  starred: ['starred'] as const,
  random: ['random'] as const,
  recentTracks: ['recent-tracks'] as const,
  lyrics: (trackId: string) => ['lyrics', trackId] as const,
  radios: ['radios'] as const,
  queue: ['queue'] as const,
  /** Listening report and history (`['listening', 'report', params]`, `['listening', 'history', range]`). */
  listening: ['listening'] as const,
  /** `/api/me/scrobbling`. */
  scrobbling: ['scrobbling'] as const,
  manage: ['manage'] as const,
  admin: ['admin'] as const,
} as const

/**
 * First key segments of queries derived from the music library; invalidated on `library`
 * server events (see `useServerEvents`). `random` is deliberately excluded so shuffled lists
 * don't reshuffle under the user.
 */
export const LIBRARY_QUERY_ROOTS: ReadonlySet<string> = new Set([
  'home',
  'albums',
  'artists',
  'tracks',
  'genres',
  'search',
  'starred',
  'recent-tracks',
  'listening',
  'playlists',
  'lyrics',
  'manage',
  'admin',
])
