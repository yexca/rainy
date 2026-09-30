/**
 * Typed functions for every native API endpoint (docs/architecture/contract.md §7).
 *
 * Conventions:
 * - Areas with several endpoints are namespaces (`api.albums.list`, `api.albums.get`), single
 *   endpoints are plain functions (`api.home()`, `api.search({q})`).
 * - GET functions take an optional trailing {@link CallOptions} so React Query's `signal` can be
 *   passed through: `queryFn: ({ signal }) => api.albums.get(id, { signal })`.
 * - URL builders (`streamUrl`, `downloadUrl`, …) return same-origin paths for `<audio>`/`<a>`.
 *   Cover images: use `coverUrl` from `@/lib/cover`.
 */
import { apiUrl, request, seg, upload, type CallOptions, type UploadProgress } from './client'
import type {
  Album,
  AlbumDetail,
  Artist,
  ArtistDetail,
  AuthStatus,
  BatchResult,
  CookieSaveResult,
  DownloadFormat,
  DownloadJob,
  DownloadSite,
  DownloadsStatus,
  EditLogEntry,
  EncodingFix,
  FolderListing,
  Genre,
  Home,
  Issue,
  IssueType,
  LibraryInfo,
  LibraryStats,
  Lyrics,
  MetadataLyrics,
  MetadataProviderId,
  MetadataResult,
  MetadataStatus,
  Page,
  PlayQueue,
  Playlist,
  PlaylistDetail,
  RadioStation,
  RenamePlan,
  ScanStatus,
  SearchResult,
  Settings,
  StarType,
  SystemInfo,
  TagEdit,
  Track,
  TrackTags,
  TrashEntry,
  User,
  YtdlpInfo,
} from './types'

export type { CallOptions, UploadProgress } from './client'

// ---------------------------------------------------------------------------------------------
// Parameter & body types
// ---------------------------------------------------------------------------------------------

export type SortOrder = 'asc' | 'desc'

export interface PageParams {
  offset?: number
  /** Default 50, max 1000. */
  limit?: number
}

export type AlbumSort =
  | 'name' | 'artist' | 'year' | 'recent' | 'random' | 'played' | 'frequent' | 'starred' | 'rating'
  | 'songCount' | 'duration'

export interface AlbumListParams extends PageParams {
  q?: string
  sort?: AlbumSort
  order?: SortOrder
  artistId?: string
  genre?: string
  fromYear?: number
  toYear?: number
  starred?: boolean
  libraryId?: number
}

export type ArtistSort = 'name' | 'albumCount' | 'songCount' | 'played' | 'frequent' | 'random' | 'recent'

export interface ArtistListParams extends PageParams {
  q?: string
  sort?: ArtistSort
  order?: SortOrder
  /** Include artists without albums of their own (track artists only). */
  all?: boolean
  starred?: boolean
}

export type TrackSort =
  | 'title' | 'artist' | 'album' | 'albumArtist' | 'year' | 'duration' | 'recent' | 'updated' | 'played'
  | 'frequent' | 'rating' | 'starred' | 'random' | 'path' | 'track' | 'size' | 'bitrate' | 'suffix'

export interface TrackListParams extends PageParams {
  q?: string
  sort?: TrackSort
  order?: SortOrder
  albumId?: string
  artistId?: string
  genre?: string
  starred?: boolean
  fromYear?: number
  toYear?: number
  libraryId?: number
  dir?: string
  dirPrefix?: string
  /** Managers only: include missing tracks, or list only missing ones. */
  missing?: 'include' | 'only'
  ids?: string[]
}

export interface SearchParams {
  q: string
  /** Max artists (default 6). */
  artists?: number
  /** Max albums (default 12). */
  albums?: number
  /** Max tracks (default 30). */
  tracks?: number
}

export interface RandomParams {
  /** Default 50, max 500. */
  size?: number
  genre?: string
  fromYear?: number
  toYear?: number
}

export interface Credentials { username: string; password: string }
export interface UpdateMeInput { displayName?: string; email?: string }
export interface ChangePasswordInput { currentPassword: string; newPassword: string }

export interface StarInput { type: StarType; ids: string[]; starred: boolean }
export interface RatingInput { type: StarType; id: string; /** 0 clears, 1–5. */ rating: number }
export interface ScrobbleInput { trackId: string; submission: boolean; /** unix ms */ time?: number }

/** `/api/starred` response (same shape as a search result). */
export type Starred = SearchResult

export interface CreatePlaylistInput { name: string; comment?: string; public?: boolean; trackIds?: string[] }
export interface UpdatePlaylistInput { name?: string; comment?: string; public?: boolean }

export interface SaveQueueInput {
  trackIds: string[]
  currentId: string
  positionMs: number
  /** Index of the current entry in `trackIds` (disambiguates a song queued twice). */
  currentIndex?: number
}

export interface RadioInput { name: string; streamUrl: string; homepageUrl?: string }

export type StreamFormat = 'raw' | 'mp3' | 'opus' | 'aac'
export interface StreamOptions {
  /** `raw` (or omitted) streams the original file unless `bitrate` forces a transcode. */
  format?: StreamFormat
  /** kbps */
  bitrate?: number
  /** Start offset in seconds (transcoded streams). */
  offset?: number
}

// ---- manage
export interface SetCoverInput {
  file: Blob
  /** File name sent with the upload (defaults to `File.name` or `cover.jpg`). */
  fileName?: string
  trackIds?: string[]
  albumId?: string
  /** Embed into the audio files (default true). */
  embed?: boolean
  /** Also save as the folder image (default false). */
  saveToFolder?: boolean
}
export interface RemoveCoverInput { trackIds?: string[]; albumId?: string; removeFolderImage?: boolean }
export interface SetLyricsInput { /** Empty text removes the lyrics. */ text: string; target: 'embedded' | 'lrc' }
export interface RenameInput { trackIds: string[]; pattern: string }

/** A file to upload; `path` may contain sub-directories (folder uploads). */
export type UploadItem = File | { file: Blob; path: string }
export interface UploadInput {
  files: UploadItem[]
  libraryId: number
  /** Target directory (library-relative). */
  dir?: string
  /** Place files by the rename pattern instead of `dir`. */
  organize?: boolean
}
export interface UploadCallOptions {
  signal?: AbortSignal
  onProgress?: (progress: UploadProgress) => void
}

export interface FolderParams { libraryId: number; dir?: string }
export interface IssuesParams extends PageParams { type: IssueType }
export interface EncodingInput { trackIds: string[]; apply: boolean }
export interface EncodingResult { items: EncodingFix[]; result?: BatchResult }
export interface EditLogParams extends PageParams { trackId?: string }
export interface Purged { purged: number }
export interface StartDownloadInput {
  /** A YouTube or bilibili link (share text containing one is accepted). */
  url: string
  libraryId: number
  dir?: string
  organize?: boolean
  format?: DownloadFormat
  /** Download the whole playlist the link belongs to (at most 100 entries). */
  playlist?: boolean
}
export interface MetadataSearchParams { provider: MetadataProviderId; q: string; limit?: number; region?: string }

// ---- admin
export interface CreateUserInput {
  username: string
  password: string
  displayName?: string
  email?: string
  isAdmin?: boolean
  canManage?: boolean
  canDownload?: boolean
}
export type UpdateUserInput = Partial<CreateUserInput>
export interface LibraryInput { name: string; path: string }
export interface StartScanInput { full?: boolean; libraryId?: number }

// ---------------------------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------------------------

function uploadName(item: UploadItem): [Blob, string] {
  if (item instanceof File) return [item, item.webkitRelativePath || item.name]
  return [item.file, item.path]
}

// ---------------------------------------------------------------------------------------------
// API
// ---------------------------------------------------------------------------------------------

const noAuthRedirect = { skipAuthHandler: true } as const

export const api = {
  // ---- §7.1 auth & me
  auth: {
    status: (opts?: CallOptions) => request<AuthStatus>('/auth/status', { ...opts, ...noAuthRedirect }),
    setup: (body: Credentials) => request<User>('/auth/setup', { method: 'POST', body, ...noAuthRedirect }),
    login: (body: Credentials) => request<User>('/auth/login', { method: 'POST', body, ...noAuthRedirect }),
    logout: () => request<void>('/auth/logout', { method: 'POST', ...noAuthRedirect }),
  },
  me: {
    get: (opts?: CallOptions) => request<User>('/me', opts),
    update: (body: UpdateMeInput) => request<User>('/me', { method: 'PUT', body }),
    // A wrong current password must not log the user out.
    changePassword: (body: ChangePasswordInput) =>
      request<void>('/me/password', { method: 'PUT', body, ...noAuthRedirect }),
    createApiKey: () => request<{ apiKey: string }>('/me/apikey', { method: 'POST' }),
    deleteApiKey: () => request<void>('/me/apikey', { method: 'DELETE' }),
  },

  // ---- §7.2 library
  home: (opts?: CallOptions) => request<Home>('/home', opts),
  albums: {
    list: (params: AlbumListParams = {}, opts?: CallOptions) =>
      request<Page<Album>>('/albums', { ...opts, query: { ...params } }),
    get: (id: string, opts?: CallOptions) => request<AlbumDetail>(`/albums/${seg(id)}`, opts),
  },
  artists: {
    list: (params: ArtistListParams = {}, opts?: CallOptions) =>
      request<Page<Artist>>('/artists', { ...opts, query: { ...params } }),
    get: (id: string, opts?: CallOptions) => request<ArtistDetail>(`/artists/${seg(id)}`, opts),
  },
  tracks: {
    list: (params: TrackListParams = {}, opts?: CallOptions) =>
      request<Page<Track>>('/tracks', { ...opts, query: { ...params } }),
    get: (id: string, opts?: CallOptions) => request<Track>(`/tracks/${seg(id)}`, opts),
  },
  genres: (opts?: CallOptions) => request<Genre[]>('/genres', opts),
  search: (params: SearchParams, opts?: CallOptions) =>
    request<SearchResult>('/search', { ...opts, query: { ...params } }),
  starred: (opts?: CallOptions) => request<Starred>('/starred', opts),
  random: (params: RandomParams = {}, opts?: CallOptions) =>
    request<Track[]>('/random', { ...opts, query: { ...params } }),
  recentTracks: (params: { limit?: number } = {}, opts?: CallOptions) =>
    request<Track[]>('/recent-tracks', { ...opts, query: { ...params } }),

  // ---- §7.3 annotations
  star: (body: StarInput) => request<void>('/star', { method: 'POST', body }),
  rating: (body: RatingInput) => request<void>('/rating', { method: 'POST', body }),
  scrobble: (body: ScrobbleInput, opts?: CallOptions) =>
    request<void>('/scrobble', { ...opts, method: 'POST', body }),

  // ---- §7.4 playlists, queue, radio
  playlists: {
    list: (opts?: CallOptions) => request<Playlist[]>('/playlists', opts),
    create: (body: CreatePlaylistInput) => request<Playlist>('/playlists', { method: 'POST', body }),
    get: (id: string, opts?: CallOptions) => request<PlaylistDetail>(`/playlists/${seg(id)}`, opts),
    update: (id: string, body: UpdatePlaylistInput) =>
      request<Playlist>(`/playlists/${seg(id)}`, { method: 'PUT', body }),
    delete: (id: string) => request<void>(`/playlists/${seg(id)}`, { method: 'DELETE' }),
    /** Append tracks. */
    addTracks: (id: string, trackIds: string[]) =>
      request<Playlist>(`/playlists/${seg(id)}/tracks`, { method: 'POST', body: { trackIds } }),
    /** Replace the whole track order. */
    setTracks: (id: string, trackIds: string[]) =>
      request<Playlist>(`/playlists/${seg(id)}/tracks`, { method: 'PUT', body: { trackIds } }),
    /** Remove entries by 0-based position. */
    removeTracks: (id: string, positions: number[]) =>
      request<Playlist>(`/playlists/${seg(id)}/tracks`, { method: 'DELETE', body: { positions } }),
  },
  queue: {
    get: (opts?: CallOptions) => request<PlayQueue>('/queue', opts),
    save: (body: SaveQueueInput, opts?: CallOptions) => request<void>('/queue', { ...opts, method: 'PUT', body }),
  },
  radios: {
    list: (opts?: CallOptions) => request<RadioStation[]>('/radios', opts),
    create: (body: RadioInput) => request<RadioStation>('/radios', { method: 'POST', body }),
    update: (id: string, body: RadioInput) => request<RadioStation>(`/radios/${seg(id)}`, { method: 'PUT', body }),
    delete: (id: string) => request<void>(`/radios/${seg(id)}`, { method: 'DELETE' }),
  },

  // ---- §7.5 media
  lyrics: (trackId: string, opts?: CallOptions) => request<Lyrics>(`/lyrics/${seg(trackId)}`, opts),
  /** Audio stream URL (`raw` → original file with Range support). */
  streamUrl: (trackId: string, opts: StreamOptions = {}) =>
    apiUrl(`/stream/${seg(trackId)}`, {
      format: opts.format,
      bitrate: opts.bitrate || undefined,
      offset: opts.offset ? Math.max(0, Math.round(opts.offset * 1000) / 1000) : undefined,
    }),
  /** Original file as an attachment (requires `canDownload`). */
  downloadUrl: (trackId: string) => apiUrl(`/download/${seg(trackId)}`),
  /** Whole album as a streamed zip. */
  albumDownloadUrl: (albumId: string) => apiUrl(`/download/album/${seg(albumId)}`),
  /** Server-sent events endpoint (see `useServerEvents`). */
  eventsUrl: () => apiUrl('/events'),

  // ---- §7.6 manage (canManage / admin)
  manage: {
    tags: {
      get: (trackId: string, opts?: CallOptions) =>
        request<TrackTags>(`/manage/tracks/${seg(trackId)}/tags`, opts),
      save: (edits: TagEdit[]) => request<BatchResult>('/manage/tags', { method: 'POST', body: { edits } }),
      rebuild: (trackIds: string[]) => request<BatchResult>('/manage/tags/rebuild', { method: 'POST', body: { trackIds } }),
    },
    /** Embedded picture bytes of a track (404 when none) — for `<img src>`. */
    pictureUrl: (trackId: string, version?: string | number) =>
      apiUrl(`/manage/tracks/${seg(trackId)}/picture`, { v: version }),
    cover: {
      set: (input: SetCoverInput, opts?: CallOptions) => {
        const form = new FormData()
        const name = input.fileName ?? (input.file instanceof File ? input.file.name : 'cover.jpg')
        form.append('file', input.file, name)
        if (input.trackIds?.length) form.append('trackIds', input.trackIds.join(','))
        if (input.albumId) form.append('albumId', input.albumId)
        if (input.embed !== undefined) form.append('embed', String(input.embed))
        if (input.saveToFolder !== undefined) form.append('saveToFolder', String(input.saveToFolder))
        return request<BatchResult>('/manage/cover', { ...opts, method: 'POST', body: form })
      },
      remove: (body: RemoveCoverInput) => request<BatchResult>('/manage/cover', { method: 'DELETE', body }),
    },
    setLyrics: (trackId: string, body: SetLyricsInput) =>
      request<Track>(`/manage/tracks/${seg(trackId)}/lyrics`, { method: 'PUT', body }),
    rename: {
      preview: (body: RenameInput, opts?: CallOptions) =>
        request<{ items: RenamePlan[] }>('/manage/rename/preview', { ...opts, method: 'POST', body }),
      apply: (body: RenameInput) => request<BatchResult>('/manage/rename', { method: 'POST', body }),
    },
    upload: (input: UploadInput, opts: UploadCallOptions = {}) => {
      const form = new FormData()
      for (const item of input.files) {
        const [blob, name] = uploadName(item)
        form.append('files', blob, name)
      }
      form.append('libraryId', String(input.libraryId))
      if (input.dir) form.append('dir', input.dir)
      if (input.organize !== undefined) form.append('organize', String(input.organize))
      return upload<BatchResult>('/manage/upload', form, opts)
    },
    /** Move files to the trash. */
    deleteTracks: (trackIds: string[]) =>
      request<BatchResult>('/manage/delete', { method: 'POST', body: { trackIds } }),
    trash: {
      list: (opts?: CallOptions) => request<TrashEntry[]>('/manage/trash', opts),
      restore: (ids: string[]) => request<BatchResult>('/manage/trash/restore', { method: 'POST', body: { ids } }),
      /** Omit `ids` to empty the trash. */
      purge: (ids?: string[]) => request<Purged>('/manage/trash/purge', { method: 'POST', body: ids ? { ids } : {} }),
    },
    missing: {
      /** Omit `trackIds` to purge every missing track. */
      purge: (trackIds?: string[]) =>
        request<Purged>('/manage/missing/purge', { method: 'POST', body: trackIds ? { trackIds } : {} }),
    },
    folders: {
      list: (params: FolderParams, opts?: CallOptions) =>
        request<FolderListing>('/manage/folders', { ...opts, query: { ...params } }),
      rescan: (body: { libraryId: number; dir: string }) =>
        request<void>('/manage/folders/rescan', { method: 'POST', body }),
    },
    issues: {
      summary: (opts?: CallOptions) => request<Record<IssueType, number>>('/manage/issues/summary', opts),
      list: (params: IssuesParams, opts?: CallOptions) =>
        request<Page<Issue>>('/manage/issues', { ...opts, query: { ...params } }),
    },
    /** Preview (`apply: false`) or apply mojibake repairs. */
    encoding: (body: EncodingInput) => request<EncodingResult>('/manage/encoding', { method: 'POST', body }),
    log: (params: EditLogParams = {}, opts?: CallOptions) =>
      request<Page<EditLogEntry>>('/manage/log', { ...opts, query: { ...params } }),
    /** Online metadata lookup (403 unless an admin enabled `settings.onlineMetadata`). */
    metadata: {
      status: (opts?: CallOptions) => request<MetadataStatus>('/manage/metadata', opts),
      search: (params: MetadataSearchParams, opts?: CallOptions) =>
        request<{ items: MetadataResult[] }>('/manage/metadata/search', { ...opts, query: { ...params } }),
      lyrics: (provider: MetadataProviderId, id: string, opts?: CallOptions) =>
        request<MetadataLyrics>('/manage/metadata/lyrics', { ...opts, query: { provider, id } }),
      /** A result's cover (or thumbnail) proxied through the server — for `<img src>` and `fetch`. */
      coverUrl: (url: string) => apiUrl('/manage/metadata/cover', { url }),
    },
    /** Downloads from YouTube / bilibili (starting one is 403 unless `settings.ytdlpEnabled`). */
    downloads: {
      status: (opts?: CallOptions) => request<DownloadsStatus>('/manage/downloads', opts),
      start: (body: StartDownloadInput) => request<DownloadJob>('/manage/downloads', { method: 'POST', body }),
      /** Cancels a queued or running job; removes a finished one from the list. */
      remove: (id: string) => request<void>(`/manage/downloads/${seg(id)}`, { method: 'DELETE' }),
    },
  },

  // ---- §7.7 admin
  admin: {
    users: {
      list: (opts?: CallOptions) => request<User[]>('/admin/users', opts),
      create: (body: CreateUserInput) => request<User>('/admin/users', { method: 'POST', body }),
      update: (id: string, body: UpdateUserInput) =>
        request<User>(`/admin/users/${seg(id)}`, { method: 'PUT', body }),
      delete: (id: string) => request<void>(`/admin/users/${seg(id)}`, { method: 'DELETE' }),
    },
    libraries: {
      list: (opts?: CallOptions) => request<LibraryInfo[]>('/admin/libraries', opts),
      create: (body: LibraryInput) => request<LibraryInfo>('/admin/libraries', { method: 'POST', body }),
      update: (id: number, body: Partial<LibraryInput>) =>
        request<LibraryInfo>(`/admin/libraries/${seg(id)}`, { method: 'PUT', body }),
      /** Removes DB rows only, never files. */
      delete: (id: number) => request<void>(`/admin/libraries/${seg(id)}`, { method: 'DELETE' }),
    },
    scan: {
      status: (opts?: CallOptions) => request<ScanStatus>('/admin/scan', opts),
      /** 409 `conflict` when a scan is already running. */
      start: (body: StartScanInput = {}) => request<ScanStatus>('/admin/scan', { method: 'POST', body }),
    },
    settings: {
      get: (opts?: CallOptions) => request<Settings>('/admin/settings', opts),
      update: (body: Partial<Settings>) => request<Settings>('/admin/settings', { method: 'PUT', body }),
    },
    stats: (opts?: CallOptions) => request<LibraryStats>('/admin/stats', opts),
    system: (opts?: CallOptions) => request<SystemInfo>('/admin/system', opts),
    clearCache: () => request<{ freed: number }>('/admin/cache/clear', { method: 'POST' }),
    ytdlp: {
      get: (opts?: CallOptions) => request<YtdlpInfo>('/admin/ytdlp', opts),
      /** Asks GitHub for the latest release. */
      check: () => request<YtdlpInfo>('/admin/ytdlp/check', { method: 'POST' }),
      /** Installs / updates in the background; poll `get` while `install.running`. */
      install: () => request<YtdlpInfo>('/admin/ytdlp/install', { method: 'POST' }),
      /** Stores cookies encrypted on the server; they are never returned. */
      setCookies: (site: DownloadSite, text: string) =>
        request<CookieSaveResult>(`/admin/ytdlp/cookies/${seg(site)}`, { method: 'PUT', body: { text } }),
      deleteCookies: (site: DownloadSite) => request<void>(`/admin/ytdlp/cookies/${seg(site)}`, { method: 'DELETE' }),
    },
  },
} as const

export type Api = typeof api
