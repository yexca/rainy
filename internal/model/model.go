// Package model holds the shared data structures of Rainy. Each struct carries `db` tags
// (SQLite column names, see internal/db/migrations) and `json` tags; the JSON form is the
// native API shape mirrored in web/src/lib/api/types.ts.
//
// Fields documented as "computed" are not stored; the store fills them before returning
// (see the Fill methods and store's fill helpers). Slices always marshal as JSON arrays,
// never null.
//
// This package must not import other internal packages.
package model

import (
	"encoding/json"
	"strings"
	"time"
)

// Library is a music folder root.
type Library struct {
	ID         int64  `db:"id" json:"id"`
	Name       string `db:"name" json:"name"`
	Path       string `db:"path" json:"path"` // absolute OS path
	CreatedAt  int64  `db:"created_at" json:"createdAt"`
	UpdatedAt  int64  `db:"updated_at" json:"updatedAt"`
	LastScanAt int64  `db:"last_scan_at" json:"lastScanAt"`
}

// User is an account. PasswordEnc is the reversibly encrypted password (needed for
// Subsonic token auth) and is never serialised.
type User struct {
	ID          string  `db:"id" json:"id"`
	Username    string  `db:"username" json:"username"`
	DisplayName string  `db:"display_name" json:"displayName"`
	Email       string  `db:"email" json:"email"`
	PasswordEnc string  `db:"password_enc" json:"-"`
	IsAdmin     bool    `db:"is_admin" json:"isAdmin"`
	CanManage   bool    `db:"can_manage" json:"canManage"`
	CanDownload bool    `db:"can_download" json:"canDownload"`
	APIKeyHash  *string `db:"api_key_hash" json:"-"`
	HasAPIKey   bool    `db:"-" json:"hasApiKey"` // computed
	CreatedAt   int64   `db:"created_at" json:"createdAt"`
	UpdatedAt   int64   `db:"updated_at" json:"updatedAt"`
	LastLoginAt int64   `db:"last_login_at" json:"lastLoginAt"`
	LastSeenAt  int64   `db:"last_seen_at" json:"lastSeenAt"`
}

// CanEdit reports whether the user may modify library files (tags, covers, rename, …).
func (u *User) CanEdit() bool { return u != nil && (u.IsAdmin || u.CanManage) }

// Fill computes derived fields.
func (u *User) Fill() { u.HasAPIKey = u.APIKeyHash != nil && *u.APIKeyHash != "" }

// Track is one audio file.
type Track struct {
	ID            string   `db:"id" json:"id"`
	LibraryID     int64    `db:"library_id" json:"libraryId"`
	Path          string   `db:"path" json:"path"` // library-relative, forward slashes
	Dir           string   `db:"dir" json:"dir"`   // library-relative parent dir, "" for root
	Filename      string   `db:"filename" json:"filename"`
	Suffix        string   `db:"suffix" json:"suffix"` // lower-case extension without dot
	Size          int64    `db:"size" json:"size"`
	Mtime         int64    `db:"mtime" json:"mtime"`
	Title         string   `db:"title" json:"title"`
	Album         string   `db:"album" json:"album"`
	Artist        string   `db:"artist" json:"artist"`
	AlbumArtist   string   `db:"album_artist" json:"albumArtist"`
	AlbumID       string   `db:"album_id" json:"albumId"`
	ArtistID      string   `db:"artist_id" json:"artistId"`
	AlbumArtistID string   `db:"album_artist_id" json:"albumArtistId"`
	TrackNumber   int      `db:"track_number" json:"trackNumber"`
	TrackTotal    int      `db:"track_total" json:"trackTotal"`
	DiscNumber    int      `db:"disc_number" json:"discNumber"`
	DiscTotal     int      `db:"disc_total" json:"discTotal"`
	DiscSubtitle  string   `db:"disc_subtitle" json:"discSubtitle"`
	Year          int      `db:"year" json:"year"`
	Date          string   `db:"date" json:"date"`
	OriginalYear  int      `db:"original_year" json:"originalYear"`
	Genre         string   `db:"genre" json:"genre"` // genres joined with "; "
	Genres        []string `db:"-" json:"genres"`    // computed from Genre
	Composer      string   `db:"composer" json:"composer"`
	Comment       string   `db:"comment" json:"comment"`
	Lyrics        string   `db:"lyrics" json:"-"` // embedded lyrics; only loaded by store.GetTrack
	HasLrc        bool     `db:"has_lrc" json:"hasLrc"`
	BPM           int      `db:"bpm" json:"bpm"`
	Compilation   bool     `db:"compilation" json:"compilation"`
	Duration      float64  `db:"duration" json:"duration"` // seconds
	Bitrate       int      `db:"bitrate" json:"bitrate"`   // kbit/s
	SampleRate    int      `db:"sample_rate" json:"sampleRate"`
	BitDepth      int      `db:"bit_depth" json:"bitDepth"`
	Channels      int      `db:"channels" json:"channels"`
	Codec         string   `db:"codec" json:"codec"`
	HasCover      bool     `db:"has_cover" json:"hasCover"` // embedded picture present

	RGTrackGain *float64 `db:"rg_track_gain" json:"rgTrackGain"`
	RGTrackPeak *float64 `db:"rg_track_peak" json:"rgTrackPeak"`
	RGAlbumGain *float64 `db:"rg_album_gain" json:"rgAlbumGain"`
	RGAlbumPeak *float64 `db:"rg_album_peak" json:"rgAlbumPeak"`

	MbzTrackID       string `db:"mbz_track_id" json:"mbzTrackId"`
	MbzAlbumID       string `db:"mbz_album_id" json:"mbzAlbumId"`
	MbzArtistID      string `db:"mbz_artist_id" json:"mbzArtistId"`
	MbzAlbumArtistID string `db:"mbz_album_artist_id" json:"mbzAlbumArtistId"`
	SortTitle        string `db:"sort_title" json:"sortTitle"`
	SortAlbum        string `db:"sort_album" json:"sortAlbum"`
	SortArtist       string `db:"sort_artist" json:"sortArtist"`
	SortAlbumArtist  string `db:"sort_album_artist" json:"sortAlbumArtist"`
	SearchText       string `db:"search_text" json:"-"`
	Missing          bool   `db:"missing" json:"missing"`
	CreatedAt        int64  `db:"created_at" json:"createdAt"`
	UpdatedAt        int64  `db:"updated_at" json:"updatedAt"`

	// Per-user annotation (zero values when queried without a user).
	StarredAt *int64 `db:"starred_at" json:"starredAt"`
	Rating    int    `db:"rating" json:"rating"`
	PlayCount int    `db:"play_count" json:"playCount"`
	PlayedAt  int64  `db:"played_at" json:"playedAt"`

	// Query helpers (selected by the store, never serialised).
	HasEmbeddedLyrics bool  `db:"has_embedded_lyrics" json:"-"` // (lyrics != '') in list queries
	AlbumUpdatedAt    int64 `db:"album_updated_at" json:"-"`    // albums.updated_at, for CoverArt

	// Computed.
	Starred     bool   `db:"-" json:"starred"`
	CoverArt    string `db:"-" json:"coverArt"`
	ContentType string `db:"-" json:"contentType"`
	HasLyrics   bool   `db:"-" json:"hasLyrics"`
}

// Fill computes the derived fields that do not depend on other packages (Genres, Starred,
// HasLyrics). CoverArt and ContentType are set by the store.
func (t *Track) Fill() {
	t.Genres = SplitGenre(t.Genre)
	t.Starred = t.StarredAt != nil
	t.HasLyrics = t.Lyrics != "" || t.HasEmbeddedLyrics || t.HasLrc
	if t.Lyrics != "" {
		t.HasEmbeddedLyrics = true
	}
}

// MarshalJSON guarantees `genres` is an array even for tracks that were never filled.
//
// Because of this method, do NOT embed Track in another struct that is marshalled to
// JSON: the promoted MarshalJSON would silently drop the outer struct's fields. Use a
// named field instead (e.g. a field "Track model.Track" tagged json:"track"). The same
// applies to EditLogEntry.
func (t Track) MarshalJSON() ([]byte, error) {
	type plain Track
	p := plain(t)
	if p.Genres == nil {
		p.Genres = SplitGenre(p.Genre)
	}
	return json.Marshal(p)
}

// GenreJoiner joins multiple genres in Track.Genre.
const GenreJoiner = "; "

// SplitGenre splits a stored Track.Genre value ("Pop; Rock") into its parts. It never
// returns nil.
func SplitGenre(s string) []string {
	out := []string{}
	for _, g := range strings.Split(s, ";") {
		if g = strings.TrimSpace(g); g != "" {
			out = append(out, g)
		}
	}
	return out
}

// Album is an aggregate over tracks sharing album_id.
type Album struct {
	ID            string  `db:"id" json:"id"`
	LibraryID     int64   `db:"library_id" json:"libraryId"`
	Name          string  `db:"name" json:"name"`
	SortName      string  `db:"sort_name" json:"sortName"`
	AlbumArtist   string  `db:"album_artist" json:"artist"`
	AlbumArtistID string  `db:"album_artist_id" json:"artistId"`
	Year          int     `db:"year" json:"year"`
	Genre         string  `db:"genre" json:"genre"`
	Compilation   bool    `db:"compilation" json:"compilation"`
	SongCount     int     `db:"song_count" json:"songCount"`
	DiscCount     int     `db:"disc_count" json:"discCount"`
	Duration      float64 `db:"duration" json:"duration"`
	Size          int64   `db:"size" json:"size"`
	CoverPath     string  `db:"cover_path" json:"-"`
	CoverTrackID  string  `db:"cover_track_id" json:"-"`
	MbzAlbumID    string  `db:"mbz_album_id" json:"mbzAlbumId"`
	SearchText    string  `db:"search_text" json:"-"`
	CreatedAt     int64   `db:"created_at" json:"createdAt"`
	UpdatedAt     int64   `db:"updated_at" json:"updatedAt"`

	StarredAt *int64 `db:"starred_at" json:"starredAt"`
	Rating    int    `db:"rating" json:"rating"`
	PlayCount int    `db:"play_count" json:"playCount"`
	PlayedAt  int64  `db:"played_at" json:"playedAt"`

	Starred  bool   `db:"-" json:"starred"`  // computed
	CoverArt string `db:"-" json:"coverArt"` // computed
}

// Fill computes Starred (CoverArt is set by the store).
func (a *Album) Fill() { a.Starred = a.StarredAt != nil }

// Artist is an aggregate over tracks by artist_id or album_artist_id.
type Artist struct {
	ID          string `db:"id" json:"id"`
	Name        string `db:"name" json:"name"`
	SortName    string `db:"sort_name" json:"sortName"`
	IndexKey    string `db:"index_key" json:"indexKey"`
	AlbumCount  int    `db:"album_count" json:"albumCount"`
	SongCount   int    `db:"song_count" json:"songCount"`
	MbzArtistID string `db:"mbz_artist_id" json:"mbzArtistId"`
	ImagePath   string `db:"image_path" json:"-"`
	SearchText  string `db:"search_text" json:"-"`
	CreatedAt   int64  `db:"created_at" json:"createdAt"`
	UpdatedAt   int64  `db:"updated_at" json:"updatedAt"`

	StarredAt *int64 `db:"starred_at" json:"starredAt"`
	Rating    int    `db:"rating" json:"rating"`
	PlayCount int    `db:"play_count" json:"playCount"`
	PlayedAt  int64  `db:"played_at" json:"playedAt"`

	Starred  bool   `db:"-" json:"starred"`  // computed
	CoverArt string `db:"-" json:"coverArt"` // computed
}

// Fill computes Starred (CoverArt is set by the store).
func (a *Artist) Fill() { a.Starred = a.StarredAt != nil }

// Genre with counts over non-missing tracks.
type Genre struct {
	ID         string `db:"id" json:"id"`
	Name       string `db:"name" json:"name"`
	SongCount  int    `db:"song_count" json:"songCount"`
	AlbumCount int    `db:"album_count" json:"albumCount"`
}

// Playlist metadata. SongCount/Duration count only non-missing tracks.
type Playlist struct {
	ID        string  `db:"id" json:"id"`
	Name      string  `db:"name" json:"name"`
	Comment   string  `db:"comment" json:"comment"`
	OwnerID   string  `db:"owner_id" json:"ownerId"`
	OwnerName string  `db:"owner_name" json:"ownerName"` // joined users.username
	Public    bool    `db:"public" json:"public"`
	SongCount int     `db:"song_count" json:"songCount"`
	Duration  float64 `db:"duration" json:"duration"`
	CreatedAt int64   `db:"created_at" json:"createdAt"`
	UpdatedAt int64   `db:"updated_at" json:"updatedAt"`
	CoverArt  string  `db:"-" json:"coverArt"` // computed "pl-<id>_<v>"
	// AlbumsUpdatedAt is the latest updated_at of the albums of its (non-missing) tracks;
	// it versions CoverArt together with UpdatedAt (the mosaic shows album covers).
	AlbumsUpdatedAt int64 `db:"albums_updated_at" json:"-"`
}

// PlayQueue is a user's saved play queue (shared between the web UI and Subsonic clients).
// store.GetPlayQueue always returns a non-nil TrackIDs. It deliberately has no MarshalJSON
// so it can be embedded in response structs (GET /api/queue adds `tracks`).
type PlayQueue struct {
	TrackIDs   []string `json:"trackIds"`
	CurrentID  string   `json:"currentId"`
	PositionMs int64    `json:"positionMs"`
	ChangedBy  string   `json:"changedBy"`
	UpdatedAt  int64    `json:"updatedAt"`
	// CurrentIndex is the position of the current entry in TrackIDs (a queue may hold the
	// same song twice). The store normalizes it: when it does not point at CurrentID it
	// becomes the first occurrence of CurrentID (-1 when absent).
	CurrentIndex int `json:"-"`
}

// CurrentPos returns the index of the current entry in TrackIDs (normalized
// CurrentIndex), or -1.
func (q *PlayQueue) CurrentPos() int {
	if q.CurrentIndex >= 0 && q.CurrentIndex < len(q.TrackIDs) && q.TrackIDs[q.CurrentIndex] == q.CurrentID {
		return q.CurrentIndex
	}
	for i, id := range q.TrackIDs {
		if id == q.CurrentID {
			return i
		}
	}
	return -1
}

// Bookmark is a saved playback position within a track.
type Bookmark struct {
	TrackID    string `db:"track_id" json:"trackId"`
	PositionMs int64  `db:"position_ms" json:"positionMs"`
	Comment    string `db:"comment" json:"comment"`
	CreatedAt  int64  `db:"created_at" json:"createdAt"`
	UpdatedAt  int64  `db:"updated_at" json:"updatedAt"`
}

// RadioStation is an internet radio stream.
type RadioStation struct {
	ID          string `db:"id" json:"id"`
	Name        string `db:"name" json:"name"`
	StreamURL   string `db:"stream_url" json:"streamUrl"`
	HomepageURL string `db:"homepage_url" json:"homepageUrl"`
	CreatedAt   int64  `db:"created_at" json:"createdAt"`
	UpdatedAt   int64  `db:"updated_at" json:"updatedAt"`
}

// EditLogEntry records a library modification.
type EditLogEntry struct {
	ID        int64           `db:"id" json:"id"`
	UserID    string          `db:"user_id" json:"userId"`
	Username  string          `db:"username" json:"username"`
	Action    string          `db:"action" json:"action"` // tags|cover|lyrics|rename|delete|restore|upload|purge|encoding
	TrackID   string          `db:"track_id" json:"trackId"`
	Path      string          `db:"path" json:"path"`
	Details   json.RawMessage `db:"details" json:"details"`
	CreatedAt int64           `db:"created_at" json:"createdAt"`
}

// MarshalJSON guarantees `details` is a JSON value (defaults to {}). Do not embed
// EditLogEntry in another JSON struct (see Track.MarshalJSON).
func (e EditLogEntry) MarshalJSON() ([]byte, error) {
	type plain EditLogEntry
	p := plain(e)
	if len(p.Details) == 0 || !json.Valid(p.Details) {
		p.Details = json.RawMessage("{}")
	}
	return json.Marshal(p)
}

// TrashEntry is a deleted file kept under <data>/trash.
type TrashEntry struct {
	ID           string `db:"id" json:"id"`
	LibraryID    int64  `db:"library_id" json:"libraryId"`
	OriginalPath string `db:"original_path" json:"originalPath"` // library-relative
	TrashPath    string `db:"trash_path" json:"trashPath"`       // relative to <data>/trash
	Size         int64  `db:"size" json:"size"`
	Title        string `db:"title" json:"title"`
	Artist       string `db:"artist" json:"artist"`
	Album        string `db:"album" json:"album"`
	TrackID      string `db:"track_id" json:"trackId"`
	DeletedBy    string `db:"deleted_by" json:"deletedBy"`
	DeletedAt    int64  `db:"deleted_at" json:"deletedAt"`
}

// Session is a web login session; the plain token is only known to the client.
type Session struct {
	TokenHash  string `db:"token_hash" json:"-"`
	UserID     string `db:"user_id" json:"userId"`
	UserAgent  string `db:"user_agent" json:"userAgent"`
	IP         string `db:"ip" json:"ip"`
	CreatedAt  int64  `db:"created_at" json:"createdAt"`
	LastSeenAt int64  `db:"last_seen_at" json:"lastSeenAt"`
	ExpiresAt  int64  `db:"expires_at" json:"expiresAt"`
}

// Settings are the runtime-editable server settings (stored in the settings table as JSON
// values merged over DefaultSettings).
type Settings struct {
	ScanInterval          string `json:"scanInterval"`      // Go duration, "0" = disabled
	GenreSeparators       string `json:"genreSeparators"`   // runes that split multi-genre tags
	IgnoredArticles       string `json:"ignoredArticles"`   // space separated
	CoverArtFiles         string `json:"coverArtFiles"`     // comma separated glob patterns, by priority
	TranscodeFormat       string `json:"transcodeFormat"`   // mp3|opus|aac
	TranscodeBitrate      int    `json:"transcodeBitrate"`  // kbps
	RenamePattern         string `json:"renamePattern"`     // see docs/architecture/contract.md §7.6
	FixEncodingOnScan     bool   `json:"fixEncodingOnScan"` // repair GBK/Big5/SJIS mojibake when reading
	EnableDownloads       bool   `json:"enableDownloads"`
	OnlineMetadata        bool   `json:"onlineMetadata"`        // allow managers to search online catalogues (outbound requests)
	OnlineMetadataChinaIP bool   `json:"onlineMetadataChinaIp"` // send a mainland-China X-Real-IP to the Chinese catalogues
	YtdlpEnabled          bool   `json:"ytdlpEnabled"`          // allow managers to download audio from YouTube / bilibili with yt-dlp (outbound requests)
	LxSourcesEnabled      bool   `json:"lxSourcesEnabled"`      // allow lx-music source scripts and online music downloads (outbound requests)
	LxSourceMode          string `json:"lxSourceMode"`          // "auto" (try enabled sources by priority) | "fixed" (only LxSourceID)
	LxSourceID            string `json:"lxSourceId"`            // the source used in "fixed" mode
}

// Values of Settings.LxSourceMode.
const (
	LxSourceModeAuto  = "auto"
	LxSourceModeFixed = "fixed"
)

// LxSource is an imported lx-music custom source script (table lx_sources). The native API
// shape is lxmusic.SourceInfo; the script itself is never returned.
type LxSource struct {
	ID               string `db:"id"`
	Name             string `db:"name"`
	Description      string `db:"description"`
	Version          string `db:"version"`
	Author           string `db:"author"`
	Homepage         string `db:"homepage"`
	SourceURL        string `db:"source_url"`
	Script           string `db:"script"` // empty in list queries
	ScriptHash       string `db:"script_hash"`
	ScriptSize       int    `db:"script_size"` // computed: length of script in bytes
	Enabled          bool   `db:"enabled"`
	Position         int    `db:"position"`
	AllowUpdateAlert bool   `db:"allow_update_alert"`
	Platforms        string `db:"platforms"` // JSON object platform → qualities
	LastError        string `db:"last_error"`
	LoadedAt         int64  `db:"loaded_at"`
	UpdateLog        string `db:"update_log"`
	UpdateURL        string `db:"update_url"`
	UpdateAt         int64  `db:"update_at"`
	CreatedAt        int64  `db:"created_at"`
	UpdatedAt        int64  `db:"updated_at"`
}

// DefaultSettings returns the settings used when nothing is stored. scanInterval is the
// RAINY_SCAN_INTERVAL value.
func DefaultSettings(scanInterval time.Duration) Settings {
	return Settings{
		ScanInterval:          FormatDuration(scanInterval),
		GenreSeparators:       ";/,",
		IgnoredArticles:       "The El La Los Las Le Les",
		CoverArtFiles:         "cover.*,folder.*,front.*,album.*,albumart*.*",
		TranscodeFormat:       "mp3",
		TranscodeBitrate:      192,
		RenamePattern:         "{albumartist}/{album}/[{disc}-]{track:2} {title}",
		FixEncodingOnScan:     true,
		EnableDownloads:       true,
		OnlineMetadata:        false,
		OnlineMetadataChinaIP: false,
		YtdlpEnabled:          false,
		LxSourcesEnabled:      false,
		LxSourceMode:          LxSourceModeAuto,
		LxSourceID:            "",
	}
}

// ScanIntervalDuration parses ScanInterval ("0"/"" or invalid → 0 = disabled).
func (s Settings) ScanIntervalDuration() time.Duration {
	if s.ScanInterval == "" || s.ScanInterval == "0" {
		return 0
	}
	d, err := time.ParseDuration(s.ScanInterval)
	if err != nil || d < 0 {
		return 0
	}
	return d
}

// IgnoredArticleList splits IgnoredArticles on whitespace.
func (s Settings) IgnoredArticleList() []string { return strings.Fields(s.IgnoredArticles) }

// CoverArtPatterns splits CoverArtFiles on commas (trimmed, empty entries dropped).
func (s Settings) CoverArtPatterns() []string {
	var out []string
	for _, p := range strings.Split(s.CoverArtFiles, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// FormatDuration renders d compactly: 0 → "0", 1h → "1h", 90m → "1h30m", 45s → "45s".
func FormatDuration(d time.Duration) string {
	if d == 0 {
		return "0"
	}
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = s[:len(s)-2]
	}
	if strings.HasSuffix(s, "h0m") {
		s = s[:len(s)-2]
	}
	return s
}
