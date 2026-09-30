package scrobble

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// Last.fm API 2.0 (https://www.last.fm/api). Every call is a signed POST to one fixed
// endpoint; the web authentication page is opened by the browser, never by the server.
const (
	lastfmAPI     = "https://ws.audioscrobbler.com/2.0/"
	lastfmAuthURL = "https://www.last.fm/api/auth/"
	lastfmBatch   = 50 // track.scrobble accepts at most 50 scrobbles
)

type lastfmCreds struct{ key, secret string }

type lastfm struct {
	http *http.Client
	base string
	ua   string
}

// lastfmSign computes api_sig: every parameter except format and callback, sorted by name,
// as name+value, followed by the shared secret, MD5 in hex.
func lastfmSign(params url.Values, secret string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k != "format" && k != "callback" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString(params.Get(k))
	}
	b.WriteString(secret)
	sum := md5.Sum([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// lastfmKind maps a Last.fm error code (https://www.last.fm/api/errorcodes).
func lastfmKind(code int) errKind {
	switch code {
	case 9: // invalid session key
		return kindAuth
	case 10, 13, 26: // invalid API key, invalid signature, suspended API key
		return kindConfig
	case 8, 11, 16, 29: // operation failed, service offline, temporarily unavailable, rate limit
		return kindRetry
	}
	return kindRejected
}

func (c *lastfm) call(ctx context.Context, creds lastfmCreds, params url.Values, out any) error {
	params.Set("api_key", creds.key)
	params.Set("api_sig", lastfmSign(params, creds.secret))
	params.Set("format", "json")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base, strings.NewReader(params.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.ua)
	status, body, err := do(c.http, req, "lastfm")
	if err != nil {
		return err
	}
	var fail struct {
		Error   int    `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &fail) == nil && fail.Error != 0 {
		return &Error{Service: "lastfm", Code: fail.Error, Message: clip(fail.Message, 200), kind: lastfmKind(fail.Error)}
	}
	if status >= 500 || status == http.StatusTooManyRequests || (status >= 300 && status < 400) {
		return &Error{Service: "lastfm", Message: fmt.Sprintf("HTTP %d", status), kind: kindRetry}
	}
	if status != http.StatusOK {
		return &Error{Service: "lastfm", Message: fmt.Sprintf("HTTP %d", status), kind: kindRejected}
	}
	if out != nil && json.Unmarshal(body, out) != nil {
		return &Error{Service: "lastfm", Message: "unexpected response", kind: kindRetry}
	}
	return nil
}

// session exchanges an authorized web token for a session key.
func (c *lastfm) session(ctx context.Context, creds lastfmCreds, token string) (name, key string, err error) {
	var out struct {
		Session struct {
			Name string `json:"name"`
			Key  string `json:"key"`
		} `json:"session"`
	}
	params := url.Values{"method": {"auth.getSession"}, "token": {token}}
	if err := c.call(ctx, creds, params, &out); err != nil {
		return "", "", err
	}
	if out.Session.Key == "" {
		return "", "", &Error{Service: "lastfm", Message: "no session in the response", kind: kindRejected}
	}
	return clip(out.Session.Name, 64), out.Session.Key, nil
}

func lastfmTrackParams(p url.Values, suffix string, t Track) {
	p.Set("artist"+suffix, t.Artist)
	p.Set("track"+suffix, t.Title)
	if t.Album != "" {
		p.Set("album"+suffix, t.Album)
	}
	if t.AlbumArtist != "" && t.AlbumArtist != t.Artist {
		p.Set("albumArtist"+suffix, t.AlbumArtist)
	}
	if t.TrackNumber > 0 {
		p.Set("trackNumber"+suffix, strconv.Itoa(t.TrackNumber))
	}
	if t.Duration >= 1 {
		p.Set("duration"+suffix, strconv.Itoa(int(t.Duration+0.5)))
	}
	if t.MbzTrackID != "" {
		p.Set("mbid"+suffix, t.MbzTrackID)
	}
}

func (c *lastfm) nowPlaying(ctx context.Context, creds lastfmCreds, sk string, t Track) error {
	p := url.Values{"method": {"track.updateNowPlaying"}, "sk": {sk}}
	lastfmTrackParams(p, "", t)
	return c.call(ctx, creds, p, nil)
}

func (c *lastfm) love(ctx context.Context, creds lastfmCreds, sk string, t Track, loved bool) error {
	method := "track.love"
	if !loved {
		method = "track.unlove"
	}
	return c.call(ctx, creds, url.Values{"method": {method}, "sk": {sk}, "artist": {t.Artist}, "track": {t.Title}}, nil)
}

// flexInt decodes a JSON number or a numeric string (Last.fm uses both).
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	n, err := strconv.Atoi(s)
	*f = flexInt(n)
	return err
}

// scrobble submits up to lastfmBatch plays and returns how many Last.fm ignored.
func (c *lastfm) scrobble(ctx context.Context, creds lastfmCreds, sk string, plays []Track) (ignored int, err error) {
	p := url.Values{"method": {"track.scrobble"}, "sk": {sk}}
	for i, t := range plays {
		suffix := "[" + strconv.Itoa(i) + "]"
		lastfmTrackParams(p, suffix, t)
		p.Set("timestamp"+suffix, strconv.FormatInt(t.PlayedAt/1000, 10))
	}
	var out struct {
		Scrobbles struct {
			Attr struct {
				Accepted flexInt `json:"accepted"`
				Ignored  flexInt `json:"ignored"`
			} `json:"@attr"`
		} `json:"scrobbles"`
	}
	if err := c.call(ctx, creds, p, &out); err != nil {
		return 0, err
	}
	return int(out.Scrobbles.Attr.Ignored), nil
}
