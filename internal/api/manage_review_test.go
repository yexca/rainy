package api

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"rainy/internal/manage"
	"rainy/internal/model"
)

// An explicit empty id list must purge nothing (only an omitted list empties the trash).
func TestManagePurgeEmptyListIsNoOp(t *testing.T) {
	e := newMgEnv(t)
	tok := e.mgrTok
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("files", "a.wav")
	_, _ = fw.Write(mgWAV())
	_ = mw.WriteField("libraryId", "1")
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/manage/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	up := mgDecode[manage.BatchResult](t, rec, 200)
	if len(up.Updated) != 1 {
		t.Fatalf("upload %+v", up)
	}
	del := mgDecode[manage.BatchResult](t, e.do("POST", "/manage/delete", tok, map[string]any{"trackIds": []string{up.Updated[0].ID}}), 200)
	if len(del.Updated) != 1 || len(del.Errors) != 0 {
		t.Fatalf("delete %+v", del)
	}
	if got := mgDecode[map[string]int](t, e.do("POST", "/manage/trash/purge", tok, map[string]any{"ids": []string{}}), 200); got["purged"] != 0 {
		t.Fatalf("[] purged %v", got)
	}
	if got := mgDecode[map[string]int](t, e.do("POST", "/manage/missing/purge", tok, map[string]any{"trackIds": []string{}}), 200); got["purged"] != 0 {
		t.Fatalf("[] purged %v", got)
	}
	if trash := mgDecode[[]model.TrashEntry](t, e.do("GET", "/manage/trash", tok, nil), 200); len(trash) != 1 {
		t.Fatalf("trash %v", trash)
	}
	if got := mgDecode[map[string]int](t, e.do("POST", "/manage/trash/purge", tok, map[string]any{}), 200); got["purged"] != 1 {
		t.Fatalf("omitted ids must empty the trash: %v", got)
	}
}

// Library roots are compared after making them absolute: a library stored with a
// relative path must still block a nested library.
func TestAdminLibraryOverlapWithRelativePath(t *testing.T) {
	e := newMgEnv(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(wd, root)
	if err != nil {
		t.Skipf("no relative path to the temp dir: %v", err)
	}
	if err := e.app.Store.CreateLibrary(context.Background(), &model.Library{Name: "rel", Path: rel}); err != nil {
		t.Fatal(err)
	}
	rec := e.do("POST", "/admin/libraries", e.adminTok, map[string]string{"path": filepath.Join(root, "sub")})
	if rec.Code != http.StatusConflict {
		t.Fatalf("nested library accepted: %d %s", rec.Code, rec.Body)
	}
}
