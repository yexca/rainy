package lxmusic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Song search in the five catalogues. Results carry the ids the catalogues (and source
// scripts) need to identify a song, in the shape lx-music's own search produces.

// Search limits.
const (
	MaxQueryLength = 200 // runes
	DefaultLimit   = 30
	MaxLimit       = 50
	maxPage        = 100
	maxJSONSize    = 4 << 20
	searchUA       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0 Safari/537.36"
)

// Quality is a quality a catalogue lists for a song.
type Quality struct {
	Type string `json:"type"`           // 128k | 320k | flac | flac24bit
	Size string `json:"size"`           // e.g. "10.29 MB"; "" when unknown
	Hash string `json:"hash,omitempty"` // Kugou: the file hash of this quality
}

// Song is a search result. Extra holds the platform ids source scripts need: Kugou hash and
// albumAudioId; QQ Music strMediaMid, albumMid and songId; Migu copyrightId, lrcUrl,
// mrcUrl and trcUrl.
type Song struct {
	Platform  string            `json:"platform"`
	ID        string            `json:"id"` // lx-music's songmid
	Title     string            `json:"title"`
	Artists   []string          `json:"artists"`
	Album     string            `json:"album"`
	AlbumID   string            `json:"albumId"`
	Duration  float64           `json:"duration"` // seconds, 0 = unknown
	CoverURL  string            `json:"coverUrl"`
	Qualities []Quality         `json:"qualities"` // lowest first
	Extra     map[string]string `json:"extra"`
	PageURL   string            `json:"pageUrl"` // the song's page on the catalogue's website
}

// SearchResult is one page of search results.
type SearchResult struct {
	Items []Song `json:"items"`
	Total int    `json:"total"`
	Page  int    `json:"page"`
	Limit int    `json:"limit"`
}

type searcher func(ctx context.Context, c *apiClient, query string, page, limit int) (*SearchResult, error)

var searchers = map[string]searcher{
	Kuwo:    searchKuwo,
	Kugou:   searchKugou,
	QQ:      searchQQ,
	NetEase: searchNetEase,
	Migu:    searchMigu,
}

// Search runs one page of a song search. page starts at 1; limit ≤ 0 means DefaultLimit.
func (s *Service) Search(ctx context.Context, platform, query string, page, limit int) (*SearchResult, error) {
	fn, ok := searchers[platform]
	if !ok {
		return nil, fmt.Errorf("%w: unknown platform %q", ErrInvalid, platform)
	}
	query = strings.Join(strings.Fields(query), " ")
	if query == "" {
		return nil, fmt.Errorf("%w: empty query", ErrInvalid)
	}
	if utf8.RuneCountInString(query) > MaxQueryLength {
		return nil, fmt.Errorf("%w: the query is longer than %d characters", ErrInvalid, MaxQueryLength)
	}
	if page < 1 {
		page = 1
	}
	if page > maxPage {
		return nil, fmt.Errorf("%w: page must be at most %d", ErrInvalid, maxPage)
	}
	if limit <= 0 {
		limit = DefaultLimit
	}
	limit = min(limit, MaxLimit)
	res, err := fn(ctx, &apiClient{http: s.fetch}, query, page, limit)
	if err != nil {
		return nil, err
	}
	items := make([]Song, 0, len(res.Items))
	for _, song := range res.Items {
		song.Platform = platform
		song.Title = cleanText(song.Title)
		song.Album = cleanText(song.Album)
		song.Artists = cleanList(song.Artists)
		song.CoverURL = httpsURL(song.CoverURL)
		if song.Extra == nil {
			song.Extra = map[string]string{}
		}
		if song.Qualities == nil {
			song.Qualities = []Quality{}
		}
		if song.ID == "" || song.Title == "" {
			continue
		}
		song.PageURL = song.pageURL()
		items = append(items, song)
		if len(items) == limit {
			break
		}
	}
	res.Items, res.Page, res.Limit = items, page, limit
	return res, nil
}

// ---- HTTP

type apiClient struct {
	http *http.Client
}

func (c *apiClient) get(ctx context.Context, rawURL string, header http.Header) ([]byte, error) {
	return c.do(ctx, http.MethodGet, rawURL, header, nil)
}

func (c *apiClient) post(ctx context.Context, rawURL string, header http.Header, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, rawURL, header, body)
}

func (c *apiClient) do(ctx context.Context, method, rawURL string, header http.Header, body []byte) ([]byte, error) {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, rd)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	req.Header.Set("User-Agent", searchUA)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	for k, v := range header {
		req.Header[k] = v
	}
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, requestError(req.URL.Hostname(), err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("%w: %s answered %d", ErrUpstream, req.URL.Hostname(), resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxJSONSize+1))
	if err != nil {
		return nil, fmt.Errorf("%w: reading %s: %v", ErrUpstream, req.URL.Hostname(), err)
	}
	if len(data) > maxJSONSize {
		return nil, fmt.Errorf("%w: %s sent more than %d bytes", ErrUpstream, req.URL.Hostname(), maxJSONSize)
	}
	return data, nil
}

func decodeJSON(data []byte, host string, v any) error {
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%w: unexpected answer from %s: %v", ErrUpstream, host, err)
	}
	return nil
}

// ---- lenient JSON values (the catalogues mix numbers and strings)

// flexInt decodes a JSON number or numeric string; anything else is 0.
type flexInt int64

func (f *flexInt) UnmarshalJSON(b []byte) error {
	var s flexString
	_ = s.UnmarshalJSON(b)
	n, err := strconv.ParseFloat(strings.TrimSpace(string(s)), 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		*f = 0
		return nil
	}
	*f = flexInt(n)
	return nil
}

// flexString decodes a JSON string or number as a string.
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*f = flexString(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err == nil {
		*f = flexString(n.String())
	}
	return nil
}

// ---- text helpers

// cleanText decodes HTML entities (several catalogues escape names) and collapses spaces.
func cleanText(s string) string {
	return strings.Join(strings.Fields(html.UnescapeString(s)), " ")
}

func cleanList(list []string) []string {
	out := make([]string, 0, len(list))
	seen := map[string]bool{}
	for _, v := range list {
		v = cleanText(v)
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func splitArtists(s, seps string) []string {
	return strings.FieldsFunc(html.UnescapeString(s), func(r rune) bool { return strings.ContainsRune(seps, r) })
}

// httpsURL upgrades plain-HTTP image links (every catalogue's image host serves HTTPS).
func httpsURL(s string) string {
	if strings.HasPrefix(s, "http://") {
		return "https://" + strings.TrimPrefix(s, "http://")
	}
	return s
}

// formatSize formats a byte count like lx-music ("10.29 MB"); 0 → "".
func formatSize(n int64) string {
	if n <= 0 {
		return ""
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	i := min(int(math.Floor(math.Log(float64(n))/math.Log(1024))), len(units)-1)
	return fmt.Sprintf("%.2f %s", float64(n)/math.Pow(1024, float64(i)), units[i])
}

// formatInterval formats seconds as lx-music's "mm:ss" ("--/--" when unknown).
func formatInterval(sec float64) string {
	s := int(sec)
	if s <= 0 {
		return "--/--"
	}
	return fmt.Sprintf("%02d:%02d", s/60, s%60)
}

var hexHash = regexp.MustCompile(`^[0-9A-Fa-f]{32}$`)
