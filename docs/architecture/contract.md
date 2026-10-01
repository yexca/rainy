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
| Link downloads | external `yt-dlp` binary (opt-in, §5.14) | installed at runtime into `<data>/ytdlp`; QuickJS (`qjs`) shipped in the image for YouTube |
| Online music | `github.com/dop251/goja` (pure-Go ECMAScript interpreter) | runs lx-music custom source scripts in a sandbox (opt-in, §5.15) |
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
│   ├── metasearch/              opt-in online metadata lookup (NetEase, QQ Music, Kugou, Kuwo, iTunes)
│   ├── ytdlp/                   opt-in downloads from YouTube / bilibili: yt-dlp binary, cookies, runs
│   ├── lxmusic/                 opt-in online music: catalogue search, lx-music source scripts (goja sandbox), downloads of their links
│   ├── listening/               listening reports from the play history (§5.17)
│   ├── scrobble/                opt-in scrobbling to Last.fm / ListenBrainz: queue, sender, account linking (§5.16)
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
| `RAINY_YTDLP_PATH` | `` | a yt-dlp binary the operator manages; empty = Rainy installs and updates yt-dlp in `<data>/ytdlp` (§5.14) |
| `RAINY_LOG_LEVEL` | `info` | debug/info/warn/error |
| `RAINY_LOG_FORMAT` | `text` (docker: `json`) | |
| `RAINY_SESSION_TTL` | `720h` | web session lifetime (sliding) |
| `RAINY_TRUST_PROXY` | `false` | honour X-Forwarded-For / X-Real-IP |
| `RAINY_DEV_CORS` | `` | extra allowed origin for /api during development (e.g. `http://localhost:5173`) |

`config.Load() (*config.Config, error)` reads them; `Config` fields:
`Address, Port string/int; DataDir, MusicDir string; ScanInterval time.Duration; ScanOnStart bool;
FFmpegPath, YtdlpPath string; LogLevel, LogFormat string; SessionTTL time.Duration; TrustProxy bool; DevCORS string`
plus helpers `DBPath()`, `CacheDir()`, `ArtworkCacheDir()`, `TrashDir()`, `TmpDir()`, `YtdlpDir()`, `SecretKeyPath()`.

Data dir layout:
```
/data/rainy.db (+ -wal, -shm)
/data/secret.key        32 random bytes, created on first start (AES key + misc HMAC)
/data/cache/artwork/    resized cover cache
/data/trash/<libraryId>/<original relative path>   deleted files (restorable)
/data/tmp/              upload staging; ytdlp-job-<id>/ and online-job-<id>/ work directories of running downloads
/data/ytdlp/            yt-dlp binary installed by Rainy, cache/, cookies/<site>.enc (0700/0600, encrypted with secret.key)
```

Runtime-editable settings live in the `settings` table (see `model.Settings`, §5.2).

## 5. Backend contracts

### 5.1 Database
Schema: `internal/db/migrations/0001_init.sql` plus later migrations in the same folder (`0002_play_queue_index.sql`
adds `play_queues.current_index`, `0003_lx_sources.sql` adds the `lx_sources` table of §5.15, `0004_listening.sql` adds the
play-history snapshot columns and the `scrobble_accounts` / `scrobble_queue` tables of §5.16) — read them, they are part
of this contract.
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
    OnlineMetadata    bool   `json:"onlineMetadata"`    // default false: allow managers to search online catalogues (§5.13)
    OnlineMetadataChinaIP bool `json:"onlineMetadataChinaIp"` // default false: metasearch.Options.ChinaIP (§5.13)
    YtdlpEnabled      bool   `json:"ytdlpEnabled"`      // default false: allow downloads from YouTube / bilibili with yt-dlp (§5.14)
    LxSourcesEnabled  bool   `json:"lxSourcesEnabled"`  // default false: allow online music and lx-music source scripts (§5.15)
    LxSourceMode      string `json:"lxSourceMode"`      // default "auto" (enabled sources by priority) | "fixed" (only LxSourceID)
    LxSourceID        string `json:"lxSourceId"`        // default "": the source used in fixed mode
    LastfmEnabled     bool   `json:"lastfmEnabled"`     // default false: allow users to scrobble to Last.fm (§5.16; also needs the API account)
    ListenBrainzEnabled bool `json:"listenBrainzEnabled"` // default false: allow users to scrobble to ListenBrainz (§5.16)
}
func DefaultSettings(scanInterval time.Duration) Settings
const LxSourceModeAuto, LxSourceModeFixed = "auto", "fixed"

// lx_sources row; db tags only — the API shape is lxmusic.SourceInfo and the script is never returned.
type LxSource struct {
    ID, Name, Description, Version, Author, Homepage, SourceURL, Script, ScriptHash string
    ScriptSize int /*computed length(script)*/; Enabled bool; Position int; AllowUpdateAlert bool
    Platforms string /*JSON {"kw":["128k",…]}*/; LastError string; LoadedAt int64
    UpdateLog, UpdateURL string; UpdateAt, CreatedAt, UpdatedAt int64
}

// play_history row for the listening history (§5.17). Names, ids and duration come from the live track when it
// still exists, else from the snapshot taken at play time; Track is the live track (annotations filled), nil when
// purged or missing (a named field, never embedded: see Track.MarshalJSON).
type Play struct { ID int64; TrackID string; PlayedAt int64; Client, Title, Artist, Album, AlbumArtist, ArtistID, AlbumID string
    Duration float64; Track *Track }                     // json camelCase, track
const ScrobbleLastfm, ScrobbleListenBrainz = "lastfm", "listenbrainz"
// scrobble_accounts / scrobble_queue rows; db tags only (API shape: scrobble.AccountStatus).
type ScrobbleAccount struct { UserID, Service, Username, CredentialEnc /*encrypted; "" = revoked*/ string; Enabled bool
    LastError string; LastErrorAt, LastSentAt, CreatedAt, UpdatedAt int64 }
type QueuedScrobble struct { ID int64; UserID, Service, TrackID, Title, Artist, Album, AlbumArtist string; TrackNumber int
    Duration float64; MbzTrackID string; PlayedAt int64; Attempts int; NextAttemptAt int64; LastError string; CreatedAt int64 }
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
RecordPlay(ctx, userID, trackID string, at int64, client string) error // +1 play_count & played_at on track, its album, its artist;
                                     // insert play_history with the track's title/artist/album/album artist/ids/duration snapshot
RecentlyPlayedTracks(ctx, userID string, limit int) ([]model.Track, error)

// Listening (§5.17): one user's plays with from <= played_at < to (to <= 0 = no bound); live track values, else the snapshot
type ListeningTotals struct { Plays int; Duration float64; Tracks, Artists, Albums int /*distinct*/ } // json camelCase
ListeningTotals(ctx, userID string, from, to int64) (ListeningTotals, error)
type PlayTime struct { At int64; Duration float64 }
ListeningTimes(ctx, userID string, from, to int64) ([]PlayTime, error)  // oldest first
type ListeningTopItem struct { ID, Name, Artist string; Plays int; Duration float64 }
ListeningTop(ctx, userID, kind string /*TopTracks|TopAlbums|TopArtists|TopGenres|TopClients*/, from, to int64, limit int) ([]ListeningTopItem, error)
                                     // most plays first, ties most recent first; plays without an id are skipped; unknown kind → ErrInvalid
ListeningFirsts(ctx, userID string, from, to int64) (tracks, artists int, err error) // first-ever plays inside the period
FirstPlayAt(ctx, userID) (int64, error)
ListPlays(ctx, userID string, from, to int64, offset, limit int) ([]model.Play, int, error) // newest first

// Scrobbling (§5.16)
ListScrobbleAccounts(ctx, userID) ([]model.ScrobbleAccount, error); GetScrobbleAccount(ctx, userID, service) (*model.ScrobbleAccount, error)
SaveScrobbleAccount(ctx, *model.ScrobbleAccount) error          // link / re-link: enabled, error cleared, queue kept
SetScrobbleAccountEnabled(ctx, userID, service string, enabled bool) error // pausing drops the account's queue; ErrNotFound
SetScrobbleAccountError(ctx, userID, service, msg string, revoked bool) error; MarkScrobbleAccountSent(ctx, userID, service string, at int64) error
DeleteScrobbleAccount(ctx, userID, service) error               // with its queue
CountScrobbleAccounts(ctx, service) (int, error)                 // linked (credential present)
EnqueueScrobble(ctx, model.QueuedScrobble, services []string) ([]string, error) // for the user's enabled, linked accounts among services
ScrobbleDueUsers(ctx, service string, now int64) ([]string, error); DueScrobbles(ctx, userID, service string, now int64, limit int) ([]model.QueuedScrobble, error)
DeleteScrobbles(ctx, ids []int64) error; DeferScrobbles(ctx, ids []int64, next int64, msg string) error
PurgeScrobblesBefore(ctx, service string, before int64) (int64, error); CountQueuedScrobbles(ctx, userID, service) (int, error)
GetValue(ctx, key) (string, error); SetValue(ctx, key, value string) error // settings rows outside model.Settings ("" deletes):
                                     // ValueLastfmAppKey "lastfm.apiKey", ValueLastfmSigningEnc "lastfm.secretEnc" (encrypted)

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
ListLxSources(ctx) ([]model.LxSource, error) /*by position, without scripts*/; GetLxSource(ctx, id) (*model.LxSource, error) /*with script*/
CreateLxSource(ctx, *model.LxSource) error /*appended to the priority order; duplicate script → ErrConflict*/
ReplaceLxSourceScript(ctx, *model.LxSource) error; SetLxSourceFlags(ctx, id string, enabled, allowUpdateAlert *bool) error
SetLxSourceLoaded(ctx, id, platformsJSON, lastError string, at int64) error; SetLxSourceUpdateAlert(ctx, id, log, url string, at int64) error
DeleteLxSource(ctx, id) error; ReorderLxSources(ctx, ids []string) error /*every id exactly once, else ErrInvalid*/
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
`(*Service).SetDownloader(d manage.Downloader)` enables link downloads (app passes `*ytdlp.Service`;
`Downloader` = `Ready(ctx) error` + `Download(ctx, ytdlp.Request, progress) ([]ytdlp.Item, error)`), and
`CloseDownloads()` cancels running jobs (called by `App.Close`). Download jobs: `StartDownload`, `DownloadJobs`,
`RemoveDownload` behind §7.6 `/manage/downloads`. `(*Service).SetOnlineSource(o manage.OnlineSource)` (app passes
`*lxmusic.Service`) enables `StartOnlineDownload` (§7.6 `/manage/online/downloads`); online jobs share the job list
(`DownloadJob.Kind` `link` | `online`) but have their own three workers.
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
    Transcoder *transcode.Service; Manage *manage.Service; Metadata *metasearch.Service; Ytdlp *ytdlp.Service
    Online *lxmusic.Service; Listening *listening.Service; Scrobble *scrobble.Service
    StartedAt time.Time
}
func New(ctx context.Context, cfg *config.Config) (*App, error) // open+migrate DB, key, services, default library
func (a *App) Settings(ctx context.Context) model.Settings       // store.GetSettings with defaults (errors logged → defaults)
func (a *App) Close() error // stops downloads, the scrobble sender, source scripts and a running yt-dlp install, then closes the DB

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

### 5.13 metasearch (owner: manage agent)
The only package that sends requests to third-party services. It is used only by the §7.6
`/manage/metadata/*` endpoints, which refuse with `403 forbidden` unless `settings.onlineMetadata`
is on, and it never writes files: results fill the tag editor's draft and reach the files through
the normal save endpoints.
```go
func New(httpClient *http.Client) *metasearch.Service // nil → 15 s timeout, HTTPS_PROXY/HTTP_PROXY honoured
func (s *Service) Providers() []ProviderInfo                // netease, qq, kugou, kuwo, itunes (display order)
func (s *Service) Search(ctx, provider, query string, limit int, region string, opts Options) ([]Result, error)
func (s *Service) Lyrics(ctx, provider, id string, opts Options) (*Lyrics, error) // ErrNotFound when none
type Options struct { ChinaIP bool } // from settings.onlineMetadataChinaIp
func (s *Service) Cover(ctx, url string) ([]byte, string, error)      // provider image hosts only, fetched over HTTPS
func CoverAllowed(url string) bool
type ProviderInfo struct { ID string; Lyrics bool; Regions []string }  // json: id lyrics regions
type Result struct { Provider, ID, Title string; Artists []string; Album, AlbumArtist string
    TrackNumber, TrackTotal, DiscNumber, DiscTotal int; Date, Genre string; Duration float64
    CoverURL, ThumbURL string }                                        // json camelCase (coverUrl, thumbUrl)
type Lyrics struct { Text, Translation string }                        // json: text translation
var ErrUnknownProvider, ErrInvalid, ErrNotFound, ErrUpstream error
```
Rules: fixed endpoint URLs; the query is one encoded parameter (or a JSON value), ≤ 200 characters;
provider ids are validated per provider before any request; responses are capped (4 MiB JSON,
20 MiB images); covers come only from `music.126.net`, `y.gtimg.cn`, `y.qq.com`, `kugou.com`,
`kuwo.cn`, `mzstatic.com`, and `migu.cn` (for §5.15; host or subdomain, default port, no userinfo), redirects stay on
the same host, and only JPEG/PNG/GIF/WebP bytes are returned. Errors never include the query.
QQ Music falls back to its older mobile search (no track numbers) when the desktop search refuses.
With `Options.ChinaIP`, requests to the NetEase, QQ Music, Kugou, and Kuwo APIs carry an
`X-Real-IP` header with a random address from a few mainland-China ISP /16 blocks (a new one per
lookup); iTunes and cover downloads never do.
Kugou's search endpoint is plain HTTP only; everything else uses HTTPS.

### 5.14 ytdlp (owner: manage agent)
Downloads audio from YouTube and bilibili with the external `yt-dlp` program and manages its binary and
sign-in cookies. Used only by the §7.6 `/manage/downloads` and §7.7 `/admin/ytdlp` endpoints, which refuse
every action that contacts another service (start a download, check for an update, install) with
`403 forbidden` unless `settings.ytdlpEnabled` is on. It never writes into a library: manage imports the
files like uploads.
```go
func New(o Options) *ytdlp.Service // removes stale <tmp>/ytdlp-* work dirs and partial installs
type Options struct { Dir, TmpDir, BinaryPath, FFmpegPath string; Cipher Cipher /* *auth.Crypto */; Client *http.Client; UserAgent string }
func (s *Service) Close()
func (s *Service) Managed() bool                       // BinaryPath == "" (RAINY_YTDLP_PATH unset)
func (s *Service) Status(ctx) Status                    // runs "yt-dlp --version" when the binary changed (cached)
func (s *Service) Ready(ctx) error                      // ErrNotInstalled | "yt-dlp cannot run: …" | ErrNoFFmpeg
func (s *Service) CheckLatest(ctx) (string, error)      // GitHub releases/latest tag, remembered for Status
func (s *Service) StartInstall() error                  // async; ErrUnmanaged, ErrBusy, ErrUnsupportedPlatform
func (s *Service) SetCookies(site, text string) (CookieSaveResult, error)
func (s *Service) DeleteCookies(site string) error
func (s *Service) Cookies() []CookieInfo; func (s *Service) HasCookies(site string) bool
func (s *Service) Download(ctx, req Request, progress func(Progress)) ([]Item, error) // partial results + error for playlists
func ParseURL(text string) (Target, error)              // ErrInvalid; Target{Site, URL}
var Sites = []string{"youtube", "bilibili"}; func ValidSite(id string) bool; func ValidFormat(f string) bool
type Request struct { Target Target; Format string /*best|m4a|mp3|opus*/; Playlist bool; Dir string /*caller-owned work dir*/ }
type Item struct { Path, Thumbnail, ID, Title string; Artists []string; Album string; AlbumArtists []string
    TrackNumber int; Date, WebpageURL, PlaylistTitle string; PlaylistIndex int }
func (it Item) Tags() map[string][]string               // TITLE ARTIST ALBUM ALBUMARTIST DATE TRACKNUMBER COMMENT(=source URL)
type Progress struct { Phase string /*downloading|processing*/; Item, Items int; Title string; Fraction, Speed float64; ETA int }
type Status struct { Managed, Installed bool; Version, Error, Latest string; CheckedAt int64; Asset, JSRuntime string
    FFmpeg bool; Install InstallState }                 // json camelCase; jsRuntime deno|node|quickjs|""
type InstallState struct { Running bool; Error string; FinishedAt int64 }
type CookieInfo struct { Site string; Configured bool; Count int; SignedIn bool; ExpiresAt, UpdatedAt int64 }
type CookieSaveResult struct { CookieInfo; Dropped int }
var ErrInvalid, ErrNotInstalled, ErrUnmanaged, ErrBusy, ErrUpstream, ErrUnsupportedPlatform, ErrNoFFmpeg error
```
Rules:
- **Links**: only the exact hosts `youtube.com`, `www.`/`m.`/`music.youtube.com`, `youtu.be`, `bilibili.com`,
  `www.`/`m.`/`space.bilibili.com` and `b23.tv`; http(s) only, no user info, no port, ≤ 2048 bytes; the first URL
  in pasted share text is used. `m.` hosts are rewritten to `www.`. A `b23.tv` share link is resolved by one
  redirect that must lead to bilibili (yt-dlp has no extractor for it).
- **Invocation** (fixed argument list, no shell): `--ignore-config --no-plugin-dirs --use-extractors default,-generic`,
  `--match-filters !is_live`, `--max-filesize 2G`, `-f bestaudio/best -x` (`--audio-format <f> --audio-quality 0`
  unless `best`), `--write-thumbnail --convert-thumbnails jpg`, `--no-playlist` (or `--yes-playlist
  --playlist-items 1:100`), `--paths <dir>/out -o %(id)s.%(ext)s`, `--cache-dir <data>/ytdlp/cache`,
  `--ffmpeg-location <abs ffmpeg>`, `--js-runtimes <deno|node|quickjs>:<path>` for the first runtime found on
  PATH, `--cookies <dir>/cookies.txt` only when cookies are stored for the site, and the link last after `--`.
  Progress and results come from `--progress-template` / `--print` marker lines; reported files must be regular
  audio files inside `<dir>/out`. The process runs in its own process group (killed as a group on cancel),
  with `TMPDIR` inside the work dir. Fragmented (DASH) MP4 results, which bilibili serves and TagLib reads no
  duration from, are remuxed with `ffmpeg -i file:<in> -map 0:a:0 -c copy -movflags +faststart file:<out>`.
- **Binary**: managed installs download the platform asset (`yt-dlp_musllinux[_aarch64]` on Alpine, `yt-dlp_linux
  [_aarch64]`, `yt-dlp_macos`, `yt-dlp[_arm64].exe`) from `https://github.com/yt-dlp/yt-dlp/releases/download/<tag>/`,
  where `<tag>` comes from the GitHub releases API and must match `YYYY.MM.DD[.N]` (URLs in the API answer are
  never followed); the file must match the release's `SHA2-256SUMS` and report `<tag>` from `--version` before it
  atomically replaces `<data>/ytdlp/yt-dlp`. Redirects stay on `github.com` / `*.githubusercontent.com` (HTTPS);
  sizes are capped (2 MiB JSON, 256 MiB binary). With `RAINY_YTDLP_PATH`, Rainy never installs or updates.
- **Cookies** are credentials: a Netscape `cookies.txt` (≤ 512 KiB, ≤ 2000 cookies) is filtered to the site's own
  domain (`youtube.com` / `bilibili.com` and subdomains), encrypted with `secret.key` (AES-GCM, like passwords) and
  written 0600 to `<data>/ytdlp/cookies/<site>.enc`. No endpoint returns them, logs and errors never quote them, and a
  run gets a private plain-text copy in its work dir that is deleted with it. Undecryptable files are ignored.

### 5.15 lxmusic (owner: manage agent)
Online music in the style of lx-music: song search in the five catalogues lx-music knows, and lx-music *custom source*
scripts that turn a search result into a download link. Used only by the §7.6 `/manage/online*` and §7.7 `/admin/sources*`
endpoints, which refuse every action that contacts another service (search, cover proxy, download, starting a script,
importing from a link) with `403 forbidden` unless `settings.lxSourcesEnabled` is on. It never writes into a library:
manage imports downloaded files like uploads.
```go
func New(o Options) *lxmusic.Service // starts the idle-unload loop; Close stops every script
type Options struct { Store *store.Store; Metadata *metasearch.Service /*lyrics, covers*/; AllowPrivate bool /*tests*/ }
var Platforms = []string{"kw", "kg", "tx", "wy", "mg"}      // Kuwo, Kugou, QQ Music, NetEase Cloud Music, Migu
var Qualities = []string{"128k", "320k", "flac", "flac24bit"} // lowest first
func (s *Service) Search(ctx, platform, query string, page, limit int) (*SearchResult, error) // limit ≤ 50 (default 30), page ≤ 100
func (s *Service) List(ctx) ([]SourceInfo, error); Get(ctx, id) (*SourceInfo, error)
func (s *Service) Import(ctx, script, sourceURL string, start bool) (*SourceInfo, error) // ErrInvalid, store.ErrConflict (duplicate)
func (s *Service) FetchScript(ctx, url string) (string, error); Refresh(ctx, id string, start bool) (*SourceInfo, error)
func (s *Service) Update(ctx, id string, enabled, allowUpdateAlert *bool) (*SourceInfo, error); Delete(ctx, id) error
func (s *Service) Reorder(ctx, ids []string) error; Reload(ctx, id) (*SourceInfo, error) // a failed start is Status "error", not an error
type Selection struct { Mode, SourceID string }         // from settings.lxSourceMode / lxSourceId
func (s *Service) Candidates(ctx, platform string, sel Selection) ([]Candidate, error) // Candidate{ID, Name}, in order
func (s *Service) Usable(ctx, sel Selection) (int, error); Available(ctx, sel Selection) (map[string][]string, error)
func (s *Service) MusicURL(ctx, sourceID string, song Song, want string) (link, quality string, err error)
func (s *Service) Download(ctx, link string, w io.Writer, limit int64, progress func(done, total int64)) (int64, error)
func (s *Service) Lyrics(ctx, song Song) (text, translation string, err error); Cover(ctx, song Song) ([]byte, error)
type Song struct { Platform, ID /*lx songmid*/, Title string; Artists []string; Album, AlbumID string; Duration float64
    CoverURL string; Qualities []Quality; Extra map[string]string; PageURL string }   // json camelCase
type Quality struct { Type, Size, Hash string }          // json: type size hash(omitempty; Kugou file hash)
func (s Song) Validate() error; (s *Song) Normalize() error; (s Song) MusicInfo() map[string]any
type SearchResult struct { Items []Song; Total, Page, Limit int }
type SourceInfo struct { ID, Name, Description, Version, Author, Homepage, SourceURL string; Size int; Enabled bool
    Position int; AllowUpdateAlert bool; Platforms []PlatformQualities; Status /*idle|loading|ready|error*/, Error string
    UpdateAlert *UpdateAlert; LoadedAt, CreatedAt, UpdatedAt int64 }                 // json camelCase
type PlatformQualities struct { Platform string; Qualities []string }; type UpdateAlert struct { Log, URL string; At int64 }
var ErrInvalid, ErrNotFound, ErrUpstream, ErrUnavailable, ErrScript, ErrBlocked, ErrClosed error
```
Rules:
- **Search** follows lx-music's own search code: Kuwo `search.kuwo.cn/r.s`, Kugou `songsearch.kugou.com/song_search_v2`
  (falling back to the plain-HTTP `mobilecdn.kugou.com/api/v3/search/song`, which lacks Hi-Res, when it fails or finds
  nothing — it answers empty for a while after many searches from one address),
  QQ Music `u.y.qq.com/cgi-bin/musics.fcg` signed with the desktop client's `zzc` signature, NetEase Cloud Music
  `music.163.com/api/cloudsearch/pc`, Migu `jadeite.migu.cn/music_search/v3/search/searchAll` signed like the Android
  client. Fixed URLs, the query as one encoded parameter (≤ 200 characters), 4 MiB responses. Results carry the ids
  source scripts need in `Extra` (Kugou `hash`, `albumAudioId`; QQ `strMediaMid`, `albumMid`, `songId`; Migu
  `copyrightId`, `lrcUrl`, `mrcUrl`, `trcUrl`) and the qualities the catalogue lists. Songs sent back by clients are
  validated (`Normalize`): known platform, per-platform id and `Extra` formats, bounded text, covers on the metasearch
  image hosts; `PageURL` is recomputed. `MusicInfo` builds lx-music's old-format musicInfo (`toOldMusicInfo`).
- **Scripts** are stored in `lx_sources` (≤ 9,000,000 bytes, UTF-8, must open with the lx-music `/** @name … */` header;
  name 24, description 36, author 56, version 36, homepage 1024 characters, homepages only `http(s)`; sha256 dedupe).
  A script starts on first use (or on reload) in its own goja runtime driven by one event-loop goroutine; it gets the
  lx-music desktop API `window.lx` (`EVENT_NAMES`, `on('request')`, `send('inited' | 'updateAlert')`, `request`,
  `utils.crypto.{aesEncrypt, rsaEncrypt, randomBytes, md5}`, `utils.buffer.{from, bufToString}`, `utils.zlib`,
  `currentScriptInfo`, `version` `2.0.0`, `env` `desktop`) plus `setTimeout`/`setInterval`, `console` (debug log,
  rate limited), `atob`/`btoa`, `TextEncoder`/`TextDecoder`, `URLSearchParams`, `navigator`; nothing else (no `require`,
  file system, processes or `fetch`). Buffers are plain `Uint8Array`s, as they reach scripts through Electron's bridge.
  The start succeeds when `inited` lists at least one of the five platforms with valid qualities (20 s); an uncaught
  error or unhandled rejection before that fails it. The platforms and qualities, or the error, are saved in the row;
  a failed start is retried after 2 minutes (or on reload); idle scripts stop after 15 minutes.
- **Limits**: every run of script code stops after 10 s (the runtime is then discarded), a request event must answer
  within 20 s with an `http(s)` link ≤ 2048 bytes, ≤ 1000 timers, ≤ 8 concurrent `lx.request`s, request bodies ≤ 4 MiB,
  responses ≤ 10 MiB, request timeout ≤ 60 s. `lx.request` follows needle's behaviour: no redirects, bodies from
  `body`/`form`/`formData` (objects as query strings, JSON only with a JSON content type), JSON responses parsed. Like
  lx-music's V8 preload, a missing options object throws `TypeError: Cannot read properties of undefined (reading
  'method')` at once and a missing callback only fails when the response arrives (scripts probe both).
  Memory is not bounded.
- **Network guard** for scripts, script links and audio downloads: `http(s)` only, no user info, and every dialled
  address (after DNS) must be public — loopback, private, link-local, CGNAT, multicast, documentation and reserved
  ranges (also IPv4-mapped and NAT64) are refused. A configured `HTTP(S)_PROXY` is dialled directly, with the target
  resolved and checked first. Audio downloads follow ≤ 5 redirects, each checked again.
- **Downloads** (manage): `Candidates` lists enabled sources by priority (fixed mode: only the chosen one), skipping
  sources known not to provide the platform. `MusicURL` picks the best quality up to the requested one that both the
  song and the source list (else the lowest above it). `Download` reports an HTTP error status as `*LinkError`;
  `LinkExpired(err)` is true for 401, 403 and 410 and for hosts that do not resolve (lx-music's "refresh the link"
  cases). Lyrics come from metasearch (Kuwo, Kugou by hash, QQ Music,
  NetEase) or Migu's `lrcUrl`/`trcUrl`; covers from the metasearch image hosts.

### 5.16 scrobble (owner: native API agent)
Sends plays, "now playing" and loved tracks to the users' linked Last.fm and ListenBrainz accounts. Nothing is sent for a
service while its admin setting (`lastfmEnabled`, `listenBrainzEnabled`, both off by default) is off; Last.fm also needs
the administrator's API key and shared secret. Every completed play goes through `Played` (native `POST /api/scrobble`
and Subsonic `scrobble` with `submission=true`), "now playing" through `NowPlaying`, and starring / unstarring tracks
(native `POST /api/star` type `track`, Subsonic `star`/`unstar` song ids) through `Loved`.
```go
func New(o Options) *scrobble.Service // starts the sender (unless o.NoWorker)
type Options struct { Store *store.Store; Cipher Cipher /* *auth.Crypto */; Settings func(ctx) model.Settings; Client *http.Client
    UserAgent, LastfmAPI, LastfmAuthURL, ListenBrainzAPI string /*tests*/; NoWorker bool }
func (s *Service) Close(); Wake(); Flush(ctx)                // Flush: send every due queued play (the sender calls it)
func (s *Service) Played(ctx, userID, trackID string, at int64, client string) error // store.RecordPlay, then queue for enabled accounts
func (s *Service) NowPlaying(userID, trackID string)          // background, best effort
func (s *Service) Loved(userID string, trackIDs []string, loved bool) // Last.fm track.love / unlove; background, ≤ 50 tracks
func (s *Service) Status(ctx, userID) ([]AccountStatus, error) // every service, in Services order
func (s *Service) LastfmAuthURL(ctx, userID, callback string) (string, error)
func (s *Service) LinkLastfm(ctx, userID, token, state string) (*AccountStatus, error)
func (s *Service) LinkListenBrainz(ctx, userID, token string) (*AccountStatus, error)
func (s *Service) SetEnabled(ctx, userID, service string, enabled bool) (*AccountStatus, error); Unlink(ctx, userID, service) error
func (s *Service) AdminInfo(ctx) (*AdminInfo, error); SetLastfmCredentials(ctx, apiKey, secret *string) error // nil = keep, "" = remove
var Services = []string{"lastfm", "listenbrainz"}; func ValidService(id string) bool; const MaxAge = 14 * 24h; MinDuration = 30
type AccountStatus struct { Service string; Available, Linked, NeedsRelink bool; Username string; Enabled bool; Queued int
    LastError string; LastErrorAt, LastSentAt, LinkedAt int64 }                            // json camelCase
type AdminInfo struct { Lastfm LastfmAdmin; ListenBrainz ListenBrainzAdmin }             // json: lastfm listenBrainz
type LastfmAdmin struct { Enabled bool; APIKey string; HasSecret, Configured bool; Users int } // json: enabled apiKey hasSecret configured users
type ListenBrainzAdmin struct { Enabled bool; Users int }
type Error struct { Service string; Code int; Message string }; func (e *Error) Retryable() bool // a service's refusal or outage
var ErrInvalid, ErrDisabled, ErrNotConfigured, ErrNotLinked, ErrExpired error
```
Rules:
- **Queue**: `Played` queues a snapshot (title, artist, album, album artist, track number, duration, MusicBrainz id, time)
  for each enabled, linked account of an active service, skipping tracks without a real title and artist (`[Unknown
  Artist]`) and, for Last.fm, tracks of 30 s or less; `[Unknown Album]` is sent without an album. The sender runs every
  minute and when woken, drops plays older than `MaxAge` for every service (Last.fm refuses them), and sends each user's due plays oldest
  first in batches (Last.fm `track.scrobble` 50, ListenBrainz `submit-listens` 100: `single` for one listen, `import`
  otherwise). Outcomes: accepted → removed (`last_sent_at`); an outage, rate limit, redirect or network error → the batch waits
  1, 2, 4 … minutes (≤ 6 h) and the account shows the error; a revoked credential (Last.fm error 9, ListenBrainz 401) →
  the credential is cleared (`needsRelink`) and the plays wait until the user links again; a wrong API key or signature
  (Last.fm 10, 13, 26) → kept and retried; any other refusal → the batch is dropped. Plays Last.fm ignores are dropped.
  Pausing an account drops its queue; unlinking deletes the account and its queue.
- **Last.fm** (`https://ws.audioscrobbler.com/2.0/`, signed POSTs: `api_sig` = MD5 of the sorted `name`+`value` pairs
  without `format`/`callback`, followed by the shared secret). Web authentication: `LastfmAuthURL` accepts an absolute
  `http(s)` callback (≤ 1024 bytes, no user info or fragment), adds a random `state` bound to the user for 15 minutes
  (in memory), and returns Last.fm's web authentication page (`https://www.last.fm/api/auth/` with the API key and `cb`); the browser opens it, Last.fm sends it back
  with `token`, and `LinkLastfm` requires the same user's `state` before exchanging the token (`auth.getSession`). The
  API key and shared secret must be 32 hex characters; the secret is encrypted with `secret.key` and never returned.
- **ListenBrainz** (`https://api.listenbrainz.org`, `Authorization: Token <user token>`): `LinkListenBrainz` checks the
  token with `/1/validate-token` first. Listens carry `submission_client` "Rainy" and its version.
- **Credentials** (session keys, user tokens, the shared secret) are encrypted with `secret.key`, never returned, logged or
  put in an error message. Requests use fixed URLs, a 15 s timeout, no redirects, 1 MiB responses, `HTTPS_PROXY` /
  `HTTP_PROXY`; the services' messages are stored clipped (200 characters) and shown as plain text.

### 5.17 listening (owner: native API agent)
Builds a user's listening report from `play_history`; a user only ever sees their own.
```go
func New(st *store.Store) *listening.Service
type Query struct { From, To int64 /*unix ms; From 0 = since the first play, To 0 = now*/; Loc *time.Location; Limit int /*≤ 50, default 10*/ }
func (s *Service) Report(ctx, userID string, q Query) (*Report, error) // ErrInvalid when To <= From
func LoadLocation(name string) *time.Location                        // IANA name; "" / unknown → UTC
func BucketFor(from, to int64) string                                // hour ≤ 3 days, day ≤ 93 days, week ≤ 731 days, else month
func Timeline(times []store.PlayTime, from, to int64, bucket string, loc *time.Location) []Bucket
func Clock(times []store.PlayTime, loc *time.Location) [7][24]int    // weekday 0 = Monday
func ActiveDays(times []store.PlayTime, loc *time.Location) (days, longest int)
type Report struct { From, To int64; TZ, Bucket string; FirstPlayAt int64; Totals store.ListeningTotals; Previous *store.ListeningTotals
    NewTracks, NewArtists, ActiveDays int; LongestRun int /*json longestStreak*/; Timeline []Bucket; Clock [7][24]int
    TopArtists, TopAlbums []TopEntry; TopTracks []TopTrack; TopGenres, Clients []TopEntry }  // json camelCase
type Bucket struct { Start int64; Plays int; Duration float64 }
type TopEntry struct { ID, Name, Artist string; Plays int; Duration float64; CoverArt string; Available bool }
type TopTrack struct { TopEntry; Track *model.Track }
```
Rules: `To` is capped at now + 1 ms; for all time `From` becomes the first play. `Previous` covers the period of the same
length right before (`nil` for all time). Buckets, the clock and active days use the viewer's time zone (`tz`); weeks start
on Monday. Durations are the sum of the played tracks' lengths. Top artists / albums / tracks carry the live library items
(`Available`, cover art, the live track) and fall back to the snapshot names when an item no longer exists.

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
- `scrobble`: `submission=false` → now playing (+ `scrobble.NowPlaying`); `true` → `scrobble.Played` (`store.RecordPlay`
  plus the Last.fm / ListenBrainz queue, §5.16; `time` param in ms). `star`/`unstar` of songs → `scrobble.Loved`.
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
| POST | `/api/scrobble` | `{trackId, submission:boolean, time?:number /*ms*/}` | 204 (submission → `scrobble.Played`, else now playing + `scrobble.NowPlaying`; starring tracks → `scrobble.Loved`, §5.16) |

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
| GET | `/api/manage/metadata` | | `MetadataStatus` |
| GET | `/api/manage/metadata/search` | `provider, q, limit? (≤30, default 20), region?` | `{items: MetadataResult[]}` |
| GET | `/api/manage/metadata/lyrics` | `provider, id` | `MetadataLyrics` (404 when the provider has none) |
| GET | `/api/manage/metadata/cover` | `url` (a result's `coverUrl`/`thumbUrl`) | image bytes (raster type, `nosniff`, sandbox CSP) |
| GET | `/api/manage/downloads` | | `DownloadsStatus` |
| POST | `/api/manage/downloads` | `{url, libraryId, dir?, organize?, format?: 'best'|'m4a'|'mp3'|'opus', playlist?}` | 202 `DownloadJob` |
| DELETE | `/api/manage/downloads/{id}` | | 204 (cancels a queued/running job; removes a finished one) |
| GET | `/api/manage/online` | | `OnlineStatus` |
| GET | `/api/manage/online/search` | `platform, q, page? (≥1), limit? (≤50, default 30)` | `OnlineSearchResult` |
| GET | `/api/manage/online/cover` | `url` (a result's `coverUrl`) | image bytes (raster type, `nosniff`, sandbox CSP) |
| POST | `/api/manage/online/downloads` | `{songs: OnlineSong[] (1–50), quality?: '128k'|'320k'|'flac'|'flac24bit' (default flac), libraryId, dir?, organize?, lyrics?, cover?}` | 202 `{jobs: DownloadJob[]}` |

The `/metadata/search`, `/lyrics`, and `/cover` endpoints answer `403 forbidden` while
`settings.onlineMetadata` is off (the default) and send nothing outside the server; invalid input
is `400`, a provider failure `503 unavailable` (§5.13).

`POST /downloads` answers `403 forbidden` while `settings.ytdlpEnabled` is off, `400` for a link that is not a
supported YouTube / bilibili link or a bad target, and `409 conflict` when yt-dlp is not installed or cannot run,
ffmpeg is missing, or 20 jobs are already waiting. Jobs live in memory (the newest 50 finished ones are kept; a
restart cancels running jobs), run two at a time for at most 3 hours, and re-check the setting when they start.
A job downloads into `<data>/tmp/ytdlp-job-<id>/`, writes the tags of `ytdlp.Item.Tags()` and a centre-cropped square
JPEG cover from the thumbnail through TagLib, and then imports the files exactly like an upload (not organized:
`<dir>/<title>.<ext>`, playlists `<dir>/<playlist>/<NN> <title>.<ext>`; organized: the rename pattern): library lock,
rescan, one `download` edit-log row per file (`{title, source, size}`), and a `library` event.

`GET /downloads` lists both kinds of jobs (`kind` `link` | `online`). `/online/search`, `/online/cover` and
`POST /online/downloads` answer `403 forbidden` while `settings.lxSourcesEnabled` is off, `400` for invalid input (unknown
platform, a song that does not validate, a bad target), `409 conflict` when 100 online jobs are already waiting, and
`503 unavailable` when a catalogue fails. An online job (three run at a time, at most 3 hours) re-checks the setting when
it starts, asks the `Candidates` in order for a link, and downloads it into `<data>/tmp/online-job-<id>/` (≤ 1 GiB); a
source error, an HTTP error or a file that is not audio (judged by its content: FLAC, MP3, M4A, Ogg/Opus, WAV, APE,
WavPack) moves on to the next source — except that an expired link (`lxmusic.LinkExpired`) first makes the job ask the
same source for a new link once, like lx-music — and the job fails with every source's reason when none delivers. It then writes
`TITLE`, `ARTIST` (one value per artist), `ALBUM`, optionally `LYRICS` (the catalogue's LRC with the translation paired
as same-timestamp lines) and the catalogue cover through TagLib, and imports the file like an upload (not organized:
`<dir>/<title> - <artists>.<ext>`; organized: the rename pattern) with one `download` edit-log row
(`{title, source: the song's catalogue page, quality, via: the source name, size}`).

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
| GET | `/api/admin/ytdlp` | | `YtdlpInfo` |
| POST | `/api/admin/ytdlp/check` | | `YtdlpInfo` (asks GitHub for the latest release; 403 while `ytdlpEnabled` is off, 503 when unreachable) |
| POST | `/api/admin/ytdlp/install` | | 202 `YtdlpInfo` (background install/update; 403 while off, 409 when `RAINY_YTDLP_PATH` is set or an install runs) |
| PUT | `/api/admin/ytdlp/cookies/{site}` | `{text}` (Netscape cookies.txt) | `CookieSaveResult` (stored encrypted; never returned) |
| DELETE | `/api/admin/ytdlp/cookies/{site}` | | 204 |
| GET | `/api/admin/sources` | | `LxSourcesInfo` |
| POST | `/api/admin/sources` | `{script}` or `{url}` (body ≤ ~19 MiB) | 201 `LxSource` (started right away while `lxSourcesEnabled` is on; `{url}` is 403 while off; 409 when the script is already imported) |
| PUT | `/api/admin/sources/order` | `{ids}` (every source once) | `LxSourcesInfo` |
| PUT | `/api/admin/sources/{id}` | `{enabled?, allowUpdateAlert?}` | `LxSource` (disabling stops the script) |
| DELETE | `/api/admin/sources/{id}` | | 204 |
| POST | `/api/admin/sources/{id}/reload` | | `LxSource` ((re)starts the script; 403 while `lxSourcesEnabled` is off) |
| POST | `/api/admin/sources/{id}/refresh` | | `LxSource` (downloads the script again from `sourceUrl`; 400 for a file import; 403 while off) |

| GET | `/api/admin/scrobbling` | | `ScrobblingAdmin` |
| PUT | `/api/admin/scrobbling/lastfm` | `{apiKey?, secret?}` (omitted = unchanged, `""` = remove; 32 hex characters) | `ScrobblingAdmin` (the secret is stored encrypted and never returned) |

Automatic fallback or one fixed source is chosen with `PUT /api/admin/settings` (`lxSourceMode` `auto` | `fixed`,
`lxSourceId`); no endpoint returns a script. Last.fm and ListenBrainz are turned on with `PUT /api/admin/settings`
(`lastfmEnabled`, `listenBrainzEnabled`).

### 7.8 Listening & scrobbling (any user; only their own data)
| Method | Path | Body / Query | Response |
|---|---|---|---|
| GET | `/api/listening/report` | `from?, to?` (unix ms; 0 = since the first play / now), `tz?` (IANA), `limit?` (≤ 50, default 10) | `ListeningReport` (400 when `to <= from`) |
| GET | `/api/listening/history` | `from?, to?, offset, limit` (default 50, ≤ 500) | `Page<Play>` (newest first) |
| GET | `/api/me/scrobbling` | | `ScrobbleAccount[]` (Last.fm, ListenBrainz) |
| POST | `/api/me/scrobbling/lastfm/auth` | `{callback}` (absolute http(s) link Last.fm sends the browser back to) | `{url}` to open (403 while `lastfmEnabled` is off, 409 without an API account, 400 bad callback) |
| POST | `/api/me/scrobbling/lastfm` | `{token, state}` (from the callback) | `ScrobbleAccount` (400 unknown / expired state or a token Last.fm refuses, 503 unreachable) |
| POST | `/api/me/scrobbling/listenbrainz` | `{token}` | `ScrobbleAccount` (403 while off, 400 invalid token, 503 unreachable) |
| PUT | `/api/me/scrobbling/{service}` | `{enabled}` (pausing drops the waiting plays) | `ScrobbleAccount` (404 not linked) |
| DELETE | `/api/me/scrobbling/{service}` | | 204 (forgets the account and its waiting plays) |

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
export type DownloadJobKind = 'link' | 'online'
export interface DownloadJob {
  id: string; kind: DownloadJobKind; url: string /*online: the song's catalogue page*/; site: '' | DownloadSite; title: string; status: DownloadJobStatus
  phase: '' | 'downloading' | 'processing'; progress: number /*0…1, -1 unknown*/; item: number; items: number
  speed: number; eta: number; error: string; libraryId: number; dir: string; organize: boolean; format: '' | DownloadFormat
  playlist: boolean; trackIds: string[]; errors: ItemError[]; createdBy: string; createdAt: number; startedAt: number; finishedAt: number
  online: OnlineJob | null
}
export type OnlinePlatform = 'kw' | 'kg' | 'tx' | 'wy' | 'mg'
export type OnlineQualityType = '128k' | '320k' | 'flac' | 'flac24bit'
export interface OnlineQuality { type: OnlineQualityType; size: string; hash?: string }
export interface OnlineSong {
  platform: OnlinePlatform; id: string; title: string; artists: string[]; album: string; albumId: string
  duration: number; coverUrl: string; qualities: OnlineQuality[]; extra: Record<string, string>; pageUrl: string
}
export interface OnlineSearchResult { items: OnlineSong[]; total: number; page: number; limit: number }
export interface OnlineStatus { enabled: boolean; sources: number; platforms: { id: OnlinePlatform; qualities: OnlineQualityType[] }[] }
export interface OnlineJob { song: OnlineSong; quality: OnlineQualityType; got: '' | OnlineQualityType; source: string; lyrics: boolean; cover: boolean }
export interface DownloadsStatus { enabled: boolean; ready: boolean; sites: { id: DownloadSite; cookies: boolean }[]; jobs: DownloadJob[] }
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
  ytdlpEnabled: boolean; lxSourcesEnabled: boolean; lxSourceMode: LxSourceMode; lxSourceId: string
  lastfmEnabled: boolean; listenBrainzEnabled: boolean
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
export interface CookieInfo { site: DownloadSite; configured: boolean; count: number; signedIn: boolean; expiresAt: number; updatedAt: number }
export interface CookieSaveResult extends CookieInfo { dropped: number }
export interface YtdlpInstallState { running: boolean; error: string; finishedAt: number }
export interface YtdlpInfo {
  enabled: boolean; managed: boolean; installed: boolean; version: string; error: string; latest: string; checkedAt: number
  asset: string; jsRuntime: '' | 'deno' | 'node' | 'quickjs'; ffmpeg: boolean; install: YtdlpInstallState; cookies: CookieInfo[]
}
export type LxSourceMode = 'auto' | 'fixed'
export type LxSourceStatus = 'idle' | 'loading' | 'ready' | 'error'
export interface LxSourceUpdateAlert { log: string; url: string; at: number }
export interface LxSource {
  id: string; name: string; description: string; version: string; author: string; homepage: string; sourceUrl: string
  size: number; enabled: boolean; position: number; allowUpdateAlert: boolean
  platforms: { platform: OnlinePlatform; qualities: OnlineQualityType[] }[]; status: LxSourceStatus; error: string
  updateAlert: LxSourceUpdateAlert | null; loadedAt: number; createdAt: number; updatedAt: number
}
export interface LxSourcesInfo { enabled: boolean; mode: LxSourceMode; sourceId: string; sources: LxSource[] }
export interface NowPlayingEntry { userId: string; username: string; trackId: string; player: string; since: number }
export interface ServerEvent { type: 'scan' | 'library' | 'nowPlaying'; data: unknown }

// ---- listening & scrobbling
export interface Play {
  id: number; trackId: string; playedAt: number; client: string
  title: string; artist: string; album: string; albumArtist: string; artistId: string; albumId: string; duration: number
  track: Track | null   // null when purged or missing
}
export interface ListeningTotals { plays: number; duration: number; tracks: number; artists: number; albums: number }
export type ListeningBucket = 'hour' | 'day' | 'week' | 'month'
export interface ListeningTimelineBucket { start: number; plays: number; duration: number }
export interface ListeningTopEntry { id: string; name: string; artist: string; plays: number; duration: number; coverArt: string; available: boolean }
export interface ListeningTopTrack extends ListeningTopEntry { track: Track | null }
export interface ListeningReport {
  from: number; to: number; tz: string; bucket: ListeningBucket; firstPlayAt: number
  totals: ListeningTotals; previous: ListeningTotals | null
  newTracks: number; newArtists: number; activeDays: number; longestStreak: number
  timeline: ListeningTimelineBucket[]; clock: number[][] /*[weekday 0 = Monday][hour]*/
  topArtists: ListeningTopEntry[]; topAlbums: ListeningTopEntry[]; topTracks: ListeningTopTrack[]
  topGenres: ListeningTopEntry[]; clients: ListeningTopEntry[]
}
export type ScrobbleService = 'lastfm' | 'listenbrainz'
export interface ScrobbleAccount {
  service: ScrobbleService; available: boolean; linked: boolean; needsRelink: boolean
  username: string; enabled: boolean; queued: number; lastError: string; lastErrorAt: number; lastSentAt: number; linkedAt: number
}
export interface ScrobblingAdmin {
  lastfm: { enabled: boolean; apiKey: string; hasSecret: boolean; configured: boolean; users: number }
  listenBrainz: { enabled: boolean; users: number }
}
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
| `/listening` | Listening report (overview + history; `?range=7d|30d|90d|12m|all|y<year>&tab=history`) | library |
| `/settings` | User settings | player |
| `/settings/lastfm` | Last.fm sign-in callback (links the account, then back to Settings → Scrobbling) | player |
| `/manage` | Tracks → Metadata (track table + tag editor) | manage |
| `/manage/upload` | Tracks → Upload (files, links) | manage |
| `/manage/online` | Tracks → Online (online music search + downloads) | manage |
| `/manage/folders` | Folder browser | manage |
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
- `@/stores/ui` — `useUI` zustand store: `tagEditor: {open: boolean, trackIds: string[]}`, `openTagEditor(ids)`, `closeTagEditor()`, `addToPlaylist: {open, trackIds}`, `openAddToPlaylist(ids)`, `closeAddToPlaylist()`; persisted as `rainy.ui`: `sidebarOpen`/`setSidebarOpen(b)`, `mascotArt`/`setMascotArt(b)` (mascot illustrations, default on), `mascotCompanion`/`setMascotCompanion(b)` (corner companion, default on).
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
- Shell slots imported from the player feature: `<PlayerDock/>` (tablet/desktop: bottom bar, floating window or floating mini bar; positions itself), `<MiniPlayer/>` (mobile, above tab bar), `<NowPlayingSheet/>`, `<AudioEngine/>` (headless), `<MascotCompanion/>` (tablet/desktop while `mascotCompanion` is on; positions itself above the player chrome).
- `@/features/player/dock` — `usePlayerDock` zustand store (persisted as `rainy.player-dock`): `mode: 'bar' | 'window' | 'compact'`, `barAutoHide: boolean`, `setMode(mode)`, `setBarAutoHide(b)`. `<PlayerDock/>` mirrors it to `<html data-player-dock="bar|collapsed|window|compact|idle">`, which `index.css` maps to `--player-reserve` (full-width bottom space: sidebar, side panel), `--player-clearance` (`.page-pad`, sticky footers) and `--player-toast-bottom/right`.
- `@/features/library/components/track-actions` — `<TrackActionsMenu tracks={Track[]} context?:{playlistId?:string, position?:number}>` renders a "…" button (DropdownMenu on desktop, Drawer action sheet on mobile) with: Play next, Add to queue, Add to playlist, Go to album, Go to artist, Star/Unstar, Download, Edit tags (managers → `openTagEditor`), Remove from playlist (when in playlist context).

### 9.3 Design system
- Aesthetic: calm, modern, Apple-Music-inspired minimalism. Lots of whitespace, big artwork, restrained colour; the accent is used sparingly (play buttons, active nav, progress).
- shadcn style `new-york`, base colour `neutral`, CSS variables, radius `0.75rem`. Light + dark (follows system, user override). Accent presets selectable in settings (CSS var `--primary`): `rain` (default, `oklch(0.62 0.17 255)` periwinkle blue), `rose`, `violet`, `emerald`, `amber`, `graphite`.
- Album colours (`features/library/lib/cover-theme.ts`, `useCoverTheme(coverArt)`): album, playlist and artist pages (`<Page {...useCoverTheme(coverArt)}>`) sample their artwork (48 px canvas) for a dominant background and a vivid accent, and sets scoped tokens (`--background`, `--foreground`, `--card`, `--secondary`, `--muted(-foreground)`, `--accent`, `--border`, `--input`, and `--primary`/`--primary-foreground`/`--ring` unless the cover is greyscale) plus `.dark` for deep pages on the `<Page>`; no artwork (404, empty playlist) keeps the plain theme; `<meta name="theme-color">` follows while mounted. A dark app theme always gets a deep page; a light theme gets a pale tint, or a deep page for dark covers. Text tokens keep ≥ 4.5:1 contrast. `useTheme()` exposes `albumColors`/`setAlbumColors` (persisted as `rainy.album-colors`, default on; Settings → Appearance).
- Font: `"Inter Variable", -apple-system, BlinkMacSystemFont, "SF Pro Text", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", "Noto Sans CJK SC", sans-serif`; tabular numbers for times (`tabular-nums`).
- Type scale: page title `text-3xl font-bold tracking-tight` (mobile large title 34px/41px bold, collapsing into a 17px semibold centered nav title on scroll — `PageHeader` handles it), section title `text-xl font-semibold`, body `text-sm`, secondary `text-muted-foreground`.
- Artwork: `rounded-lg` (≥ 160px) / `rounded-md` (thumbnails), subtle `shadow-sm` and 1px `ring-black/5 dark:ring-white/10`; artists are circles. Always square (`aspect-square object-cover`), lazy loaded, fade-in, gradient placeholder with a music-note icon when missing.
- Glass surfaces (tab bar, mini player, top bar on scroll): `bg-background/75 backdrop-blur-xl backdrop-saturate-150 border-border/60`.
- Layout: desktop ≥ 1024px → left sidebar (240px, shadcn Sidebar, collapsible to icons) + content + 80px bottom player bar. 768–1023px → collapsed icon sidebar. < 768px → bottom tab bar (Home, Library, Search, and Manage for managers) 49px + safe area, floating mini player (56px, `rounded-xl`, 8px side margins) above it.
- Navigation tiers (`layouts/nav.ts`): `LIBRARY_NAV` for listening (including the listening report `/listening`, which the
  phones' Library hub lists too); `TRACKS_NAV` ("Tracks", `/manage`) is always visible to managers (sidebar item active on every `TRACKS_TABS` route; on phones the "Manage" tab, which also covers `/admin`); its pages `TRACKS_TABS` — Metadata `/manage`, Upload `/manage/upload`, Online `/manage/online` — are route tabs (`TracksTabs`, links with `aria-current`) in each page header; `MANAGE_NAV` (folders, doctor, trash, history) and `ADMIN_NAV` are folded — sidebar Collapsibles "Library tools" / "Admin" that start closed and open while one of their routes is active (a DropdownMenu flyout on the icon rail), and one "Library tools" DropdownMenu (`ManageSections`) next to the tabs on phones.
- CSS vars in `index.css`: `--tabbar-h`, `--miniplayer-h`, `--playerbar-h`, `--player-window-w` (400px), `--compact-player-h` (68px), `--player-reserve`, `--player-clearance`, `--mascot-right` (companion right offset; beside the floating window in `window` mode), `--safe-top/bottom` (env(safe-area-inset-*)); pages use `.page-pad` bottom padding utility so content never hides behind player chrome. Never hard-code `--playerbar-h` for spacing: the bar may be hidden or replaced by a floating player.
- Motion: `motion/react` springs (`type:'spring', stiffness: 400, damping: 36`), `MotionConfig reducedMotion="user"`. Press feedback on mobile: `active:scale-[0.97]` transitions.
- Icons: lucide-react, `size-4`/`size-5`, `strokeWidth={1.75}`; transport controls use filled glyphs (`fill="currentColor"`).
- Density: desktop list rows 48px; mobile rows 60px with 44px artwork, iOS-style hairline separators inset from the artwork.
- Touch: `-webkit-tap-highlight-color: transparent`, `overscroll-behavior: none` on body, `touch-action: manipulation`; hit targets ≥ 44px on mobile.
- Accessibility: every icon button has `aria-label`; focus rings visible; colour contrast AA.
- Mascot (`@/components/mascot`): `<MascotArt pose className>` renders a decorative transparent WebP (`alt=""`, sized by a height class) from `web/src/assets/mascot/`; `MascotPose` = `lost | offline | forbidden | empty | welcome | settings` (full-body scenes) `| idle | listening | sleeping` (upper-body crops with a flat bottom edge). `useMascotArt()` (`@/hooks/use-mascot-art`) reads `useUI.mascotArt`. `<EmptyState art={pose}>` shows the pose instead of its icon at the default size when illustrations are on (compact keeps the icon); `ErrorState` uses `offline`, `NotFoundPage`/404 `lost`, `ForbiddenPage` `forbidden`, render errors `offline`, main library empties `empty`, no search results `lost`; `AuthCard` shows `welcome` beside the card at ≥ 1024px; Settings › Mascot shows `settings` and the two switches. `<MascotCompanion/>` (`features/player/components/mascot-companion.tsx`) is fixed at `right: var(--mascot-right)` (+ `--player-panel-w` at xl), `bottom: var(--player-clearance)`: `listening` while playing (`animate-mascot-sway`, motion-safe), `idle` when paused, `sleeping` after 2 min paused; click → a 4 s speech bubble (`common:mascot.lines.*`), × → `setMascotCompanion(false)` with a toast. Not shown on phones. The app icon (`<Logo>`, `src/assets/brand/rainy-icon.webp`, PWA icons) is the mascot listening to music on a rain-blue tile.

### 9.4 iOS-style player (player agent)
- **Mini player** (mobile): floating glass pill above the tab bar: artwork 40px rounded-md, title/artist (one line each, ellipsis), play/pause + next; thin progress line along the bottom edge; tap → Now Playing; swipe up also opens.
- **Now Playing** (full-screen sheet, mobile; large centered modal/overlay on desktop): slides up with a spring, drag-down to dismiss (grabber at top), background = blurred, saturated artwork colours (`fast-average-color` or canvas sampling + large blurred artwork layer), light text. Artwork large and rounded, **scales down with a spring when paused** and back up when playing (Apple Music behaviour). Title (semibold) + artist (tappable → artist page, closes sheet) with a star button and "…" menu. Scrubber: thin track that thickens while dragging, elapsed / −remaining underneath in tabular nums. Transport: prev / play-pause / next, large filled glyphs, press-scale feedback. Volume slider (hidden on iOS where `audio.volume` is read-only). Bottom row: lyrics toggle, AirPlay button when `window.WebKitPlaybackTargetAvailabilityEvent` exists (`audio.webkitShowPlaybackTargetPicker()`), queue toggle. Lyrics view: synced lines, active line bold/bright with others dimmed, auto-scroll with smooth centring, tap a line to seek; plain lyrics scroll normally. Bilingual lyrics (`groupBilingual`: consecutive synced lines with the same `start` → original + translations; otherwise space-separated `original 中文` lines when most lines split) render the translation beneath each line (~0.68em, dimmer); lines carry `lang` from `guessLang` (`ja`/`ko`/`zh`, with matching CJK font stacks in `index.css`). `TranslationToggle` (bottom action row on phones, stage header, side panel header; only while lyrics are open and have translations) flips the persisted playback pref `lyricsTranslation` (default on, also in Settings → Playback). Queue view: "Playing Next" list with drag handles (dnd-kit), shuffle / repeat toggles, clear.
- **Desktop player bar**: artwork + title/artist (links) + star | transport + scrubber with times | lyrics, queue, volume, layout menu, expand. Side panel (right, 360px) for queue / lyrics (bar layout only).
- **Player layouts** (≥ 768px, layout menu = `PictureInPicture2` button in the bar, window and mini bar; persisted in `usePlayerDock`): *bottom bar* (default); *auto-hide* (NetEase style: the bar slides below the viewport leaving a centred handle tab; hovering the handle / bottom edge or keyboard focus peeks it until the pointer or focus leaves, clicking the handle pins it open; on a pinned bar the handle appears on hover and turns auto-hide on); *floating window* (bottom-right 400×min(640px, viewport − 3rem), artwork backdrop like Now Playing: queue position, grabber → mini bar, layout menu, full screen; artwork that scales down while paused, title/star/…, scrubber, shuffle/prev/play/next/repeat, volume, lyrics/translation/AirPlay/queue — lyrics and queue replace the artwork inside the window; not modal); *floating mini bar* (bottom-right 390×68px glass card: artwork + title/artist → opens the window, layout menu, play/pause, next, thin progress line; dragging sideways on it scrubs relative to the current position — a full-width drag = 20 % of the track, 20 s…10 min — with an origin → target ±delta preview above it, Escape cancels). Floating layouts render nothing while idle and hide under full-screen Now Playing, which is available from every layout and returns to it on close. Phones keep the mini pill + sheet.
- Audio engine: single `HTMLAudioElement`, `preload="auto"`, next track preloaded ~20 s before the end with a second element; Media Session (metadata with artwork 96–512, play/pause/prev/next/seekto/seekbackward/seekforward, `setPositionState`); scrobble "now playing" on start and submission after 50 % or 4 min; queue persisted to localStorage immediately and to `/api/queue` debounced (10 s) so the Subsonic clients can resume it; restore on load (paused). Streaming quality setting (original / 320 / 192 / 128 kbps + format) → stream URL params. ReplayGain option (track/album/off) applied via volume scaling (not on iOS). Keyboard: space, ←/→ seek 5s, shift+←/→ prev/next, m mute. Errors → toast and skip.
- **PWA**: `vite-plugin-pwa` (generateSW, `registerType: 'prompt'` with an "update available" toast), manifest (name "Rainy", short_name "Rainy", `display: standalone`, theme/background colours for light+dark, icons 64/192/512/maskable + apple-touch-icon 180 and `favicon.ico`, generated by `pnpm generate-pwa-assets` from `public/icon-source.png` (rounded tile) and `public/icon-source-maskable.png` (full bleed), which the precache ignores; `webp` is precached so the mascot art works offline), iOS meta tags (`apple-mobile-web-app-capable`, `apple-mobile-web-app-status-bar-style=black-translucent`, `viewport-fit=cover`), navigateFallback `index.html` with denylist `/api`, `/rest`. Runtime caching: `/api/cover/*` CacheFirst (500 entries, 30 days); never cache `/api/stream`, `/api/download`, `/api/events`.

### 9.5 Management UI (manage agent)
- **Tracks** (sidebar entry; page title "Tracks") has three route tabs: Metadata, Upload, Online. Upload and Online share
  the destination (`useDestination`, `DestinationSection`: library, folder, organize by tags — the folder and library are
  kept while switching tabs, `organize` is remembered) and the server job list (`DownloadJobs`, links on Upload, online
  music on Online; `useJobNotifications` toasts finished jobs and refreshes the library).
- **Metadata** `/manage`: virtualized track table (checkbox column, cover, title, artist, album, album artist, #, disc, year, genre, format, bitrate, path; sortable; column visibility), search + filters (album/artist/genre/folder/missing), multi-select (click, shift-range, ctrl/cmd toggle, select all matching). Toolbar: Edit tags, Cover, Rename/Organize, Fix encoding, Rebuild tags, Delete, Rescan. Mobile: list with selection mode.
- **Tag editor** (`TagEditorHost`, opened via `useUI.openTagEditor(ids)`): right-side Sheet (desktop, ~560px) / full-screen Drawer (mobile). Tabs: *Details* (common fields; with multiple tracks selected, differing values show a "Multiple values" placeholder and are only written if edited — each field has a revert button), *Cover* (preview, drop/paste/upload, remove, apply to whole album, save as folder image), *Lyrics* (textarea with LRC highlighting, "insert timestamp at current playback time" button that also restamps following lines sharing the old timestamps, target embedded / .lrc; `analyzeBilingual` runs on every edit and, when most non-Chinese lines are `original 中文`, shows a banner with a preview and "Split lines" → `toPairedLrc` rewrites timed lines as same-timestamp pairs, original first; nothing is applied without the user; a "Bilingual" badge marks already paired text), *All tags* (raw key/value table incl. custom keys, add/remove), *File* (read-only file info). *Search online* (`OnlineDialog`, `lib/online.ts`): pick a catalogue, search (prefilled with title + artist, or album + album artist for several tracks), open a result to see current → new values with checkboxes (fields that would change are pre-selected; with several tracks only album-level fields are offered), plus the cover (proxied through `/manage/metadata/cover`) and, for one track, lyrics with an optional translation paired as same-timestamp lines; "Fill in editor" only changes the draft — the user saves as usual. When `onlineMetadata` is off the dialog explains how to enable it. Tools menu with preview-before-apply: auto-number tracks (by current order), tags from filename pattern, find & replace in a field (regex optional), case transforms, copy field to field, clear field. Save → `POST /api/manage/tags` with per-track diffs only; show per-track errors.
- **Rename / organize** dialog: pattern input with token chips + saved default from settings, live preview table (from → to, status badges), apply.
- **Upload**: drag & drop zone (files + folders), per-file progress (XHR upload progress), target library + folder picker, "organize by tags" toggle.
  *From a link* (`LinkDownload`): YouTube / bilibili link (share text accepted; `detectSite` shows a site chip), audio format
  (original / M4A / MP3 / Opus, remembered), "whole playlist", same destination and organize options; the job list polls
  `/manage/downloads` every second while a job is active (progress, entry n/m, speed, ETA, cancel / retry / remove, Edit tags
  when done) and toasts + refreshes the library when a job this page saw finishes. When the feature is off or yt-dlp is not
  installed, the section says so (admins get a link to Settings → yt-dlp).
- **Online** `/manage/online`: when `lxSourcesEnabled` is off or no source is usable, an empty state explains it (admins get
  a link to Settings → Sources). Otherwise a collapsible "Download options" summary (destination, quality — Hi-Res /
  FLAC / 320K / 128K, "a lower one is used when missing", embed lyrics, embed cover; quality, toggles and platform are
  remembered), a search form (platform select + query), and results as rows (checkbox, proxied cover, title, artists ·
  album, duration, the quality badge a download would ask for with all sizes in its title, catalogue page link, a
  download button); "Load more" pages through results (`useInfiniteQuery`, key `['online-search', platform, q]`, not a
  library root). A floating bar downloads the selection (sent in batches of 50).
- **Folder browser**, **Doctor** (issue summary cards → lists with quick actions), **Trash** (restore / purge), **History** (edit log with readable diffs).
- **Admin**: users (table + dialog; roles: admin, manager, download), libraries & scan (cards with path, counts, writable badge; quick/full scan buttons; live progress via `/api/events`), server settings form, system info (versions, ffmpeg, sizes, clear cache).
  Settings tabs General / yt-dlp / Sources / Scrobbling / System. *Scrobbling* (`ScrobblingSettings`): the `lastfmEnabled` and
  `listenBrainzEnabled` switches (saved immediately), the Last.fm API key and write-only shared secret with a link to create an
  API account, the number of linked users per service, and a "Your account" row per service showing whether the signed-in
  administrator connected their own account (the switches only allow a service; everyone links in Settings → Scrobbling),
  with a link to `/settings#scrobbling`. *Sources* (`SourcesSettings`): the `lxSourcesEnabled` switch, the mode
  (automatic fallback / always one source + a source select), and the sources in priority order (up / down buttons;
  name, version, status badge, description, author · file or link · size · last start, platform/best-quality chips, the
  start error, the script's update notice with its link and "Update from link"; an enable switch and a menu: test /
  restart, update from link, show / hide update notices, homepage, remove with confirmation). `SourceImportDialog`
  warns that scripts are third-party code running on the server, then imports a `.js` file (read in the browser) or a
  link (needs the setting on). *yt-dlp* (`YtdlpSettings`): the `ytdlpEnabled` switch (saved immediately), installed
  version, latest release, Check for updates / Install / Update to X (polls while installing), ffmpeg and JavaScript runtime
  status, and per-site sign-in cookies (count, signed in, expiry, updated; Add / Replace / Remove). `CookieDialog` first shows
  a warning that must be acknowledged (cookies are account credentials; a malicious program could impersonate the user;
  never share them; Rainy keeps them encrypted on the server and uploads them nowhere), then the export guide for the
  "Get cookies.txt LOCALLY" extension and a paste / file field (no spell check or autofill); the text is cleared on close.

### 9.5a Listening UI (library + player agents)
- **Listening** `/listening` (`features/library/pages/listening-page.tsx`; pure helpers in `features/library/lib/listening.ts`,
  unit-tested): a period select (last 7 / 30 / 90 days, last 12 months, all time, then each calendar year with plays; rolling
  periods start at a local midnight), tabs Overview / History, and the browser's time zone sent as `tz`. Overview: stat tiles
  (plays and listening time with the change against the previous period, artists and songs with the new ones, albums, days
  with music + longest streak, plays per day, first play), a one-hue bar timeline (plays or hours; 2px gaps; hover / touch /
  arrow-key tooltip; a hidden data table), top artists and albums shelves, top songs (tapping plays them as a queue; purged
  items are dimmed, without links), a weekday × hour heatmap (one hue, five steps on a square-root scale, legend), top genres
  and players as bar lists, and a link to the scrobbling settings. History: plays grouped by local day (Today, Yesterday,
  dates), 100 per page with "Load more"; each row plays its track and has the track menu. Query keys start with `listening`
  (a `LIBRARY_QUERY_ROOTS` root).
- **Settings → Scrobbling** (`ScrobblingSection`, `#scrobbling`): per service the linked account, a pause switch, the waiting
  plays, the last sent time and last error, "Connect again" when the service revoked access, and Disconnect with a
  confirmation. Last.fm "Connect" asks `/me/scrobbling/lastfm/auth` with the callback `<origin>/settings/lastfm` and navigates
  to the returned page; `lastfm-callback-page` posts `{token, state}` once and returns to `/settings#scrobbling`.
  ListenBrainz takes the user token in a password field. Services the administrator has not enabled say so.

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
  removes apostrophes and turns other punctuation/symbols into spaces ("AC/DC" → "ac dc").
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
