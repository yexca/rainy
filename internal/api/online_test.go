package api

import (
	"net/http"
	"strings"
	"testing"

	"rainy/internal/lxmusic"
)

const testSourceScript = `/**
 * @name Synthetic source
 * @version 0.1.0
 */
const marker = 'synthetic-script-body-marker'
lx.on(lx.EVENT_NAMES.request, () => Promise.resolve('https://cdn.example.com/a.mp3'))
lx.send(lx.EVENT_NAMES.inited, { sources: { kw: { type: 'music', actions: ['musicUrl'], qualitys: ['128k', '320k'] } } })
`

func TestOnlineEndpoints(t *testing.T) {
	e := newMgEnv(t)

	status := mgDecode[onlineStatus](t, e.do("GET", "/manage/online", e.mgrTok, nil), http.StatusOK)
	if status.Enabled || status.Sources != 0 || len(status.Platforms) != 5 || status.Platforms[0].Qualities == nil {
		t.Fatalf("initial status %+v", status)
	}

	// Off by default: nothing that would contact another service runs.
	song := map[string]any{"platform": "kw", "id": "1", "title": "Synthetic"}
	for _, rt := range []struct {
		method, path, tok string
		body              any
	}{
		{"GET", "/manage/online/search?platform=kw&q=rain", e.mgrTok, nil},
		{"GET", "/manage/online/cover?url=https://y.gtimg.cn/music/photo_new/x.jpg", e.mgrTok, nil},
		{"POST", "/manage/online/downloads", e.mgrTok, map[string]any{"songs": []any{song}, "libraryId": 1}},
		{"POST", "/admin/sources", e.adminTok, map[string]any{"url": "https://example.com/source.js"}},
	} {
		if code := mgCode(t, e.do(rt.method, rt.path, rt.tok, rt.body), http.StatusForbidden); code != CodeForbidden {
			t.Fatalf("%s %s while disabled: %s", rt.method, rt.path, code)
		}
	}
	if rec := e.do("GET", "/admin/sources", e.mgrTok, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("manager listing sources: %d", rec.Code)
	}

	// A script can be imported while the feature is off; it is not started.
	rec := e.do("POST", "/admin/sources", e.adminTok, map[string]any{"script": testSourceScript})
	src := mgDecode[lxmusic.SourceInfo](t, rec, http.StatusCreated)
	if src.Name != "Synthetic source" || src.Status != lxmusic.StatusIdle || src.Size != len(testSourceScript) || strings.Contains(rec.Body.String(), "marker") {
		t.Fatalf("import %s", rec.Body)
	}
	if code := mgCode(t, e.do("POST", "/admin/sources", e.adminTok, map[string]any{"script": testSourceScript}), http.StatusConflict); code != CodeConflict {
		t.Fatalf("duplicate: %s", code)
	}
	for _, body := range []map[string]any{{}, {"script": "no header"}, {"script": testSourceScript, "url": "https://example.com/x.js"}} {
		if code := mgCode(t, e.do("POST", "/admin/sources", e.adminTok, body), http.StatusBadRequest); code != CodeBadRequest {
			t.Fatalf("import %v: %s", body, code)
		}
	}
	if code := mgCode(t, e.do("POST", "/admin/sources/"+src.ID+"/reload", e.adminTok, nil), http.StatusForbidden); code != CodeForbidden {
		t.Fatalf("reload while disabled: %s", code)
	}

	for _, bad := range []map[string]any{{"lxSourceMode": "random"}, {"lxSourceId": "../x"}} {
		if code := mgCode(t, e.do("PUT", "/admin/settings", e.adminTok, bad), http.StatusBadRequest); code != CodeBadRequest {
			t.Fatalf("settings %v: %s", bad, code)
		}
	}
	if rec := e.do("PUT", "/admin/settings", e.mgrTok, map[string]any{"lxSourcesEnabled": true}); rec.Code != http.StatusForbidden {
		t.Fatalf("manager enabling: %d", rec.Code)
	}
	if rec := e.do("PUT", "/admin/settings", e.adminTok, map[string]any{"lxSourcesEnabled": true, "lxSourceMode": "fixed", "lxSourceId": src.ID}); rec.Code != http.StatusOK {
		t.Fatalf("admin enabling: %d %s", rec.Code, rec.Body)
	}

	src = mgDecode[lxmusic.SourceInfo](t, e.do("POST", "/admin/sources/"+src.ID+"/reload", e.adminTok, nil), http.StatusOK)
	if src.Status != lxmusic.StatusReady || len(src.Platforms) != 1 || src.Platforms[0].Platform != "kw" || src.Error != "" {
		t.Fatalf("reloaded %+v", src)
	}
	rec = e.do("GET", "/admin/sources", e.adminTok, nil)
	list := mgDecode[sourcesInfo](t, rec, http.StatusOK)
	if !list.Enabled || list.Mode != "fixed" || list.SourceID != src.ID || len(list.Sources) != 1 || strings.Contains(rec.Body.String(), "marker") {
		t.Fatalf("list %s", rec.Body)
	}
	status = mgDecode[onlineStatus](t, e.do("GET", "/manage/online", e.mgrTok, nil), http.StatusOK)
	if !status.Enabled || status.Sources != 1 || strings.Join(status.Platforms[0].Qualities, ",") != "128k,320k" {
		t.Fatalf("status %+v", status)
	}

	if code := mgCode(t, e.do("GET", "/manage/online/search?platform=xm&q=rain", e.mgrTok, nil), http.StatusBadRequest); code != CodeBadRequest {
		t.Fatalf("unknown platform: %s", code)
	}
	if code := mgCode(t, e.do("GET", "/manage/online/cover?url=https://evil.example.com/x.jpg", e.mgrTok, nil), http.StatusBadRequest); code != CodeBadRequest {
		t.Fatalf("foreign cover: %s", code)
	}
	if code := mgCode(t, e.do("POST", "/manage/online/downloads", e.mgrTok, map[string]any{"songs": []any{map[string]any{"platform": "kw", "id": "1;x", "title": "t"}}, "libraryId": 1}), http.StatusBadRequest); code != CodeBadRequest {
		t.Fatalf("invalid song: %s", code)
	}
	if code := mgCode(t, e.do("PUT", "/admin/sources/order", e.adminTok, map[string]any{"ids": []string{"unknown"}}), http.StatusBadRequest); code != CodeBadRequest {
		t.Fatalf("bad order: %s", code)
	}

	off := false
	src = mgDecode[lxmusic.SourceInfo](t, e.do("PUT", "/admin/sources/"+src.ID, e.adminTok, map[string]any{"enabled": off}), http.StatusOK)
	if src.Enabled || src.Status != lxmusic.StatusIdle {
		t.Fatalf("disabled %+v", src)
	}
	if status := mgDecode[onlineStatus](t, e.do("GET", "/manage/online", e.mgrTok, nil), http.StatusOK); status.Sources != 0 {
		t.Fatalf("disabled source still usable: %+v", status)
	}
	if rec := e.do("DELETE", "/admin/sources/"+src.ID, e.adminTok, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
	if rec := e.do("PUT", "/admin/sources/"+src.ID, e.adminTok, map[string]any{"enabled": true}); rec.Code != http.StatusNotFound {
		t.Fatalf("update deleted: %d", rec.Code)
	}
}
