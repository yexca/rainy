package metasearch

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"rainy/internal/util"
)

// kuwo searches Kuwo's client catalogue. It has no track numbers or dates.
type kuwo struct{}

const (
	kuwoSearchURL = "https://search.kuwo.cn/r.s"
	kuwoLyricURL  = "https://kuwo.cn/openapi/v1/www/lyric/getlyric"
	kuwoCoverURL  = "https://img2.kuwo.cn/star/albumcover/"
)

var (
	kuwoID       = regexp.MustCompile(`^[0-9]{1,20}$`)
	kuwoAlbumPic = regexp.MustCompile(`^[0-9]+/([0-9A-Za-z_/.-]+\.(?:jpe?g|png|webp))$`)
)

func (kuwo) info() ProviderInfo { return ProviderInfo{ID: "kuwo", Lyrics: true, Regions: []string{}} }

func (kuwo) search(ctx context.Context, c *client, query string, limit int, _ string) ([]Result, error) {
	q := url.Values{
		"all": {query}, "client": {"kt"}, "pn": {"0"}, "rn": {strconv.Itoa(limit)},
		"ver": {"kwplayer_ar_9.2.3.2"}, "vipver": {"1"}, "show_copyright_off": {"1"},
		"newver": {"1"}, "correct": {"1"}, "ft": {"music"}, "cluster": {"0"},
		"strategy": {"2012"}, "encoding": {"utf8"}, "rformat": {"json"}, "vermerge": {"1"},
		"mobi": {"1"}, "issubtitle": {"1"},
	}
	data, err := c.get(ctx, kuwoSearchURL+"?"+q.Encode(), nil, maxJSONSize)
	if err != nil {
		return nil, err
	}
	var body struct {
		List []struct {
			ID       flexString `json:"DC_TARGETID"`
			RID      string     `json:"MUSICRID"`
			SongName string     `json:"SONGNAME"`
			Name     string     `json:"NAME"`
			Artist   string     `json:"ARTIST"`
			Album    string     `json:"ALBUM"`
			Duration flexInt    `json:"DURATION"`
			AlbumPic string     `json:"web_albumpic_short"`
		} `json:"abslist"`
	}
	if err := decode(data, "search.kuwo.cn", &body); err != nil {
		return nil, err
	}
	out := make([]Result, 0, len(body.List))
	for _, s := range body.List {
		id := string(s.ID)
		if id == "" {
			id = strings.TrimPrefix(s.RID, "MUSIC_")
		}
		if !kuwoID.MatchString(id) {
			continue
		}
		r := Result{
			ID:       id,
			Title:    util.FirstNonEmpty(s.SongName, s.Name),
			Artists:  splitArtists(s.Artist, "&"),
			Album:    s.Album,
			Duration: float64(s.Duration),
		}
		if m := kuwoAlbumPic.FindStringSubmatch(s.AlbumPic); m != nil {
			r.CoverURL = kuwoCoverURL + "500/" + m[1]
			r.ThumbURL = kuwoCoverURL + "120/" + m[1]
		}
		out = append(out, r)
	}
	return out, nil
}

func (kuwo) lyrics(ctx context.Context, c *client, id string) (*Lyrics, error) {
	if !kuwoID.MatchString(id) {
		return nil, fmt.Errorf("%w: invalid id", ErrInvalid)
	}
	data, err := c.get(ctx, kuwoLyricURL+"?"+url.Values{"musicId": {id}}.Encode(), nil, maxJSONSize)
	if err != nil {
		return nil, err
	}
	var body struct {
		Code flexInt `json:"code"`
		Data *struct {
			Lines []struct {
				Text string     `json:"lineLyric"`
				Time flexString `json:"time"` // seconds, "12.34"
			} `json:"lrclist"`
		} `json:"data"`
	}
	if err := decode(data, "kuwo.cn", &body); err != nil {
		return nil, err
	}
	if body.Data == nil || len(body.Data.Lines) == 0 {
		return nil, fmt.Errorf("%w: no lyrics", ErrNotFound)
	}
	var b strings.Builder
	for _, l := range body.Data.Lines {
		sec, err := strconv.ParseFloat(string(l.Time), 64)
		if err != nil || sec < 0 {
			continue
		}
		b.WriteString(lrcStamp(sec))
		b.WriteString(clean(l.Text))
		b.WriteByte('\n')
	}
	return &Lyrics{Text: b.String()}, nil
}

// lrcStamp formats seconds as an LRC time tag, [mm:ss.xx].
func lrcStamp(sec float64) string {
	cs := int64(sec*100 + 0.5)
	return fmt.Sprintf("[%02d:%02d.%02d]", cs/6000, cs/100%60, cs%100)
}
