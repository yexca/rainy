package api

// Shared fixture for the native API tests of the library/media/auth areas (owner: C).
// Names are prefixed with "native" to stay clear of other areas' test helpers in this
// package.

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rainy/internal/app"
	"rainy/internal/config"
	"rainy/internal/model"
	"rainy/internal/util"
)

// nativeEnv is a fully wired app with users and a small library on disk.
type nativeEnv struct {
	t     *testing.T
	app   *app.App
	h     http.Handler
	music string
	lib   *model.Library

	admin, alice, bob              *model.User // admin | can download | no download
	adminTok, aliceTok, bobTok     string
	blueSkies, tones, hits         string // album ids
	rainyDays, sineWave, guest, va string // artist ids
	// Tracks by key: one, umbrella, puddles (Blue Skies, CD1/CD1/CD2), gone (missing),
	// tone (WAV with .lrc), feature (Hits, by The Rainy Days), guestTrack (Hits, by Guest).
	tr map[string]model.Track
}

// newNativeApp creates an app with an empty data dir and an (empty) music folder that is
// registered as library #1. No users exist.
func newNativeApp(t *testing.T) (*app.App, string) {
	t.Helper()
	music := filepath.Join(t.TempDir(), "music")
	if err := os.MkdirAll(music, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.DataDir = filepath.Join(t.TempDir(), "data")
	cfg.MusicDir = music
	a, err := app.New(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a, music
}

// nativeRandBytes returns n deterministic pseudo-random bytes.
func nativeRandBytes(seed uint64, n int) []byte {
	r := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(r.UintN(256))
	}
	return b
}

// nativeWAV returns a mono 16-bit PCM WAV sine tone.
func nativeWAV(seconds float64) []byte {
	const rate = 22050
	n := int(seconds * rate)
	var buf bytes.Buffer
	le := binary.LittleEndian
	buf.WriteString("RIFF")
	_ = binary.Write(&buf, le, uint32(36+n*2))
	buf.WriteString("WAVEfmt ")
	for _, v := range []any{uint32(16), uint16(1), uint16(1), uint32(rate), uint32(rate * 2), uint16(2), uint16(16)} {
		_ = binary.Write(&buf, le, v)
	}
	buf.WriteString("data")
	_ = binary.Write(&buf, le, uint32(n*2))
	for i := range n {
		_ = binary.Write(&buf, le, int16(6000*math.Sin(2*math.Pi*330*float64(i)/rate)))
	}
	return buf.Bytes()
}

// nativePNG is a valid 1×1 PNG.
var nativePNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0xf8, 0xcf, 0xc0, 0xf0,
	0x1f, 0x00, 0x05, 0x00, 0x01, 0xff, 0x89, 0x99, 0x3d, 0x1d, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45,
	0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

// nativeToneLRC is the sidecar of the "tone" track.
const nativeToneLRC = "[ti:Tone]\n[00:00.50]first line\n[00:01.50][00:02.50]echo\n"

func newNativeEnv(t *testing.T) *nativeEnv {
	t.Helper()
	a, music := newNativeApp(t)
	ctx := context.Background()
	e := &nativeEnv{t: t, app: a, h: New(a).Routes(), music: music, tr: map[string]model.Track{}}

	libs, err := a.Store.ListLibraries(ctx)
	if err != nil || len(libs) != 1 {
		t.Fatalf("libraries: %v %v", libs, err)
	}
	e.lib = &libs[0]

	mkUser := func(name string, admin, manage, download bool) (*model.User, string) {
		u := &model.User{Username: name, IsAdmin: admin, CanManage: manage, CanDownload: download}
		if err := a.Auth.CreateUser(ctx, u, "secret-"+name); err != nil {
			t.Fatal(err)
		}
		tok, err := a.Auth.CreateSession(ctx, u.ID, "test", "127.0.0.1")
		if err != nil {
			t.Fatal(err)
		}
		return u, tok
	}
	e.admin, e.adminTok = mkUser("admin", true, true, true)
	e.alice, e.aliceTok = mkUser("alice", false, false, true)
	e.bob, e.bobTok = mkUser("bob", false, false, false)

	e.rainyDays, e.sineWave = util.ArtistID("The Rainy Days"), util.ArtistID("Sine Wave")
	e.guest, e.va = util.ArtistID("Guest Singer"), util.ArtistID("Various Artists")
	e.blueSkies = util.AlbumID("The Rainy Days", "Blue Skies")
	e.tones = util.AlbumID("Sine Wave", "Tones")
	e.hits = util.AlbumID("Various Artists", "Hits")

	type spec struct {
		key, path, title, artist, albumArtist, album, genre string
		track, disc, year                                   int
		data                                                []byte
		missing, compilation, lrc                           bool
		lyrics                                              string
	}
	specs := []spec{
		{key: "one", path: "The Rainy Days/Blue Skies/CD1/01 Rain One.mp3", title: "Rain One", artist: "The Rainy Days", album: "Blue Skies", genre: "Pop", track: 1, disc: 1, year: 2020, data: nativeRandBytes(1, 64<<10), lyrics: "plain embedded words\nsecond line"},
		{key: "umbrella", path: "The Rainy Days/Blue Skies/CD1/02 Umbrella.mp3", title: "Umbrella", artist: "The Rainy Days", album: "Blue Skies", genre: "Pop; Rock", track: 2, disc: 1, year: 2020, data: nativeRandBytes(2, 32<<10)},
		{key: "puddles", path: "The Rainy Days/Blue Skies/CD2/01 Puddles.mp3", title: "Puddles", artist: "The Rainy Days", album: "Blue Skies", genre: "Rock", track: 1, disc: 2, year: 2020, data: nativeRandBytes(3, 16<<10)},
		{key: "gone", path: "The Rainy Days/Blue Skies/CD1/03 Gone.mp3", title: "Gone Missing", artist: "The Rainy Days", album: "Blue Skies", genre: "Pop", track: 3, disc: 1, year: 2020, missing: true},
		{key: "tone", path: "Sine Wave/Tones/01 Tone.wav", title: "Tone", artist: "Sine Wave", album: "Tones", genre: "Electronic", track: 1, disc: 1, year: 2019, data: nativeWAV(3), lrc: true},
		{key: "feature", path: "Various Artists/Hits/01 Feature.mp3", title: "Feature", artist: "The Rainy Days", albumArtist: "Various Artists", album: "Hits", genre: "Pop", track: 1, disc: 1, year: 2024, data: nativeRandBytes(4, 8<<10), compilation: true},
		{key: "guestTrack", path: "Various Artists/Hits/02 Guest.mp3", title: "Guest Appearance", artist: "Guest Singer", albumArtist: "Various Artists", album: "Hits", genre: "Pop", track: 2, disc: 1, year: 2024, data: nativeRandBytes(5, 8<<10), compilation: true},
	}
	var tracks []model.Track
	for i, s := range specs {
		if s.albumArtist == "" {
			s.albumArtist = s.artist
		}
		abs := filepath.Join(music, filepath.FromSlash(s.path))
		if s.data != nil {
			if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(abs, s.data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if s.lrc {
			if err := os.WriteFile(strings.TrimSuffix(abs, ".wav")+".lrc", []byte(nativeToneLRC), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		tr := model.Track{
			ID: "trk-" + s.key, LibraryID: e.lib.ID, Path: s.path, Size: int64(len(s.data)), Mtime: util.NowMs(),
			Title: s.title, Album: s.album, Artist: s.artist, AlbumArtist: s.albumArtist,
			AlbumID: util.AlbumID(s.albumArtist, s.album), ArtistID: util.ArtistID(s.artist), AlbumArtistID: util.ArtistID(s.albumArtist),
			TrackNumber: s.track, DiscNumber: s.disc, Year: s.year, Genre: s.genre, Lyrics: s.lyrics, HasLrc: s.lrc,
			Compilation: s.compilation, Missing: s.missing, Duration: 180, Bitrate: 320, Codec: "MPEG",
			CreatedAt: int64(1_700_000_000_000 + i*1000),
		}
		if s.key == "tone" {
			tr.Duration, tr.Bitrate, tr.Codec, tr.BitDepth, tr.SampleRate, tr.Channels = 3, 352, "PCM", 16, 22050, 1
		}
		tracks = append(tracks, tr)
	}
	if err := a.Store.UpsertTracks(ctx, tracks); err != nil {
		t.Fatal(err)
	}
	if err := a.Store.RefreshAll(ctx); err != nil {
		t.Fatal(err)
	}
	cover := filepath.Join(music, "The Rainy Days", "Blue Skies", "cover.png")
	if err := os.WriteFile(cover, nativePNG, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.Store.SetAlbumCoverPath(ctx, e.blueSkies, cover); err != nil {
		t.Fatal(err)
	}
	for _, s := range specs {
		tr, err := a.Store.GetTrack(ctx, "trk-"+s.key, "")
		if err != nil {
			t.Fatal(err)
		}
		e.tr[s.key] = *tr
	}
	return e
}

// id returns the id of the fixture track key.
func (e *nativeEnv) id(key string) string {
	tr, ok := e.tr[key]
	if !ok {
		e.t.Fatalf("no fixture track %q", key)
	}
	return tr.ID
}

// nativeRequest builds a request; body may be nil, a string (sent as-is) or a value
// (JSON-encoded). token "" sends no credentials.
func nativeRequest(t *testing.T, method, target, token string, body any) *http.Request {
	t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		rd = strings.NewReader(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			t.Fatal(err)
		}
		rd = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, target, rd)
	if rd != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

// do serves a request through the /api router.
func (e *nativeEnv) do(method, target, token string, body any) *httptest.ResponseRecorder {
	return nativeServe(e.t, e.h, nativeRequest(e.t, method, target, token, body))
}

func nativeServe(t *testing.T, h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// nativeExpect fails the test unless rec has the wanted status.
func nativeExpect(t *testing.T, rec *httptest.ResponseRecorder, status int) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status %d, want %d; body: %s", rec.Code, status, rec.Body.String())
	}
}

// nativeJSON decodes rec's body into a T, after checking the status.
func nativeJSON[T any](t *testing.T, rec *httptest.ResponseRecorder, status int) T {
	t.Helper()
	nativeExpect(t, rec, status)
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decoding %s: %v", rec.Body.String(), err)
	}
	return v
}

// nativeErrorCode returns the error code of an error response.
func nativeErrorCode(t *testing.T, rec *httptest.ResponseRecorder, status int) string {
	t.Helper()
	body := nativeJSON[struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}](t, rec, status)
	if body.Error.Message == "" {
		t.Errorf("error without message: %s", rec.Body.String())
	}
	return body.Error.Code
}

// nativeTrackIDs returns the ids of tracks.
func nativeTrackIDs(ts []model.Track) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.ID
	}
	return out
}

// nativeAlbumIDs returns the ids of albums.
func nativeAlbumIDs(as []model.Album) []string {
	out := make([]string, len(as))
	for i, a := range as {
		out[i] = a.ID
	}
	return out
}
