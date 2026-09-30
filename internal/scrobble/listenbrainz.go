package scrobble

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"rainy/internal/buildinfo"
)

// ListenBrainz API (https://listenbrainz.readthedocs.io/en/latest/users/api/). Requests
// carry the user's token in the Authorization header.
const (
	listenBrainzAPI   = "https://api.listenbrainz.org"
	listenBrainzBatch = 100 // well below the API's 1000 listens per request
)

type listenBrainz struct {
	http *http.Client
	base string
	ua   string
}

func (c *listenBrainz) request(ctx context.Context, method, path, token string, body any, out any) error {
	var rd *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.base, "/")+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Token "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.ua)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	status, resp, err := do(c.http, req, "listenbrainz")
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		var fail struct {
			Error string `json:"error"`
		}
		msg := fmt.Sprintf("HTTP %d", status)
		if json.Unmarshal(resp, &fail) == nil && fail.Error != "" {
			msg = clip(fail.Error, 200)
		}
		kind := kindRejected
		switch {
		case status == http.StatusUnauthorized:
			kind = kindAuth
		case status == http.StatusTooManyRequests || status >= 500 || (status >= 300 && status < 400):
			kind = kindRetry
		}
		return &Error{Service: "listenbrainz", Code: status, Message: msg, kind: kind}
	}
	if out != nil && json.Unmarshal(resp, out) != nil {
		return &Error{Service: "listenbrainz", Message: "unexpected response", kind: kindRetry}
	}
	return nil
}

// validate checks a user token and returns the account name.
func (c *listenBrainz) validate(ctx context.Context, token string) (string, error) {
	var out struct {
		Valid    bool   `json:"valid"`
		UserName string `json:"user_name"`
	}
	if err := c.request(ctx, http.MethodGet, "/1/validate-token", token, nil, &out); err != nil {
		return "", err
	}
	if !out.Valid || out.UserName == "" {
		return "", &Error{Service: "listenbrainz", Message: "the token is not valid", kind: kindAuth}
	}
	return clip(out.UserName, 64), nil
}

type lbListen struct {
	ListenedAt int64      `json:"listened_at,omitempty"`
	Track      lbMetadata `json:"track_metadata"`
}

type lbMetadata struct {
	Artist  string         `json:"artist_name"`
	Title   string         `json:"track_name"`
	Release string         `json:"release_name,omitempty"`
	Info    map[string]any `json:"additional_info"`
}

func lbTrack(t Track) lbMetadata {
	info := map[string]any{
		"media_player":              "Rainy",
		"submission_client":         "Rainy",
		"submission_client_version": buildinfo.Version,
	}
	if t.Duration >= 1 {
		info["duration_ms"] = int64(t.Duration * 1000)
	}
	if t.TrackNumber > 0 {
		info["tracknumber"] = t.TrackNumber
	}
	if t.MbzTrackID != "" {
		info["recording_mbid"] = t.MbzTrackID
	}
	if t.AlbumArtist != "" && t.AlbumArtist != t.Artist {
		info["release_artist_name"] = t.AlbumArtist
	}
	return lbMetadata{Artist: t.Artist, Title: t.Title, Release: t.Album, Info: info}
}

func (c *listenBrainz) nowPlaying(ctx context.Context, token string, t Track) error {
	body := map[string]any{"listen_type": "playing_now", "payload": []lbListen{{Track: lbTrack(t)}}}
	return c.request(ctx, http.MethodPost, "/1/submit-listens", token, body, nil)
}

func (c *listenBrainz) submit(ctx context.Context, token string, plays []Track) error {
	listens := make([]lbListen, 0, len(plays))
	for _, t := range plays {
		listens = append(listens, lbListen{ListenedAt: t.PlayedAt / 1000, Track: lbTrack(t)})
	}
	kind := "import"
	if len(listens) == 1 {
		kind = "single"
	}
	return c.request(ctx, http.MethodPost, "/1/submit-listens", token, map[string]any{"listen_type": kind, "payload": listens}, nil)
}
