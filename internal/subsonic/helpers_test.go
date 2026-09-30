package subsonic

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"flag"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"rainy/internal/app"
	"rainy/internal/artwork"
	"rainy/internal/auth"
	"rainy/internal/config"
	"rainy/internal/db/dbtest"
	"rainy/internal/events"
	"rainy/internal/model"
	"rainy/internal/nowplaying"
	"rainy/internal/scanner"
	"rainy/internal/scrobble"
	"rainy/internal/store"
	"rainy/internal/transcode"
	"rainy/internal/util"
)

var update = flag.Bool("update", false, "rewrite golden files")

// Fixed fixture timestamps (unix ms) so snapshots are stable.
const (
	tsCreated = int64(1_700_000_000_000) // 2023-11-14T22:13:20Z
	tsUpdated = int64(1_700_000_100_000)
	tsStarred = int64(1_700_000_200_000)
)

// fixture is a test server with a small library.
type fixture struct {
	t        *testing.T
	app      *app.App
	api      *API
	handler  http.Handler
	music    string
	lib1     int64
	lib2     int64
	admin    *model.User
	alice    *model.User
	bob      *model.User
	fileData []byte // content of track1's file
}

// Artist / album ids used by the fixtures.
var (
	artistRainy    = util.ArtistID("The Rainy Days")
	artistNorthern = util.ArtistID("Northern Echo")
	artistStatic   = util.ArtistID("Static Noise")
	albumWet       = util.AlbumID("The Rainy Days", "Wet Streets")
	albumSkies     = util.AlbumID("Northern Echo", "Skies")
	albumFar       = util.AlbumID("Static Noise", "Far")
)

func f64(v float64) *float64 { return &v }

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	d := dbtest.New(t)
	st := store.New(d)
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	cfg.MusicDir = t.TempDir()
	cfg.FFmpegPath = filepath.Join(t.TempDir(), "no-ffmpeg-here")
	bus := events.NewBus()
	key := bytes.Repeat([]byte{7}, 32)
	a := &app.App{
		Cfg:        cfg,
		DB:         d,
		Store:      st,
		Auth:       auth.NewService(st, auth.NewCrypto(key), time.Hour),
		Bus:        bus,
		NowPlaying: nowplaying.New(),
		Scanner:    scanner.New(st, bus, cfg),
		Artwork:    artwork.New(st, filepath.Join(cfg.DataDir, "artwork")),
		Transcoder: transcode.New(cfg.FFmpegPath),
		StartedAt:  time.Now(),
	}
	a.Scrobble = scrobble.New(scrobble.Options{Store: st, Cipher: auth.NewCrypto(key), Settings: a.Settings, NoWorker: true})
	t.Cleanup(a.Scrobble.Close)
	f := &fixture{t: t, app: a, music: cfg.MusicDir}
	f.api = New(a)
	f.handler = f.api.Routes()

	lib1 := &model.Library{Name: "Music", Path: cfg.MusicDir}
	lib2 := &model.Library{Name: "Other", Path: t.TempDir()}
	for _, l := range []*model.Library{lib1, lib2} {
		if err := st.CreateLibrary(ctx, l); err != nil {
			t.Fatal(err)
		}
	}
	f.lib1, f.lib2 = lib1.ID, lib2.ID

	f.admin = f.createUser("admin", "adminpass", true)
	f.alice = f.createUser("alice", "alicepass", false)
	f.bob = f.createUser("bob", "bobpass", false)

	// A real file for track1 (content is irrelevant to raw streaming).
	f.fileData = make([]byte, 4096)
	for i := range f.fileData {
		f.fileData[i] = byte(i * 7)
	}
	rel := "The Rainy Days/Wet Streets/01 Rain Song.mp3"
	abs, err := util.SafeJoin(cfg.MusicDir, rel)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, f.fileData, 0o644); err != nil {
		t.Fatal(err)
	}

	tracks := []model.Track{
		{
			ID: "track1", LibraryID: lib1.ID, Path: rel, Size: int64(len(f.fileData)), Mtime: tsCreated,
			Title: "Rain Song", Album: "Wet Streets", Artist: "The Rainy Days", AlbumArtist: "The Rainy Days",
			AlbumID: albumWet, ArtistID: artistRainy, AlbumArtistID: artistRainy,
			TrackNumber: 1, TrackTotal: 2, DiscNumber: 1, DiscTotal: 1, DiscSubtitle: "Morning",
			Year: 2020, Date: "2020-05-01", Genre: "Rock; Indie", Composer: "J. Doe", Comment: "first",
			Lyrics: "[00:01.00]Hello rain\n[00:03.50]Goodbye sun", BPM: 120,
			Duration: 185.4, Bitrate: 320, SampleRate: 44100, BitDepth: 0, Channels: 2, Codec: "mp3",
			RGTrackGain: f64(-6.5), RGTrackPeak: f64(0.98),
			MbzTrackID: "mbz-track-1", SortTitle: "rain song", CreatedAt: tsCreated, UpdatedAt: tsUpdated,
		},
		{
			ID: "track2", LibraryID: lib1.ID, Path: "The Rainy Days/Wet Streets/02 Puddles.mp3", Size: 5000, Mtime: tsCreated,
			Title: "Puddles", Album: "Wet Streets", Artist: "The Rainy Days", AlbumArtist: "The Rainy Days",
			AlbumID: albumWet, ArtistID: artistRainy, AlbumArtistID: artistRainy,
			TrackNumber: 2, TrackTotal: 2, DiscNumber: 1, DiscTotal: 1, DiscSubtitle: "Morning",
			Year: 2020, Genre: "Rock", Duration: 200.6, Bitrate: 256, SampleRate: 44100, Channels: 2, Codec: "mp3",
			CreatedAt: tsCreated, UpdatedAt: tsUpdated,
		},
		{
			ID: "track3", LibraryID: lib1.ID, Path: "Northern Echo/Skies/01 Clouds.flac", Size: 30000, Mtime: tsCreated,
			Title: "Clouds", Album: "Skies", Artist: "Northern Echo", AlbumArtist: "Northern Echo",
			AlbumID: albumSkies, ArtistID: artistNorthern, AlbumArtistID: artistNorthern,
			TrackNumber: 1, DiscNumber: 1, Year: 2018, Genre: "Ambient; Indie",
			Duration: 301, Bitrate: 1411, SampleRate: 96000, BitDepth: 24, Channels: 2, Codec: "flac",
			CreatedAt: tsCreated + 1000, UpdatedAt: tsUpdated,
		},
		{
			ID: "track4", LibraryID: lib2.ID, Path: "Static Noise/Far/01 Elsewhere.ogg", Size: 9000, Mtime: tsCreated,
			Title: "Elsewhere", Album: "Far", Artist: "Static Noise", AlbumArtist: "Static Noise",
			AlbumID: albumFar, ArtistID: artistStatic, AlbumArtistID: artistStatic,
			TrackNumber: 1, DiscNumber: 1, Year: 2022, Genre: "Electronic",
			Duration: 99.5, Bitrate: 160, SampleRate: 48000, Channels: 2, Codec: "vorbis",
			CreatedAt: tsCreated + 2000, UpdatedAt: tsUpdated,
		},
	}
	if err := st.UpsertTracks(ctx, tracks); err != nil {
		t.Fatal(err)
	}
	if err := st.RefreshAll(ctx); err != nil {
		t.Fatal(err)
	}
	f.exec(`UPDATE albums SET updated_at = ?`, tsUpdated)
	f.exec(`UPDATE artists SET updated_at = ?, created_at = ?`, tsUpdated, tsCreated)
	f.exec(`UPDATE libraries SET created_at = ?, updated_at = ?, last_scan_at = ?`, tsCreated, tsCreated, tsUpdated)
	return f
}

func (f *fixture) exec(query string, args ...any) {
	f.t.Helper()
	if _, err := f.app.DB.W.Exec(query, args...); err != nil {
		f.t.Fatalf("%s: %v", query, err)
	}
}

func (f *fixture) createUser(name, pass string, admin bool) *model.User {
	f.t.Helper()
	u := &model.User{Username: name, IsAdmin: admin, CanDownload: true}
	if err := f.app.Auth.CreateUser(context.Background(), u, pass); err != nil {
		f.t.Fatal(err)
	}
	return u
}

// seedAnnotations stars and rates items for alice and creates her playlist.
func (f *fixture) seedAnnotations() {
	f.t.Helper()
	ctx := context.Background()
	st := f.app.Store
	must := func(err error) {
		f.t.Helper()
		if err != nil {
			f.t.Fatal(err)
		}
	}
	must(st.SetStarred(ctx, f.alice.ID, "track", []string{"track1"}, true))
	must(st.SetStarred(ctx, f.alice.ID, "album", []string{albumWet}, true))
	must(st.SetStarred(ctx, f.alice.ID, "artist", []string{artistNorthern}, true))
	must(st.SetRating(ctx, f.alice.ID, "album", albumSkies, 4))
	must(st.SetRating(ctx, f.alice.ID, "track", "track2", 5))
	f.exec(`UPDATE annotations SET starred_at = ? WHERE starred_at IS NOT NULL`, tsStarred)
	pl := &model.Playlist{ID: "playlist1", Name: "Rainy Mix", Comment: "for grey days", OwnerID: f.alice.ID}
	must(st.CreatePlaylist(ctx, pl, []string{"track2", "track1", "track3"}))
	f.exec(`UPDATE playlists SET created_at = ?, updated_at = ?`, tsCreated, tsUpdated)
}

// creds returns password-auth parameters.
func creds(user, pass string) url.Values {
	return url.Values{"u": {user}, "p": {pass}, "v": {"1.16.1"}, "c": {"test"}}
}

func (f *fixture) aliceParams() url.Values { return creds("alice", "alicepass") }
func (f *fixture) adminParams() url.Values { return creds("admin", "adminpass") }

// merge returns base plus extra key/value pairs.
func merge(base url.Values, kv ...string) url.Values {
	out := url.Values{}
	for k, v := range base {
		out[k] = append([]string(nil), v...)
	}
	for i := 0; i+1 < len(kv); i += 2 {
		out.Add(kv[i], kv[i+1])
	}
	return out
}

// get performs GET /<method>?params.
func (f *fixture) get(method string, params url.Values) *httptest.ResponseRecorder {
	f.t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/"+method+"?"+params.Encode(), nil)
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	return rec
}

// post performs a form POST.
func (f *fixture) post(method string, params url.Values) *httptest.ResponseRecorder {
	f.t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/"+method, strings.NewReader(params.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, req)
	return rec
}

// jsonEnvelope is a decoded JSON response.
type jsonEnvelope = map[string]any

// callJSON performs a JSON call and returns the inner subsonic-response object.
func (f *fixture) callJSON(method string, params url.Values, kv ...string) jsonEnvelope {
	f.t.Helper()
	rec := f.get(method, merge(params, append([]string{"f", "json"}, kv...)...))
	var outer map[string]jsonEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &outer); err != nil {
		f.t.Fatalf("%s: invalid JSON %q: %v", method, rec.Body.String(), err)
	}
	env, ok := outer["subsonic-response"]
	if !ok {
		f.t.Fatalf("%s: no subsonic-response in %s", method, rec.Body.String())
	}
	return env
}

// ok asserts a successful response and returns it.
func (f *fixture) ok(method string, params url.Values, kv ...string) jsonEnvelope {
	f.t.Helper()
	env := f.callJSON(method, params, kv...)
	if env["status"] != "ok" {
		f.t.Fatalf("%s: expected ok, got %v", method, env["error"])
	}
	return env
}

// failCode asserts a failed response and returns its error code.
func (f *fixture) failCode(method string, params url.Values, kv ...string) int {
	f.t.Helper()
	env := f.callJSON(method, params, kv...)
	if env["status"] != "failed" {
		f.t.Fatalf("%s: expected failure, got %v", method, env)
	}
	e, _ := env["error"].(map[string]any)
	code, _ := e["code"].(float64)
	return int(code)
}

// path walks a decoded JSON value by keys / indexes.
func jpath(t *testing.T, v any, keys ...any) any {
	t.Helper()
	for _, k := range keys {
		switch k := k.(type) {
		case string:
			m, ok := v.(map[string]any)
			if !ok {
				t.Fatalf("path %v: %T is not an object", keys, v)
			}
			v = m[k]
		case int:
			s, ok := v.([]any)
			if !ok || k >= len(s) {
				t.Fatalf("path %v: index %d out of %T %v", keys, k, v, v)
			}
			v = s[k]
		}
	}
	return v
}

func length(t *testing.T, v any) int {
	t.Helper()
	if v == nil {
		return 0
	}
	s, ok := v.([]any)
	if !ok {
		t.Fatalf("%T is not an array", v)
	}
	return len(s)
}

func md5hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// golden compares got with testdata/<name>, rewriting it with -update.
func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	p := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("reading golden %s (run go test -update): %v", p, err)
	}
	if !bytes.Equal(bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n")), got) {
		t.Errorf("%s mismatch\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

// prettyJSON indents a JSON body.
func prettyJSON(t *testing.T, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := json.Indent(&buf, body, "", "  "); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, body)
	}
	buf.WriteByte('\n')
	return buf.Bytes()
}

var (
	openTagRe  = regexp.MustCompile(`><([^/])`)
	closeTagRe = regexp.MustCompile(`(</[^>]+>)<`)
)

// prettyXML validates an XML body and starts every element on its own line.
func prettyXML(t *testing.T, body []byte) []byte {
	t.Helper()
	dec := xml.NewDecoder(bytes.NewReader(body))
	for {
		if _, err := dec.Token(); err != nil {
			if err.Error() == "EOF" {
				break
			}
			t.Fatalf("invalid XML: %v\n%s", err, body)
		}
	}
	out := openTagRe.ReplaceAll(body, []byte(">\n<$1"))
	out = closeTagRe.ReplaceAll(out, []byte("$1\n<"))
	return append(out, '\n')
}
