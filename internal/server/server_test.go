package server

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"rainy/internal/app"
	"rainy/internal/config"
)

func newApp(t *testing.T) *app.App {
	t.Helper()
	cfg := config.Default()
	cfg.DataDir = filepath.Join(t.TempDir(), "data")
	cfg.MusicDir = t.TempDir()
	cfg.DevCORS = "http://localhost:5173"
	a, err := app.New(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

func get(t *testing.T, h http.Handler, path string, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest("GET", path, nil)
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

var testDist = fstest.MapFS{
	"dist/index.html":             {Data: []byte("<!doctype html><title>Rainy</title><div id=root></div>")},
	"dist/assets/index-abc123.js": {Data: []byte(strings.Repeat("console.log('rainy');\n", 200))},
	"dist/sw.js":                  {Data: []byte("self.addEventListener('fetch', () => {})")},
	"dist/manifest.webmanifest":   {Data: []byte(`{"name":"Rainy"}`)},
	"dist/favicon.svg":            {Data: []byte("<svg/>")},
	"dist/.gitkeep":               {Data: nil},
}

func TestSPA(t *testing.T) {
	h := newSPA(testDist)
	cases := []struct {
		path, status, cache, ctype, bodyHas string
	}{
		{"/", "200", cacheNoCache, "text/html", "<div id=root>"},
		{"/index.html", "200", cacheNoCache, "text/html", "<div id=root>"},
		{"/albums/abc", "200", cacheNoCache, "text/html", "<div id=root>"},    // client route
		{"/manage/doctor", "200", cacheNoCache, "text/html", "<div id=root>"}, // nested client route
		{"/assets/index-abc123.js", "200", cacheImmutable, "text/javascript", "console.log"},
		{"/sw.js", "200", cacheNoCache, "text/javascript", "addEventListener"},
		{"/manifest.webmanifest", "200", cacheNoCache, "application/manifest+json", "Rainy"},
		{"/favicon.svg", "200", cacheDefault, "image/svg+xml", "<svg/>"},
		{"/assets/missing.js", "404", "no-store", "", ""},
		{"/missing.png", "404", "no-store", "", ""},
		{"/../../etc/passwd", "200", cacheNoCache, "text/html", "<div id=root>"}, // cleaned, never escapes
	}
	for _, c := range cases {
		w := get(t, h, c.path, nil)
		if got := w.Code; http.StatusText(got) == "" || itoa(got) != c.status {
			t.Errorf("%s: status %d, want %s", c.path, got, c.status)
			continue
		}
		if cc := w.Header().Get("Cache-Control"); cc != c.cache {
			t.Errorf("%s: Cache-Control %q, want %q", c.path, cc, c.cache)
		}
		if c.ctype != "" && !strings.HasPrefix(w.Header().Get("Content-Type"), c.ctype) {
			t.Errorf("%s: Content-Type %q, want %q", c.path, w.Header().Get("Content-Type"), c.ctype)
		}
		if !strings.Contains(w.Body.String(), c.bodyHas) {
			t.Errorf("%s: body %q lacks %q", c.path, w.Body.String(), c.bodyHas)
		}
	}
	// ETag revalidation.
	w := get(t, h, "/", nil)
	etag := w.Header().Get("ETag")
	if etag == "" || w.Header().Get("Last-Modified") != "" {
		t.Fatalf("etag %q", etag)
	}
	if w := get(t, h, "/", map[string]string{"If-None-Match": etag}); w.Code != http.StatusNotModified {
		t.Fatalf("revalidation: %d", w.Code)
	}
	r := httptest.NewRequest("POST", "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatal(rec.Code)
	}
}

func itoa(i int) string { b, _ := json.Marshal(i); return string(b) }

func TestSPANotBuilt(t *testing.T) {
	h := newSPA(fstest.MapFS{"dist/.gitkeep": {}})
	w := get(t, h, "/albums", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "not built") || w.Header().Get("Cache-Control") != cacheNoCache {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}

func TestHandler(t *testing.T) {
	a := newApp(t)
	h := Handler(a)

	w := get(t, h, "/api/health", nil)
	if w.Code != 200 || strings.TrimSpace(w.Body.String()) != `{"status":"ok"}` || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("health: %d %q", w.Code, w.Body)
	}
	w = get(t, h, "/api/nope", nil)
	if w.Code != 404 || !strings.Contains(w.Body.String(), `"code":"not_found"`) {
		t.Fatalf("api 404: %d %s", w.Code, w.Body)
	}
	for _, p := range []string{"/api/manage/anything", "/api/admin/users"} {
		if w := get(t, h, p, nil); w.Code != 401 || !strings.Contains(w.Body.String(), `"unauthorized"`) {
			t.Fatalf("%s: %d %s", p, w.Code, w.Body)
		}
	}
	r := httptest.NewRequest("DELETE", "/api/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != 405 || !strings.Contains(rec.Body.String(), "method_not_allowed") {
		t.Fatalf("405: %d %s", rec.Code, rec.Body)
	}

	// Subsonic stub: valid envelopes + CORS.
	w = get(t, h, "/rest/ping.view?f=json", nil)
	if w.Header().Get("Access-Control-Allow-Origin") != "*" || !strings.Contains(w.Body.String(), `"subsonic-response"`) ||
		!strings.Contains(w.Body.String(), `"status":"failed"`) {
		t.Fatalf("rest json: %s", w.Body)
	}
	w = get(t, h, "/rest/ping", nil)
	if !strings.Contains(w.Body.String(), `<subsonic-response xmlns="http://subsonic.org/restapi" status="failed" version="1.16.1"`) {
		t.Fatalf("rest xml: %s", w.Body)
	}
	pre := httptest.NewRequest("OPTIONS", "/rest/ping", nil)
	pre.Header.Set("Origin", "http://example.com")
	pre.Header.Set("Access-Control-Request-Method", "POST")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, pre)
	if rec.Code != 204 || !strings.Contains(rec.Header().Get("Access-Control-Allow-Methods"), "POST") {
		t.Fatalf("preflight %d %v", rec.Code, rec.Header())
	}

	// Dev CORS for /api only for the configured origin.
	w = get(t, h, "/api/health", map[string]string{"Origin": "http://localhost:5173"})
	if w.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" || w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("dev cors: %v", w.Header())
	}
	w = get(t, h, "/api/health", map[string]string{"Origin": "http://evil.example"})
	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("foreign origin allowed")
	}

	// SPA through the real router (web.Dist: built or not, "/" is HTML).
	w = get(t, h, "/", nil)
	if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("spa: %d %v", w.Code, w.Header())
	}
}

func TestCompress(t *testing.T) {
	big := strings.Repeat(`{"k":"value"},`, 500)
	mux := http.NewServeMux()
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "["+big+"{}]")
	})
	mux.HandleFunc("/small", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", "2")
		_, _ = io.WriteString(w, "{}")
	})
	mux.HandleFunc("/audio", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = io.WriteString(w, big)
	})
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: x\n\n")
		w.(http.Flusher).Flush()
	})
	mux.HandleFunc("/nocontent", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
	})
	h := compress(mux)
	gz := map[string]string{"Accept-Encoding": "gzip, deflate, br"}

	w := get(t, h, "/json", gz)
	if w.Header().Get("Content-Encoding") != "gzip" || !strings.Contains(w.Header().Get("Vary"), "Accept-Encoding") {
		t.Fatalf("json not gzipped: %v", w.Header())
	}
	zr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(zr)
	if !json.Valid(body) {
		t.Fatalf("bad gzip body %q", body[:20])
	}
	if w := get(t, h, "/json", nil); w.Header().Get("Content-Encoding") != "" {
		t.Fatal("gzip without Accept-Encoding")
	}
	if w := get(t, h, "/json", map[string]string{"Accept-Encoding": "gzip;q=0"}); w.Header().Get("Content-Encoding") != "" {
		t.Fatal("gzip with q=0")
	}
	for _, p := range []string{"/small", "/audio", "/sse", "/nocontent"} {
		w := get(t, h, p, gz)
		if w.Header().Get("Content-Encoding") != "" {
			t.Errorf("%s must not be gzipped", p)
		}
	}
	if w := get(t, h, "/sse", gz); w.Body.String() != "data: x\n\n" || !w.Flushed {
		t.Fatalf("sse passthrough: %q flushed=%v", w.Body, w.Flushed)
	}
}

func TestRecoverer(t *testing.T) {
	h := recoverer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { panic("boom") }))
	w := get(t, h, "/", nil)
	if w.Code != 500 || !strings.Contains(w.Body.String(), `"internal"`) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	defer func() {
		if recover() != http.ErrAbortHandler {
			t.Fatal("ErrAbortHandler must propagate")
		}
	}()
	recoverer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { panic(http.ErrAbortHandler) })).
		ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
}

func TestServeGracefulShutdown(t *testing.T) {
	a := newApp(t)
	srv := New(a)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- ServeListener(ctx, srv, ln, 5*time.Second) }()
	resp, err := http.Get("http://" + ln.Addr().String() + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatal(resp.StatusCode)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("shutdown timed out")
	}
}
