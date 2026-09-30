package api

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"rainy/internal/app"
	"rainy/internal/config"
	"rainy/internal/manage"
	"rainy/internal/model"
)

// mgEnv is an app with three users (admin, manager, listener) and bearer tokens.
type mgEnv struct {
	t                *testing.T
	app              *app.App
	h                http.Handler
	music            string
	admin            *model.User
	adminTok, mgrTok string
	userTok          string
	listener         *model.User
}

func newMgEnv(t *testing.T) *mgEnv {
	t.Helper()
	ctx := context.Background()
	music := filepath.Join(t.TempDir(), "music")
	if err := os.MkdirAll(filepath.Join(music, "Artist", "Album"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.DataDir = filepath.Join(t.TempDir(), "data")
	cfg.MusicDir = music
	a, err := app.New(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// Let background scans started by the handlers finish before closing the DB.
		for i := 0; i < 100 && a.Scanner.Status().Scanning; i++ {
			time.Sleep(50 * time.Millisecond)
		}
		_ = a.Close()
	})
	e := &mgEnv{t: t, app: a, h: New(a).Routes(), music: music}
	mk := func(name string, admin, manager bool) (*model.User, string) {
		u := &model.User{Username: name, IsAdmin: admin, CanManage: manager, CanDownload: true}
		if err := a.Auth.CreateUser(ctx, u, "secret-pass"); err != nil {
			t.Fatal(err)
		}
		tok, err := a.Auth.CreateSession(ctx, u.ID, "test", "127.0.0.1")
		if err != nil {
			t.Fatal(err)
		}
		return u, tok
	}
	e.admin, e.adminTok = mk("admin", true, false)
	_, e.mgrTok = mk("manager", false, true)
	e.listener, e.userTok = mk("listener", false, false)
	return e
}

// waitScan waits for a background scan to finish.
func (e *mgEnv) waitScan() {
	for i := 0; i < 200 && e.app.Scanner.Status().Scanning; i++ {
		time.Sleep(25 * time.Millisecond)
	}
}

// mgWAV returns a short silent 16-bit mono PCM WAV file.
func mgWAV() []byte {
	const rate, samples = 8000, 8000
	var b bytes.Buffer
	le := func(v any) { _ = binary.Write(&b, binary.LittleEndian, v) }
	b.WriteString("RIFF")
	le(uint32(36 + samples*2))
	b.WriteString("WAVEfmt ")
	le(uint32(16))
	le(uint16(1))
	le(uint16(1))
	le(uint32(rate))
	le(uint32(rate * 2))
	le(uint16(2))
	le(uint16(16))
	b.WriteString("data")
	le(uint32(samples * 2))
	b.Write(make([]byte, samples*2))
	return b.Bytes()
}

func (e *mgEnv) do(method, target, token string, body any) *httptest.ResponseRecorder {
	e.t.Helper()
	var rd *bytes.Reader
	ctype := ""
	switch b := body.(type) {
	case nil:
		rd = bytes.NewReader(nil)
	case []byte:
		rd = bytes.NewReader(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			e.t.Fatal(err)
		}
		rd = bytes.NewReader(raw)
		ctype = "application/json"
	}
	req := httptest.NewRequest(method, target, rd)
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return rec
}

func mgDecode[T any](t *testing.T, rec *httptest.ResponseRecorder, status int) T {
	t.Helper()
	var v T
	if rec.Code != status {
		t.Fatalf("status %d, want %d: %s", rec.Code, status, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decoding %q: %v", rec.Body.String(), err)
	}
	return v
}

func mgCode(t *testing.T, rec *httptest.ResponseRecorder, status int) string {
	t.Helper()
	body := mgDecode[struct {
		Error struct{ Code, Message string } `json:"error"`
	}](t, rec, status)
	return body.Error.Code
}

func TestManageAndAdminAuthz(t *testing.T) {
	e := newMgEnv(t)
	manageRoutes := [][2]string{
		{"GET", "/manage/tracks/x/tags"}, {"POST", "/manage/tags"}, {"POST", "/manage/tags/rebuild"}, {"GET", "/manage/tracks/x/picture"},
		{"POST", "/manage/cover"}, {"DELETE", "/manage/cover"}, {"PUT", "/manage/tracks/x/lyrics"},
		{"POST", "/manage/rename/preview"}, {"POST", "/manage/rename"}, {"POST", "/manage/upload"},
		{"POST", "/manage/delete"}, {"GET", "/manage/trash"}, {"POST", "/manage/trash/restore"},
		{"POST", "/manage/trash/purge"}, {"POST", "/manage/missing/purge"}, {"GET", "/manage/folders"},
		{"POST", "/manage/folders/rescan"}, {"GET", "/manage/issues/summary"}, {"GET", "/manage/issues"},
		{"POST", "/manage/encoding"}, {"GET", "/manage/log"}, {"GET", "/manage/unknown"},
		{"GET", "/manage/metadata"}, {"GET", "/manage/metadata/search"}, {"GET", "/manage/metadata/lyrics"},
		{"GET", "/manage/metadata/cover"}, {"GET", "/manage/downloads"}, {"POST", "/manage/downloads"},
		{"DELETE", "/manage/downloads/x"},
	}
	for _, rt := range manageRoutes {
		if rec := e.do(rt[0], rt[1], "", nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("anonymous %s %s: %d", rt[0], rt[1], rec.Code)
		}
		if rec := e.do(rt[0], rt[1], e.userTok, nil); rec.Code != http.StatusForbidden {
			t.Errorf("listener %s %s: %d", rt[0], rt[1], rec.Code)
		}
	}
	adminRoutes := [][2]string{
		{"GET", "/admin/users"}, {"POST", "/admin/users"}, {"PUT", "/admin/users/x"}, {"DELETE", "/admin/users/x"},
		{"GET", "/admin/libraries"}, {"POST", "/admin/libraries"}, {"PUT", "/admin/libraries/1"},
		{"DELETE", "/admin/libraries/1"}, {"GET", "/admin/scan"}, {"POST", "/admin/scan"},
		{"GET", "/admin/settings"}, {"PUT", "/admin/settings"}, {"GET", "/admin/stats"},
		{"GET", "/admin/system"}, {"POST", "/admin/cache/clear"}, {"GET", "/admin/ytdlp"},
		{"POST", "/admin/ytdlp/check"}, {"POST", "/admin/ytdlp/install"}, {"PUT", "/admin/ytdlp/cookies/youtube"},
		{"DELETE", "/admin/ytdlp/cookies/youtube"},
	}
	for _, rt := range adminRoutes {
		for _, tok := range []string{e.userTok, e.mgrTok} {
			if rec := e.do(rt[0], rt[1], tok, nil); rec.Code != http.StatusForbidden {
				t.Errorf("non-admin %s %s: %d", rt[0], rt[1], rec.Code)
			}
		}
	}
	// Managers and admins get through.
	for _, tok := range []string{e.mgrTok, e.adminTok} {
		if rec := e.do("GET", "/manage/log", tok, nil); rec.Code != http.StatusOK {
			t.Errorf("manage log: %d %s", rec.Code, rec.Body)
		}
	}
}

func TestManageReadonlyErrorMapping(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/manage/tags", nil)
	writeManageErr(rec, req, fmt.Errorf("saving: %w", &manage.ReadonlyError{Path: "a.flac", Err: syscall.EROFS}))
	body := mgDecode[struct {
		Error struct{ Code, Message string } `json:"error"`
	}](t, rec, http.StatusConflict)
	if body.Error.Code != CodeReadonly || !strings.Contains(body.Error.Message, "PUID") {
		t.Fatalf("%+v", body)
	}
	rec = httptest.NewRecorder()
	writeManageErr(rec, req, errors.New("secret"))
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("internal: %d %s", rec.Code, rec.Body)
	}
}

func TestManageEndpoints(t *testing.T) {
	e := newMgEnv(t)
	tok := e.mgrTok
	if code := mgCode(t, e.do("POST", "/manage/tags", tok, map[string]any{"edits": []any{}}), 400); code != CodeBadRequest {
		t.Fatal(code)
	}
	if code := mgCode(t, e.do("POST", "/manage/tags/rebuild", tok, map[string]any{"trackIds": []string{}}), 400); code != CodeBadRequest {
		t.Fatal(code)
	}
	if code := mgCode(t, e.do("POST", "/manage/tags", tok, []byte("{")), 400); code != CodeBadRequest {
		t.Fatal(code)
	}
	if code := mgCode(t, e.do("POST", "/manage/rename/preview", tok, map[string]any{"trackIds": []string{"x"}, "pattern": "{nope}"}), 400); code != CodeBadRequest {
		t.Fatal(code)
	}
	if code := mgCode(t, e.do("GET", "/manage/tracks/nope/tags", tok, nil), 404); code != CodeNotFound {
		t.Fatal(code)
	}
	if code := mgCode(t, e.do("PUT", "/manage/tracks/nope/lyrics", tok, map[string]string{"text": "", "target": "lrc"}), 404); code != CodeNotFound {
		t.Fatal(code)
	}
	sum := mgDecode[map[string]int](t, e.do("GET", "/manage/issues/summary", tok, nil), 200)
	if len(sum) != 5 {
		t.Fatalf("summary %v", sum)
	}
	page := mgDecode[Page[manage.Issue]](t, e.do("GET", "/manage/issues?type=missing_tags", tok, nil), 200)
	if page.Items == nil {
		t.Fatal("items must be []")
	}
	if code := mgCode(t, e.do("GET", "/manage/issues?type=zzz", tok, nil), 400); code != CodeBadRequest {
		t.Fatal(code)
	}
	if trash := mgDecode[[]model.TrashEntry](t, e.do("GET", "/manage/trash", tok, nil), 200); trash == nil || len(trash) != 0 {
		t.Fatalf("trash %v", trash)
	}
	if got := mgDecode[map[string]int](t, e.do("POST", "/manage/trash/purge", tok, nil), 200); got["purged"] != 0 {
		t.Fatal(got)
	}
	listing := mgDecode[manage.FolderListing](t, e.do("GET", "/manage/folders?libraryId=1", tok, nil), 200)
	if len(listing.Folders) != 1 || listing.Folders[0].Name != "Artist" || !listing.Writable {
		t.Fatalf("listing %+v", listing)
	}
	if code := mgCode(t, e.do("GET", "/manage/folders?libraryId=1&dir=../..", tok, nil), 400); code != CodeBadRequest {
		t.Fatal(code)
	}
	if rec := e.do("POST", "/manage/folders/rescan", tok, map[string]any{"libraryId": 1, "dir": "Artist"}); rec.Code != http.StatusAccepted {
		t.Fatalf("rescan %d %s", rec.Code, rec.Body)
	}
	if rec := e.do("POST", "/manage/cover", tok, map[string]any{}); rec.Code != http.StatusBadRequest {
		t.Fatalf("cover without multipart %d", rec.Code)
	}

	// Upload through HTTP (multipart streaming).
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("files", "new.wav")
	_, _ = fw.Write(mgWAV())
	fw, _ = mw.CreateFormFile("files", "junk.mp3")
	_, _ = fw.Write([]byte("ID3 not really audio"))
	_ = mw.WriteField("libraryId", "1")
	_ = mw.WriteField("dir", "Artist/Album")
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/manage/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	res := mgDecode[manage.BatchResult](t, rec, 200)
	if len(res.Errors) != 1 || res.Errors[0].Path != "junk.mp3" || len(res.Updated) != 1 || res.Updated[0].Path != "Artist/Album/new.wav" {
		t.Fatalf("upload %+v", res)
	}
	if _, err := os.Stat(filepath.Join(e.music, "Artist", "Album", "new.wav")); err != nil {
		t.Fatal("uploaded file missing")
	}
	if _, err := os.Stat(filepath.Join(e.music, "Artist", "Album", "junk.mp3")); err == nil {
		t.Fatal("invalid audio must not be placed in the library")
	}
	logPage := mgDecode[Page[json.RawMessage]](t, e.do("GET", "/manage/log?limit=5", tok, nil), 200)
	if logPage.Total != 1 {
		t.Fatalf("log %+v", logPage)
	}
}

func TestAdminUsers(t *testing.T) {
	e := newMgEnv(t)
	tok := e.adminTok
	created := mgDecode[model.User](t, e.do("POST", "/admin/users", tok, map[string]any{
		"username": "bob", "password": "hunter22", "displayName": "Bob", "email": "bob@example.com", "canManage": true}), 201)
	if !created.CanManage || created.IsAdmin || !created.CanDownload || created.DisplayName != "Bob" {
		t.Fatalf("created %+v", created)
	}
	if code := mgCode(t, e.do("POST", "/admin/users", tok, map[string]any{"username": "BOB", "password": "hunter22"}), 409); code != CodeConflict {
		t.Fatal(code)
	}
	for _, body := range []map[string]any{
		{"username": "x"}, {"username": "ok", "password": "1"}, {"username": "bad name", "password": "hunter22"},
		{"username": "ok2", "password": "hunter22", "email": "nope"},
	} {
		if rec := e.do("POST", "/admin/users", tok, body); rec.Code != http.StatusBadRequest {
			t.Errorf("%v: %d %s", body, rec.Code, rec.Body)
		}
	}
	users := mgDecode[[]model.User](t, e.do("GET", "/admin/users", tok, nil), 200)
	if len(users) != 4 {
		t.Fatalf("users %d", len(users))
	}

	// Self-protection.
	if code := mgCode(t, e.do("PUT", "/admin/users/"+e.admin.ID, tok, map[string]any{"isAdmin": false}), 403); code != CodeForbidden {
		t.Fatal(code)
	}
	if code := mgCode(t, e.do("DELETE", "/admin/users/"+e.admin.ID, tok, nil), 403); code != CodeForbidden {
		t.Fatal(code)
	}

	// Promote bob, then bob demotes the original admin: bob remains the only admin.
	up := mgDecode[model.User](t, e.do("PUT", "/admin/users/"+created.ID, tok, map[string]any{"isAdmin": true, "displayName": "Robert"}), 200)
	if !up.IsAdmin || up.DisplayName != "Robert" || up.Email != "bob@example.com" {
		t.Fatalf("updated %+v", up)
	}
	bobTok, err := e.app.Auth.CreateSession(context.Background(), created.ID, "t", "")
	if err != nil {
		t.Fatal(err)
	}
	mgDecode[model.User](t, e.do("PUT", "/admin/users/"+e.admin.ID, bobTok, map[string]any{"isAdmin": false}), 200)

	// Password reset by an admin revokes the target's sessions.
	mgDecode[model.User](t, e.do("PUT", "/admin/users/"+e.listener.ID, bobTok, map[string]any{"password": "new-password"}), 200)
	if rec := e.do("GET", "/manage/log", e.userTok, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("old session still valid: %d", rec.Code)
	}
	if u, _ := e.app.Store.GetUser(context.Background(), e.listener.ID); !e.app.Auth.CheckPassword(u, "new-password") {
		t.Fatal("password not changed")
	}
	if rec := e.do("PUT", "/admin/users/"+e.listener.ID, bobTok, map[string]any{"password": "1"}); rec.Code != http.StatusBadRequest {
		t.Fatalf("weak password %d", rec.Code)
	}

	if rec := e.do("DELETE", "/admin/users/"+e.admin.ID, bobTok, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete %d %s", rec.Code, rec.Body)
	}
	if rec := e.do("DELETE", "/admin/users/nope", bobTok, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("delete unknown %d", rec.Code)
	}
	var admins int
	for _, u := range mgDecode[[]model.User](t, e.do("GET", "/admin/users", bobTok, nil), 200) {
		if u.IsAdmin {
			admins++
		}
	}
	if admins != 1 {
		t.Fatalf("admins %d", admins)
	}
}

func TestAdminLibraries(t *testing.T) {
	e := newMgEnv(t)
	tok := e.adminTok
	libs := mgDecode[[]LibraryInfo](t, e.do("GET", "/admin/libraries", tok, nil), 200)
	if len(libs) != 1 || !libs[0].Exists || !libs[0].Writable || libs[0].Path != e.music {
		t.Fatalf("libs %+v", libs)
	}
	if code := mgCode(t, e.do("POST", "/admin/libraries", tok, map[string]string{"path": filepath.Join(e.music, "nope")}), 400); code != CodeBadRequest {
		t.Fatal(code)
	}
	if code := mgCode(t, e.do("POST", "/admin/libraries", tok, map[string]string{"path": filepath.Join(e.music, "Artist")}), 409); code != CodeConflict {
		t.Fatal(code)
	}
	if code := mgCode(t, e.do("POST", "/admin/libraries", tok, map[string]string{"path": filepath.Dir(e.music)}), 409); code != CodeConflict {
		t.Fatal(code)
	}
	if code := mgCode(t, e.do("POST", "/admin/libraries", tok, map[string]string{"path": e.app.Cfg.DataDir}), 400); code != CodeBadRequest {
		t.Fatal(code)
	}
	other := t.TempDir()
	lib := mgDecode[LibraryInfo](t, e.do("POST", "/admin/libraries", tok, map[string]string{"name": " Podcasts ", "path": other}), 201)
	if lib.Name != "Podcasts" || !lib.Exists || lib.ID == 0 {
		t.Fatalf("created %+v", lib)
	}
	upd := mgDecode[LibraryInfo](t, e.do("PUT", fmt.Sprintf("/admin/libraries/%d", lib.ID), tok, map[string]string{"name": "Talk"}), 200)
	if upd.Name != "Talk" || upd.Path != lib.Path {
		t.Fatalf("updated %+v", upd)
	}
	if rec := e.do("PUT", fmt.Sprintf("/admin/libraries/%d", lib.ID), tok, map[string]string{"path": e.music}); rec.Code != http.StatusConflict {
		t.Fatalf("path taken: %d", rec.Code)
	}
	e.waitScan()
	if rec := e.do("DELETE", fmt.Sprintf("/admin/libraries/%d", lib.ID), tok, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete %d", rec.Code)
	}
	if rec := e.do("DELETE", "/admin/libraries/999", tok, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("delete unknown %d", rec.Code)
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatal("deleting a library must not touch files")
	}
}

func TestAdminSettingsAndSystem(t *testing.T) {
	e := newMgEnv(t)
	tok := e.adminTok
	for _, body := range []map[string]any{
		{"transcodeBitrate": 500}, {"transcodeBitrate": 16}, {"transcodeFormat": "flac"}, {"scanInterval": "soon"},
		{"scanInterval": "10s"}, {"renamePattern": "{bogus}"}, {"coverArtFiles": "a/b.jpg"}, {"coverArtFiles": " , "},
		{"whatever": 1}, {"enableDownloads": "yes"},
	} {
		if rec := e.do("PUT", "/admin/settings", tok, body); rec.Code != http.StatusBadRequest {
			t.Errorf("%v: %d %s", body, rec.Code, rec.Body)
		}
	}
	got := mgDecode[model.Settings](t, e.do("PUT", "/admin/settings", tok, map[string]any{
		"scanInterval": "90m", "transcodeBitrate": 256, "transcodeFormat": "OPUS", "enableDownloads": false}), 200)
	if got.ScanInterval != "1h30m" || got.TranscodeBitrate != 256 || got.TranscodeFormat != "opus" || got.EnableDownloads || got.RenamePattern == "" {
		t.Fatalf("settings %+v", got)
	}
	if again := mgDecode[model.Settings](t, e.do("GET", "/admin/settings", tok, nil), 200); again != got {
		t.Fatalf("not persisted: %+v", again)
	}
	if got := mgDecode[model.Settings](t, e.do("PUT", "/admin/settings", tok, map[string]any{"scanInterval": "0"}), 200); got.ScanInterval != "0" {
		t.Fatalf("disable scans: %+v", got)
	}

	sys := mgDecode[SystemInfo](t, e.do("GET", "/admin/system", tok, nil), 200)
	if sys.Version == "" || sys.GoVersion == "" || sys.DBSize == 0 || len(sys.Libraries) != 1 || sys.FFmpeg.Path == "" {
		t.Fatalf("system %+v", sys)
	}
	mgDecode[map[string]any](t, e.do("GET", "/admin/stats", tok, nil), 200)
	st := mgDecode[map[string]any](t, e.do("GET", "/admin/scan", tok, nil), 200)
	if _, ok := st["phase"]; !ok {
		t.Fatalf("scan status %v", st)
	}
	if rec := e.do("POST", "/admin/scan", tok, map[string]any{"libraryId": 99}); rec.Code != http.StatusNotFound {
		t.Fatalf("scan unknown library %d", rec.Code)
	}
	if rec := e.do("POST", "/admin/cache/clear", tok, nil); rec.Code != http.StatusOK {
		t.Fatalf("cache clear %d %s", rec.Code, rec.Body)
	}
}
