# Rainy — Architecture & Contracts

Rainy is a self-hosted music server for NAS devices. It streams your music library to the
built-in web app (installable PWA with an iOS-style player) and to any Subsonic/OpenSubsonic
client (Symfonium, play:Sub, Substreamer, Feishin, Amperfy, DSub, …). Unlike Navidrome it has
first-class **library management**: tag editing (single + batch), cover art, lyrics,
rename/organize by pattern, upload, trash, a "library doctor" and an edit history.

This document is the **single source of truth** for everyone working on the code. When code
and this document disagree, fix the code (or, if the document is wrong, update the document in
the same change and say so in your report).

---------------------------------------------------------------------------------------------

## 1. Stack

| Layer | Choice | Notes |
|---|---|---|
| Language | Go 1.26 | module name `rainy` |
| HTTP | `github.com/go-chi/chi/v5` | net/http compatible |
| DB | SQLite via `modernc.org/sqlite` (pure Go, no CGO) + `github.com/jmoiron/sqlx` | WAL mode; separate reader pool + single-connection writer |
| Tags | `go.senan.xyz/taglib` (TagLib 2 compiled to WASM, pure Go) | read + write tags, properties, embedded pictures |
| Images | `image/jpeg`, `image/png`, `golang.org/x/image/{draw,webp}` | cover resizing, JPEG output |
| Text | `golang.org/x/text` (normalization, GBK/Big5/Shift-JIS decoders), `github.com/mozillazg/go-pinyin` | search normalization, CJK index letters, mojibake repair |
| Transcoding | external `ffmpeg` binary | shipped in the docker image |
| Frontend | Vite + React 19 + TypeScript + Tailwind CSS v4 + shadcn/ui | |
| FE data | `@tanstack/react-query` v5, `react-router` v7 (data router), `zustand` | |
| FE misc | `motion` (`motion/react`), `vaul` (via shadcn Drawer), `sonner`, `lucide-react`, `@tanstack/react-virtual`, `@dnd-kit/*`, `i18next` + `react-i18next`, `vite-plugin-pwa` | |
| Deploy | multi-stage Dockerfile (node → go → alpine+ffmpeg), docker compose | amd64 + arm64 |

CGO is **disabled** for release builds (`CGO_ENABLED=0`). Do not add CGO dependencies.

## 2. Repository layout

```
rainy/
├── cmd/rainy/main.go            entrypoint + small CLI (serve | user reset-password | version)
├── internal/
│   ├── buildinfo/               Version, Commit, BuildDate (ldflags)
│   ├── config/                  env config
│   ├── db/                      Open, Migrate, migrations/*.sql (embedded), dbtest helper
│   ├── model/                   shared structs (DB rows + JSON shapes)
│   ├── util/                    ids, time, text normalization, cover-art ids, mime, safe paths
│   ├── store/                   all shared SQL (repository layer)
│   ├── auth/                    password crypto, sessions, middleware, rate limit
│   ├── events/                  in-process pub/sub (SSE feed)
│   ├── nowplaying/              in-memory "now playing" tracker
│   ├── tags/                    taglib wrapper: read/write tags, pictures, mojibake repair
│   ├── scanner/                 library scanner + scheduler + targeted rescans
│   ├── artwork/                 cover art resolution, resizing, disk cache
│   ├── transcode/               ffmpeg streaming
│   ├── lyrics/                  LRC parsing, sidecar/embedded lyrics loading
│   ├── manage/                  tag editing, covers, lyrics, rename, upload, trash, doctor, edit log
│   ├── app/                     dependency container wiring every service
│   ├── api/                     native JSON API for the web UI (/api)
│   ├── subsonic/                Subsonic + OpenSubsonic API (/rest)
│   └── server/                  router assembly, middleware, SPA static serving, graceful shutdown
├── web/                         Vite React app (see §9); `web/embed.go` embeds `web/dist`
├── scripts/gen-testdata.sh      generates a small tagged test library into testdata/music (gitignored)
├── docs/                      public documentation (docs/README.md); docs/architecture/contract.md is this file
├── Dockerfile, docker-compose.yml, deploy/compose/dev.yml, .dockerignore, Makefile, VERSION
└── README.md (English) + docs/readme/README.{zh-Hans,zh-Hant,ja}.md
```

## 3. Conventions

### General
- IDs are opaque strings. Tracks, users, playlists, radio stations, trash entries: `util.NewID()`
  (22-char random base62). Albums, artists, genres: deterministic `util.HashID(kind, parts...)`
  so they survive re-scans (see schema comments for exact inputs; all inputs lower-cased and
  trimmed).
- Timestamps: `int64` unix **milliseconds** in DB, Go models and JSON. `0` = never/unknown.
  Subsonic responses convert to RFC3339 (`time.UnixMilli(ms).UTC().Format(time.RFC3339)`).
- Durations: `float64` seconds in DB/model/JSON (Subsonic: integer seconds, rounded).
- Paths in the DB/API are **library-relative with forward slashes**. Convert with
  `filepath.ToSlash`/`filepath.FromSlash`; build absolute paths only via `util.SafeJoin(root, rel)`,
  which rejects anything escaping the root (`..`, absolute input, symlink escapes where checked).
- The server may run on Windows during development — never hardcode `/` for OS paths.
- Logging: `log/slog` (JSON handler in docker, text handler in dev). No `fmt.Println`.
- Errors: wrap with `fmt.Errorf("doing x: %w", err)`. Store returns `store.ErrNotFound` for
  missing rows; callers map it to HTTP 404 / Subsonic error 70.
- Context: every store / service method takes `ctx context.Context` first.
- Keep packages free of import cycles: `model`, `util`, `config`, `buildinfo` import nothing
  internal; `store` imports `db`, `model`, `util`; services import `store` but never `api`,
  `subsonic`, `server` or `app`.

### Go style
- Idiomatic, small functions, table-driven tests (`*_test.go` next to code).
- Tests must not need network or ffmpeg unless they `t.Skip` when unavailable.
- Use `dbtest.New(t)` (temp-file SQLite, migrated) for store-backed tests.

### TypeScript style
- Strict TS. No `any` unless unavoidable (then comment why).
- Path alias `@/` → `web/src/`.
- Components: function components, named exports, file names kebab-case.
- All user-visible strings go through i18next (`t('key')`), with both `en` and `zh` translations.

## 4. Configuration

Environment variables (all optional):

| Var | Default | Meaning |
|---|---|---|
| `RAINY_ADDRESS` | `0.0.0.0` | listen address |
| `RAINY_PORT` | `7650` | listen port |
| `RAINY_DATA_DIR` | `./data` (docker: `/data`) | DB, secret key, caches, trash |
| `RAINY_MUSIC_DIR` | `./music` (docker: `/music`) | default library; created as library #1 on first start if no library exists |
| `RAINY_SCAN_INTERVAL` | `1h` | periodic quick scan; `0` disables. Default for the DB setting `scanInterval` |
| `RAINY_SCAN_ON_START` | `true` | quick scan at startup |
| `RAINY_FFMPEG_PATH` | `ffmpeg` | ffmpeg binary |
| `RAINY_LOG_LEVEL` | `info` | debug/info/warn/error |
| `RAINY_LOG_FORMAT` | `text` (docker: `json`) | |
| `RAINY_SESSION_TTL` | `720h` | web session lifetime (sliding) |
| `RAINY_TRUST_PROXY` | `false` | honour X-Forwarded-For / X-Real-IP |
| `RAINY_DEV_CORS` | `` | extra allowed origin for /api during development (e.g. `http://localhost:5173`) |

`config.Load() (*config.Config, error)` reads them; `Config` fields:
`Address, Port string/int; DataDir, MusicDir string; ScanInterval time.Duration; ScanOnStart bool;
FFmpegPath string; LogLevel, LogFormat string; SessionTTL time.Duration; TrustProxy bool; DevCORS string`
plus helpers `DBPath()`, `CacheDir()`, `ArtworkCacheDir()`, `TrashDir()`, `TmpDir()`, `SecretKeyPath()`.

Data dir layout:
```
/data/rainy.db (+ -wal, -shm)
/data/secret.key        32 random bytes, created on first start (AES key + misc HMAC)
/data/cache/artwork/    resized cover cache
/data/trash/<libraryId>/<original relative path>   deleted files (restorable)
/data/tmp/              upload staging
```

Runtime-editable settings live in the `settings` table (see `model.Settings`, §5.2).

## 5. Backend contracts

### 5.1 Database
Schema: `internal/db/migrations/0001_init.sql` plus later migrations in the same folder (`0002_play_queue_index.sql`
adds `play_queues.current_index`) — read them, they are part of this contract.
Migrations are embedded, applied in lexical order, tracked in `schema_migrations(version TEXT PK, applied_at INTEGER)`.

`internal/db`:
```go
type DB struct {
    R *sqlx.DB // reader pool (max open = 2*NumCPU)
    W *sqlx.DB // writer, MaxOpenConns(1)
}
func Open(path string) (*DB, error) // pragmas: journal_mode=WAL, busy_timeout=10000, foreign_keys=ON, synchronous=NORMAL, cache_size=-20000
func (d *DB) Migrate(ctx context.Context) error
func (d *DB) Tx(ctx context.Context, fn func(tx *sqlx.Tx) error) error // on W; rollback on error/panic
func (d *DB) Close() error
// package dbtest: func New(t testing.TB) *db.DB — temp dir file DB, migrated, closed via t.Cleanup
```

### 5.2 Models (`internal/model`)
Go structs carry both `db:"col"` and `json:"camelCase"` tags; the JSON form **is** the native API
shape (mirrored in `web/src/lib/api/types.ts`, §8). Fields marked *computed* are `db:"-"` and
filled by the store before returning (`Fill()` helpers).

```go
type Library struct { ID int64; Name, Path string; CreatedAt, UpdatedAt, LastScanAt int64 }
// json: id,name,path,createdAt,updatedAt,lastScanAt

type User struct {
    ID, Username, DisplayName, Email string
    PasswordEnc string          // json:"-"
    IsAdmin, CanManage, CanDownload bool
    APIKeyHash *string          // json:"-"
    HasAPIKey bool              // computed
    CreatedAt, UpdatedAt, LastLoginAt, LastSeenAt int64
}
func (u *User) CanEdit() bool { return u.IsAdmin || u.CanManage }

type Track struct {
    ID string; LibraryID int64; Path, Dir, Filename, Suffix string; Size, Mtime int64
    Title, Album, Artist, AlbumArtist, AlbumID, ArtistID, AlbumArtistID string
    TrackNumber, TrackTotal, DiscNumber, DiscTotal int; DiscSubtitle string
    Year int; Date string; OriginalYear int
    Genre string; Genres []string /*computed from Genre, split on "; "*/; Composer, Comment string
    Lyrics string /*json:"-" — only loaded by GetTrack*/; HasLrc bool; BPM int; Compilation bool
    Duration float64; Bitrate, SampleRate, BitDepth, Channels int; Codec string; HasCover bool
    RGTrackGain, RGTrackPeak, RGAlbumGain, RGAlbumPeak *float64
    MbzTrackID, MbzAlbumID, MbzArtistID, MbzAlbumArtistID string
    SortTitle, SortAlbum, SortArtist, SortAlbumArtist string
    SearchText string /*json:"-"*/; Missing bool; CreatedAt, UpdatedAt int64
    // per-user annotation (LEFT JOIN annotations, zero values when no user)
    StarredAt *int64; Rating, PlayCount int; PlayedAt int64
    // computed
    Starred bool; CoverArt string; ContentType string; HasLyrics bool
}
// JSON names: id, libraryId, path, dir, filename, suffix, size, mtime, title, album, artist,
// albumArtist, albumId, artistId, albumArtistId, trackNumber, trackTotal, discNumber, discTotal,
// discSubtitle, year, date, originalYear, genre, genres, composer, comment, hasLrc, bpm,
// compilation, duration, bitrate, sampleRate, bitDepth, channels, codec, hasCover, rgTrackGain,
// rgTrackPeak, rgAlbumGain, rgAlbumPeak, mbzTrackId, mbzAlbumId, mbzArtistId, mbzAlbumArtistId,
// sortTitle, sortAlbum, sortArtist, sortAlbumArtist, missing, createdAt, updatedAt, starredAt,
// rating, playCount, playedAt, starred, coverArt, contentType, hasLyrics

type Album struct {
    ID string; LibraryID int64; Name, SortName string
    AlbumArtist string /*json:"artist"*/; AlbumArtistID string /*json:"artistId"*/
    Year int; Genre string; Compilation bool; SongCount, DiscCount int; Duration float64; Size int64
    CoverPath, CoverTrackID string /*json:"-"*/; MbzAlbumID string; SearchText string /*json:"-"*/
    CreatedAt, UpdatedAt int64
    StarredAt *int64; Rating, PlayCount int; PlayedAt int64
    Starred bool; CoverArt string // computed
}
type Artist struct {
    ID, Name, SortName, IndexKey string; AlbumCount, SongCount int; MbzArtistID string
    ImagePath string /*json:"-"*/; SearchText string /*json:"-"*/; CreatedAt, UpdatedAt int64
    StarredAt *int64; Rating, PlayCount int; PlayedAt int64
    Starred bool; CoverArt string // computed
}
type Genre struct { ID, Name string; SongCount, AlbumCount int }
type Playlist struct {
    ID, Name, Comment, OwnerID string; OwnerName string /*joined*/; Public bool
    SongCount int; Duration float64 /*computed via join over non-missing tracks*/
    CreatedAt, UpdatedAt int64; CoverArt string /*computed "pl-<id>_<v>"*/
}
type PlayQueue struct { TrackIDs []string; CurrentID string; PositionMs int64; ChangedBy string; UpdatedAt int64
    CurrentIndex int /*json:"-"; position of the current entry in TrackIDs, normalized by the store (CurrentPos)*/ }
type Bookmark struct { TrackID string; PositionMs int64; Comment string; CreatedAt, UpdatedAt int64 }
type RadioStation struct { ID, Name, StreamURL, HomepageURL string; CreatedAt, UpdatedAt int64 }
type EditLogEntry struct { ID int64; UserID, Username, Action, TrackID, Path string; Details json.RawMessage; CreatedAt int64 }
type TrashEntry struct { ID string; LibraryID int64; OriginalPath, TrashPath string; Size int64; Title, Artist, Album, TrackID, DeletedBy string; DeletedAt int64 }
type Session struct { TokenHash, UserID, UserAgent, IP string; CreatedAt, LastSeenAt, ExpiresAt int64 }

type Settings struct {
    ScanInterval      string `json:"scanInterval"`      // Go duration, "0" = disabled; default from RAINY_SCAN_INTERVAL
    GenreSeparators   string `json:"genreSeparators"`   // default ";/,"
    IgnoredArticles   string `json:"ignoredArticles"`   // default "The El La Los Las Le Les"
    CoverArtFiles     string `json:"coverArtFiles"`     // default "cover.*,folder.*,front.*,album.*,albumart*.*"
    TranscodeFormat   string `json:"transcodeFormat"`   // default "mp3" (mp3|opus|aac)
    TranscodeBitrate  int    `json:"transcodeBitrate"`  // default 192
    RenamePattern     string `json:"renamePattern"`     // default "{albumartist}/{album}/[{disc}-]{track:2} {title}"
    FixEncodingOnScan bool   `json:"fixEncodingOnScan"` // default true: repair GBK/Big5/SJIS mojibake when *reading* (files untouched)
    EnableDownloads   bool   `json:"enableDownloads"`   // default true
}
func DefaultSettings(scanInterval time.Duration) Settings
```
JSON tags on every struct follow the camelCase of the Go field name unless noted
(`ID`→`id`, `LibraryID`→`libraryId`, `BPM`→`bpm`, `RGTrackGain`→`rgTrackGain`, `StreamURL`→`streamUrl`,
`HomepageURL`→`homepageUrl`, `MbzTrackID`→`mbzTrackId`, `TrackIDs`→`trackIds`).
Slices are never `null` in JSON responses (`[]`).

### 5.3 util
```go
func NewID() string                              // 22 chars base62, crypto/rand
func HashID(parts ...string) string              // lower-hex md5 of strings.Join(parts,"\x00"), first 22 chars
func NowMs() int64
func NormalizeSearch(parts ...string) string     // lower, NFKD, strip marks, full-width→ASCII, collapse spaces; joined by " "
func SearchTokens(q string) []string             // NormalizeSearch(q) split on spaces (empty → nil)
func SortName(name string, ignoredArticles []string) string // strip leading article + space, lowercase NormalizeSearch
func IndexKey(name string, ignoredArticles []string) string  // "A".."Z" | "#"; Han → pinyin initial (go-pinyin)
func CoverArtID(kind string, id string, version int64) string // kind: "al","ar","tr","pl" → "al-<id>_<base36(version)>"
func ParseCoverArtID(s string) (kind, id string)  // tolerant: missing prefix → kind "", strips "_<ver>"
func MimeType(suffix string) string               // audio + image types; default application/octet-stream
func SplitMulti(values []string, seps string) []string // split each value on any rune in seps, trim, dedupe (case-insensitive), drop empty
func SafeJoin(root, rel string) (string, error)  // clean; reject abs, "..", NUL; result must stay under root
func IsHiddenOrSystem(name string) bool           // ".*", "@eaDir", "#recycle", "#snapshot", ".@__thumb", "$RECYCLE.BIN", "lost+found", "System Volume Information"
```

### 5.4 store (`internal/store`)
```go
var ErrNotFound = errors.New("not found")
type Store struct{ ... }
func New(d *db.DB) *Store
func (s *Store) DB() *db.DB   // for package-local specialised queries elsewhere (read via R, write via W)

// Users & sessions
CreateUser(ctx, u *model.User) error; GetUser(ctx, id) (*model.User, error)
GetUserByUsername(ctx, username) (*model.User, error); GetUserByAPIKeyHash(ctx, hash) (*model.User, error)
ListUsers(ctx) ([]model.User, error); UpdateUser(ctx, u *model.User) error; DeleteUser(ctx, id) error
CountUsers(ctx) (int, error); SetUserPassword(ctx, id, passwordEnc string) error
SetUserAPIKeyHash(ctx, id string, hash *string) error; TouchUserLogin(ctx, id) error; TouchUserSeen(ctx, id) error
CreateSession(ctx, *model.Session) error; GetSession(ctx, tokenHash) (*model.Session, error)
TouchSession(ctx, tokenHash string, expiresAt int64) error; DeleteSession(ctx, tokenHash) error
DeleteUserSessions(ctx, userID) error; PurgeExpiredSessions(ctx) error

// Libraries
ListLibraries(ctx) ([]model.Library, error); GetLibrary(ctx, id int64) (*model.Library, error)
CreateLibrary(ctx, *model.Library) error; UpdateLibrary(ctx, *model.Library) error
DeleteLibrary(ctx, id int64) error /* cascades tracks; then RefreshAll */; SetLibraryScanned(ctx, id int64, at int64) error

// Tracks
type TrackQuery struct {
    UserID string; Q string; IDs []string; AlbumID, ArtistID /*artist_id OR album_artist_id*/ string
    Genre string /*genre name*/; LibraryID int64; Dir string /*exact*/; DirPrefix string
    FromYear, ToYear int; Starred bool; Missing string /*"" exclude missing | "include" | "only"*/
    Sort string; Order string /*"asc"|"desc"|"" natural*/; Offset, Limit int /*Limit 0 = unlimited*/
}
// Sort values: "title","artist","album" (album,disc,track),"albumArtist","year","duration","recent"(created_at),
// "updated","played"(played_at),"frequent"(play_count),"rating","starred"(starred_at),"random","path",
// "track"(disc,track — album order),"size","bitrate","suffix". Unknown → "title". Always add a stable tiebreak (id).
ListTracks(ctx, TrackQuery) ([]model.Track, int /*total ignoring offset/limit*/, error)
GetTrack(ctx, id, userID string) (*model.Track, error)                 // includes Lyrics, includes missing
GetTracks(ctx, ids []string, userID string) ([]model.Track, error)     // order of ids preserved; unknown ids skipped; includes missing
GetTrackByPath(ctx, libraryID int64, path string) (*model.Track, error)
type TrackFileState struct { ID string; Size, Mtime int64; Missing bool }
TrackFileStates(ctx, libraryID int64) (map[string]TrackFileState, error) // key: path
UpsertTracks(ctx, tracks []model.Track) error      // one tx; INSERT ... ON CONFLICT(id) DO UPDATE; keeps existing created_at;
                                                   // rewrites track_genres from t.Genres (or SplitMulti(Genre,";")); upserts genres
MarkTracksMissing(ctx, ids []string, missing bool) error
DeleteTracks(ctx, ids []string) error               // purge rows + track annotations
UpdateTrackPath(ctx, id string, path string) error  // sets path, dir, filename, suffix, updated_at
AlbumIDsForTracks(ctx, ids []string) ([]string, error); ArtistIDsForTracks(ctx, ids []string) ([]string, error)

// Aggregates (called by scanner/manage after track changes)
RefreshAlbums(ctx, ids []string) error   // nil = all. Recompute from non-missing tracks (name/artist from most common,
                                         // year=max, genre=most common, counts, duration, size, created_at=min, cover_track_id=first has_cover
                                         // ordered by disc,track; updated_at = max(tracks.updated_at, now if changed)); delete albums with 0 tracks.
                                         // Preserves cover_path (scanner sets it).
RefreshArtists(ctx, ids []string) error  // nil = all; album_count/song_count; delete artists with no tracks (as artist or album artist)
RefreshGenres(ctx) error                 // delete orphan genres
RefreshAll(ctx) error
SetAlbumCoverPath(ctx, albumID, coverPath string) error // bumps updated_at when changed
SetArtistImagePath(ctx, artistID, path string) error
TouchAlbums(ctx, ids []string) error     // bump updated_at (cover cache busting)

// Albums / artists / genres
type AlbumQuery struct {
    UserID, Q, ArtistID, Genre string; LibraryID int64; FromYear, ToYear int; Starred bool
    Sort string /*"name","artist","year","recent","random","played","frequent","starred","rating","songCount","duration"*/
    Order string; Offset, Limit int
}
ListAlbums(ctx, AlbumQuery) ([]model.Album, int, error); GetAlbum(ctx, id, userID) (*model.Album, error)
AlbumsAppearsOn(ctx, artistID, userID string) ([]model.Album, error) // albums containing tracks by artist whose album artist differs
type ArtistQuery struct {
    UserID, Q string; LibraryID int64; AlbumArtistsOnly bool /*album_count>0*/; Starred bool
    Sort string /*"name","albumCount","songCount","played","frequent","random","recent"*/; Order string; Offset, Limit int
}
ListArtists(ctx, ArtistQuery) ([]model.Artist, int, error); GetArtist(ctx, id, userID) (*model.Artist, error)
ListGenres(ctx) ([]model.Genre, error)   // with song/album counts over non-missing tracks, sorted by name

// Search (Q "" = match everything — Subsonic search3 full-sync compatibility)
type SearchQuery struct { UserID, Q string; LibraryID int64; ArtistOffset, ArtistLimit, AlbumOffset, AlbumLimit, TrackOffset, TrackLimit int }
type SearchResult struct { Artists []model.Artist; Albums []model.Album; Tracks []model.Track }
Search(ctx, SearchQuery) (*SearchResult, error) // every token must match search_text (LIKE %tok% ESCAPE '\'); artists only album_count>0 or song_count>0

// Annotations & plays (itemType: "track"|"album"|"artist")
SetStarred(ctx, userID, itemType string, ids []string, starred bool) error
SetRating(ctx, userID, itemType, id string, rating int) error   // 0 clears
RecordPlay(ctx, userID, trackID string, at int64, client string) error // +1 play_count & played_at on track, its album, its artist; insert play_history
RecentlyPlayedTracks(ctx, userID string, limit int) ([]model.Track, error)

// Playlists (tracks join non-missing tracks only)
ListPlaylists(ctx, userID string) ([]model.Playlist, error) // own + public
GetPlaylist(ctx, id string) (*model.Playlist, error)
PlaylistTracks(ctx, id, userID string) ([]model.Track, error) // playlist order
CreatePlaylist(ctx, *model.Playlist, trackIDs []string) error; UpdatePlaylist(ctx, *model.Playlist) error; DeletePlaylist(ctx, id) error
SetPlaylistTracks(ctx, id string, trackIDs []string) error; AppendPlaylistTracks(ctx, id string, trackIDs []string) error
RemovePlaylistPositions(ctx, id string, positions []int) error // then renumber contiguous

// Queue, bookmarks, radio, settings, edit log, trash, stats
GetPlayQueue(ctx, userID) (*model.PlayQueue, error); SavePlayQueue(ctx, userID string, q *model.PlayQueue) error
ListBookmarks(ctx, userID) ([]model.Bookmark, error); UpsertBookmark(ctx, userID string, b *model.Bookmark) error; DeleteBookmark(ctx, userID, trackID) error
ListRadioStations(ctx) ([]model.RadioStation, error); GetRadioStation(ctx, id); CreateRadioStation(ctx, *model.RadioStation) error; UpdateRadioStation; DeleteRadioStation
GetSettings(ctx, defaults model.Settings) (model.Settings, error); SaveSettings(ctx, model.Settings) error
AddEditLog(ctx, *model.EditLogEntry) error; ListEditLog(ctx, trackID string, offset, limit int) ([]model.EditLogEntry, int, error)
AddTrash(ctx, *model.TrashEntry) error; ListTrash(ctx) ([]model.TrashEntry, error); GetTrash(ctx, id) (*model.TrashEntry, error); DeleteTrash(ctx, id) error
type FormatStat struct { Suffix string; Count int; Size int64 }
type LibraryStats struct { Tracks, Albums, Artists, Genres, Playlists, Users, MissingTracks int; TotalDuration float64; TotalSize int64; Formats []FormatStat; PlaysLast30Days int }
Stats(ctx) (*LibraryStats, error)
```
All returned tracks/albums/artists/playlists have computed fields filled:
- `Track.CoverArt` = `CoverArtID("tr", id, updated_at)` if `HasCover`, else `CoverArtID("al", album_id, album.updated_at-or-track.updated_at)`.
- `Album.CoverArt` = `CoverArtID("al", id, updated_at)`; `Artist.CoverArt` = `CoverArtID("ar", id, updated_at)`;
  `Playlist.CoverArt` = `CoverArtID("pl", id, max(updated_at, albums_updated_at))` (latest version of its albums).
- `ContentType` = `util.MimeType(suffix)`, `Starred` = `StarredAt != nil`, `HasLyrics` = `lyrics != '' || has_lrc`
  (list queries select `(lyrics != '') AS has_embedded_lyrics` instead of the full text).

### 5.5 auth
```go
const CookieName = "rainy_session"
func LoadOrCreateKey(path string) ([]byte, error)
type Crypto struct{...}; func NewCrypto(key []byte) *Crypto
func (c *Crypto) Encrypt(plain string) (string, error); func (c *Crypto) Decrypt(enc string) (string, error)
func NewToken() string            // 32 random bytes, base64url
func HashToken(tok string) string // sha256 hex
type Service struct{...}
func NewService(st *store.Store, c *Crypto, sessionTTL time.Duration) *Service
func (s *Service) Crypto() *Crypto
func (s *Service) CreateUser(ctx, u *model.User, password string) error      // validates username/password (≥4 chars), encrypts
func (s *Service) CheckPassword(u *model.User, password string) bool          // constant-time compare after decrypt
func (s *Service) ChangePassword(ctx, userID, password string) error          // also deletes other sessions? no — keeps current
func (s *Service) Login(ctx, username, password, userAgent, ip string) (token string, u *model.User, err error) // rate limited: ErrRateLimited, ErrInvalidCredentials
func (s *Service) Logout(ctx, token string) error
func (s *Service) SessionUser(ctx, token string) (*model.User, error)         // sliding expiry (touch at most once per minute)
func (s *Service) GenerateAPIKey(ctx, userID string) (string, error)          // returns plain key once, stores hash
func (s *Service) UserByAPIKey(ctx, key string) (*model.User, error)
func (s *Service) Middleware(next http.Handler) http.Handler // cookie or "Authorization: Bearer <token>" → ctx user (never rejects)
func RequireUser(next http.Handler) http.Handler     // 401 JSON if no user
func RequireManager(next http.Handler) http.Handler  // 403 unless CanEdit()
func RequireAdmin(next http.Handler) http.Handler    // 403 unless IsAdmin
func WithUser(ctx context.Context, u *model.User) context.Context; func UserFrom(ctx context.Context) *model.User
func SetSessionCookie(w http.ResponseWriter, r *http.Request, token string, ttl time.Duration) // HttpOnly, SameSite=Lax, Secure if TLS/X-Forwarded-Proto=https, Path=/
func ClearSessionCookie(w http.ResponseWriter, r *http.Request)
var ErrInvalidCredentials, ErrRateLimited error
```
Login rate limit: in-memory, per (ip) and per (username): 10 failures / 10 min → locked for 5 min.

### 5.6 events / nowplaying
```go
// events
type Event struct { Type string `json:"type"`; Data any `json:"data"` }
// Types: "scan" (data scanner.Status), "library" (data {"reason": "..."}), "nowPlaying" (data []nowplaying.Entry)
type Bus struct{...}; func NewBus() *Bus
func (b *Bus) Publish(e Event); func (b *Bus) Subscribe() (<-chan Event, func()) // buffered 32; slow subscribers drop events

// nowplaying
type Entry struct { UserID, Username, TrackID, Player string; Since int64 } // json camelCase
type Tracker struct{...}; func New() *Tracker
func (t *Tracker) Set(e Entry); func (t *Tracker) List() []Entry // entries older than 15 min are dropped
```

### 5.7 tags (owner: scanner agent)
```go
var AudioExtensions = map[string]bool{"mp3","flac","m4a","m4b","mp4","aac","alac","ogg","oga","opus","wav","aif","aiff","wma","ape","wv","mpc","dsf","dff","tta","spx"} // keys without dot
func IsAudioFile(name string) bool
type Metadata struct {
    Title, Album, Artist, AlbumArtist string
    TrackNumber, TrackTotal, DiscNumber, DiscTotal int; DiscSubtitle string
    Year int; Date string; OriginalYear int
    Genres []string; Composer, Comment, Lyrics string; BPM int; Compilation bool
    RGTrackGain, RGTrackPeak, RGAlbumGain, RGAlbumPeak *float64
    MbzTrackID, MbzAlbumID, MbzArtistID, MbzAlbumArtistID string
    SortTitle, SortAlbum, SortArtist, SortAlbumArtist string
    Duration float64; Bitrate, SampleRate, BitDepth, Channels int; Codec string
    HasPicture bool
    Raw map[string][]string // all tags as returned by taglib (keys upper-case)
}
type ReadOptions struct { FixEncoding bool } // repair mojibake in text fields (not Raw)
func Read(path string, opt ReadOptions) (*Metadata, error) // tags + properties; ffprobe fallback when enabled
func Rebuild(path string, fallback map[string][]string) (source string, err error) // read old tags, TagLib-write verified copy, replace original
func ReadRaw(path string) (map[string][]string, error)
func Write(path string, changes map[string][]string) error // merge: listed keys replaced, empty slice deletes the key; other keys untouched
func ReadPicture(path string) ([]byte, error)               // first embedded picture, nil if none
func WritePicture(path string, img []byte) error             // front cover at index 0; nil removes
func FixMojibake(s string) (fixed string, encoding string, changed bool) // strict GBK/Big5/Shift-JIS heuristics
```
Parsing rules: `TRACKNUMBER "3/12"` → 3,12 (also `TRACKTOTAL`/`TOTALTRACKS`); same for discs;
`DATE`/`YEAR` → first 4-digit year; `ORIGINALDATE`/`ORIGINALYEAR`; `COMPILATION` "1"/"true";
ReplayGain `"-6.50 dB"`; lyrics from `LYRICS` then `UNSYNCEDLYRICS`.

### 5.8 scanner (owner: scanner agent)
```go
var ErrScanInProgress = errors.New("scan already in progress")
type Options struct { Full bool; LibraryID int64 /*0 = all*/ }
type Status struct {
    Scanning bool; Full bool; LibraryID int64; Phase string /*"idle"|"walking"|"reading"|"refreshing"|"done"|"error"*/
    FilesSeen, Added, Updated, Removed, Moved, Errors int; StartedAt, FinishedAt int64; LastError string
} // json camelCase
type Scanner struct{...}
func New(st *store.Store, bus *events.Bus, cfg *config.Config) *Scanner
func (s *Scanner) Start(ctx context.Context, opt Options) error  // async (detached ctx), ErrScanInProgress
func (s *Scanner) Run(ctx context.Context, opt Options) error    // sync
func (s *Scanner) Status() Status
func (s *Scanner) RescanFiles(ctx context.Context, libraryID int64, relPaths []string) ([]model.Track, error)
    // sync; for each path: re-read (or mark missing / delete if file gone), upsert, refresh affected
    // albums/artists/genres (old and new ids), update album cover paths; returns current tracks for existing files
func (s *Scanner) RescanDir(ctx context.Context, libraryID int64, relDir string) error // quick scan of one dir subtree
func (s *Scanner) Schedule(ctx context.Context) // loop: reads settings.ScanInterval each cycle; runs quick scans until ctx done
func (s *Scanner) LockLibrary() func()          // exclusive lock around file mutations (manage) and scans; returns unlock
```
Behaviour: walk each library (skip `util.IsHiddenOrSystem` names, dirs containing a `.rainyignore`
file), audio files only; quick scan re-reads only when size/mtime differ or track missing; full scan
re-reads everything. Worker pool (NumCPU) reads tags; single writer upserts in batches of 200.
Unseen tracks → `missing=1`. Moved-file detection after walk: a new file matching a missing track
by size + duration (±1s) + title reuses that track id (path updated). Fallbacks when building a
`model.Track`: title ← filename without extension; album ← `"[Unknown Album]"`; artist ←
`"[Unknown Artist]"`; album artist ← ALBUMARTIST, else `"Various Artists"` if compilation, else artist.
`created_at` on first discovery = min(mtime, now). After a scan: `RefreshAll`, album cover paths
via `artwork.FindFolderImage` in each album's directories (settings.CoverArtFiles), artist images
(`artist.*` in the parent dir of the album dir), publish `scan` events (throttled ≤ 2/s) and a final
`library` event, `SetLibraryScanned`.

### 5.9 artwork (owner: scanner agent)
```go
var ErrNotFound = errors.New("artwork not found")
type Image struct { Data []byte; ContentType string; ModTime time.Time }
type Service struct{...}
func New(st *store.Store, cacheDir string) *Service
func (s *Service) Get(ctx context.Context, coverArtID string, size int) (*Image, error)
    // kinds: "al" (cover_path → embedded of cover_track_id), "tr" (embedded → its album), "ar" (image_path → latest album cover),
    // "pl" (2x2 mosaic of first 4 distinct album covers, or single), "" (try track, album, artist, playlist ids).
    // size>0 → fit within size×size, JPEG q85, cached on disk keyed by (id, version, size); size=0 → original bytes.
func (s *Service) Placeholder(size int) *Image // neutral gradient PNG/JPEG with a music-note glyph (for Subsonic clients)
func (s *Service) ClearCache() (freed int64, err error)
func (s *Service) CacheSize() (int64, error)
func FindFolderImage(dir string, patterns []string) string // first match by pattern priority, case-insensitive; jpg/jpeg/png/webp
```

### 5.10 transcode & lyrics (owner: native API agent; used by Subsonic too)
```go
// transcode
type Service struct{...}; func New(ffmpegPath string) *Service
func (s *Service) Available() bool; func (s *Service) Version() string; func (s *Service) Path() string
type Options struct { Format string /*mp3|opus|aac*/; BitRate int /*kbps*/; Offset float64 /*seconds*/ }
func (s *Service) Stream(ctx context.Context, w io.Writer, inputPath string, o Options) error
func ContentType(format string) string   // mp3→audio/mpeg, opus→audio/ogg, aac→audio/aac
func Suffix(format string) string        // mp3, opus, aac
// Decide: requestedFormat ""/"raw" and maxBitRate 0 → no transcode; "raw" → never; known format → transcode;
// maxBitRate>0 && < track bitrate (or lossless) → transcode to def.Format at maxBitRate.
func Decide(t *model.Track, requestedFormat string, maxBitRate int, defFormat string, defBitRate int) (format string, bitRate int, transcode bool)

// lyrics
type Line struct { Start int64 `json:"start"` /*ms; -1 when unsynced*/; Text string `json:"text"` }
type Lyrics struct { Synced bool; Lines []Line; Source string /*"lrc"|"embedded"|"none"*/; Raw string; Offset int64; Lang string } // json camelCase
func Parse(text string) *Lyrics      // LRC ([mm:ss.xx], multi-timestamp lines, [offset:], metadata tags, strips <mm:ss.xx> word stamps) or plain text
func LrcPath(audioAbsPath string) string
func Load(t *model.Track, audioAbsPath string) (*Lyrics, error) // sidecar .lrc first, then embedded t.Lyrics; Source "none" + empty Lines if neither
```

### 5.11 manage (owner: manage agent)
```go
func New(st *store.Store, sc *scanner.Scanner, art *artwork.Service, bus *events.Bus, cfg *config.Config) *manage.Service
```
Everything else inside `manage` is private to that agent; the HTTP contract (§7.6) is what matters.
Rules: every file operation goes through `util.SafeJoin`, holds `scanner.LockLibrary()`, calls
`scanner.RescanFiles` afterwards, writes an `edit_log` row, and publishes a `library` event.
Read-only mounts must fail gracefully (`readonly` API error with a helpful message).

### 5.12 app & server
```go
// app
type App struct {
    Cfg *config.Config; DB *db.DB; Store *store.Store; Auth *auth.Service; Bus *events.Bus
    NowPlaying *nowplaying.Tracker; Scanner *scanner.Scanner; Artwork *artwork.Service
    Transcoder *transcode.Service; Manage *manage.Service; StartedAt time.Time
}
func New(ctx context.Context, cfg *config.Config) (*App, error) // open+migrate DB, key, services, default library
func (a *App) Settings(ctx context.Context) model.Settings       // store.GetSettings with defaults (errors logged → defaults)
func (a *App) Close() error

// server
func New(a *app.App) *http.Server
```
Router:
```
/rest/*          subsonic.New(app).Routes()       (CORS: allow any origin; GET+POST)
/api/*           api.New(app).Routes()            (auth middleware; JSON; gzip for JSON only)
/api/health      {"status":"ok"} (public; docker healthcheck)
/*               SPA from embedded web/dist: exact file if exists, else index.html.
                 Cache-Control: /assets/* → public, max-age=31536000, immutable;
                 index.html, sw.js, registerSW.js, manifest.webmanifest → no-cache
```
`api` package layout (each file owned by one agent, see §11): `api.go` (New, Routes, mounts the
per-area `routesX(r chi.Router)` funcs), `helpers.go` (writeJSON, writeError, decodeJSON, pagination,
`userFrom`), and per-area files that each define their own `func (a *API) routesX(r chi.Router)`.

## 6. Subsonic / OpenSubsonic (`/rest`) — owner: subsonic agent

- Routes: `/rest/{method}` and `/rest/{method}.view`, GET and POST (`application/x-www-form-urlencoded`
  merged with query; OpenSubsonic `formPost`).
- Response: `f=xml` (default), `json`, `jsonp` (`callback`). Envelope attributes: `status`,
  `version="1.16.1"`, `type="rainy"`, `serverVersion=buildinfo.Version`, `openSubsonic=true`.
  XML namespace `http://subsonic.org/restapi`. Model response structs with both `xml` and `json` tags
  (lists → JSON arrays, omit empty optional fields).
- Auth: `u` + (`p` plain or `enc:<hex>`) | (`t`=md5(password+`s`), `s`) | `apiKey` (OpenSubsonic).
  Error codes: 0 generic, 10 missing parameter, 40 wrong credentials, 41 token auth not supported,
  42 provided auth mechanism not supported, 43 multiple conflicting auth, 44 invalid API key, 50 not authorized, 70 not found.
- IDs: same ids as the native API. Folder browsing is **simulated**: `getIndexes` lists album artists,
  `getMusicDirectory(artistId)` → albums as directories, `getMusicDirectory(albumId)` → songs.
  `getMusicFolders` → libraries.
- Implement: `ping getLicense getOpenSubsonicExtensions getMusicFolders getIndexes getMusicDirectory
  getGenres getArtists getArtist getAlbum getSong getArtistInfo getArtistInfo2 getAlbumInfo getAlbumInfo2
  getSimilarSongs getSimilarSongs2 getTopSongs getAlbumList getAlbumList2 getRandomSongs getSongsByGenre
  getNowPlaying getStarred getStarred2 search search2 search3 getPlaylists getPlaylist createPlaylist
  updatePlaylist deletePlaylist stream download getCoverArt getLyrics getLyricsBySongId getAvatar star
  unstar setRating scrobble getBookmarks createBookmark deleteBookmark getPlayQueue savePlayQueue
  getPlayQueueByIndex savePlayQueueByIndex getUser getUsers createUser updateUser deleteUser changePassword
  getScanStatus startScan getInternetRadioStations createInternetRadioStation updateInternetRadioStation
  deleteInternetRadioStation`; stubs returning empty lists for podcasts, shares, chat, jukebox
  (`jukeboxControl` → error 0 "not supported").
- OpenSubsonic extensions advertised: `formPost`, `songLyrics`, `transcodeOffset`, `apiKeyAuthentication`,
  `indexBasedQueue`.
- `search3` with `query=""` (or `""` quoted) returns everything, paged — required by Symfonium & co.
- `getAlbumList2` types: random, newest, highest, frequent, recent, alphabeticalByName,
  alphabeticalByArtist, starred, byYear (fromYear>toYear → descending), byGenre.
- `stream`: `maxBitRate`, `format` (`raw` = original), `timeOffset`, `estimateContentLength`; raw files via
  `http.ServeContent` (Range support!). Transcoding via `transcode.Decide`/`Stream`.
- `scrobble`: `submission=false` → now playing; `true` → `store.RecordPlay` (`time` param in ms).
- Song (`Child`) fields: id, parent (albumId), isDir=false, title, album, artist, track, year, genre,
  coverArt, size, contentType, suffix, transcodedContentType/transcodedSuffix when relevant, duration,
  bitRate, bitDepth, samplingRate, channelCount, path, playCount, played, discNumber, created, albumId,
  artistId, type="music", mediaType="song", starred, userRating, bpm, comment, sortName, musicBrainzId,
  genres[{name}], artists[{id,name}], displayArtist, albumArtists[{id,name}], displayAlbumArtist,
  displayComposer, replayGain{trackGain,albumGain,trackPeak,albumPeak}.

## 7. Native API (`/api`) — JSON contract

- Auth: session cookie `rainy_session` (set by login/setup) or `Authorization: Bearer <token>`.
  `SameSite=Lax` cookie; all state-changing endpoints are non-GET.
- Errors: HTTP status + `{"error":{"code":"<code>","message":"<human text>"}}`, codes:
  `bad_request 400, unauthorized 401, forbidden 403, not_found 404, conflict 409, readonly 409,
  rate_limited 429, internal 500, not_implemented 501, unavailable 503`.
- Lists: `?offset=0&limit=50&sort=<field>&order=asc|desc` → `{"items":[...],"total":N}` (`Page<T>`).
  limit default 50, max 1000.
- Booleans in query strings: `1|true`.

### 7.1 Auth & me
| Method | Path | Body | Response |
|---|---|---|---|
| GET | `/api/auth/status` (public) | | `{initialized:boolean, user:User|null, version:string}` |
| POST | `/api/auth/setup` (public, only when 0 users) | `{username,password}` | `User` (+cookie) — creates admin |
| POST | `/api/auth/login` (public) | `{username,password}` | `User` (+cookie) |
| POST | `/api/auth/logout` | | 204 |
| GET | `/api/me` | | `User` |
| PUT | `/api/me` | `{displayName?,email?}` | `User` |
| PUT | `/api/me/password` | `{currentPassword,newPassword}` | 204 |
| POST | `/api/me/apikey` | | `{apiKey:string}` (shown once) |
| DELETE | `/api/me/apikey` | | 204 |

### 7.2 Library
| Method | Path | Query | Response |
|---|---|---|---|
| GET | `/api/home` | | `Home` |
| GET | `/api/albums` | `q, sort, order, offset, limit, artistId, genre, fromYear, toYear, starred, libraryId` | `Page<Album>` |
| GET | `/api/albums/{id}` | | `AlbumDetail` (`Album & {tracks: Track[], discs: number[]}`) |
| GET | `/api/artists` | `q, sort, order, offset, limit, all (include non-album artists), starred` | `Page<Artist>` |
| GET | `/api/artists/{id}` | | `ArtistDetail` (`Artist & {albums: Album[], appearsOn: Album[], topTracks: Track[]}`) |
| GET | `/api/tracks` | `q, sort, order, offset, limit, albumId, artistId, genre, starred, fromYear, toYear, libraryId, dir, dirPrefix, missing(include|only), ids(comma)` | `Page<Track>` |
| GET | `/api/tracks/{id}` | | `Track` |
| GET | `/api/genres` | | `Genre[]` |
| GET | `/api/search` | `q, artists=6, albums=12, tracks=30` | `SearchResult` |
| GET | `/api/starred` | | `{artists: Artist[], albums: Album[], tracks: Track[]}` |
| GET | `/api/random` | `size=50 (max 500), genre, fromYear, toYear` | `Track[]` |
| GET | `/api/recent-tracks` | `limit=50` | `Track[]` (this user's play history, newest first, distinct) |

`Home` = `{recentlyAdded: Album[12], recentlyPlayed: Album[12], mostPlayed: Album[12], random: Album[12], starred: Album[12], stats: {tracks,albums,artists}}`.
Tracks in `/api/tracks` include `missing` only when requested; `missing` filter parameters are only
honoured for managers.

### 7.3 Annotations
| POST | `/api/star` | `{type:'track'|'album'|'artist', ids:string[], starred:boolean}` | 204 |
|---|---|---|---|
| POST | `/api/rating` | `{type, id, rating:0-5}` | 204 |
| POST | `/api/scrobble` | `{trackId, submission:boolean, time?:number /*ms*/}` | 204 |

### 7.4 Playlists & queue & radio
| Method | Path | Body | Response |
|---|---|---|---|
| GET | `/api/playlists` | | `Playlist[]` (own + public) |
| POST | `/api/playlists` | `{name, comment?, public?, trackIds?}` | `Playlist` |
| GET | `/api/playlists/{id}` | | `PlaylistDetail` (`Playlist & {tracks: Track[], readonly: boolean}`) |
| PUT | `/api/playlists/{id}` | `{name?, comment?, public?}` | `Playlist` |
| DELETE | `/api/playlists/{id}` | | 204 |
| POST | `/api/playlists/{id}/tracks` | `{trackIds}` (append) | `Playlist` |
| PUT | `/api/playlists/{id}/tracks` | `{trackIds}` (replace the visible order; hidden entries of missing tracks are kept next to the entry they followed) | `Playlist` |
| DELETE | `/api/playlists/{id}/tracks` | `{positions:number[]}` | `Playlist` |
| GET | `/api/queue` | | `{trackIds, currentId, positionMs, changedBy, updatedAt, tracks: Track[]}` |
| PUT | `/api/queue` | `{trackIds, currentId, positionMs, currentIndex?}` (`currentIndex` picks which copy of a song queued twice is current) | 204 |
| GET | `/api/radios` | | `RadioStation[]` |
| POST/PUT/DELETE | `/api/radios`, `/api/radios/{id}` (admin) | `{name, streamUrl, homepageUrl?}` | `RadioStation` / 204 |

Only the owner (or an admin) may modify a playlist; others get 403.

### 7.5 Media
| GET | `/api/stream/{trackId}` | `format=raw|mp3|opus|aac, bitrate, offset` | audio. raw → `http.ServeContent` (Range); transcode → chunked |
|---|---|---|---|
| GET | `/api/download/{trackId}` | | file (attachment; requires canDownload + settings.enableDownloads) |
| GET | `/api/download/album/{albumId}` | | streamed zip (store method) |
| GET | `/api/cover/{coverArtId}` | `size` | image. `Cache-Control: public, max-age=31536000, immutable` when id has `_<version>`; `404` when none |
| GET | `/api/lyrics/{trackId}` | | `Lyrics` |
| GET | `/api/events` | | `text/event-stream`: `event: scan|library|nowPlaying`, `data: <json>`; `: ping` every 25 s |

### 7.6 Manage (requires `canManage` or admin) — owner: manage agent
| Method | Path | Body / Query | Response |
|---|---|---|---|
| GET | `/api/manage/tracks/{id}/tags` | | `TrackTags` |
| POST | `/api/manage/tags` | `{edits: TagEdit[]}` | `BatchResult` |
| POST | `/api/manage/tags/rebuild` | `{trackIds: string[]}` | `BatchResult` |
| GET | `/api/manage/tracks/{id}/picture` | | embedded picture bytes (404 if none) |
| POST | `/api/manage/cover` | multipart: `file`, `trackIds` (comma list) **or** `albumId`, `embed` (default true), `saveToFolder` (default false) | `BatchResult` |
| DELETE | `/api/manage/cover` | `{trackIds?, albumId?, removeFolderImage?:boolean}` | `BatchResult` |
| PUT | `/api/manage/tracks/{id}/lyrics` | `{text, target:'embedded'|'lrc'}` (empty text removes) | `Track` |
| POST | `/api/manage/rename/preview` | `{trackIds, pattern}` | `{items: RenamePlan[]}` |
| POST | `/api/manage/rename` | `{trackIds, pattern}` | `BatchResult` |
| POST | `/api/manage/upload` | multipart: `files` (repeated), `libraryId`, `dir?`, `organize?` | `BatchResult` |
| POST | `/api/manage/delete` | `{trackIds}` | `BatchResult` (files moved to trash) |
| GET | `/api/manage/trash` | | `TrashEntry[]` |
| POST | `/api/manage/trash/restore` | `{ids}` | `BatchResult` |
| POST | `/api/manage/trash/purge` | `{ids?}` (omit = empty trash) | `{purged:number}` |
| POST | `/api/manage/missing/purge` | `{trackIds?}` (omit = all missing) | `{purged:number}` |
| GET | `/api/manage/folders` | `libraryId, dir` | `FolderListing` |
| POST | `/api/manage/folders/rescan` | `{libraryId, dir}` | 202 |
| GET | `/api/manage/issues/summary` | | `Record<IssueType, number>` |
| GET | `/api/manage/issues` | `type, offset, limit` | `Page<Issue>` |
| POST | `/api/manage/encoding` | `{trackIds, apply:boolean}` | `{items: EncodingFix[], result?: BatchResult}` |
| GET | `/api/manage/log` | `trackId?, offset, limit` | `Page<EditLogEntry>` |

Tag keys are TagLib property names, upper-case (`TITLE, ARTIST, ALBUM, ALBUMARTIST, TRACKNUMBER,
DISCNUMBER, DATE, GENRE, COMPOSER, COMMENT, LYRICS, BPM, COMPILATION, DISCSUBTITLE, …`). `[]` deletes.
Rename pattern tokens: `{title} {artist} {album} {albumartist} {track} {track:N} {disc} {disc:N}
{year} {genre} {composer}`; `/` separates directories; `[...]` is an optional segment dropped if every
token inside is empty; the original extension is always appended; illegal characters (`<>:"/\|?*`
and control chars) → `_`; trailing dots/spaces trimmed; each component ≤ 200 bytes. Sidecar `.lrc`
files move with their track; directories left empty are removed (never the library root).

### 7.7 Admin (requires admin) — owner: manage agent
| Method | Path | Body | Response |
|---|---|---|---|
| GET | `/api/admin/users` | | `User[]` |
| POST | `/api/admin/users` | `{username,password,displayName?,email?,isAdmin?,canManage?,canDownload?}` | `User` |
| PUT | `/api/admin/users/{id}` | same fields optional + `password?` | `User` (cannot demote/delete yourself) |
| DELETE | `/api/admin/users/{id}` | | 204 |
| GET | `/api/admin/libraries` | | `LibraryInfo[]` |
| POST | `/api/admin/libraries` | `{name, path}` | `LibraryInfo` (path must be an existing dir) |
| PUT | `/api/admin/libraries/{id}` | `{name?, path?}` | `LibraryInfo` |
| DELETE | `/api/admin/libraries/{id}` | | 204 (DB rows only, never files) |
| GET | `/api/admin/scan` | | `ScanStatus` |
| POST | `/api/admin/scan` | `{full?:boolean, libraryId?:number}` | 202 `ScanStatus` (409 if running) |
| GET | `/api/admin/settings` | | `Settings` |
| PUT | `/api/admin/settings` | partial `Settings` | `Settings` |
| GET | `/api/admin/stats` | | `LibraryStats` |
| GET | `/api/admin/system` | | `SystemInfo` |
| POST | `/api/admin/cache/clear` | | `{freed:number}` |

## 8. TypeScript contract (`web/src/lib/api/types.ts`)

```ts
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
  fixEncodingOnScan: boolean; enableDownloads: boolean
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
export interface NowPlayingEntry { userId: string; username: string; trackId: string; player: string; since: number }
export interface ServerEvent { type: 'scan' | 'library' | 'nowPlaying'; data: unknown }
```

## 9. Frontend architecture (`web/`)

```
web/src/
├── main.tsx, app.tsx (providers), router.tsx, index.css
├── components/ui/*            shadcn components (generated; edit only to theme)
├── components/                shared: cover-art, page-header, empty-state, error-state, spinner, theme-provider, logo
├── layouts/                   app-shell (sidebar / mobile tab bar / top bar / player slots), auth-layout
├── lib/api/{client,types,endpoints}.ts   fetch wrapper, contract types, one typed function per endpoint
├── lib/{utils,format,cover,i18n,query-client,platform}.ts
├── lib/lyrics/bilingual.ts    bilingual lyrics analysis, split, and display grouping (no imports; unit-tested in web/tests)
├── stores/ui.ts               global UI state: tag-editor + add-to-playlist openers, sidebar collapsed
├── hooks/                     use-media-query, use-auth, use-server-events
├── features/auth/             login + first-run setup pages
├── features/library/          browsing pages + track list + album/artist cards + context menus + add-to-playlist
├── features/player/           audio engine, queue store, player bar, mini player, now-playing sheet, lyrics, queue, PWA glue
├── features/settings/         user settings page
├── features/manage/           library manager, tag editor, rename, upload, doctor, trash, history
├── features/admin/            users, libraries & scan, server settings, system
└── locales/{en,zh}/{common,auth,library,player,settings,manage,admin}.json
```

### 9.1 Routes
| Path | Page | Owner |
|---|---|---|
| `/login`, `/setup` | auth pages (no shell) | FE foundation |
| `/` | Home | library |
| `/search` | Search | library |
| `/library` | Mobile library hub (list of sections) | library |
| `/albums`, `/albums/:id` | Albums grid, album detail | library |
| `/artists`, `/artists/:id` | Artists, artist detail | library |
| `/songs` | Songs (virtualized) | library |
| `/genres`, `/genres/:name` | Genres, genre detail | library |
| `/favorites` | Starred | library |
| `/playlists`, `/playlists/:id` | Playlists, playlist detail | library |
| `/radio` | Internet radio | library |
| `/settings` | User settings | player |
| `/manage` | Metadata (track table + tag editor) | manage |
| `/manage/folders` | Folder browser | manage |
| `/manage/upload` | Upload | manage |
| `/manage/doctor` | Library doctor | manage |
| `/manage/trash` | Trash | manage |
| `/manage/history` | Edit history | manage |
| `/admin/users`, `/admin/libraries`, `/admin/settings` | Admin | manage |

Every page module lives at `web/src/features/<area>/pages/<name>-page.tsx` and default-exports its
component; `router.tsx` lazy-loads them. Manage routes are guarded (canManage/admin), admin routes
guarded (isAdmin); unauthenticated users are redirected to `/login` (or `/setup` when not initialized).

### 9.2 Cross-feature contracts
- `@/lib/api/endpoints` — `api.albums.list(params)`, `api.albums.get(id)`, … one function per endpoint in §7.
  Errors throw `ApiError { status, code, message }`. `401` from any call (except auth endpoints)
  invalidates the `['auth']` query so the router redirects to `/login`.
- `@/lib/cover` — `coverUrl(coverArt: string, size: number): string` snaps size to 64/128/256/512/1024 (device-pixel aware), returns `''` for empty ids.
- `@/lib/format` — `formatDuration(sec)`, `formatDurationLong(sec)`, `formatBytes(n)`, `formatDate(ms)`, `formatRelative(ms)`, `formatBitrate`.
- `@/stores/ui` — `useUI` zustand store: `tagEditor: {open: boolean, trackIds: string[]}`, `openTagEditor(ids)`, `closeTagEditor()`, `addToPlaylist: {open, trackIds}`, `openAddToPlaylist(ids)`, `closeAddToPlaylist()`.
  The shell mounts `<TagEditorHost/>` (from `@/features/manage/tag-editor-host`) and `<AddToPlaylistHost/>` (from `@/features/library/components/add-to-playlist-host`).
- `@/features/player/store` — `usePlayer` zustand store (owned by the player agent; the foundation writes a working version):
  ```ts
  type RepeatMode = 'off' | 'all' | 'one'
  interface PlayerState {
    queue: Track[]; index: number            // -1 = nothing loaded
    isPlaying: boolean; shuffle: boolean; repeat: RepeatMode; volume: number; muted: boolean
    nowPlayingOpen: boolean; panel: 'none' | 'queue' | 'lyrics'
    playTracks(tracks: Track[], startIndex?: number, opts?: { shuffle?: boolean }): void
    playNext(tracks: Track[]): void; addToQueue(tracks: Track[]): void
    play(): void; pause(): void; togglePlay(): void; next(): void; prev(): void
    jumpTo(index: number): void; removeAt(index: number): void; move(from: number, to: number): void; clearQueue(): void
    setVolume(v: number): void; toggleMute(): void; toggleShuffle(): void; cycleRepeat(): void
    setNowPlayingOpen(open: boolean): void; setPanel(p: 'none' | 'queue' | 'lyrics'): void
  }
  export const usePlayer: UseBoundStore<StoreApi<PlayerState>>
  export const useCurrentTrack: () => Track | undefined
  ```
  Playback position lives in a separate high-frequency store `usePlayback` (`currentTime, duration, buffered, seek(t)`) so lists don't re-render.
- Shell slots imported from the player feature: `<PlayerDock/>` (tablet/desktop: bottom bar, floating window or floating mini bar; positions itself), `<MiniPlayer/>` (mobile, above tab bar), `<NowPlayingSheet/>`, `<AudioEngine/>` (headless).
- `@/features/player/dock` — `usePlayerDock` zustand store (persisted as `rainy.player-dock`): `mode: 'bar' | 'window' | 'compact'`, `barAutoHide: boolean`, `setMode(mode)`, `setBarAutoHide(b)`. `<PlayerDock/>` mirrors it to `<html data-player-dock="bar|collapsed|window|compact|idle">`, which `index.css` maps to `--player-reserve` (full-width bottom space: sidebar, side panel), `--player-clearance` (`.page-pad`, sticky footers) and `--player-toast-bottom/right`.
- `@/features/library/components/track-actions` — `<TrackActionsMenu tracks={Track[]} context?:{playlistId?:string, position?:number}>` renders a "…" button (DropdownMenu on desktop, Drawer action sheet on mobile) with: Play next, Add to queue, Add to playlist, Go to album, Go to artist, Star/Unstar, Download, Edit tags (managers → `openTagEditor`), Remove from playlist (when in playlist context).

### 9.3 Design system
- Aesthetic: calm, modern, Apple-Music-inspired minimalism. Lots of whitespace, big artwork, restrained colour; the accent is used sparingly (play buttons, active nav, progress).
- shadcn style `new-york`, base colour `neutral`, CSS variables, radius `0.75rem`. Light + dark (follows system, user override). Accent presets selectable in settings (CSS var `--primary`): `rain` (default, `oklch(0.62 0.17 255)` periwinkle blue), `rose`, `violet`, `emerald`, `amber`, `graphite`.
- Font: `"Inter Variable", -apple-system, BlinkMacSystemFont, "SF Pro Text", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", "Noto Sans CJK SC", sans-serif`; tabular numbers for times (`tabular-nums`).
- Type scale: page title `text-3xl font-bold tracking-tight` (mobile large title 34px/41px bold, collapsing into a 17px semibold centered nav title on scroll — `PageHeader` handles it), section title `text-xl font-semibold`, body `text-sm`, secondary `text-muted-foreground`.
- Artwork: `rounded-lg` (≥ 160px) / `rounded-md` (thumbnails), subtle `shadow-sm` and 1px `ring-black/5 dark:ring-white/10`; artists are circles. Always square (`aspect-square object-cover`), lazy loaded, fade-in, gradient placeholder with a music-note icon when missing.
- Glass surfaces (tab bar, mini player, top bar on scroll): `bg-background/75 backdrop-blur-xl backdrop-saturate-150 border-border/60`.
- Layout: desktop ≥ 1024px → left sidebar (240px, shadcn Sidebar, collapsible to icons) + content + 80px bottom player bar. 768–1023px → collapsed icon sidebar. < 768px → bottom tab bar (Home, Library, Search, and Metadata for managers) 49px + safe area, floating mini player (56px, `rounded-xl`, 8px side margins) above it.
- Navigation tiers (`layouts/nav.ts`): `LIBRARY_NAV` for listening; `METADATA_NAV` (`/manage`) is always visible to managers (sidebar item, phone tab); `MANAGE_NAV` (folders, upload, doctor, trash, history) and `ADMIN_NAV` are folded — sidebar Collapsibles "Library tools" / "Admin" that start closed and open while one of their routes is active (a DropdownMenu flyout on the icon rail), and one "Library tools" DropdownMenu (`ManageSections`) on the phone Metadata page.
- CSS vars in `index.css`: `--tabbar-h`, `--miniplayer-h`, `--playerbar-h`, `--player-window-w` (400px), `--compact-player-h` (68px), `--player-reserve`, `--player-clearance`, `--safe-top/bottom` (env(safe-area-inset-*)); pages use `.page-pad` bottom padding utility so content never hides behind player chrome. Never hard-code `--playerbar-h` for spacing: the bar may be hidden or replaced by a floating player.
- Motion: `motion/react` springs (`type:'spring', stiffness: 400, damping: 36`), `MotionConfig reducedMotion="user"`. Press feedback on mobile: `active:scale-[0.97]` transitions.
- Icons: lucide-react, `size-4`/`size-5`, `strokeWidth={1.75}`; transport controls use filled glyphs (`fill="currentColor"`).
- Density: desktop list rows 48px; mobile rows 60px with 44px artwork, iOS-style hairline separators inset from the artwork.
- Touch: `-webkit-tap-highlight-color: transparent`, `overscroll-behavior: none` on body, `touch-action: manipulation`; hit targets ≥ 44px on mobile.
- Accessibility: every icon button has `aria-label`; focus rings visible; colour contrast AA.

### 9.4 iOS-style player (player agent)
- **Mini player** (mobile): floating glass pill above the tab bar: artwork 40px rounded-md, title/artist (one line each, ellipsis), play/pause + next; thin progress line along the bottom edge; tap → Now Playing; swipe up also opens.
- **Now Playing** (full-screen sheet, mobile; large centered modal/overlay on desktop): slides up with a spring, drag-down to dismiss (grabber at top), background = blurred, saturated artwork colours (`fast-average-color` or canvas sampling + large blurred artwork layer), light text. Artwork large and rounded, **scales down with a spring when paused** and back up when playing (Apple Music behaviour). Title (semibold) + artist (tappable → artist page, closes sheet) with a star button and "…" menu. Scrubber: thin track that thickens while dragging, elapsed / −remaining underneath in tabular nums. Transport: prev / play-pause / next, large filled glyphs, press-scale feedback. Volume slider (hidden on iOS where `audio.volume` is read-only). Bottom row: lyrics toggle, AirPlay button when `window.WebKitPlaybackTargetAvailabilityEvent` exists (`audio.webkitShowPlaybackTargetPicker()`), queue toggle. Lyrics view: synced lines, active line bold/bright with others dimmed, auto-scroll with smooth centring, tap a line to seek; plain lyrics scroll normally. Bilingual lyrics (`groupBilingual`: consecutive synced lines with the same `start` → original + translations; otherwise space-separated `original 中文` lines when most lines split) render the translation beneath each line (~0.68em, dimmer); lines carry `lang` from `guessLang` (`ja`/`ko`/`zh`, with matching CJK font stacks in `index.css`). `TranslationToggle` (bottom action row on phones, stage header, side panel header; only while lyrics are open and have translations) flips the persisted playback pref `lyricsTranslation` (default on, also in Settings → Playback). Queue view: "Playing Next" list with drag handles (dnd-kit), shuffle / repeat toggles, clear.
- **Desktop player bar**: artwork + title/artist (links) + star | transport + scrubber with times | lyrics, queue, volume, layout menu, expand. Side panel (right, 360px) for queue / lyrics (bar layout only).
- **Player layouts** (≥ 768px, layout menu = `PictureInPicture2` button in the bar, window and mini bar; persisted in `usePlayerDock`): *bottom bar* (default); *auto-hide* (NetEase style: the bar slides below the viewport leaving a centred handle tab; hovering the handle / bottom edge or keyboard focus peeks it until the pointer or focus leaves, clicking the handle pins it open; on a pinned bar the handle appears on hover and turns auto-hide on); *floating window* (bottom-right 400×min(640px, viewport − 3rem), artwork backdrop like Now Playing: queue position, grabber → mini bar, layout menu, full screen; artwork that scales down while paused, title/star/…, scrubber, shuffle/prev/play/next/repeat, volume, lyrics/translation/AirPlay/queue — lyrics and queue replace the artwork inside the window; not modal); *floating mini bar* (bottom-right 390×68px glass card: artwork + title/artist → opens the window, layout menu, play/pause, next, thin progress line; dragging sideways on it scrubs relative to the current position — a full-width drag = 20 % of the track, 20 s…10 min — with an origin → target ±delta preview above it, Escape cancels). Floating layouts render nothing while idle and hide under full-screen Now Playing, which is available from every layout and returns to it on close. Phones keep the mini pill + sheet.
- Audio engine: single `HTMLAudioElement`, `preload="auto"`, next track preloaded ~20 s before the end with a second element; Media Session (metadata with artwork 96–512, play/pause/prev/next/seekto/seekbackward/seekforward, `setPositionState`); scrobble "now playing" on start and submission after 50 % or 4 min; queue persisted to localStorage immediately and to `/api/queue` debounced (10 s) so the Subsonic clients can resume it; restore on load (paused). Streaming quality setting (original / 320 / 192 / 128 kbps + format) → stream URL params. ReplayGain option (track/album/off) applied via volume scaling (not on iOS). Keyboard: space, ←/→ seek 5s, shift+←/→ prev/next, m mute. Errors → toast and skip.
- **PWA**: `vite-plugin-pwa` (generateSW, `registerType: 'prompt'` with an "update available" toast), manifest (name "Rainy", short_name "Rainy", `display: standalone`, theme/background colours for light+dark, icons 192/512/maskable + apple-touch-icon 180), iOS meta tags (`apple-mobile-web-app-capable`, `apple-mobile-web-app-status-bar-style=black-translucent`, `viewport-fit=cover`), navigateFallback `index.html` with denylist `/api`, `/rest`. Runtime caching: `/api/cover/*` CacheFirst (500 entries, 30 days); never cache `/api/stream`, `/api/download`, `/api/events`.

### 9.5 Management UI (manage agent)
- **Metadata** `/manage`: virtualized track table (checkbox column, cover, title, artist, album, album artist, #, disc, year, genre, format, bitrate, path; sortable; column visibility), search + filters (album/artist/genre/folder/missing), multi-select (click, shift-range, ctrl/cmd toggle, select all matching). Toolbar: Edit tags, Cover, Rename/Organize, Fix encoding, Rebuild tags, Delete, Rescan. Mobile: list with selection mode.
- **Tag editor** (`TagEditorHost`, opened via `useUI.openTagEditor(ids)`): right-side Sheet (desktop, ~560px) / full-screen Drawer (mobile). Tabs: *Details* (common fields; with multiple tracks selected, differing values show a "Multiple values" placeholder and are only written if edited — each field has a revert button), *Cover* (preview, drop/paste/upload, remove, apply to whole album, save as folder image), *Lyrics* (textarea with LRC highlighting, "insert timestamp at current playback time" button that also restamps following lines sharing the old timestamps, target embedded / .lrc; `analyzeBilingual` runs on every edit and, when most non-Chinese lines are `original 中文`, shows a banner with a preview and "Split lines" → `toPairedLrc` rewrites timed lines as same-timestamp pairs, original first; nothing is applied without the user; a "Bilingual" badge marks already paired text), *All tags* (raw key/value table incl. custom keys, add/remove), *File* (read-only file info). Tools menu with preview-before-apply: auto-number tracks (by current order), tags from filename pattern, find & replace in a field (regex optional), case transforms, copy field to field, clear field. Save → `POST /api/manage/tags` with per-track diffs only; show per-track errors.
- **Rename / organize** dialog: pattern input with token chips + saved default from settings, live preview table (from → to, status badges), apply.
- **Upload**: drag & drop zone (files + folders), per-file progress (XHR upload progress), target library + folder picker, "organize by tags" toggle.
- **Folder browser**, **Doctor** (issue summary cards → lists with quick actions), **Trash** (restore / purge), **History** (edit log with readable diffs).
- **Admin**: users (table + dialog; roles: admin, manager, download), libraries & scan (cards with path, counts, writable badge; quick/full scan buttons; live progress via `/api/events`), server settings form, system info (versions, ffmpeg, sizes, clear cache).

### 9.6 i18n
- i18next with namespaces = files in `locales/<lng>/<ns>.json`; languages `zh` (简体中文) and `en`; detection: saved choice → `navigator.language` (`zh*` → zh) → en.
- Each feature owns its namespace file(s); `common` is owned by the FE foundation (feature agents may add keys only to their own namespaces).

## 10. Build, run, test

```bash
# backend (dev)
go run ./cmd/rainy                         # serves embedded web/dist (build web first) on :7650
go test ./...                               # unit tests
go vet ./...
# frontend (dev)
cd web && pnpm install && pnpm dev          # vite on :5173, proxies /api and /rest to :7650
pnpm build                                  # outputs web/dist (embedded by web/embed.go)
pnpm lint && pnpm typecheck
# test library
bash scripts/gen-testdata.sh                # → testdata/music (needs ffmpeg)
RAINY_MUSIC_DIR=./testdata/music RAINY_DATA_DIR=./testdata/data go run ./cmd/rainy
# full build (Makefile targets: docs/development/local-dev.md)
make docker-build                           # local image rainy:dev
make docker-up                              # development compose (deploy/compose/dev.yml)
```

## 11. Ownership (parallel work rules)

Agents work concurrently in one tree. **Only edit files you own.** If you need something from
another area that does not exist yet, code against the contract in this document; if the contract
is insufficient, write a note in your final report instead of editing someone else's file.
Do not run `go get`, `go mod tidy`, `pnpm add` or change `go.mod`/`package.json` in wave 2 —
all dependencies are installed by the foundation; report missing ones.
Build/typecheck only your own packages (`go build ./internal/<pkg>/...`, `go test ./internal/<pkg>/...`;
for TS filter `pnpm typecheck` output to your paths) because other areas may be mid-edit.

| Area | Owner | Files |
|---|---|---|
| Backend foundation | BE0 | `go.mod`, `go.sum`, `cmd/`, `internal/{buildinfo,config,db,model,util,store,auth,events,nowplaying,app,server}/`, `internal/api/{api.go,helpers.go}`, stubs of every other package, `web/embed.go`, `scripts/gen-testdata.sh`, `Makefile`, `.gitignore` |
| Frontend foundation | FE0 | `web/*` config files, `web/src/{main.tsx,app.tsx,router.tsx,index.css}`, `web/src/components/**`, `web/src/layouts/**`, `web/src/lib/**`, `web/src/stores/**`, `web/src/hooks/**`, `web/src/features/auth/**`, `web/src/locales/*/{common,auth}.json`, stubs of other features' shared modules |
| Scanner / tags / artwork | A | `internal/{tags,scanner,artwork}/**` |
| Subsonic | B | `internal/subsonic/**` |
| Native API + media | C | `internal/api/{auth,library,annotations,playlists,queue,media,radio,events}*.go`, `internal/{transcode,lyrics}/**` |
| Manage + admin API | D | `internal/manage/**`, `internal/api/{manage,admin}*.go` |
| Library UI | E | `web/src/features/library/**`, `web/src/locales/*/library.json` |
| Player + PWA + settings UI | F | `web/src/features/{player,settings}/**`, `web/public/**` (icons), `web/pwa.config.ts` (VitePWA options imported by `vite.config.ts`), `web/index.html`, `web/src/locales/*/{player,settings}.json` |
| Manage + admin UI | G | `web/src/features/{manage,admin}/**`, `web/src/locales/*/{manage,admin}.json` |
| Docker + docs | H | `Dockerfile`, `docker-compose.yml`, `.dockerignore`, `docker/**`, `README.md`, `docs/readme/**`, `.github/**` |

## 12. Foundation implementation notes (BE0, wave 1)

Clarifications and additions made while implementing the foundation. They extend the
contracts above (nothing above was removed or renamed); read the package doc comments for details.

- **util**: `HashID` trims + lower-cases every part itself (callers may pass raw names); helpers
  `AlbumID(albumArtist, name)`, `ArtistID(name)`, `GenreID(name)`. `NormalizeSearch` additionally
  removes apostrophes and turns other punctuation/symbols into spaces ("EX/AMPLE" → "ex ample").
  Extras: `EscapeLike`, `StripArticle`, `SplitCoverArtID` (kind, id, version), `PathParts(rel)`
  (dir, filename, lower suffix), `ToRel`, `IsWithin`, `EnsureWithinRoot` (symlink-aware check),
  `ErrUnsafePath`, `IsImageSuffix`, `RFC3339Ms`, `FirstNonEmpty`, cover kind constants.
- **model**: `Track`/`EditLogEntry` implement `MarshalJSON` so `genres` is never `null` and `details`
  defaults to `{}` — therefore never embed `Track`/`EditLogEntry` in another struct (the embedded
  MarshalJSON would swallow the outer fields); `store.GetPlayQueue` guarantees non-nil `TrackIDs`. `Track` has two never-serialised helper columns
  (`HasEmbeddedLyrics`, `AlbumUpdatedAt`). `model.SplitGenre`, `GenreJoiner`, `FormatDuration`,
  `Settings.ScanIntervalDuration()/IgnoredArticleList()/CoverArtPatterns()`.
- **store**: `ErrConflict` (unique violations → HTTP 409) and `ErrInvalid` (bad arguments → 400)
  besides `ErrNotFound`. `UpsertTracks` owns the derived columns: it recomputes `dir`, `filename`,
  `suffix`, `genre` (joined `"; "`) and `search_text` from the other fields, and fills empty ids /
  timestamps. `TrackQuery.Played` / `AlbumQuery.Played` restrict to items the user has played
  (use with sort `played`/`frequent` for "recently/most played"). `FromYear > ToYear` is swapped.
  `Search`: a limit ≤ 0 returns no items of that kind. `GetPlayQueue` returns an empty queue (not
  `ErrNotFound`). `RemovePlaylistPositions` takes indexes into the *visible* list returned by
  `PlaylistTracks` (missing tracks skipped — same as Subsonic `songIndexToRemove`). Playlist
  create/set/append silently drop unknown track ids. `DeleteTracks` also removes bookmarks.
  Extras: `TouchSessionAt`, `LibraryTrackCounts`. `SearchResult`, `LibraryStats`, `FormatStat`
  carry JSON tags matching §8.
- **auth**: `ErrValidation` (wrapped by `ErrInvalidUsername`, `ErrWeakPassword`), `ValidateUsername`
  (1–64 letters/digits/`._-@+`), `ValidatePassword`, `CreateSession(ctx, userID, ua, ip)` (for
  first-run setup), `RevokeAPIKey`, `Password(u)` (plain password for Subsonic token auth),
  `SessionTTL()`, `RequestToken(r)`, `ClientIP(r)`. The middleware accepts an API key as a Bearer
  token too, and refreshes the cookie when the sliding expiry moves.
- **events**: type constants `TypeScan`, `TypeLibrary`, `TypeNowPlaying`; `events.Library(reason)`.
  **nowplaying**: `Tracker.Remove(userID, player)`.
- **api/helpers.go**: `writeJSON`, `writeError`, `writeErr(w, r, err)` (maps via `errorFrom`, logs
  5xx), `writeNoContent`, `decodeJSON` (empty body = OK), `pageParams`, `queryBool/Int/Int64/List`,
  `userFrom`, `writePage[T]`, `nonNil[T]`, error type `*api.Error` with constructors `badRequest`,
  `forbidden`, `notFound`, `newError`. Stubbed features return errors wrapping
  `errors.ErrUnsupported` → 501 `not_implemented`. Do not declare these names in your own files.
- **server**: gzip applies to compressible types (JSON, XML, JS, CSS, HTML, SVG, text) on `/api`,
  `/rest` and the SPA — never audio, images or `text/event-stream`; 206/304/HEAD pass through.
  On shutdown, request contexts are cancelled 3 s after `Shutdown` starts (ends SSE streams).
  `server.Handler(app)` (router only) and `server.ServeListener` exist for tests.

## 13. Wave 2 implementation notes

- **scanner**: `Run`/`Start` hold `LockLibrary` for the whole scan; `RescanFiles`/`RescanDir` never take it
  (they share an internal mutex with scans) — callers doing file mutations hold `LockLibrary` (or
  `TryLockLibrary(ctx)` in HTTP handlers) across the mutation *and* the rescan, passing both old and new
  paths on renames. Deleted files are marked missing (not purged). `ErrLibraryUnavailable` aborts a library
  whose root is missing/unreadable/empty while the DB has tracks. Extra exports: `TryLockLibrary`,
  `ErrLibraryUnavailable`, `ErrClosed`, `Phase*`, `IgnoreFile`, `UnknownAlbum`/`UnknownArtist`/`VariousArtist`.
  Multi-valued ARTIST/ALBUMARTIST/COMPOSER are joined with " / ". Covers are also found one level up from
  `CD1`/`Disc 2` folders. iTunes `TCMP`/`CPIL` count as compilation; Opus R128 gains are a ReplayGain fallback.
- **tags**: extra `ReadProperties`/`Properties`/`Picture`, `Encoding*` constants, errors `ErrUnsupported`,
  `ErrWriteFailed`, `ErrInvalidKey`, `ErrInvalidImage`. `WritePicture` converts WebP/GIF/BMP to JPEG; nil removes
  all pictures. `FixMojibake` also repairs UTF-8-read-as-Latin-1; Big5 with ASCII trail bytes is left alone.
- **artwork**: never upscales; cache keyed by source file identity; folder/artist images outside library roots refused.
- **transcode**: `Decide` returns `("raw",0,false)` when no transcode is needed; a requested format equal to the
  file's format within the cap is served raw — except an explicit format with `offset>0` always transcodes.
  `IsLossless` checks known lossy codecs before bit depth (TagLib reports 16-bit for AAC). Opus is capped at
  256 kbps. Input is passed as an absolute `file:` URL. If ffmpeg is missing, originals are streamed.
- **lyrics**: `[offset:]` is already applied to line starts (the `offset` field is informational). Untimed lines
  following a timed line are kept at that time (translations). `DecodeText` handles BOM/UTF-16/GBK/Big5/SJIS.
- **native API**: setup / create playlist / create radio / admin create user / create library return 201;
  logout is public; password change is rate limited (429); scrobble accepts optional `player`; someone else's
  private playlist is 404; admins can edit all playlists; album zip = `Artist - Album/` folder incl. `.lrc` +
  folder image; cover sizes snap to 64/128/256/512/1024/2048; SSE sends `retry: 5000`, the current now-playing
  list on connect, `X-Accel-Buffering: no`; `/api/search` with blank q returns empty lists.
- **manage/admin API**: batch endpoints return 200 with per-item errors (409 `readonly` only if every item
  failed read-only); lock wait gives up after 90 s → 409; `{disc}` is empty for single-disc albums; empty dir
  levels dropped; rename cannot swap names within one batch; `POST /manage/folders/rescan` → 202
  `{"status":"accepted"}`; `DELETE /admin/libraries/{id}` → 409 while scanning; `removeFolderImage` moves images
  to trash; `POST /manage/encoding` with empty `trackIds` = all detected issues; uploads keep sub-directories when
  not organizing; settings PUT rejects unknown keys, `scanInterval` 0 or ≥1m (normalized); libraries can't overlap
  the data dir; admin password reset of another user revokes their sessions; emptied-dir cleanup also removes
  junk-only content (`.DS_Store`, `Thumbs.db`, `desktop.ini`, `@eaDir`, `._*`). Edit-log `details`: tags/lyrics/
  encoding `{"changes":{KEY:{old,new}}}` (+`target`/`encoding`); rename/restore `{from,to}`; delete
  `{trashId,trashPath}`; cover `{op,embedded,folderImage?}`; upload `{name,size}`; purge `{trashId}`/`{missing:true}`.
- **subsonic**: ape/wma/dsf/dff/wv/mpc/tta are transcoded to the default format unless `format=raw`
  (`transcodedContentType/Suffix` set); similar/top songs computed locally; `getArtist` falls back to appears-on
  albums; `download` accepts album/playlist ids (zip); `getAvatar` → 70; `startScan` and radio writes are
  admin-only; `tokenInfo` implemented; failed logins rate-limited (20/10 min per IP and username).
- **server**: with `RAINY_TRUST_PROXY` only `X-Real-IP` or the last `X-Forwarded-For` hop is trusted.
- **frontend library**: detail pages use their own collapsing header instead of `PageHeader`; `TrackActionsMenu`
  has optional `trigger`; `useToggleStar`/`patchStarred` in `features/library/lib/star.ts` are the shared star helpers.
- **manage review**: replacing a folder cover moves the old `cover.*` images to trash (trash entry with empty
  `trackId`, `delete` log row with `replacedBy`); `POST /manage/upload` has a 32 GiB per-request cap (413);
  library path change/delete take `TryLockLibrary` (15 s) → 409 when busy; the picture endpoint serves only raster
  image types (else `application/octet-stream`) with a sandbox CSP; shared `.lrc` sidecars stay with the remaining file.
- **final review**: `GET /api/queue` also returns `changedBy`. Rename moves folder images only out of a leaf
  folder: never from the library root, nor from a folder whose sub-folders still hold audio (artist.jpg stays).
  A full/quick scan that finds a live track moved onto the path of an old *missing* row gives the file the live
  track's id and purges the stale row (its annotations are lost; it could never come back anyway). A directory
  that is now completely empty but held ≥ 20 live tracks (`offlineDirMinTracks`) is treated like an unreadable
  one (offline docker bind mount / NAS share): its tracks are left alone with a warning until the directory is
  deleted. Playlist cover-art ids are versioned by max(playlist, its albums) `updated_at`
  (`Playlist.AlbumsUpdatedAt`), and mosaic cache keys include each album's version, so mosaics follow cover and
  fallback changes. The play queue stores the current entry's index (`current_index`), used by Subsonic
  `getPlayQueueByIndex`/`savePlayQueueByIndex` and the web player's `PUT /api/queue`.

### Known limitations
- Manage rename onto a path still owned by a *missing* track is refused as a conflict ("the path belongs to
  another (missing) track"); purge the missing track first (Doctor → missing files).
- An intentionally emptied folder that held ≥ 20 tracks keeps its tracks (not marked missing) until the folder
  itself is removed; the scanner logs a warning for it.
- `GET /api/queue` does not return the current index: a web player restoring a queue in which the current song
  appears twice resumes at its first occurrence.
- Cover images are not access-checked beyond login: any user who knows a private playlist's cover-art id can
  fetch its mosaic (ids are random 22-char strings and the mosaic only shows album covers). The service worker's
  `rainy-covers` cache survives logout on a shared device.
- `/api/me/password` answers 403 (not 401) for a wrong current password so the client stays logged in.
- **tags ffprobe fallback and rebuild**: TagLib's WASM build aborts on some real files (e.g. WAV with both an `id3 ` chunk and a
  legacy-encoded LIST/INFO chunk). `tags.SetFFmpeg(cfg.FFmpegPath)` (called in `app.New`) enables a fallback:
  `ReadRaw`/`ReadProperties`/`ReadPicture` use ffprobe/ffmpeg when TagLib fails; duplicate keys prefer values that
  decoded cleanly (no U+FFFD). `POST /api/manage/tags/rebuild` first reads the current tags, then writes a verified
  same-directory copy through TagLib. For WAV, it makes LIST/INFO chunks inert as JUNK while retaining their bytes,
  audio and ID3 chunks; unreadable or absent fields fall back to the indexed title/artist/album fields. The original is replaced
  only after the copy is readable. Other formats still require TagLib support and return per-track errors if it fails.
