package lxmusic

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Search endpoints per catalogue, following lx-music's search code
// (src/renderer/utils/musicSdk/<platform>/musicSearch.js).

// ---- Kuwo

const (
	kuwoSearchURL = "https://search.kuwo.cn/r.s"
	kuwoCoverURL  = "https://img2.kuwo.cn/star/albumcover/500/"
)

var (
	kuwoMinfo    = regexp.MustCompile(`level:(\w+),bitrate:(\d+),format:(\w+),size:([\w.]+)`)
	kuwoAlbumPic = regexp.MustCompile(`^[0-9]+/([0-9A-Za-z_/.-]+\.(?:jpe?g|png|webp))$`)
	numericID    = regexp.MustCompile(`^[0-9]{1,20}$`)
)

func searchKuwo(ctx context.Context, c *apiClient, query string, page, limit int) (*SearchResult, error) {
	q := url.Values{
		"client": {"kt"}, "all": {query}, "pn": {strconv.Itoa(page - 1)}, "rn": {strconv.Itoa(limit)},
		"uid": {"794762570"}, "ver": {"kwplayer_ar_9.2.2.1"}, "vipver": {"1"}, "show_copyright_off": {"1"},
		"newver": {"1"}, "ft": {"music"}, "cluster": {"0"}, "strategy": {"2012"}, "encoding": {"utf8"},
		"rformat": {"json"}, "vermerge": {"1"}, "mobi": {"1"}, "issubtitle": {"1"},
	}
	data, err := c.get(ctx, kuwoSearchURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var body struct {
		Total flexInt `json:"TOTAL"`
		List  []struct {
			RID      string     `json:"MUSICRID"`
			Name     string     `json:"SONGNAME"`
			Artist   string     `json:"ARTIST"`
			Album    string     `json:"ALBUM"`
			AlbumID  flexString `json:"ALBUMID"`
			Duration flexInt    `json:"DURATION"`
			Minfo    string     `json:"N_MINFO"`
			AlbumPic string     `json:"web_albumpic_short"`
		} `json:"abslist"`
	}
	if err := decodeJSON(data, "search.kuwo.cn", &body); err != nil {
		return nil, err
	}
	res := &SearchResult{Total: int(body.Total)}
	for _, it := range body.List {
		id := strings.TrimPrefix(it.RID, "MUSIC_")
		if !numericID.MatchString(id) {
			continue
		}
		sizes := map[string]string{}
		for _, part := range strings.Split(it.Minfo, ";") {
			m := kuwoMinfo.FindStringSubmatch(part)
			if m == nil {
				continue
			}
			q := map[string]string{"4000": "flac24bit", "2000": "flac", "320": "320k", "128": "128k"}[m[2]]
			if q != "" {
				sizes[q] = kuwoSize(m[4])
			}
		}
		song := Song{
			ID:       id,
			Title:    it.Name,
			Artists:  splitArtists(it.Artist, "&"),
			Album:    it.Album,
			AlbumID:  string(it.AlbumID),
			Duration: float64(it.Duration),
			Extra:    map[string]string{},
		}
		for _, q := range Qualities {
			if size, ok := sizes[q]; ok {
				song.Qualities = append(song.Qualities, Quality{Type: q, Size: size})
			}
		}
		if m := kuwoAlbumPic.FindStringSubmatch(it.AlbumPic); m != nil {
			song.CoverURL = kuwoCoverURL + m[1]
		}
		res.Items = append(res.Items, song)
	}
	return res, nil
}

// kuwoSize turns Kuwo's "10.29Mb" into "10.29 MB".
func kuwoSize(s string) string {
	s = strings.ToUpper(s)
	if i := strings.IndexFunc(s, func(r rune) bool { return r >= 'A' && r <= 'Z' }); i > 0 {
		return s[:i] + " " + s[i:]
	}
	return s
}

// ---- Kugou

const (
	kugouSearchURL = "https://songsearch.kugou.com/song_search_v2"
	// The older mobile search (plain HTTP only) answers when the v2 search returns nothing,
	// which it does for a while after many searches from one address. It lacks Hi-Res.
	kugouMobileSearchURL = "http://mobilecdn.kugou.com/api/v3/search/song"
)

type kugouItem struct {
	OriSongName string      `json:"OriSongName"`
	Suffix      string      `json:"Suffix"`
	Singers     []idName    `json:"Singers"`
	SingerName  string      `json:"SingerName"`
	AlbumName   string      `json:"AlbumName"`
	AlbumID     flexString  `json:"AlbumID"`
	AudioID     flexString  `json:"Audioid"`
	MixSongID   flexString  `json:"MixSongID"`
	Duration    flexInt     `json:"Duration"`
	FileHash    string      `json:"FileHash"`
	FileSize    flexInt     `json:"FileSize"`
	HQFileHash  string      `json:"HQFileHash"`
	HQFileSize  flexInt     `json:"HQFileSize"`
	SQFileHash  string      `json:"SQFileHash"`
	SQFileSize  flexInt     `json:"SQFileSize"`
	ResFileHash string      `json:"ResFileHash"`
	ResFileSize flexInt     `json:"ResFileSize"`
	Image       string      `json:"Image"`
	Grp         []kugouItem `json:"Grp"`
}

type idName struct {
	Name string `json:"name"`
	Mid  string `json:"mid"`
}

// searchKugou asks the v2 search (lx-music's) and falls back to the mobile search when it
// fails or finds nothing.
func searchKugou(ctx context.Context, c *apiClient, query string, page, limit int) (*SearchResult, error) {
	res, err := searchKugouV2(ctx, c, query, page, limit)
	if err == nil && len(res.Items) > 0 {
		return res, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	mobile, mobileErr := searchKugouMobile(ctx, c, query, page, limit)
	if mobileErr == nil && (len(mobile.Items) > 0 || err != nil) {
		return mobile, nil
	}
	if err != nil {
		return nil, err
	}
	return res, nil
}

func searchKugouMobile(ctx context.Context, c *apiClient, query string, page, limit int) (*SearchResult, error) {
	q := url.Values{
		"format": {"json"}, "keyword": {query}, "page": {strconv.Itoa(page)},
		"pagesize": {strconv.Itoa(limit)}, "showtype": {"1"},
	}
	data, err := c.get(ctx, kugouMobileSearchURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var body struct {
		Status flexInt `json:"status"`
		Data   struct {
			Total flexInt `json:"total"`
			Info  []struct {
				SongName     string     `json:"songname"`
				SingerName   string     `json:"singername"`
				AlbumName    string     `json:"album_name"`
				AlbumID      flexString `json:"album_id"`
				AudioID      flexString `json:"audio_id"`
				AlbumAudioID flexString `json:"album_audio_id"`
				Duration     flexInt    `json:"duration"`
				Hash         string     `json:"hash"`
				FileSize     flexInt    `json:"filesize"`
				HQHash       string     `json:"320hash"`
				HQFileSize   flexInt    `json:"320filesize"`
				SQHash       string     `json:"sqhash"`
				SQFileSize   flexInt    `json:"sqfilesize"`
				Trans        struct {
					UnionCover string `json:"union_cover"`
				} `json:"trans_param"`
			} `json:"info"`
		} `json:"data"`
	}
	if err := decodeJSON(data, "mobilecdn.kugou.com", &body); err != nil {
		return nil, err
	}
	if body.Status != 1 {
		return nil, fmt.Errorf("%w: mobilecdn.kugou.com answered status %d", ErrUpstream, body.Status)
	}
	em := strings.NewReplacer("<em>", "", "</em>", "")
	res := &SearchResult{Total: int(body.Data.Total)}
	for _, it := range body.Data.Info {
		hash := strings.ToUpper(it.Hash) // the v2 search (and so lx-music) uses upper-case hashes
		if !numericID.MatchString(string(it.AudioID)) || !hexHash.MatchString(hash) {
			continue
		}
		song := Song{
			ID:       string(it.AudioID),
			Title:    em.Replace(it.SongName),
			Artists:  splitArtists(em.Replace(it.SingerName), "、"),
			Album:    em.Replace(it.AlbumName),
			AlbumID:  string(it.AlbumID),
			Duration: float64(it.Duration),
			Extra:    map[string]string{"hash": hash, "albumAudioId": string(it.AlbumAudioID)},
		}
		for _, f := range []struct {
			q    string
			hash string
			size flexInt
		}{{"128k", it.Hash, it.FileSize}, {"320k", it.HQHash, it.HQFileSize}, {"flac", it.SQHash, it.SQFileSize}} {
			if h := strings.ToUpper(f.hash); f.size != 0 && hexHash.MatchString(h) {
				song.Qualities = append(song.Qualities, Quality{Type: f.q, Size: formatSize(int64(f.size)), Hash: h})
			}
		}
		if strings.Contains(it.Trans.UnionCover, "{size}") {
			song.CoverURL = strings.Replace(it.Trans.UnionCover, "{size}", "480", 1)
		}
		res.Items = append(res.Items, song)
	}
	return res, nil
}

func searchKugouV2(ctx context.Context, c *apiClient, query string, page, limit int) (*SearchResult, error) {
	q := url.Values{
		"platform": {"AndroidFilter"}, "iscorrection": {"1"}, "keyword": {query}, "hifiquality": {"0"},
		"pagesize": {strconv.Itoa(limit)}, "PrivilegeFilter": {"0"}, "page": {strconv.Itoa(page)},
	}
	data, err := c.get(ctx, kugouSearchURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var body struct {
		ErrorCode flexInt `json:"error_code"`
		Data      struct {
			Total flexInt     `json:"total"`
			Lists []kugouItem `json:"lists"`
		} `json:"data"`
	}
	if err := decodeJSON(data, "songsearch.kugou.com", &body); err != nil {
		return nil, err
	}
	if body.ErrorCode != 0 {
		return nil, fmt.Errorf("%w: songsearch.kugou.com answered error %d", ErrUpstream, body.ErrorCode)
	}
	res := &SearchResult{Total: int(body.Data.Total)}
	seen := map[string]bool{}
	add := func(it kugouItem) {
		key := string(it.AudioID) + it.FileHash
		if seen[key] || !numericID.MatchString(string(it.AudioID)) || !hexHash.MatchString(it.FileHash) {
			return
		}
		seen[key] = true
		title := it.OriSongName
		if it.Suffix != "" {
			title += " " + it.Suffix
		}
		song := Song{
			ID:       string(it.AudioID),
			Title:    title,
			Album:    it.AlbumName,
			AlbumID:  string(it.AlbumID),
			Duration: float64(it.Duration),
			Extra:    map[string]string{"hash": it.FileHash, "albumAudioId": string(it.MixSongID)},
		}
		for _, s := range it.Singers {
			song.Artists = append(song.Artists, s.Name)
		}
		if len(song.Artists) == 0 {
			song.Artists = splitArtists(it.SingerName, "、")
		}
		for _, f := range []struct {
			q    string
			hash string
			size flexInt
		}{{"128k", it.FileHash, it.FileSize}, {"320k", it.HQFileHash, it.HQFileSize}, {"flac", it.SQFileHash, it.SQFileSize}, {"flac24bit", it.ResFileHash, it.ResFileSize}} {
			if f.size != 0 && hexHash.MatchString(f.hash) {
				song.Qualities = append(song.Qualities, Quality{Type: f.q, Size: formatSize(int64(f.size)), Hash: f.hash})
			}
		}
		if strings.Contains(it.Image, "{size}") {
			song.CoverURL = strings.Replace(it.Image, "{size}", "480", 1)
		}
		res.Items = append(res.Items, song)
	}
	for _, it := range body.Data.Lists {
		add(it)
		for _, child := range it.Grp {
			add(child)
		}
	}
	return res, nil
}

// ---- QQ Music (signed like the desktop client)

const qqSearchURL = "https://u.y.qq.com/cgi-bin/musics.fcg"

var qqMid = regexp.MustCompile(`^[0-9A-Za-z]{8,32}$`)

var (
	qqPart1    = []int{23, 14, 6, 36, 16, 40, 7, 19}
	qqPart2    = []int{16, 1, 32, 12, 19, 27, 8, 5}
	qqScramble = []byte{89, 39, 179, 150, 218, 82, 58, 252, 177, 52, 186, 123, 120, 64, 242, 133, 143, 161, 121, 179}
)

// qqSign computes the zzc signature of a request body (lx-music tx/utils/crypto.js).
func qqSign(body []byte) string {
	sum := sha1.Sum(body)
	h := hex.EncodeToString(sum[:])
	var b strings.Builder
	b.WriteString("zzc")
	pick := func(indexes []int) {
		for _, i := range indexes {
			if i < len(h) { // lx-music reads hash[40] as undefined, which joins as ""
				b.WriteByte(h[i])
			}
		}
	}
	pick(qqPart1)
	part3 := make([]byte, len(qqScramble))
	for i, v := range qqScramble {
		part3[i] = v ^ sum[i]
	}
	b.WriteString(strings.NewReplacer("/", "", "+", "", "=", "").Replace(base64.StdEncoding.EncodeToString(part3)))
	pick(qqPart2)
	return strings.ToLower(b.String())
}

func qqSearchID() string {
	const digits = "0123456789ABCDEF"
	b := make([]byte, 32)
	for i := range b {
		b[i] = digits[rand.IntN(16)]
	}
	return string(b) + fmt.Sprintf("%05d", rand.IntN(100000))
}

func searchQQ(ctx context.Context, c *apiClient, query string, page, limit int) (*SearchResult, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		res, retry, err := searchQQOnce(ctx, c, query, page, limit)
		if err == nil {
			return res, nil
		}
		lastErr = err
		if !retry || ctx.Err() != nil {
			break
		}
	}
	return nil, lastErr
}

func searchQQOnce(ctx context.Context, c *apiClient, query string, page, limit int) (res *SearchResult, retry bool, err error) {
	body, err := json.Marshal(map[string]any{
		"comm": map[string]any{
			"_channelid": "0", "_os_version": "6.2.9200-2", "ct": "19", "cv": "2151",
			"guid": "1F70E520B2EAA7D25E11760783C53CA9", "patch": "118", "psrf_access_token_expiresAt": 0,
			"psrf_qqaccess_token": "", "psrf_qqopenid": "", "psrf_qqunionid": "", "tmeAppID": "qqmusic",
			"tmeLoginType": 0, "uin": "0", "wid": "7223299733393904640",
		},
		"music.search.SearchCgiService": map[string]any{
			"module": "music.search.SearchCgiService",
			"method": "DoSearchForQQMusicDesktop",
			"param": map[string]any{
				"grp": 1, "num_per_page": limit, "page_num": page, "query": query,
				"remoteplace": "txt.newclient.top", "search_type": 0, "searchid": qqSearchID(),
			},
		},
	})
	if err != nil {
		return nil, false, err
	}
	header := http.Header{"User-Agent": {"QQMusic 14090508(android 12)"}, "Content-Type": {"application/json"}}
	data, err := c.post(ctx, qqSearchURL+"?sign="+qqSign(body), header, body)
	if err != nil {
		return nil, false, err
	}
	var out struct {
		Code flexInt `json:"code"`
		Req  struct {
			Code flexInt `json:"code"`
			Data struct {
				Meta struct {
					Sum flexInt `json:"sum"`
				} `json:"meta"`
				Body struct {
					Song struct {
						List []struct {
							ID       flexString `json:"id"`
							Mid      string     `json:"mid"`
							Title    string     `json:"title"`
							Name     string     `json:"name"`
							Interval flexInt    `json:"interval"`
							Singers  []idName   `json:"singer"`
							Album    struct {
								Mid  string `json:"mid"`
								Name string `json:"name"`
							} `json:"album"`
							File struct {
								MediaMid  string  `json:"media_mid"`
								Size128   flexInt `json:"size_128mp3"`
								Size320   flexInt `json:"size_320mp3"`
								SizeFlac  flexInt `json:"size_flac"`
								SizeHiRes flexInt `json:"size_hires"`
							} `json:"file"`
						} `json:"list"`
					} `json:"song"`
				} `json:"body"`
			} `json:"data"`
		} `json:"music.search.SearchCgiService"`
	}
	if err := decodeJSON(data, "u.y.qq.com", &out); err != nil {
		return nil, false, err
	}
	if out.Code != 0 || out.Req.Code != 0 {
		return nil, true, fmt.Errorf("%w: u.y.qq.com answered code %d/%d", ErrUpstream, out.Code, out.Req.Code)
	}
	res = &SearchResult{Total: int(out.Req.Data.Meta.Sum)}
	for _, it := range out.Req.Data.Body.Song.List {
		if !qqMid.MatchString(it.Mid) || !qqMid.MatchString(it.File.MediaMid) {
			continue
		}
		title := it.Title
		if title == "" {
			title = it.Name
		}
		song := Song{
			ID:       it.Mid,
			Title:    title,
			Album:    it.Album.Name,
			AlbumID:  it.Album.Mid,
			Duration: float64(it.Interval),
			Extra:    map[string]string{"strMediaMid": it.File.MediaMid, "albumMid": it.Album.Mid, "songId": string(it.ID)},
		}
		for _, s := range it.Singers {
			song.Artists = append(song.Artists, s.Name)
		}
		for _, f := range []struct {
			q    string
			size flexInt
		}{{"128k", it.File.Size128}, {"320k", it.File.Size320}, {"flac", it.File.SizeFlac}, {"flac24bit", it.File.SizeHiRes}} {
			if f.size != 0 {
				song.Qualities = append(song.Qualities, Quality{Type: f.q, Size: formatSize(int64(f.size))})
			}
		}
		switch {
		case qqMid.MatchString(it.Album.Mid):
			song.CoverURL = "https://y.gtimg.cn/music/photo_new/T002R500x500M000" + it.Album.Mid + ".jpg"
		case len(it.Singers) > 0 && qqMid.MatchString(it.Singers[0].Mid):
			song.CoverURL = "https://y.gtimg.cn/music/photo_new/T001R500x500M000" + it.Singers[0].Mid + ".jpg"
		}
		res.Items = append(res.Items, song)
	}
	return res, false, nil
}

// ---- NetEase Cloud Music

const neteaseSearchURL = "https://music.163.com/api/cloudsearch/pc"

func searchNetEase(ctx context.Context, c *apiClient, query string, page, limit int) (*SearchResult, error) {
	form := url.Values{"s": {query}, "type": {"1"}, "limit": {strconv.Itoa(limit)}, "offset": {strconv.Itoa(limit * (page - 1))}}
	header := http.Header{"Referer": {"https://music.163.com/"}, "Content-Type": {"application/x-www-form-urlencoded"}}
	data, err := c.post(ctx, neteaseSearchURL, header, []byte(form.Encode()))
	if err != nil {
		return nil, err
	}
	type size struct {
		Size flexInt `json:"size"`
	}
	var body struct {
		Code   flexInt `json:"code"`
		Result struct {
			SongCount flexInt `json:"songCount"`
			Songs     []struct {
				ID       flexString `json:"id"`
				Name     string     `json:"name"`
				Duration flexInt    `json:"dt"` // ms
				Artists  []idName   `json:"ar"`
				Album    struct {
					ID     flexString `json:"id"`
					Name   string     `json:"name"`
					PicURL string     `json:"picUrl"`
				} `json:"al"`
				H         *size `json:"h"`
				L         *size `json:"l"`
				SQ        *size `json:"sq"`
				HR        *size `json:"hr"`
				Privilege *struct {
					MaxBR      flexInt `json:"maxbr"`
					MaxBRLevel string  `json:"maxBrLevel"`
				} `json:"privilege"`
			} `json:"songs"`
		} `json:"result"`
	}
	if err := decodeJSON(data, "music.163.com", &body); err != nil {
		return nil, err
	}
	if body.Code != 200 && body.Code != 0 {
		return nil, fmt.Errorf("%w: music.163.com answered code %d", ErrUpstream, body.Code)
	}
	sizeOf := func(s *size) string {
		if s == nil {
			return ""
		}
		return formatSize(int64(s.Size))
	}
	res := &SearchResult{Total: int(body.Result.SongCount)}
	for _, it := range body.Result.Songs {
		if !numericID.MatchString(string(it.ID)) {
			continue
		}
		song := Song{
			ID:       string(it.ID),
			Title:    it.Name,
			Album:    it.Album.Name,
			AlbumID:  string(it.Album.ID),
			Duration: float64(it.Duration) / 1000,
			CoverURL: it.Album.PicURL,
			Extra:    map[string]string{},
		}
		for _, a := range it.Artists {
			song.Artists = append(song.Artists, a.Name)
		}
		// lx-music derives the qualities from the privilege: maxbr 999000 → up to FLAC,
		// 320000 → up to 320k, otherwise 128k; maxBrLevel "hires" adds Hi-Res.
		has := map[string]bool{}
		if p := it.Privilege; p != nil {
			switch {
			case p.MaxBR >= 999000:
				has["flac"], has["320k"], has["128k"] = true, true, true
			case p.MaxBR >= 320000:
				has["320k"], has["128k"] = true, true
			case p.MaxBR > 0:
				has["128k"] = true
			}
			if p.MaxBRLevel == "hires" {
				has["flac24bit"] = true
			}
		} else {
			has["128k"], has["320k"], has["flac"], has["flac24bit"] = it.L != nil, it.H != nil, it.SQ != nil, it.HR != nil
		}
		sizes := map[string]string{"128k": sizeOf(it.L), "320k": sizeOf(it.H), "flac": sizeOf(it.SQ), "flac24bit": sizeOf(it.HR)}
		for _, q := range Qualities {
			if has[q] {
				song.Qualities = append(song.Qualities, Quality{Type: q, Size: sizes[q]})
			}
		}
		res.Items = append(res.Items, song)
	}
	return res, nil
}

// ---- Migu (signed like the Android client)

const (
	miguSearchURL = "https://jadeite.migu.cn/music_search/v3/search/searchAll"
	miguDeviceID  = "963B7AA0D21511ED807EE5846EC87D20"
	miguSignSalt  = "6cdc72a439cef99a3418d2a78aa28c73" // public, from the Android client (lx-music mg/musicSearch.js)
	miguImageBase = "https://d.musicapp.migu.cn"
)

func searchMigu(ctx context.Context, c *apiClient, query string, page, limit int) (*SearchResult, error) {
	now := strconv.FormatInt(time.Now().UnixMilli(), 10)
	sum := md5.Sum([]byte(query + miguSignSalt + "yyapp2d16148780a1dcc7408e06336b98cfd50" + miguDeviceID + now))
	q := url.Values{
		"isCorrect": {"0"}, "isCopyright": {"1"},
		"searchSwitch": {`{"song":1,"album":0,"singer":0,"tagSong":1,"mvSong":0,"bestShow":1,"songlist":0,"lyricSong":0}`},
		"pageSize":     {strconv.Itoa(limit)}, "text": {query}, "pageNo": {strconv.Itoa(page)}, "sort": {"0"}, "sid": {"USS"},
	}
	header := http.Header{
		"uiVersion": {"A_music_3.6.1"}, "deviceId": {miguDeviceID}, "timestamp": {now},
		"sign": {hex.EncodeToString(sum[:])}, "channel": {"0146921"},
		"User-Agent": {"Mozilla/5.0 (Linux; U; Android 11.0.0; zh-cn; MI 11 Build/OPR1.170623.032) AppleWebKit/534.30 (KHTML, like Gecko) Version/4.0 Mobile Safari/534.30"},
	}
	data, err := c.get(ctx, miguSearchURL+"?"+q.Encode(), header)
	if err != nil {
		return nil, err
	}
	var body struct {
		Code string `json:"code"`
		Info string `json:"info"`
		Data *struct {
			Total flexInt `json:"totalCount"`
			List  [][]struct {
				SongID      flexString `json:"songId"`
				CopyrightID flexString `json:"copyrightId"`
				Name        string     `json:"name"`
				Singers     []idName   `json:"singerList"`
				Album       string     `json:"album"`
				AlbumID     flexString `json:"albumId"`
				Duration    flexInt    `json:"duration"`
				Img1        string     `json:"img1"`
				Img2        string     `json:"img2"`
				Img3        string     `json:"img3"`
				LrcURL      string     `json:"lrcUrl"`
				MrcURL      string     `json:"mrcurl"`
				TrcURL      string     `json:"trcUrl"`
				Formats     []struct {
					Type  string  `json:"formatType"`
					ASize flexInt `json:"asize"`
					ISize flexInt `json:"isize"`
				} `json:"audioFormats"`
			} `json:"resultList"`
		} `json:"songResultData"`
	}
	if err := decodeJSON(data, "jadeite.migu.cn", &body); err != nil {
		return nil, err
	}
	if body.Code != "000000" {
		return nil, fmt.Errorf("%w: jadeite.migu.cn answered code %q", ErrUpstream, body.Code)
	}
	res := &SearchResult{}
	if body.Data == nil {
		return res, nil
	}
	res.Total = int(body.Data.Total)
	seen := map[string]bool{}
	for _, group := range body.Data.List {
		for _, it := range group {
			id, cid := string(it.SongID), string(it.CopyrightID)
			if !validMiguID(id) || !validMiguID(cid) || seen[cid] {
				continue
			}
			seen[cid] = true
			song := Song{
				ID:       id,
				Title:    it.Name,
				Album:    it.Album,
				AlbumID:  string(it.AlbumID),
				Duration: float64(it.Duration),
				Extra:    map[string]string{"copyrightId": cid},
			}
			for _, s := range it.Singers {
				song.Artists = append(song.Artists, s.Name)
			}
			for k, v := range map[string]string{"lrcUrl": it.LrcURL, "mrcUrl": it.MrcURL, "trcUrl": it.TrcURL} {
				if u := httpsURL(v); strings.HasPrefix(u, "https://") && len(u) <= 512 {
					song.Extra[k] = u
				}
			}
			sizes := map[string]flexInt{}
			for _, f := range it.Formats {
				q := map[string]string{"PQ": "128k", "HQ": "320k", "SQ": "flac", "ZQ24": "flac24bit"}[f.Type]
				if q == "" {
					continue
				}
				sizes[q] = f.ASize
				if sizes[q] == 0 {
					sizes[q] = f.ISize
				}
			}
			for _, q := range Qualities {
				if n, ok := sizes[q]; ok {
					song.Qualities = append(song.Qualities, Quality{Type: q, Size: formatSize(int64(n))})
				}
			}
			for _, img := range []string{it.Img3, it.Img2, it.Img1} {
				if img == "" {
					continue
				}
				if strings.HasPrefix(img, "/") {
					img = miguImageBase + img
				}
				song.CoverURL = img
				break
			}
			res.Items = append(res.Items, song)
		}
	}
	return res, nil
}

var miguID = regexp.MustCompile(`^[0-9A-Za-z]{1,32}$`)

func validMiguID(s string) bool { return miguID.MatchString(s) }
