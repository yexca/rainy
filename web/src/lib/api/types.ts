/**
 * Native API (`/api`) JSON contract — docs/architecture/contract.md §8.
 * Mirrors the Go models in `internal/model`. Keep both sides in sync with the document.
 */

export interface Page<T> { items: T[]; total: number }
export interface ApiErrorBody { error: { code: string; message: string } }

export interface User {
  id: string; username: string; displayName: string; email: string
  isAdmin: boolean; canManage: boolean; canDownload: boolean; hasApiKey: boolean
  createdAt: number; updatedAt: number; lastLoginAt: number; lastSeenAt: number
}
export interface AuthStatus { initialized: boolean; user: User | null; version: string }

export interface Track {
  id: string; libraryId: number; path: string; dir: string; filename: string; suffix: string
  size: number; mtime: number
  title: string; album: string; artist: string; albumArtist: string
  albumId: string; artistId: string; albumArtistId: string
  trackNumber: number; trackTotal: number; discNumber: number; discTotal: number; discSubtitle: string
  year: number; date: string; originalYear: number
  genre: string; genres: string[]; composer: string; comment: string
  hasLrc: boolean; hasLyrics: boolean; bpm: number; compilation: boolean
  duration: number; bitrate: number; sampleRate: number; bitDepth: number; channels: number
  codec: string; hasCover: boolean
  rgTrackGain: number | null; rgTrackPeak: number | null; rgAlbumGain: number | null; rgAlbumPeak: number | null
  mbzTrackId: string; mbzAlbumId: string; mbzArtistId: string; mbzAlbumArtistId: string
  sortTitle: string; sortAlbum: string; sortArtist: string; sortAlbumArtist: string
  missing: boolean; createdAt: number; updatedAt: number
  starred: boolean; starredAt: number | null; rating: number; playCount: number; playedAt: number
  coverArt: string; contentType: string
}
export interface Album {
  id: string; libraryId: number; name: string; sortName: string; artist: string; artistId: string
  year: number; genre: string; compilation: boolean; songCount: number; discCount: number
  duration: number; size: number; mbzAlbumId: string; createdAt: number; updatedAt: number
  starred: boolean; starredAt: number | null; rating: number; playCount: number; playedAt: number
  coverArt: string
}
export interface AlbumDetail extends Album { tracks: Track[]; discs: number[] }
export interface Artist {
  id: string; name: string; sortName: string; indexKey: string; albumCount: number; songCount: number
  mbzArtistId: string; createdAt: number; updatedAt: number
  starred: boolean; starredAt: number | null; rating: number; playCount: number; playedAt: number
  coverArt: string
}
export interface ArtistDetail extends Artist { albums: Album[]; appearsOn: Album[]; topTracks: Track[] }
export interface Genre { id: string; name: string; songCount: number; albumCount: number }
export interface SearchResult { artists: Artist[]; albums: Album[]; tracks: Track[] }
export interface Home {
  recentlyAdded: Album[]; recentlyPlayed: Album[]; mostPlayed: Album[]; random: Album[]; starred: Album[]
  stats: { tracks: number; albums: number; artists: number }
}
export interface Playlist {
  id: string; name: string; comment: string; ownerId: string; ownerName: string; public: boolean
  songCount: number; duration: number; createdAt: number; updatedAt: number; coverArt: string
}
export interface PlaylistDetail extends Playlist { tracks: Track[]; readonly: boolean }
export interface PlayQueue { trackIds: string[]; currentId: string; positionMs: number; /** Client name of the last writer. */ changedBy: string; updatedAt: number; tracks: Track[] }
export interface RadioStation { id: string; name: string; streamUrl: string; homepageUrl: string; createdAt: number; updatedAt: number }
export interface LyricsLine { start: number; text: string }   // start ms, -1 unsynced
export interface Lyrics { synced: boolean; lines: LyricsLine[]; source: 'lrc' | 'embedded' | 'none'; raw: string; offset: number; lang: string }

export type StarType = 'track' | 'album' | 'artist'

// ---- manage
export type TagMap = Record<string, string[]>
export interface PictureInfo { type: string; description: string; mimeType: string }
export interface FileInfo {
  path: string; size: number; mtime: number; format: string; codec: string; duration: number
  bitrate: number; sampleRate: number; bitDepth: number; channels: number
}
export interface TrackTags {
  track: Track; tags: TagMap; pictures: PictureInfo[]; file: FileInfo; writable: boolean
  lyrics: string          // embedded LYRICS text ('' if none)
  lrc: string | null      // sidecar .lrc content, null if no sidecar
}
export interface TagEdit { trackId: string; tags: TagMap }
export interface ItemError { trackId: string; path: string; error: string }
export interface BatchResult { updated: Track[]; errors: ItemError[] }
export interface RenamePlan { trackId: string; from: string; to: string; status: 'ok' | 'unchanged' | 'conflict' | 'invalid'; message?: string }
export interface FolderEntry { name: string; path: string }
export interface FileEntry { name: string; path: string; size: number; mtime: number; isAudio: boolean; isImage: boolean; trackId: string | null }
export interface FolderListing { libraryId: number; dir: string; parent: string | null; writable: boolean; folders: FolderEntry[]; files: FileEntry[] }
export type IssueType = 'missing_tags' | 'no_cover' | 'duplicates' | 'missing_files' | 'encoding'
export interface Issue { key: string; type: IssueType; message: string; tracks: Track[]; album?: Album }
export interface EncodingFix { trackId: string; path: string; encoding: string; changes: Record<string, { old: string[]; new: string[] }> }
export interface EditLogEntry { id: number; userId: string; username: string; action: string; trackId: string; path: string; details: unknown; createdAt: number }
export type MetadataProviderId = 'netease' | 'qq' | 'kugou' | 'kuwo' | 'itunes'
export interface MetadataProvider { id: MetadataProviderId; lyrics: boolean; regions: string[] }
export interface MetadataStatus { enabled: boolean; providers: MetadataProvider[] }
export interface MetadataResult {
  provider: MetadataProviderId; id: string; title: string; artists: string[]; album: string; albumArtist: string
  trackNumber: number; trackTotal: number; discNumber: number; discTotal: number
  date: string; genre: string; duration: number; coverUrl: string; thumbUrl: string   // 0 / '' = unknown
}
export interface MetadataLyrics { text: string; translation: string }
export type DownloadSite = 'youtube' | 'bilibili'
export type DownloadFormat = 'best' | 'm4a' | 'mp3' | 'opus'
export type DownloadJobStatus = 'queued' | 'running' | 'importing' | 'done' | 'error' | 'canceled'
export interface DownloadJob {
  id: string; url: string; site: DownloadSite; title: string; status: DownloadJobStatus
  phase: '' | 'downloading' | 'processing'
  progress: number                  // 0…1 overall, -1 = unknown
  item: number; items: number       // current playlist entry (1-based) / entries, 0 = unknown
  speed: number; eta: number        // bytes per second; seconds, -1 = unknown
  error: string                     // failure, or what failed for some entries of a finished job
  libraryId: number; dir: string; organize: boolean; format: DownloadFormat; playlist: boolean
  trackIds: string[]; errors: ItemError[]; createdBy: string; createdAt: number; startedAt: number; finishedAt: number
}
export interface DownloadsStatus {
  enabled: boolean; ready: boolean               // yt-dlp installed and runnable, ffmpeg present
  sites: { id: DownloadSite; cookies: boolean }[]
  jobs: DownloadJob[]                            // newest first
}
export interface TrashEntry { id: string; libraryId: number; originalPath: string; trashPath: string; size: number; title: string; artist: string; album: string; trackId: string; deletedBy: string; deletedAt: number }

// ---- admin
export interface Library { id: number; name: string; path: string; createdAt: number; updatedAt: number; lastScanAt: number }
export interface LibraryInfo extends Library { trackCount: number; exists: boolean; writable: boolean }
export interface ScanStatus {
  scanning: boolean; full: boolean; libraryId: number
  phase: 'idle' | 'walking' | 'reading' | 'refreshing' | 'done' | 'error'
  filesSeen: number; added: number; updated: number; removed: number; moved: number; errors: number
  startedAt: number; finishedAt: number; lastError: string
}
export interface Settings {
  scanInterval: string; genreSeparators: string; ignoredArticles: string; coverArtFiles: string
  transcodeFormat: 'mp3' | 'opus' | 'aac'; transcodeBitrate: number; renamePattern: string
  fixEncodingOnScan: boolean; enableDownloads: boolean; onlineMetadata: boolean; onlineMetadataChinaIp: boolean
  ytdlpEnabled: boolean
}
export interface LibraryStats {
  tracks: number; albums: number; artists: number; genres: number; playlists: number; users: number
  missingTracks: number; totalDuration: number; totalSize: number
  formats: { suffix: string; count: number; size: number }[]; playsLast30Days: number
}
export interface SystemInfo {
  version: string; commit: string; buildDate: string; goVersion: string; os: string; arch: string
  uptimeSec: number; dataDir: string; dbSize: number; cacheSize: number; trashSize: number
  ffmpeg: { available: boolean; version: string; path: string }
  libraries: LibraryInfo[]
}
export interface CookieInfo {
  site: DownloadSite; configured: boolean; count: number
  signedIn: boolean; expiresAt: number; updatedAt: number   // ms; expiresAt 0 = unknown / session
}
export interface CookieSaveResult extends CookieInfo { dropped: number }
export interface YtdlpInstallState { running: boolean; error: string; finishedAt: number }
export interface YtdlpInfo {
  enabled: boolean; managed: boolean; installed: boolean; version: string; error: string
  latest: string; checkedAt: number; asset: string
  jsRuntime: '' | 'deno' | 'node' | 'quickjs'; ffmpeg: boolean
  install: YtdlpInstallState; cookies: CookieInfo[]
}
export interface NowPlayingEntry { userId: string; username: string; trackId: string; player: string; since: number }
export interface ServerEvent { type: 'scan' | 'library' | 'nowPlaying'; data: unknown }
