package api

import (
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"rainy/internal/auth"
	"rainy/internal/ytdlp"
)

// countingWeb fails every outbound request and counts them.
type countingWeb struct{ calls int }

func (c *countingWeb) RoundTrip(*http.Request) (*http.Response, error) {
	c.calls++
	return nil, errors.New("offline test")
}

const cookieValue = "synthetic-sessdata-value"

func TestYtdlpEndpoints(t *testing.T) {
	e := newMgEnv(t)
	web := &countingWeb{}
	svc := ytdlp.New(ytdlp.Options{
		Dir: filepath.Join(e.app.Cfg.DataDir, "ytdlp"), TmpDir: e.app.Cfg.TmpDir(),
		Cipher: auth.NewCrypto([]byte("synthetic test key")), Client: &http.Client{Transport: web},
	})
	t.Cleanup(svc.Close)
	e.app.Ytdlp = svc
	e.app.Manage.SetDownloader(svc)
	link := "https://www.youtube.com/watch?v=synthetic01"

	status := mgDecode[downloadsStatus](t, e.do("GET", "/manage/downloads", e.mgrTok, nil), http.StatusOK)
	if status.Enabled || status.Ready || len(status.Sites) != 2 || status.Jobs == nil || len(status.Jobs) != 0 {
		t.Fatalf("initial status %+v", status)
	}

	// Off by default: nothing that would contact another service runs.
	for _, rt := range [][3]string{
		{"POST", "/manage/downloads", e.mgrTok}, {"POST", "/admin/ytdlp/check", e.adminTok}, {"POST", "/admin/ytdlp/install", e.adminTok},
	} {
		body := map[string]any{"url": link, "libraryId": 1}
		if code := mgCode(t, e.do(rt[0], rt[1], rt[2], body), http.StatusForbidden); code != CodeForbidden {
			t.Fatalf("%s %s while disabled: %s", rt[0], rt[1], code)
		}
	}
	if web.calls != 0 {
		t.Fatalf("%d outbound requests while disabled", web.calls)
	}

	// Only an administrator can turn it on.
	if rec := e.do("PUT", "/admin/settings", e.mgrTok, map[string]any{"ytdlpEnabled": true}); rec.Code != http.StatusForbidden {
		t.Fatalf("manager enabling: %d", rec.Code)
	}
	if rec := e.do("PUT", "/admin/settings", e.adminTok, map[string]any{"ytdlpEnabled": true}); rec.Code != http.StatusOK {
		t.Fatalf("admin enabling: %d %s", rec.Code, rec.Body)
	}

	if code := mgCode(t, e.do("POST", "/manage/downloads", e.mgrTok, map[string]any{"url": "https://evil.example.com/x", "libraryId": 1}), http.StatusBadRequest); code != CodeBadRequest {
		t.Fatalf("foreign host: %s", code)
	}
	if code := mgCode(t, e.do("POST", "/manage/downloads", e.mgrTok, map[string]any{"url": link, "libraryId": 1}), http.StatusConflict); code != CodeConflict {
		t.Fatalf("not installed: %s", code)
	}
	// GitHub is unreachable in the test: a clean 503, and the request did go out now.
	if code := mgCode(t, e.do("POST", "/admin/ytdlp/check", e.adminTok, nil), http.StatusServiceUnavailable); code != CodeUnavailable {
		t.Fatalf("check offline: %s", code)
	}
	if web.calls == 0 {
		t.Fatal("the update check sent nothing")
	}

	// Cookies are write-only.
	text := "# Netscape HTTP Cookie File\n.bilibili.com\tTRUE\t/\tFALSE\t0\tSESSDATA\t" + cookieValue + "\n" +
		".mail.example.com\tTRUE\t/\tTRUE\t0\tSID\tsynthetic-other\n"
	if rec := e.do("PUT", "/admin/ytdlp/cookies/bilibili", e.mgrTok, map[string]string{"text": text}); rec.Code != http.StatusForbidden {
		t.Fatalf("manager setting cookies: %d", rec.Code)
	}
	rec := e.do("PUT", "/admin/ytdlp/cookies/bilibili", e.adminTok, map[string]string{"text": text})
	saved := mgDecode[ytdlp.CookieSaveResult](t, rec, http.StatusOK)
	if saved.Count != 1 || saved.Dropped != 1 || !saved.SignedIn || strings.Contains(rec.Body.String(), cookieValue) {
		t.Fatalf("save %s", rec.Body)
	}
	for _, rt := range [][2]string{{"/admin/ytdlp", e.adminTok}, {"/manage/downloads", e.mgrTok}, {"/admin/settings", e.adminTok}, {"/admin/system", e.adminTok}} {
		rec := e.do("GET", rt[0], rt[1], nil)
		if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), cookieValue) || strings.Contains(rec.Body.String(), "SESSDATA") {
			t.Fatalf("GET %s leaks cookies or failed: %d %s", rt[0], rec.Code, rec.Body)
		}
	}
	info := mgDecode[ytdlpInfo](t, e.do("GET", "/admin/ytdlp", e.adminTok, nil), http.StatusOK)
	if !info.Enabled || !info.Managed || len(info.Cookies) != 2 || !info.Cookies[1].Configured || info.Cookies[0].Configured {
		t.Fatalf("admin info %+v", info)
	}
	status = mgDecode[downloadsStatus](t, e.do("GET", "/manage/downloads", e.mgrTok, nil), http.StatusOK)
	if !status.Enabled || status.Sites[1].ID != "bilibili" || !status.Sites[1].Cookies || status.Sites[0].Cookies {
		t.Fatalf("manager status %+v", status)
	}

	if code := mgCode(t, e.do("PUT", "/admin/ytdlp/cookies/bilibili", e.adminTok, map[string]string{"text": "[{}]"}), http.StatusBadRequest); code != CodeBadRequest {
		t.Fatalf("JSON export: %s", code)
	}
	if rec := e.do("PUT", "/admin/ytdlp/cookies/example", e.adminTok, map[string]string{"text": text}); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown site: %d", rec.Code)
	}
	if rec := e.do("DELETE", "/admin/ytdlp/cookies/bilibili", e.adminTok, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
	if svc.HasCookies("bilibili") {
		t.Fatal("cookies still stored after delete")
	}
	if rec := e.do("DELETE", "/manage/downloads/unknown", e.mgrTok, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("remove unknown job: %d", rec.Code)
	}
}
