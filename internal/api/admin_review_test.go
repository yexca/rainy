package api

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Library changes must not hang while the library lock is held (a long scan or file
// operation): they give up with 409 conflict.
func TestAdminLibraryChangesDoNotHangOnLibraryLock(t *testing.T) {
	e := newMgEnv(t)
	e.waitScan()
	old := adminLockTimeout
	adminLockTimeout = 200 * time.Millisecond
	t.Cleanup(func() { adminLockTimeout = old })

	other := filepath.Join(t.TempDir(), "other")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	unlock := e.app.Scanner.LockLibrary()
	type result struct {
		name string
		rec  *httptest.ResponseRecorder
	}
	results := make(chan result, 2)
	go func() {
		results <- result{"update path", e.do("PUT", "/admin/libraries/1", e.adminTok, map[string]string{"path": other})}
	}()
	go func() { results <- result{"delete", e.do("DELETE", "/admin/libraries/1", e.adminTok, nil)} }()
	deadline := time.After(5 * time.Second)
	for range 2 {
		select {
		case r := <-results:
			if r.rec.Code != http.StatusConflict {
				t.Errorf("%s while the library is locked: %d %s", r.name, r.rec.Code, r.rec.Body)
			}
		case <-deadline:
			unlock()
			t.Fatal("library change blocked on the library lock")
		}
	}
	unlock()
	if lib, err := e.app.Store.GetLibrary(t.Context(), 1); err != nil || lib.Path == other {
		t.Fatalf("library changed while locked: %+v %v", lib, err)
	}
	// Renaming only does not need the lock.
	if rec := e.do("PUT", "/admin/libraries/1", e.adminTok, map[string]string{"name": "Renamed"}); rec.Code != http.StatusOK {
		t.Fatalf("rename: %d %s", rec.Code, rec.Body)
	}
}

// The whole upload request is capped (413), not only each file.
func TestManageUploadTotalLimit(t *testing.T) {
	e := newMgEnv(t)
	old := maxUploadRequest
	maxUploadRequest = 64 << 10
	t.Cleanup(func() { maxUploadRequest = old })
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("libraryId", "1")
	for i := range 6 { // 6 × ~16 KiB
		fw, _ := mw.CreateFormFile("files", fmt.Sprintf("t%d.wav", i))
		_, _ = fw.Write(mgWAV())
	}
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/manage/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+e.mgrTok)
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	entries, _ := os.ReadDir(e.app.Cfg.TmpDir())
	if len(entries) != 0 {
		t.Fatalf("staged files left behind: %d", len(entries))
	}
	matches, _ := filepath.Glob(filepath.Join(e.music, "t*.wav"))
	if len(matches) != 0 {
		t.Fatalf("files of a rejected upload were placed: %v", matches)
	}
}

// Embedded pictures come from untrusted files: never serve them as anything but an image
// (an HTML "picture" would run in the app's origin).
func TestPictureContentType(t *testing.T) {
	for in, want := range map[string]string{
		"image/jpeg":               "image/jpeg",
		"image/png":                "image/png",
		"text/html; charset=utf-8": "application/octet-stream",
		"text/xml; charset=utf-8":  "application/octet-stream",
		"application/pdf":          "application/octet-stream",
		"image/svg+xml":            "application/octet-stream",
	} {
		if got := pictureContentType(in); got != want {
			t.Errorf("%s → %s, want %s", in, got, want)
		}
	}
}
