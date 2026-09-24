package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"
)

// Cache policies for the embedded single-page app.
const (
	cacheImmutable = "public, max-age=31536000, immutable" // content-hashed files (assets/*)
	cacheNoCache   = "no-cache"                            // entry points that must revalidate
	cacheDefault   = "public, max-age=3600"                // icons, fonts outside assets/, …
)

// noCacheFiles must always be revalidated so new deployments are picked up.
var noCacheFiles = map[string]bool{
	"index.html": true, "sw.js": true, "registerSW.js": true, "manifest.webmanifest": true,
}

// staticExt are extensions for which a missing file is a real 404 (not an SPA route).
var staticExt = map[string]bool{
	".js": true, ".mjs": true, ".css": true, ".map": true, ".json": true, ".webmanifest": true,
	".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".gif": true, ".svg": true, ".ico": true,
	".woff": true, ".woff2": true, ".ttf": true, ".txt": true, ".xml": true, ".wasm": true,
}

// contentTypes are set explicitly (Windows' registry-based mime table is unreliable).
var contentTypes = map[string]string{
	".html":        "text/html; charset=utf-8",
	".js":          "text/javascript; charset=utf-8",
	".mjs":         "text/javascript; charset=utf-8",
	".css":         "text/css; charset=utf-8",
	".json":        "application/json",
	".map":         "application/json",
	".webmanifest": "application/manifest+json",
	".svg":         "image/svg+xml",
	".png":         "image/png",
	".jpg":         "image/jpeg",
	".jpeg":        "image/jpeg",
	".webp":        "image/webp",
	".gif":         "image/gif",
	".ico":         "image/x-icon",
	".woff":        "font/woff",
	".woff2":       "font/woff2",
	".ttf":         "font/ttf",
	".txt":         "text/plain; charset=utf-8",
	".xml":         "application/xml",
	".wasm":        "application/wasm",
}

// spa serves the embedded frontend: exact files when they exist, index.html for client
// routes, or a "frontend not built" page when the build is missing.
type spa struct {
	files    fs.FS
	built    bool
	mu       sync.Mutex
	contents map[string]*spaFile
}

type spaFile struct {
	data []byte
	etag string
}

// newSPA serves dist (an FS whose "dist/" directory holds the Vite build output).
func newSPA(dist fs.FS) *spa {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		sub = dist
	}
	s := &spa{files: sub, contents: map[string]*spaFile{}}
	if fi, err := fs.Stat(sub, "index.html"); err == nil && !fi.IsDir() {
		s.built = true
	} else {
		slog.Warn("frontend not built: web/dist/index.html is missing (run `pnpm build` in web/)")
	}
	return s
}

// load reads and caches a file with its ETag. ok is false for missing files and dirs.
func (s *spa) load(name string) (*spaFile, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f, ok := s.contents[name]; ok {
		return f, true
	}
	fi, err := fs.Stat(s.files, name)
	if err != nil || fi.IsDir() {
		return nil, false // misses are not cached: client routes are unbounded
	}
	data, err := fs.ReadFile(s.files, name)
	if err != nil {
		return nil, false
	}
	sum := sha256.Sum256(data)
	f := &spaFile{data: data, etag: `"` + hex.EncodeToString(sum[:8]) + `"`}
	s.contents[name] = f // embedded files never change
	return f, true
}

func (s *spa) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "same-origin")
	if !s.built {
		h.Set("Cache-Control", cacheNoCache)
		h.Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(notBuiltPage))
		return
	}

	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if name == "" {
		name = "index.html"
	}
	f, ok := s.load(name)
	if !ok {
		if strings.HasPrefix(name, "assets/") || staticExt[strings.ToLower(path.Ext(name))] {
			h.Set("Cache-Control", "no-store")
			http.Error(w, "404 page not found", http.StatusNotFound)
			return
		}
		// Client-side route: serve the app shell.
		name = "index.html"
		f, _ = s.load(name)
		if f == nil {
			http.Error(w, "404 page not found", http.StatusNotFound)
			return
		}
	}

	switch {
	case strings.HasPrefix(name, "assets/"):
		h.Set("Cache-Control", cacheImmutable)
	case noCacheFiles[name]:
		h.Set("Cache-Control", cacheNoCache)
	default:
		h.Set("Cache-Control", cacheDefault)
	}
	if name == "index.html" {
		h.Set("X-Frame-Options", "SAMEORIGIN")
	}
	if ct, ok := contentTypes[strings.ToLower(path.Ext(name))]; ok {
		h.Set("Content-Type", ct)
	}
	h.Set("ETag", f.etag)
	// Zero modtime: no Last-Modified; revalidation uses the content ETag.
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(f.data))
}

const notBuiltPage = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Rainy</title>
<style>
  :root { color-scheme: light dark; }
  body { margin: 0; min-height: 100vh; display: grid; place-items: center;
         font: 15px/1.6 -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif; }
  main { max-width: 34rem; padding: 2rem; }
  h1 { font-size: 1.4rem; margin: 0 0 .5rem; }
  code { background: rgba(127,127,127,.15); padding: .1rem .35rem; border-radius: .3rem; }
  p { opacity: .8; }
</style>
</head>
<body>
<main>
  <h1>Rainy is running, but the web app was not built.</h1>
  <p>Build the frontend and restart the server:</p>
  <p><code>cd web &amp;&amp; pnpm install &amp;&amp; pnpm build</code></p>
  <p>The Subsonic API (<code>/rest</code>) and the JSON API (<code>/api</code>) are available.</p>
</main>
</body>
</html>
`
