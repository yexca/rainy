package metasearch

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// kugou searches Kugou's mobile catalogue. It has no track numbers or dates.
type kugou struct{}

const (
	// The mobile search endpoint is only served over plain HTTP.
	kugouSearchURL        = "http://mobilecdn.kugou.com/api/v3/search/song"
	kugouLyricSearchURL   = "https://lyrics.kugou.com/search"
	kugouLyricDownloadURL = "https://lyrics.kugou.com/download"
	kugouCoverSize        = "480"
	kugouThumbSize        = "150"
)

var (
	kugouHash   = regexp.MustCompile(`^[0-9A-Fa-f]{32}$`)
	kugouLyrics = regexp.MustCompile(`^[0-9A-Za-z]{1,64}$`)
	emTags      = strings.NewReplacer("<em>", "", "</em>", "")
)

func (kugou) info() ProviderInfo { return ProviderInfo{ID: "kugou", Lyrics: true, Regions: []string{}} }

func (kugou) search(ctx context.Context, c *client, query string, limit int, _ string) ([]Result, error) {
	q := url.Values{
		"format": {"json"}, "keyword": {query}, "page": {"1"},
		"pagesize": {strconv.Itoa(limit)}, "showtype": {"1"},
	}
	data, err := c.get(ctx, kugouSearchURL+"?"+q.Encode(), nil, maxJSONSize)
	if err != nil {
		return nil, err
	}
	var body struct {
		Status flexInt `json:"status"`
		Data   struct {
			Info []struct {
				Hash       string  `json:"hash"`
				SongName   string  `json:"songname"`
				SingerName string  `json:"singername"`
				AlbumName  string  `json:"album_name"`
				Duration   flexInt `json:"duration"`
				Trans      struct {
					UnionCover string `json:"union_cover"`
				} `json:"trans_param"`
			} `json:"info"`
		} `json:"data"`
	}
	if err := decode(data, "mobilecdn.kugou.com", &body); err != nil {
		return nil, err
	}
	if body.Status != 1 {
		return nil, fmt.Errorf("%w: mobilecdn.kugou.com answered status %d", ErrUpstream, body.Status)
	}
	out := make([]Result, 0, len(body.Data.Info))
	for _, s := range body.Data.Info {
		if !kugouHash.MatchString(s.Hash) {
			continue
		}
		r := Result{
			ID:       strings.ToLower(s.Hash),
			Title:    emTags.Replace(s.SongName),
			Artists:  splitArtists(emTags.Replace(s.SingerName), "、"),
			Album:    emTags.Replace(s.AlbumName),
			Duration: float64(s.Duration),
		}
		if cover := s.Trans.UnionCover; strings.Contains(cover, "{size}") {
			r.CoverURL = strings.Replace(cover, "{size}", kugouCoverSize, 1)
			r.ThumbURL = strings.Replace(cover, "{size}", kugouThumbSize, 1)
		}
		out = append(out, r)
	}
	return out, nil
}

func (kugou) lyrics(ctx context.Context, c *client, id string) (*Lyrics, error) {
	if !kugouHash.MatchString(id) {
		return nil, fmt.Errorf("%w: invalid id", ErrInvalid)
	}
	q := url.Values{"ver": {"1"}, "man": {"yes"}, "client": {"pc"}, "hash": {id}}
	data, err := c.get(ctx, kugouLyricSearchURL+"?"+q.Encode(), nil, maxJSONSize)
	if err != nil {
		return nil, err
	}
	var found struct {
		Candidates []struct {
			ID        flexString `json:"id"`
			AccessKey string     `json:"accesskey"`
		} `json:"candidates"`
	}
	if err := decode(data, "lyrics.kugou.com", &found); err != nil {
		return nil, err
	}
	if len(found.Candidates) == 0 {
		return nil, fmt.Errorf("%w: no lyrics", ErrNotFound)
	}
	best := found.Candidates[0]
	if !kugouLyrics.MatchString(string(best.ID)) || !kugouLyrics.MatchString(best.AccessKey) {
		return nil, fmt.Errorf("%w: lyrics.kugou.com sent an invalid candidate", ErrUpstream)
	}
	q = url.Values{
		"ver": {"1"}, "client": {"pc"}, "id": {string(best.ID)}, "accesskey": {best.AccessKey},
		"fmt": {"lrc"}, "charset": {"utf8"},
	}
	data, err = c.get(ctx, kugouLyricDownloadURL+"?"+q.Encode(), nil, maxJSONSize)
	if err != nil {
		return nil, err
	}
	var dl struct {
		Content string `json:"content"`
	}
	if err := decode(data, "lyrics.kugou.com", &dl); err != nil {
		return nil, err
	}
	text, err := base64.StdEncoding.DecodeString(dl.Content)
	if err != nil {
		return nil, fmt.Errorf("%w: lyrics.kugou.com sent invalid lyrics", ErrUpstream)
	}
	return &Lyrics{Text: string(text)}, nil
}
