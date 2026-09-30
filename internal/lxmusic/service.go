package lxmusic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"rainy/internal/metasearch"
	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/util"
)

// Source management: imported scripts live in the lx_sources table and start on first use
// (an administrator's "reload" starts one at once). Idle scripts are stopped again, and a
// script that failed to start is not retried for a while unless an administrator asks.

const (
	idleUnload   = 15 * time.Minute
	failedRetry  = 2 * time.Minute
	loadTimeout  = 30 * time.Second
	maxSourceURL = 2048
)

// Source statuses (SourceInfo.Status).
const (
	StatusIdle    = "idle"    // not running
	StatusLoading = "loading" // starting
	StatusReady   = "ready"   // running
	StatusError   = "error"   // the last start failed (Error says why)
)

// Selection says which sources may serve a download (from the settings).
type Selection struct {
	Mode     string // model.LxSourceModeAuto | model.LxSourceModeFixed
	SourceID string // the only source in fixed mode
}

// PlatformQualities lists the qualities a source provides for one platform.
type PlatformQualities struct {
	Platform  string   `json:"platform"`
	Qualities []string `json:"qualities"`
}

// SourceInfo is an imported source as the native API returns it (never the script).
type SourceInfo struct {
	ID               string              `json:"id"`
	Name             string              `json:"name"`
	Description      string              `json:"description"`
	Version          string              `json:"version"`
	Author           string              `json:"author"`
	Homepage         string              `json:"homepage"`
	SourceURL        string              `json:"sourceUrl"`
	Size             int                 `json:"size"` // script bytes
	Enabled          bool                `json:"enabled"`
	Position         int                 `json:"position"`
	AllowUpdateAlert bool                `json:"allowUpdateAlert"`
	Platforms        []PlatformQualities `json:"platforms"` // from the last successful start
	Status           string              `json:"status"`
	Error            string              `json:"error"`
	UpdateAlert      *UpdateAlert        `json:"updateAlert"` // nil when the script sent none
	LoadedAt         int64               `json:"loadedAt"`
	CreatedAt        int64               `json:"createdAt"`
	UpdatedAt        int64               `json:"updatedAt"`
}

// Options configures the service.
type Options struct {
	Store    *store.Store
	Metadata *metasearch.Service // lyrics and covers of search results
	// AllowPrivate lets scripts and downloads reach loopback and private addresses (tests).
	AllowPrivate bool
}

type loadedSource struct {
	ready     chan struct{} // closed once the start finished
	rt        *runtime      // nil when the start failed
	platforms map[string][]string
	err       error
	failedAt  time.Time
	lastUsed  time.Time
	inUse     int
}

// Service runs searches, source scripts and downloads of the links they return.
type Service struct {
	st     *store.Store
	meta   *metasearch.Service
	script *http.Client // lx.request: redirects go back to the script
	fetch  *http.Client // search APIs, script imports, audio downloads

	mu     sync.Mutex
	loaded map[string]*loadedSource
	closed bool
	stop   chan struct{}
	wg     sync.WaitGroup
}

// New creates the service and starts its idle-unload loop (stopped by Close).
func New(o Options) *Service {
	script, fetch := newClients(o.AllowPrivate)
	s := &Service{st: o.Store, meta: o.Metadata, script: script, fetch: fetch, loaded: map[string]*loadedSource{}, stop: make(chan struct{})}
	s.wg.Add(1)
	go s.janitor()
	return s
}

// Close stops every running script.
func (s *Service) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	close(s.stop)
	loaded := s.loaded
	s.loaded = map[string]*loadedSource{}
	s.mu.Unlock()
	for _, ls := range loaded {
		go func() {
			<-ls.ready
			if ls.rt != nil {
				ls.rt.close(ErrClosed)
			}
		}()
	}
	s.wg.Wait()
}

func (s *Service) janitor() {
	defer s.wg.Done()
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-tick.C:
			s.unloadIdle(time.Now().Add(-idleUnload))
		}
	}
}

// unloadIdle stops running scripts unused since before.
func (s *Service) unloadIdle(before time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, ls := range s.loaded {
		select {
		case <-ls.ready:
		default:
			continue
		}
		if ls.inUse == 0 && ls.lastUsed.Before(before) && (ls.rt != nil || ls.failedAt.Before(before)) {
			delete(s.loaded, id)
			if ls.rt != nil {
				ls.rt.close(ErrClosed)
			}
		}
	}
}

// unload stops a source's script (the next use starts it again).
func (s *Service) unload(id string) {
	s.mu.Lock()
	ls := s.loaded[id]
	delete(s.loaded, id)
	s.mu.Unlock()
	if ls != nil {
		go func() {
			<-ls.ready
			if ls.rt != nil {
				ls.rt.close(ErrClosed)
			}
		}()
	}
}

// acquire returns the running script of a source, starting it when needed. release must
// be called when done.
func (s *Service) acquire(ctx context.Context, id string) (*loadedSource, func(), error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, nil, ErrClosed
	}
	ls := s.loaded[id]
	if ls != nil {
		select {
		case <-ls.ready:
			switch {
			case ls.rt != nil && !ls.rt.alive():
				ls = nil // stopped (e.g. an endless loop): start again
			case ls.rt == nil && time.Since(ls.failedAt) > failedRetry:
				ls = nil
			}
		default:
		}
	}
	if ls == nil {
		ls = &loadedSource{ready: make(chan struct{})}
		s.loaded[id] = ls
		go s.start(id, ls)
	}
	ls.inUse++
	s.mu.Unlock()
	release := func() {
		s.mu.Lock()
		ls.inUse--
		ls.lastUsed = time.Now()
		s.mu.Unlock()
	}
	select {
	case <-ls.ready:
	case <-ctx.Done():
		release()
		return nil, nil, ctx.Err()
	}
	if ls.rt == nil {
		release()
		return nil, nil, ls.err
	}
	return ls, release, nil
}

// start runs a source's script and records the outcome in the database.
func (s *Service) start(id string, ls *loadedSource) {
	defer close(ls.ready)
	ctx, cancel := context.WithTimeout(context.Background(), loadTimeout)
	defer cancel()
	src, err := s.st.GetLxSource(ctx, id)
	if err != nil {
		ls.err, ls.failedAt = err, time.Now()
		return
	}
	info := ScriptInfo{Name: src.Name, Description: src.Description, Version: src.Version, Author: src.Author, Homepage: src.Homepage}
	rt, platforms, err := startRuntime(ctx, runtimeConfig{
		id: id, info: info, script: src.Script, client: s.script,
		onAlert: func(a UpdateAlert) {
			if err := s.st.SetLxSourceUpdateAlert(context.Background(), id, a.Log, a.URL, a.At); err != nil && !errors.Is(err, store.ErrNotFound) {
				slog.Warn("lx source: saving the update notice", "sourceId", id, "err", err)
			}
		},
	})
	s.mu.Lock()
	ls.rt, ls.platforms, ls.err, ls.lastUsed = rt, platforms, err, time.Now()
	if err != nil {
		ls.failedAt = time.Now()
	}
	s.mu.Unlock()
	platformsJSON, lastError := "", ""
	if err != nil {
		lastError = strings.TrimPrefix(err.Error(), ErrScript.Error()+": ")
		slog.Info("lx source: failed to start", "source", src.Name, "sourceId", id, "err", lastError)
	} else {
		b, _ := json.Marshal(platforms)
		platformsJSON = string(b)
	}
	if err := s.st.SetLxSourceLoaded(context.Background(), id, platformsJSON, lastError, util.NowMs()); err != nil && !errors.Is(err, store.ErrNotFound) {
		slog.Warn("lx source: saving the start result", "sourceId", id, "err", err)
	}
}

// ---- administration

// List returns every source by priority.
func (s *Service) List(ctx context.Context) ([]SourceInfo, error) {
	rows, err := s.st.ListLxSources(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]SourceInfo, 0, len(rows))
	for i := range rows {
		out = append(out, s.info(&rows[i]))
	}
	return out, nil
}

// Get returns one source.
func (s *Service) Get(ctx context.Context, id string) (*SourceInfo, error) {
	src, err := s.st.GetLxSource(ctx, id)
	if err != nil {
		return nil, err
	}
	info := s.info(src)
	return &info, nil
}

func (s *Service) info(src *model.LxSource) SourceInfo {
	info := SourceInfo{
		ID: src.ID, Name: src.Name, Description: src.Description, Version: src.Version, Author: src.Author,
		Homepage: src.Homepage, SourceURL: src.SourceURL, Size: src.ScriptSize, Enabled: src.Enabled,
		Position: src.Position, AllowUpdateAlert: src.AllowUpdateAlert, Platforms: platformList(src.Platforms),
		Status: StatusIdle, Error: src.LastError, LoadedAt: src.LoadedAt, CreatedAt: src.CreatedAt, UpdatedAt: src.UpdatedAt,
	}
	if src.UpdateLog != "" {
		info.UpdateAlert = &UpdateAlert{Log: src.UpdateLog, URL: src.UpdateURL, At: src.UpdateAt}
	}
	if src.LastError != "" {
		info.Status = StatusError
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if ls := s.loaded[src.ID]; ls != nil {
		select {
		case <-ls.ready:
			if ls.rt != nil && ls.rt.alive() {
				info.Status, info.Error = StatusReady, ""
			}
		default:
			info.Status = StatusLoading
		}
	}
	return info
}

func platformList(raw string) []PlatformQualities {
	var m map[string][]string
	_ = json.Unmarshal([]byte(raw), &m)
	out := []PlatformQualities{}
	for _, p := range Platforms {
		if qs := m[p]; len(qs) > 0 {
			out = append(out, PlatformQualities{Platform: p, Qualities: qs})
		}
	}
	return out
}

// Import stores a new source script; with start it is started right away so the result
// shows whether it works. A script that is already imported is store.ErrConflict.
func (s *Service) Import(ctx context.Context, script, sourceURL string, start bool) (*SourceInfo, error) {
	script, err := NormalizeScript(script)
	if err != nil {
		return nil, err
	}
	head, err := ParseScript(script)
	if err != nil {
		return nil, err
	}
	src := &model.LxSource{
		Name: head.Name, Description: head.Description, Version: head.Version, Author: head.Author, Homepage: head.Homepage,
		SourceURL: sourceURL, Script: script, ScriptHash: ScriptHash(script), Enabled: true, AllowUpdateAlert: true,
	}
	if err := s.st.CreateLxSource(ctx, src); err != nil {
		return nil, err
	}
	if start {
		return s.Reload(ctx, src.ID)
	}
	return s.Get(ctx, src.ID)
}

// FetchScript downloads a script from an http(s) URL (public addresses only).
func (s *Service) FetchScript(ctx context.Context, rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	u, err := url.Parse(rawURL)
	if err != nil || len(rawURL) > maxSourceURL || checkURL(u, true) != nil {
		return "", fmt.Errorf("%w: enter an http or https link to the script", ErrInvalid)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	req.Header.Set("User-Agent", searchUA)
	resp, err := s.fetch.Do(req)
	if err != nil {
		if errors.Is(err, ErrBlocked) || errors.Is(requestError(u.Hostname(), err), ErrBlocked) {
			return "", fmt.Errorf("%w: %v", ErrInvalid, requestError(u.Hostname(), err))
		}
		return "", requestError(u.Hostname(), err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: %s answered %d", ErrUpstream, u.Hostname(), resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxScriptSize+1))
	if err != nil {
		return "", fmt.Errorf("%w: reading %s: %v", ErrUpstream, u.Hostname(), err)
	}
	if len(data) > MaxScriptSize {
		return "", fmt.Errorf("%w: the script is larger than %d bytes", ErrInvalid, MaxScriptSize)
	}
	return string(data), nil
}

// Refresh downloads a source's script again from its link and replaces it when it changed.
func (s *Service) Refresh(ctx context.Context, id string, start bool) (*SourceInfo, error) {
	src, err := s.st.GetLxSource(ctx, id)
	if err != nil {
		return nil, err
	}
	if src.SourceURL == "" {
		return nil, fmt.Errorf("%w: this source was imported from a file; import the new version instead", ErrInvalid)
	}
	text, err := s.FetchScript(ctx, src.SourceURL)
	if err != nil {
		return nil, err
	}
	text, err = NormalizeScript(text)
	if err != nil {
		return nil, err
	}
	head, err := ParseScript(text)
	if err != nil {
		return nil, err
	}
	if hash := ScriptHash(text); hash != src.ScriptHash {
		next := &model.LxSource{
			ID: id, Name: head.Name, Description: head.Description, Version: head.Version, Author: head.Author,
			Homepage: head.Homepage, Script: text, ScriptHash: hash,
		}
		if err := s.st.ReplaceLxSourceScript(ctx, next); err != nil {
			return nil, err
		}
		s.unload(id)
	}
	if start {
		return s.Reload(ctx, id)
	}
	return s.Get(ctx, id)
}

// Update changes the enabled and update-notice switches (nil = unchanged). Disabling stops
// the script.
func (s *Service) Update(ctx context.Context, id string, enabled, allowUpdateAlert *bool) (*SourceInfo, error) {
	if err := s.st.SetLxSourceFlags(ctx, id, enabled, allowUpdateAlert); err != nil {
		return nil, err
	}
	if enabled != nil && !*enabled {
		s.unload(id)
	}
	return s.Get(ctx, id)
}

// Delete removes a source and stops its script.
func (s *Service) Delete(ctx context.Context, id string) error {
	if err := s.st.DeleteLxSource(ctx, id); err != nil {
		return err
	}
	s.unload(id)
	return nil
}

// Reorder sets the priority order (every source id exactly once).
func (s *Service) Reorder(ctx context.Context, ids []string) error {
	return s.st.ReorderLxSources(ctx, ids)
}

// Reload (re)starts a source's script and returns its state; a failed start is reported
// in the result (Status "error"), not as an error.
func (s *Service) Reload(ctx context.Context, id string) (*SourceInfo, error) {
	if _, err := s.st.GetLxSource(ctx, id); err != nil {
		return nil, err
	}
	s.unload(id)
	if _, release, err := s.acquire(ctx, id); err == nil {
		release()
	} else if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return s.Get(ctx, id)
}

// ---- downloads

// Candidate is a source that may provide a song.
type Candidate struct {
	ID   string
	Name string
}

// Candidates returns the sources to try for a platform, in order: in automatic mode every
// enabled source by priority, in fixed mode only the chosen one. Sources known not to
// provide the platform are left out.
func (s *Service) Candidates(ctx context.Context, platform string, sel Selection) ([]Candidate, error) {
	if !ValidPlatform(platform) {
		return nil, fmt.Errorf("%w: unknown platform %q", ErrInvalid, platform)
	}
	rows, err := s.st.ListLxSources(ctx)
	if err != nil {
		return nil, err
	}
	var out []Candidate
	for _, src := range rows {
		if !src.Enabled || (sel.Mode == model.LxSourceModeFixed && src.ID != sel.SourceID) {
			continue
		}
		if known := platformList(src.Platforms); len(known) > 0 && src.LastError == "" &&
			!slices.ContainsFunc(known, func(p PlatformQualities) bool { return p.Platform == platform }) {
			continue
		}
		out = append(out, Candidate{ID: src.ID, Name: src.Name})
	}
	return out, nil
}

// Usable counts the sources the selection allows (enabled; in fixed mode only the chosen one).
func (s *Service) Usable(ctx context.Context, sel Selection) (int, error) {
	rows, err := s.st.ListLxSources(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, src := range rows {
		if src.Enabled && (sel.Mode != model.LxSourceModeFixed || src.ID == sel.SourceID) {
			n++
		}
	}
	return n, nil
}

// Available returns, per platform, the qualities the selected sources are known to provide.
func (s *Service) Available(ctx context.Context, sel Selection) (map[string][]string, error) {
	rows, err := s.st.ListLxSources(ctx)
	if err != nil {
		return nil, err
	}
	sets := map[string]map[string]bool{}
	for _, src := range rows {
		if !src.Enabled || (sel.Mode == model.LxSourceModeFixed && src.ID != sel.SourceID) {
			continue
		}
		for _, p := range platformList(src.Platforms) {
			if sets[p.Platform] == nil {
				sets[p.Platform] = map[string]bool{}
			}
			for _, q := range p.Qualities {
				sets[p.Platform][q] = true
			}
		}
	}
	out := map[string][]string{}
	for p, set := range sets {
		for _, q := range Qualities {
			if set[q] {
				out[p] = append(out[p], q)
			}
		}
	}
	return out, nil
}

// MusicURL asks one source for a link to the song, in the best quality up to want that
// both the song and the source offer. It returns the link and the quality asked for.
func (s *Service) MusicURL(ctx context.Context, sourceID string, song Song, want string) (string, string, error) {
	if err := song.Validate(); err != nil {
		return "", "", err
	}
	ls, release, err := s.acquire(ctx, sourceID)
	if err != nil {
		return "", "", err
	}
	defer release()
	if len(ls.platforms[song.Platform]) == 0 {
		return "", "", fmt.Errorf("%w: the source does not provide %s", ErrUnavailable, song.Platform)
	}
	quality := pickQuality(song.Qualities, ls.platforms[song.Platform], want)
	if quality == "" {
		return "", "", fmt.Errorf("%w: the source offers no quality of this song", ErrUnavailable)
	}
	link, err := ls.rt.call(ctx, song.Platform, quality, song.MusicInfo())
	if err != nil {
		return "", quality, err
	}
	return link, quality, nil
}

// pickQuality chooses the best quality up to want that the song (when it lists any) and
// the source both offer, else the lowest one above want; "" when there is none.
func pickQuality(song []Quality, source []string, want string) string {
	offered := map[string]bool{}
	for _, q := range song {
		offered[q.Type] = true
	}
	limit := qualityRank(want)
	if limit < 0 {
		limit = len(Qualities) - 1
	}
	var best, above string
	for _, q := range Qualities { // lowest first
		if !slices.Contains(source, q) || (len(song) > 0 && !offered[q]) {
			continue
		}
		if qualityRank(q) <= limit {
			best = q
		} else if above == "" {
			above = q
		}
	}
	if best != "" {
		return best
	}
	return above
}

// Lyrics returns the song's lyrics and translation (LRC) from its catalogue.
func (s *Service) Lyrics(ctx context.Context, song Song) (text, translation string, err error) {
	provider, id := "", song.ID
	switch song.Platform {
	case Kuwo:
		provider = "kuwo"
	case Kugou:
		provider, id = "kugou", song.Extra["hash"]
	case QQ:
		provider = "qq"
	case NetEase:
		provider = "netease"
	case Migu:
		return s.miguLyrics(ctx, song)
	default:
		return "", "", fmt.Errorf("%w: unknown platform", ErrInvalid)
	}
	if s.meta == nil {
		return "", "", ErrNotFound
	}
	l, err := s.meta.Lyrics(ctx, provider, id, metasearch.Options{})
	if err != nil {
		if errors.Is(err, metasearch.ErrNotFound) {
			return "", "", fmt.Errorf("%w: no lyrics", ErrNotFound)
		}
		return "", "", err
	}
	return l.Text, l.Translation, nil
}

// miguLyrics downloads the LRC files Migu links from its search results.
func (s *Service) miguLyrics(ctx context.Context, song Song) (string, string, error) {
	get := func(key string) string {
		raw := song.Extra[key]
		if raw == "" || !miguLink(raw) {
			return ""
		}
		data, err := (&apiClient{http: s.fetch}).get(ctx, raw, nil)
		if err != nil {
			return ""
		}
		text, _ := decodeBytes(data, "utf8")
		return strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	}
	text := get("lrcUrl")
	if text == "" {
		return "", "", fmt.Errorf("%w: no lyrics", ErrNotFound)
	}
	return text, get("trcUrl"), nil
}

// Cover downloads the song's cover from its catalogue's image host.
func (s *Service) Cover(ctx context.Context, song Song) ([]byte, error) {
	if song.CoverURL == "" || s.meta == nil {
		return nil, fmt.Errorf("%w: no cover", ErrNotFound)
	}
	data, _, err := s.meta.Cover(ctx, song.CoverURL)
	return data, err
}

// Download streams an audio link into w (at most limit bytes). progress receives the bytes
// written so far and the expected total (-1 when unknown).
func (s *Service) Download(ctx context.Context, link string, w io.Writer, limit int64, progress func(done, total int64)) (int64, error) {
	u, err := url.Parse(link)
	if err != nil {
		return 0, fmt.Errorf("%w: invalid link", ErrUpstream)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, fmt.Errorf("%w: invalid link", ErrUpstream)
	}
	req.Header.Set("User-Agent", searchUA)
	resp, err := s.fetch.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		return 0, requestError(u.Hostname(), err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return 0, &LinkError{Host: resp.Request.URL.Hostname(), Status: resp.StatusCode}
	}
	total := resp.ContentLength
	if total > limit {
		return 0, fmt.Errorf("%w: the file is larger than %d MiB", ErrUpstream, limit>>20)
	}
	var done int64
	buf := make([]byte, 64<<10)
	last := time.Time{}
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if done+int64(n) > limit {
				return done, fmt.Errorf("%w: the file is larger than %d MiB", ErrUpstream, limit>>20)
			}
			if _, err := w.Write(buf[:n]); err != nil {
				return done, err
			}
			done += int64(n)
			if progress != nil && time.Since(last) > 250*time.Millisecond {
				last = time.Now()
				progress(done, total)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			if ctx.Err() != nil {
				return done, ctx.Err()
			}
			return done, fmt.Errorf("%w: reading from %s: %v", ErrUpstream, resp.Request.URL.Hostname(), rerr)
		}
	}
	if total > 0 && done != total {
		return done, fmt.Errorf("%w: the download stopped early", ErrUpstream)
	}
	if progress != nil {
		progress(done, total)
	}
	return done, nil
}

// LinkError is an audio link that answered with an HTTP error status.
type LinkError struct {
	Host   string
	Status int
}

func (e *LinkError) Error() string { return fmt.Sprintf("%s answered %d", e.Host, e.Status) }

func (e *LinkError) Unwrap() error { return ErrUpstream }

// LinkExpired reports whether a failed download means the link is no longer valid, so the
// source should be asked for a new one. lx-music does that for HTTP 401, 403 and 410 and for
// hosts that do not resolve.
func LinkExpired(err error) bool {
	var le *LinkError
	if errors.As(err, &le) {
		return le.Status == http.StatusUnauthorized || le.Status == http.StatusForbidden || le.Status == http.StatusGone
	}
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr) && dnsErr.IsNotFound
}

// CoverAllowed reports whether a search result's cover URL may be proxied.
func CoverAllowed(rawURL string) bool { return metasearch.CoverAllowed(rawURL) }
