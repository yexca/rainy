package metasearch

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// netease searches NetEase Cloud Music through its plain (unencrypted) PC endpoints.
type netease struct{}

const (
	neteaseSearchURL = "https://music.163.com/api/cloudsearch/pc"
	neteaseLyricURL  = "https://music.163.com/api/song/lyric"
)

func (netease) info() ProviderInfo {
	return ProviderInfo{ID: "netease", Lyrics: true, Regions: []string{}}
}

func neteaseHeader() http.Header {
	return http.Header{
		"Referer":      {"https://music.163.com/"},
		"Content-Type": {"application/x-www-form-urlencoded"},
	}
}

type neteaseSong struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	No          flexInt   `json:"no"`
	CD          flexInt   `json:"cd"`
	Duration    int64     `json:"dt"` // ms
	PublishTime int64     `json:"publishTime"`
	Artists     []idName  `json:"ar"`
	Album       neteaseAl `json:"al"`
}

type neteaseAl struct {
	Name   string `json:"name"`
	PicURL string `json:"picUrl"`
}

type idName struct {
	Name string `json:"name"`
}

func (netease) search(ctx context.Context, c *client, query string, limit int, _ string) ([]Result, error) {
	form := url.Values{"s": {query}, "type": {"1"}, "limit": {strconv.Itoa(limit)}, "offset": {"0"}}
	data, err := c.post(ctx, neteaseSearchURL, neteaseHeader(), form.Encode(), maxJSONSize)
	if err != nil {
		return nil, err
	}
	var body struct {
		Code   int `json:"code"`
		Result struct {
			Songs []neteaseSong `json:"songs"`
		} `json:"result"`
	}
	if err := decode(data, "music.163.com", &body); err != nil {
		return nil, err
	}
	if body.Code != 0 && body.Code != http.StatusOK {
		return nil, fmt.Errorf("%w: music.163.com answered code %d", ErrUpstream, body.Code)
	}
	out := make([]Result, 0, len(body.Result.Songs))
	for _, s := range body.Result.Songs {
		r := Result{
			ID:          strconv.FormatInt(s.ID, 10),
			Title:       s.Name,
			Album:       s.Album.Name,
			TrackNumber: int(s.No),
			DiscNumber:  int(s.CD),
			Date:        dateFromMillis(s.PublishTime),
			Duration:    float64(s.Duration) / 1000,
		}
		for _, a := range s.Artists {
			r.Artists = append(r.Artists, a.Name)
		}
		if s.Album.PicURL != "" {
			r.CoverURL = s.Album.PicURL
			r.ThumbURL = s.Album.PicURL + "?param=150y150"
		}
		out = append(out, r)
	}
	return out, nil
}

func (netease) lyrics(ctx context.Context, c *client, id string) (*Lyrics, error) {
	if _, err := strconv.ParseInt(id, 10, 64); err != nil {
		return nil, fmt.Errorf("%w: invalid id", ErrInvalid)
	}
	q := url.Values{"os": {"pc"}, "id": {id}, "lv": {"-1"}, "kv": {"-1"}, "tv": {"-1"}}
	data, err := c.get(ctx, neteaseLyricURL+"?"+q.Encode(), http.Header{"Referer": {"https://music.163.com/"}}, maxJSONSize)
	if err != nil {
		return nil, err
	}
	var body struct {
		Lrc    struct{ Lyric string } `json:"lrc"`
		Tlyric struct{ Lyric string } `json:"tlyric"`
	}
	if err := decode(data, "music.163.com", &body); err != nil {
		return nil, err
	}
	return &Lyrics{Text: body.Lrc.Lyric, Translation: body.Tlyric.Lyric}, nil
}
