package api

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"rainy/internal/metasearch"
)

// metadataWeb stands in for the providers: it answers by host and counts requests, so a
// test can assert that nothing leaves the server while the feature is off.
type metadataWeb struct {
	calls  int
	fail   bool
	realIP string // X-Real-IP of the last request
}

func (m *metadataWeb) RoundTrip(r *http.Request) (*http.Response, error) {
	m.calls++
	m.realIP = r.Header.Get("X-Real-IP")
	if m.fail {
		return nil, errors.New("connection refused")
	}
	rec := httptest.NewRecorder()
	switch r.URL.Host {
	case "search.kuwo.cn":
		_, _ = io.WriteString(rec, `{"abslist":[{"DC_TARGETID":"42","SONGNAME":"Synthetic Song","ARTIST":"Test Artist",
			"ALBUM":"Test Album","DURATION":"120","web_albumpic_short":"120/a/1.jpg"}]}`)
	case "kuwo.cn":
		_, _ = io.WriteString(rec, `{"code":200,"data":{"lrclist":[{"lineLyric":"Line","time":"1"}]}}`)
	case "img2.kuwo.cn":
		_, _ = rec.Write([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR synthetic"))
	default:
		rec.WriteHeader(http.StatusNotFound)
	}
	resp := rec.Result()
	resp.Request = r
	return resp, nil
}

func TestManageMetadata(t *testing.T) {
	e := newMgEnv(t)
	web := &metadataWeb{}
	e.app.Metadata = metasearch.New(&http.Client{Transport: web})

	status := mgDecode[metadataStatus](t, e.do("GET", "/manage/metadata", e.mgrTok, nil), http.StatusOK)
	if status.Enabled || len(status.Providers) != 5 {
		t.Fatalf("status = %+v, want disabled with 5 providers", status)
	}

	search := "/manage/metadata/search?provider=kuwo&q=" + url.QueryEscape("synthetic song")
	cover := "/manage/metadata/cover?url=" + url.QueryEscape("https://img2.kuwo.cn/star/albumcover/500/a/1.jpg")
	lyrics := "/manage/metadata/lyrics?provider=kuwo&id=42"

	// Off by default: every lookup is refused before any outbound request.
	for _, target := range []string{search, cover, lyrics} {
		if code := mgCode(t, e.do("GET", target, e.mgrTok, nil), http.StatusForbidden); code != CodeForbidden {
			t.Fatalf("%s while disabled: code %q", target, code)
		}
	}
	if web.calls != 0 {
		t.Fatalf("%d outbound requests while disabled", web.calls)
	}

	// Only an administrator can turn it on.
	if rec := e.do("PUT", "/admin/settings", e.mgrTok, map[string]any{"onlineMetadata": true}); rec.Code != http.StatusForbidden {
		t.Fatalf("manager enabling: %d", rec.Code)
	}
	if rec := e.do("PUT", "/admin/settings", e.adminTok, map[string]any{"onlineMetadata": true}); rec.Code != http.StatusOK {
		t.Fatalf("admin enabling: %d %s", rec.Code, rec.Body)
	}
	status = mgDecode[metadataStatus](t, e.do("GET", "/manage/metadata", e.mgrTok, nil), http.StatusOK)
	if !status.Enabled {
		t.Fatal("still disabled after enabling")
	}

	res := mgDecode[struct{ Items []metasearch.Result }](t, e.do("GET", search, e.mgrTok, nil), http.StatusOK)
	if len(res.Items) != 1 || res.Items[0].Title != "Synthetic Song" || res.Items[0].Provider != "kuwo" {
		t.Fatalf("search = %+v", res.Items)
	}
	if web.realIP != "" {
		t.Fatalf("X-Real-IP %q sent while onlineMetadataChinaIp is off", web.realIP)
	}
	if rec := e.do("PUT", "/admin/settings", e.adminTok, map[string]any{"onlineMetadataChinaIp": true}); rec.Code != http.StatusOK {
		t.Fatalf("admin enabling China IP: %d %s", rec.Code, rec.Body)
	}
	mgDecode[struct{ Items []metasearch.Result }](t, e.do("GET", search, e.mgrTok, nil), http.StatusOK)
	if web.realIP == "" {
		t.Fatal("no X-Real-IP sent with onlineMetadataChinaIp on")
	}

	l := mgDecode[metasearch.Lyrics](t, e.do("GET", lyrics, e.mgrTok, nil), http.StatusOK)
	if l.Text != "[00:01.00]Line" {
		t.Fatalf("lyrics = %+v", l)
	}

	rec := e.do("GET", cover, e.mgrTok, nil)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" ||
		rec.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(rec.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Fatalf("cover: %d %v", rec.Code, rec.Header())
	}

	// Bad input is a 400 and never reaches the network.
	calls := web.calls
	for _, target := range []string{
		"/manage/metadata/search?provider=nope&q=x",
		"/manage/metadata/search?provider=kuwo&q=",
		"/manage/metadata/cover?url=" + url.QueryEscape("http://192.0.2.1/admin"),
		"/manage/metadata/cover?url=" + url.QueryEscape("https://img2.kuwo.cn.example.com/x.jpg"),
		"/manage/metadata/lyrics?provider=kuwo&id=" + url.QueryEscape("1&x=2"),
	} {
		if code := mgCode(t, e.do("GET", target, e.mgrTok, nil), http.StatusBadRequest); code != CodeBadRequest {
			t.Errorf("%s: code %q", target, code)
		}
	}
	if web.calls != calls {
		t.Fatal("invalid lookups reached the network")
	}

	// A provider outage is a 503 that does not echo the query.
	web.fail = true
	rec = e.do("GET", "/manage/metadata/search?provider=kuwo&q=private-query", e.mgrTok, nil)
	if code := mgCode(t, rec, http.StatusServiceUnavailable); code != CodeUnavailable || strings.Contains(rec.Body.String(), "private-query") {
		t.Fatalf("outage: %s", rec.Body)
	}
}
