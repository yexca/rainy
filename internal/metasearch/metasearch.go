// Package metasearch looks up track metadata, lyrics and covers in public online music
// catalogues (NetEase Cloud Music, QQ Music, Kugou, Kuwo and the iTunes Search API) for the
// tag editor (docs/architecture/contract.md §5.13).
//
// It is the only part of Rainy that makes outbound requests, so it is off unless an
// administrator enables the `onlineMetadata` setting, and it only runs when a manager
// searches explicitly. Results are suggestions: nothing here writes files. The caller fills
// the tag editor's draft, and the normal save path (TagLib, edit log, rescan) applies it.
//
// Requests use fixed endpoint URLs with the query as a single encoded parameter, a short
// timeout, and bounded response sizes. Covers are fetched only from the providers' own
// image hosts (see coverHosts), so the cover proxy cannot be pointed at arbitrary URLs.
package metasearch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// Errors returned by the Service. Upstream failures wrap ErrUpstream.
var (
	ErrUnknownProvider = errors.New("unknown metadata provider")
	ErrInvalid         = errors.New("invalid request")
	ErrNotFound        = errors.New("not found")
	ErrUpstream        = errors.New("metadata provider request failed")
)

// Limits.
const (
	MaxQueryLength = 200 // runes
	DefaultLimit   = 20
	MaxLimit       = 30

	maxJSONSize    = 4 << 20
	maxCoverSize   = 20 << 20
	requestTimeout = 15 * time.Second
	userAgent      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"
)

// Result is one search hit. Unknown numbers are 0 and unknown strings "".
type Result struct {
	Provider    string   `json:"provider"`
	ID          string   `json:"id"` // provider-specific, opaque; pass back to Lyrics
	Title       string   `json:"title"`
	Artists     []string `json:"artists"` // never nil
	Album       string   `json:"album"`
	AlbumArtist string   `json:"albumArtist"`
	TrackNumber int      `json:"trackNumber"`
	TrackTotal  int      `json:"trackTotal"`
	DiscNumber  int      `json:"discNumber"`
	DiscTotal   int      `json:"discTotal"`
	Date        string   `json:"date"` // YYYY or YYYY-MM-DD
	Genre       string   `json:"genre"`
	Duration    float64  `json:"duration"` // seconds
	CoverURL    string   `json:"coverUrl"` // full-size image on the provider's image host
	ThumbURL    string   `json:"thumbUrl"` // small image for the result list
}

// Lyrics are a provider's lyrics for one song: LRC (or plain) text and an optional
// translation in the same format.
type Lyrics struct {
	Text        string `json:"text"`
	Translation string `json:"translation"`
}

// ProviderInfo describes a provider for the UI.
type ProviderInfo struct {
	ID      string   `json:"id"`
	Lyrics  bool     `json:"lyrics"`  // Lyrics is supported
	Regions []string `json:"regions"` // store regions for Search (empty: not applicable)
}

// provider is one online catalogue.
type provider interface {
	info() ProviderInfo
	search(ctx context.Context, c *client, query string, limit int, region string) ([]Result, error)
	lyrics(ctx context.Context, c *client, id string) (*Lyrics, error)
}

// Service runs searches against the providers.
type Service struct {
	c         *client
	providers map[string]provider
	order     []string
}

// New creates the service. A nil httpClient uses a client with a request timeout and the
// environment's proxy settings (HTTPS_PROXY / HTTP_PROXY).
func New(httpClient *http.Client) *Service {
	if httpClient == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.DialContext = (&net.Dialer{Timeout: 10 * time.Second}).DialContext
		transport.ResponseHeaderTimeout = requestTimeout
		httpClient = &http.Client{Timeout: requestTimeout, Transport: transport}
	}
	s := &Service{c: &client{http: httpClient}, providers: map[string]provider{}}
	for _, p := range []provider{netease{}, qqMusic{}, kugou{}, kuwo{}, itunes{}} {
		id := p.info().ID
		s.providers[id] = p
		s.order = append(s.order, id)
	}
	return s
}

// Providers lists the providers in display order.
func (s *Service) Providers() []ProviderInfo {
	out := make([]ProviderInfo, 0, len(s.order))
	for _, id := range s.order {
		out = append(out, s.providers[id].info())
	}
	return out
}

func (s *Service) provider(id string) (provider, error) {
	p, ok := s.providers[id]
	if !ok {
		return nil, fmt.Errorf("%w %q", ErrUnknownProvider, id)
	}
	return p, nil
}

// Options tune one lookup.
type Options struct {
	// ChinaIP sends an X-Real-IP header with a random mainland-China address (see chinaIP)
	// to the Chinese catalogues, which hide some songs from requests outside China. It is
	// never sent to iTunes or to the image hosts.
	ChinaIP bool
}

// clientFor returns the client for one lookup with providerID.
func (s *Service) clientFor(providerID string, opts Options) *client {
	if !opts.ChinaIP || !chinaProviders[providerID] {
		return s.c
	}
	c := *s.c
	c.realIP = chinaIP()
	return &c
}

// Search queries one provider. limit ≤ 0 means DefaultLimit; region is used only by
// providers that list regions (the first one is the default).
func (s *Service) Search(ctx context.Context, providerID, query string, limit int, region string, opts Options) ([]Result, error) {
	p, err := s.provider(providerID)
	if err != nil {
		return nil, err
	}
	query = strings.Join(strings.Fields(query), " ")
	if query == "" {
		return nil, fmt.Errorf("%w: empty query", ErrInvalid)
	}
	if utf8.RuneCountInString(query) > MaxQueryLength {
		return nil, fmt.Errorf("%w: the query is longer than %d characters", ErrInvalid, MaxQueryLength)
	}
	if limit <= 0 {
		limit = DefaultLimit
	}
	limit = min(limit, MaxLimit)
	if regions := p.info().Regions; len(regions) > 0 {
		region = strings.ToLower(strings.TrimSpace(region))
		if region == "" {
			region = regions[0]
		} else if !contains(regions, region) {
			return nil, fmt.Errorf("%w: unknown region %q", ErrInvalid, region)
		}
	}
	results, err := p.search(ctx, s.clientFor(providerID, opts), query, limit, region)
	if err != nil {
		return nil, err
	}
	out := results[:0]
	for _, r := range results {
		r.Provider = providerID
		r.Title = clean(r.Title)
		r.Album = clean(r.Album)
		r.AlbumArtist = clean(r.AlbumArtist)
		r.Genre = clean(r.Genre)
		r.Artists = cleanList(r.Artists)
		r.CoverURL = httpsURL(r.CoverURL)
		r.ThumbURL = httpsURL(r.ThumbURL)
		if r.ID == "" || r.Title == "" {
			continue
		}
		out = append(out, r)
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

// Lyrics fetches the lyrics of a search result. ErrNotFound when the provider has none.
func (s *Service) Lyrics(ctx context.Context, providerID, id string, opts Options) (*Lyrics, error) {
	p, err := s.provider(providerID)
	if err != nil {
		return nil, err
	}
	if !p.info().Lyrics {
		return nil, fmt.Errorf("%w: %s has no lyrics", ErrNotFound, providerID)
	}
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 128 {
		return nil, fmt.Errorf("%w: invalid id", ErrInvalid)
	}
	l, err := p.lyrics(ctx, s.clientFor(providerID, opts), id)
	if err != nil {
		return nil, err
	}
	l.Text = normalizeNewlines(l.Text)
	l.Translation = normalizeNewlines(l.Translation)
	if strings.TrimSpace(l.Text) == "" {
		return nil, fmt.Errorf("%w: no lyrics", ErrNotFound)
	}
	if !hasText(l.Translation) {
		l.Translation = ""
	}
	return l, nil
}

// coverHosts are the image hosts covers may be fetched from (the host itself or a
// subdomain).
var coverHosts = []string{
	"music.126.net", // NetEase
	"y.gtimg.cn",    // QQ Music
	"y.qq.com",      // QQ Music
	"kugou.com",     // Kugou (imge.kugou.com)
	"kuwo.cn",       // Kuwo (img1–4.kuwo.cn)
	"mzstatic.com",  // iTunes
}

// CoverAllowed reports whether rawURL points at one of the providers' image hosts.
func CoverAllowed(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
		return false
	}
	if p := u.Port(); p != "" && p != "80" && p != "443" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, h := range coverHosts {
		if host == h || strings.HasSuffix(host, "."+h) {
			return true
		}
	}
	return false
}

// Cover downloads a cover image from a provider image host (always over HTTPS) and returns
// its bytes and sniffed raster content type.
func (s *Service) Cover(ctx context.Context, rawURL string) ([]byte, string, error) {
	if !CoverAllowed(rawURL) {
		return nil, "", fmt.Errorf("%w: not a provider image URL", ErrInvalid)
	}
	u := httpsURL(rawURL)
	data, err := s.c.get(ctx, u, nil, maxCoverSize)
	if err != nil {
		return nil, "", err
	}
	ctype := http.DetectContentType(data)
	switch ctype {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return data, ctype, nil
	}
	return nil, "", fmt.Errorf("%w: the provider did not return an image", ErrUpstream)
}

// ---- HTTP

type client struct {
	http   *http.Client
	realIP string // X-Real-IP to send ("" = none)
}

func (c *client) get(ctx context.Context, rawURL string, header http.Header, limit int64) ([]byte, error) {
	return c.do(ctx, http.MethodGet, rawURL, header, nil, limit)
}

func (c *client) post(ctx context.Context, rawURL string, header http.Header, body string, limit int64) ([]byte, error) {
	return c.do(ctx, http.MethodPost, rawURL, header, strings.NewReader(body), limit)
}

func (c *client) do(ctx context.Context, method, rawURL string, header http.Header, body io.Reader, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	if c.realIP != "" {
		req.Header.Set("X-Real-IP", c.realIP)
	}
	for k, v := range header {
		req.Header[k] = v
	}
	// Redirects stay on the same host (image CDNs sometimes switch scheme); anything else
	// is refused so an allow-listed URL cannot bounce the request elsewhere.
	cl := *c.http
	cl.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		if len(via) >= 3 || !strings.EqualFold(r.URL.Hostname(), via[0].URL.Hostname()) {
			return fmt.Errorf("redirect to %s refused", r.URL.Hostname())
		}
		return nil
	}
	resp, err := cl.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w: %s: %v", ErrUpstream, req.URL.Hostname(), unwrapURLError(err))
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%w: %s answered 404", ErrNotFound, req.URL.Hostname())
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("%w: %s answered %d", ErrUpstream, req.URL.Hostname(), resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("%w: reading %s: %v", ErrUpstream, req.URL.Hostname(), err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%w: %s sent more than %d bytes", ErrUpstream, req.URL.Hostname(), limit)
	}
	return data, nil
}

// unwrapURLError drops the *url.Error wrapper, whose message repeats the full URL
// (including the search query).
func unwrapURLError(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

// ---- small helpers

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// httpsURL upgrades plain-HTTP URLs (several providers still return them) to HTTPS.
func httpsURL(s string) string {
	u, err := url.Parse(s)
	if err != nil || u.Scheme != "http" {
		return s
	}
	u.Scheme = "https"
	return u.String()
}

func normalizeNewlines(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.TrimSpace(strings.ReplaceAll(s, "\r", "\n"))
}

// hasText reports whether LRC text has any line with text after its timestamps (providers
// return translations consisting only of empty timed lines).
func hasText(s string) bool {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		for strings.HasPrefix(line, "[") {
			end := strings.IndexByte(line, ']')
			if end < 0 {
				break
			}
			line = strings.TrimSpace(line[end+1:])
		}
		if line != "" && line != "//" {
			return true
		}
	}
	return false
}
