package metasearch

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"rainy/internal/util"
)

// qqMusic searches QQ Music through the desktop client's search service, falling back to
// the mobile search.
type qqMusic struct{}

const (
	qqSearchURL       = "https://u.y.qq.com/cgi-bin/musicu.fcg"
	qqMobileSearchURL = "https://shc.y.qq.com/soso/fcgi-bin/search_for_qq_cp"
	qqLyricURL        = "https://c.y.qq.com/lyric/fcgi-bin/fcg_query_lyric_new.fcg"
	qqCoverURL        = "https://y.gtimg.cn/music/photo_new/T002R%dx%dM000%s.jpg"
)

// qqMid matches QQ Music song and album mids.
var qqMid = regexp.MustCompile(`^[0-9A-Za-z]{8,32}$`)

func (qqMusic) info() ProviderInfo { return ProviderInfo{ID: "qq", Lyrics: true, Regions: []string{}} }

func qqHeader() http.Header {
	return http.Header{"Referer": {"https://y.qq.com/"}, "Origin": {"https://y.qq.com"}}
}

type qqSong struct {
	Mid        string   `json:"mid"`
	Name       string   `json:"name"`
	Title      string   `json:"title"`
	Interval   flexInt  `json:"interval"` // seconds
	IndexAlbum flexInt  `json:"index_album"`
	IndexCD    flexInt  `json:"index_cd"` // 0-based
	TimePublic string   `json:"time_public"`
	Singers    []idName `json:"singer"`
	Album      struct {
		Mid        string `json:"mid"`
		Name       string `json:"name"`
		Title      string `json:"title"`
		TimePublic string `json:"time_public"`
	} `json:"album"`
}

// search asks the desktop client's search service first. That service now and then refuses
// a caller for a while (service code 2001) while the older mobile search keeps answering, so a
// refusal falls back to it; the fallback lacks track and disc numbers.
func (qqMusic) search(ctx context.Context, c *client, query string, limit int, _ string) ([]Result, error) {
	results, refused, err := qqDesktopSearch(ctx, c, query, limit)
	if refused {
		return qqMobileSearch(ctx, c, query, limit)
	}
	return results, err
}

// qqDesktopSearch runs the desktop search; refused reports a service-level refusal.
func qqDesktopSearch(ctx context.Context, c *client, query string, limit int) (results []Result, refused bool, err error) {
	reqBody, err := json.Marshal(map[string]any{
		"req": map[string]any{
			"module": "music.search.SearchCgiService",
			"method": "DoSearchForQQMusicDesktop",
			"param": map[string]any{
				"search_type":  0,
				"query":        query,
				"page_num":     1,
				"num_per_page": limit,
			},
		},
	})
	if err != nil {
		return nil, false, err
	}
	h := qqHeader()
	h.Set("Content-Type", "application/json")
	data, err := c.post(ctx, qqSearchURL, h, string(reqBody), maxJSONSize)
	if err != nil {
		return nil, false, err
	}
	var body struct {
		Code int `json:"code"`
		Req  struct {
			Code int `json:"code"`
			Data struct {
				Body struct {
					Song struct {
						List []qqSong `json:"list"`
					} `json:"song"`
				} `json:"body"`
			} `json:"data"`
		} `json:"req"`
	}
	if err := decode(data, "u.y.qq.com", &body); err != nil {
		return nil, false, err
	}
	if body.Code != 0 || body.Req.Code != 0 {
		return nil, true, fmt.Errorf("%w: u.y.qq.com answered code %d/%d", ErrUpstream, body.Code, body.Req.Code)
	}
	out := make([]Result, 0, len(body.Req.Data.Body.Song.List))
	for _, s := range body.Req.Data.Body.Song.List {
		r := Result{
			ID:          s.Mid,
			Title:       util.FirstNonEmpty(s.Title, s.Name),
			Album:       util.FirstNonEmpty(s.Album.Title, s.Album.Name),
			TrackNumber: int(s.IndexAlbum),
			Date:        normalizeDate(util.FirstNonEmpty(s.TimePublic, s.Album.TimePublic)),
			Duration:    float64(s.Interval),
		}
		for _, a := range s.Singers {
			r.Artists = append(r.Artists, a.Name)
		}
		if qqMid.MatchString(s.Album.Mid) {
			r.DiscNumber = int(s.IndexCD) + 1
		}
		qqSetCover(&r, s.Album.Mid)
		out = append(out, r)
	}
	return out, false, nil
}

// qqMobileSearch runs the older mobile ("soso") search.
func qqMobileSearch(ctx context.Context, c *client, query string, limit int) ([]Result, error) {
	q := url.Values{
		"format": {"json"}, "w": {query}, "p": {"1"}, "n": {strconv.Itoa(limit)}, "cr": {"1"},
		"t": {"0"}, "aggr": {"0"}, "flag": {"1"}, "ie": {"utf-8"}, "inCharset": {"utf-8"},
		"outCharset": {"utf-8"}, "g_tk": {"5381"}, "platform": {"h5"}, "needNewCode": {"1"},
	}
	data, err := c.get(ctx, qqMobileSearchURL+"?"+q.Encode(), qqHeader(), maxJSONSize)
	if err != nil {
		return nil, err
	}
	var body struct {
		Code int `json:"code"`
		Data struct {
			Song struct {
				List []struct {
					SongMid   string   `json:"songmid"`
					SongName  string   `json:"songname"`
					AlbumMid  string   `json:"albummid"`
					AlbumName string   `json:"albumname"`
					Interval  flexInt  `json:"interval"` // seconds
					PubTime   int64    `json:"pubtime"`  // unix seconds
					Singers   []idName `json:"singer"`
				} `json:"list"`
			} `json:"song"`
		} `json:"data"`
	}
	if err := decode(data, "shc.y.qq.com", &body); err != nil {
		return nil, err
	}
	if body.Code != 0 {
		return nil, fmt.Errorf("%w: shc.y.qq.com answered code %d", ErrUpstream, body.Code)
	}
	out := make([]Result, 0, len(body.Data.Song.List))
	for _, s := range body.Data.Song.List {
		r := Result{
			ID:       s.SongMid,
			Title:    s.SongName,
			Album:    s.AlbumName,
			Date:     dateFromMillis(s.PubTime * 1000),
			Duration: float64(s.Interval),
		}
		for _, a := range s.Singers {
			r.Artists = append(r.Artists, a.Name)
		}
		qqSetCover(&r, s.AlbumMid)
		out = append(out, r)
	}
	return out, nil
}

func qqSetCover(r *Result, albumMid string) {
	if qqMid.MatchString(albumMid) {
		r.CoverURL = fmt.Sprintf(qqCoverURL, 800, 800, albumMid)
		r.ThumbURL = fmt.Sprintf(qqCoverURL, 150, 150, albumMid)
	}
}

func (qqMusic) lyrics(ctx context.Context, c *client, id string) (*Lyrics, error) {
	if !qqMid.MatchString(id) {
		return nil, fmt.Errorf("%w: invalid id", ErrInvalid)
	}
	q := url.Values{"songmid": {id}, "g_tk": {"5381"}, "format": {"json"}}
	data, err := c.get(ctx, qqLyricURL+"?"+q.Encode(), qqHeader(), maxJSONSize)
	if err != nil {
		return nil, err
	}
	var body struct {
		Code  int    `json:"code"`
		Lyric string `json:"lyric"`
		Trans string `json:"trans"`
	}
	if err := decode(data, "c.y.qq.com", &body); err != nil {
		return nil, err
	}
	if body.Code != 0 || body.Lyric == "" {
		return nil, fmt.Errorf("%w: no lyrics", ErrNotFound)
	}
	text, err := base64.StdEncoding.DecodeString(body.Lyric)
	if err != nil {
		return nil, fmt.Errorf("%w: c.y.qq.com sent invalid lyrics", ErrUpstream)
	}
	trans, _ := base64.StdEncoding.DecodeString(body.Trans)
	return &Lyrics{Text: string(text), Translation: string(trans)}, nil
}
