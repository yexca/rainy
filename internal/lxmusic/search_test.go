package lxmusic

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"testing"
)

func mustAddr(t *testing.T, s string) netip.Addr {
	t.Helper()
	a, err := netip.ParseAddr(s)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// fakeAPI answers requests by host with canned JSON and records them; it never dials a server.
type fakeAPI struct {
	bodies map[string]string // host → body
	got    []*http.Request
	sent   []string
}

func (f *fakeAPI) RoundTrip(r *http.Request) (*http.Response, error) {
	f.got = append(f.got, r)
	if r.Body != nil {
		b, _ := io.ReadAll(r.Body)
		f.sent = append(f.sent, string(b))
	}
	body, ok := f.bodies[r.URL.Hostname()]
	if !ok {
		return nil, fmt.Errorf("unexpected host %s", r.URL.Hostname())
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}, Request: r}, nil
}

func searchWith(t *testing.T, api *fakeAPI, platform string) *SearchResult {
	t.Helper()
	s := &Service{fetch: &http.Client{Transport: api}}
	res, err := s.Search(context.Background(), platform, "  synthetic   query ", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestSearchKuwo(t *testing.T) {
	api := &fakeAPI{bodies: map[string]string{"search.kuwo.cn": `{"TOTAL":"2","abslist":[
		{"MUSICRID":"MUSIC_111","SONGNAME":"Rain &amp; Sun","ARTIST":"Singer A&Singer B","ALBUM":"Album","ALBUMID":"9","DURATION":"201",
		 "N_MINFO":"level:ff,bitrate:2000,format:flac,size:20.5Mb;level:p,bitrate:320,format:mp3,size:7.7Mb;level:h,bitrate:128,format:mp3,size:3.1Mb;level:zp,bitrate:20000,format:zp,size:zpMb",
		 "web_albumpic_short":"120/s3s1/1/123.jpg"},
		{"MUSICRID":"MUSIC_bad","SONGNAME":"Broken"}]}`}}
	res := searchWith(t, api, Kuwo)
	if q := api.got[0].URL.Query(); q.Get("all") != "synthetic query" || q.Get("pn") != "0" || q.Get("rn") != "10" {
		t.Fatalf("query %v", q)
	}
	if res.Total != 2 || len(res.Items) != 1 {
		t.Fatalf("result %+v", res)
	}
	s := res.Items[0]
	if s.ID != "111" || s.Title != "Rain & Sun" || fmt.Sprint(s.Artists) != "[Singer A Singer B]" || s.Duration != 201 ||
		s.CoverURL != "https://img2.kuwo.cn/star/albumcover/500/s3s1/1/123.jpg" || s.AlbumID != "9" {
		t.Fatalf("song %+v", s)
	}
	if fmt.Sprint(s.Qualities) != "[{128k 3.1 MB } {320k 7.7 MB } {flac 20.5 MB }]" {
		t.Fatalf("qualities %v", s.Qualities)
	}
}

func TestSearchKugou(t *testing.T) {
	api := &fakeAPI{bodies: map[string]string{"songsearch.kugou.com": `{"error_code":0,"data":{"total":5,"lists":[
		{"OriSongName":"Song","Suffix":"(Live)","Singers":[{"name":"A"},{"name":"B"}],"AlbumName":"Album","AlbumID":"7","Audioid":42,"MixSongID":"4242","Duration":180,
		 "FileHash":"0123456789ABCDEF0123456789ABCDEF","FileSize":1048576,"HQFileHash":"1123456789ABCDEF0123456789ABCDEF","HQFileSize":2097152,
		 "SQFileHash":"","SQFileSize":0,"ResFileHash":"2123456789ABCDEF0123456789ABCDEF","ResFileSize":3145728,
		 "Image":"http://imge.kugou.com/stdmusic/{size}/1.jpg",
		 "Grp":[{"OriSongName":"Song","Singers":[{"name":"C"}],"Audioid":43,"FileHash":"3123456789ABCDEF0123456789ABCDEF","FileSize":1},
		        {"OriSongName":"Dup","Audioid":42,"FileHash":"0123456789ABCDEF0123456789ABCDEF","FileSize":1}]}]}}`}}
	res := searchWith(t, api, Kugou)
	if len(res.Items) != 2 {
		t.Fatalf("items %+v", res.Items)
	}
	s := res.Items[0]
	if s.ID != "42" || s.Title != "Song (Live)" || s.Extra["hash"] != "0123456789ABCDEF0123456789ABCDEF" || s.Extra["albumAudioId"] != "4242" ||
		s.CoverURL != "https://imge.kugou.com/stdmusic/480/1.jpg" {
		t.Fatalf("song %+v", s)
	}
	if fmt.Sprint(s.Qualities) != "[{128k 1.00 MB 0123456789ABCDEF0123456789ABCDEF} {320k 2.00 MB 1123456789ABCDEF0123456789ABCDEF} {flac24bit 3.00 MB 2123456789ABCDEF0123456789ABCDEF}]" {
		t.Fatalf("qualities %v", s.Qualities)
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("a search result does not validate: %v", err)
	}
}

func TestSearchKugouFallsBackToMobileSearch(t *testing.T) {
	api := &fakeAPI{bodies: map[string]string{
		"songsearch.kugou.com": `{"error_code":0,"data":{"total":0,"lists":[]}}`,
		"mobilecdn.kugou.com": `{"status":1,"data":{"total":7,"info":[
			{"songname":"<em>Song</em>","singername":"A、B","album_name":"Album","album_id":"7","audio_id":42,"album_audio_id":4242,"duration":180,
			 "hash":"0123456789abcdef0123456789abcdef","filesize":1048576,"320hash":"1123456789abcdef0123456789abcdef","320filesize":2097152,
			 "sqhash":"","sqfilesize":0,"trans_param":{"union_cover":"http://imge.kugou.com/stdmusic/{size}/1.jpg"}}]}}`,
	}}
	res := searchWith(t, api, Kugou)
	if len(api.got) != 2 || api.got[1].URL.Hostname() != "mobilecdn.kugou.com" || res.Total != 7 || len(res.Items) != 1 {
		t.Fatalf("requests %d, result %+v", len(api.got), res)
	}
	s := res.Items[0]
	if s.ID != "42" || s.Title != "Song" || fmt.Sprint(s.Artists) != "[A B]" || s.Extra["hash"] != "0123456789ABCDEF0123456789ABCDEF" ||
		s.Extra["albumAudioId"] != "4242" || s.CoverURL != "https://imge.kugou.com/stdmusic/480/1.jpg" ||
		fmt.Sprint(s.Qualities) != "[{128k 1.00 MB 0123456789ABCDEF0123456789ABCDEF} {320k 2.00 MB 1123456789ABCDEF0123456789ABCDEF}]" {
		t.Fatalf("song %+v", s)
	}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestSearchQQSignsRequests(t *testing.T) {
	if got := qqSign([]byte(`{"a":1}`)); got != "zzc7746109xq501htmv4ipz7c8owlxfpihjm1fd695c7" {
		t.Fatalf("qqSign = %s", got)
	}
	if got := qqSign([]byte("synthetic body")); got != "zzcaad07a2vc5rtbkrhvrhhltx73qhwbbbd3w7d3224c8" {
		t.Fatalf("qqSign = %s", got)
	}
	api := &fakeAPI{bodies: map[string]string{"u.y.qq.com": `{"code":0,"music.search.SearchCgiService":{"code":0,"data":{"meta":{"sum":3},"body":{"song":{"list":[
		{"id":101,"mid":"000aaaaaaaaaaa","title":"Title","name":"Name","interval":269,"singer":[{"name":"S","mid":"000bbbbbbbbbbb"}],
		 "album":{"mid":"000ccccccccccc","name":"Album"},"file":{"media_mid":"000ddddddddddd","size_128mp3":100,"size_320mp3":200,"size_flac":0,"size_hires":0}},
		{"id":1,"mid":"000eeeeeeeeeee","title":"No media","album":{},"file":{}}]}}}}}`}}
	res := searchWith(t, api, QQ)
	if !strings.HasPrefix(api.got[0].URL.RawQuery, "sign=zzc") || api.got[0].URL.Query().Get("sign") != qqSign([]byte(api.sent[0])) {
		t.Fatalf("request %s", api.got[0].URL)
	}
	if len(res.Items) != 1 || res.Total != 3 {
		t.Fatalf("result %+v", res)
	}
	s := res.Items[0]
	if s.ID != "000aaaaaaaaaaa" || s.Extra["strMediaMid"] != "000ddddddddddd" || s.Extra["songId"] != "101" ||
		s.CoverURL != "https://y.gtimg.cn/music/photo_new/T002R500x500M000000ccccccccccc.jpg" || len(s.Qualities) != 2 {
		t.Fatalf("song %+v", s)
	}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestSearchNetEase(t *testing.T) {
	api := &fakeAPI{bodies: map[string]string{"music.163.com": `{"code":200,"result":{"songCount":9,"songs":[
		{"id":101,"name":"Song","dt":278961,"ar":[{"name":"A"}],"al":{"id":201,"name":"Album","picUrl":"http://p1.music.126.net/x.jpg"},
		 "h":{"size":11161644},"l":{"size":4464684},"sq":{"size":56037067},"hr":{"size":47185529},"privilege":{"maxbr":999000,"maxBrLevel":"hires"}},
		{"id":7,"name":"Low","ar":[],"al":{},"l":{"size":1},"privilege":{"maxbr":128000,"maxBrLevel":"standard"}}]}}`}}
	res := searchWith(t, api, NetEase)
	if api.sent[0] != "limit=10&offset=0&s=synthetic+query&type=1" {
		t.Fatalf("form %s", api.sent[0])
	}
	if len(res.Items) != 2 {
		t.Fatalf("items %+v", res.Items)
	}
	s := res.Items[0]
	if s.Duration != 278.961 || s.CoverURL != "https://p1.music.126.net/x.jpg" || len(s.Qualities) != 4 || s.Qualities[3].Size != "45.00 MB" {
		t.Fatalf("song %+v", s)
	}
	if info := s.MusicInfo(); info["songmid"] != int64(101) || info["albumId"] != int64(201) {
		t.Fatalf("musicInfo %v", info)
	}
	if q := res.Items[1].Qualities; len(q) != 1 || q[0].Type != "128k" {
		t.Fatalf("low %v", q)
	}
}

func TestSearchMigu(t *testing.T) {
	api := &fakeAPI{bodies: map[string]string{"jadeite.migu.cn": `{"code":"000000","songResultData":{"totalCount":"4","resultList":[[
		{"songId":"101","copyrightId":"11111111111","name":"Song","singerList":[{"name":"A"}],"album":"Album","albumId":"201","duration":270,
		 "img3":"/data/oss/resource/x.webp","lrcUrl":"https://d.musicapp.migu.cn/lrc","mrcurl":"javascript:x",
		 "audioFormats":[{"formatType":"PQ","asize":"4317311"},{"formatType":"SQ","asize":"0","isize":"31931278"},{"formatType":"Z3D","asize":"1"}]},
		{"songId":"101","copyrightId":"11111111111","name":"Duplicate"}]]}}`}}
	res := searchWith(t, api, Migu)
	h := api.got[0].Header
	// The Android client's header names are sent exactly as written (not canonicalised).
	exact := func(name string) string {
		for k, v := range h {
			if k == name && len(v) == 1 {
				return v[0]
			}
		}
		return ""
	}
	if exact("deviceId") == "" || len(exact("sign")) != 32 || exact("timestamp") == "" {
		t.Fatalf("headers %v", h)
	}
	if len(res.Items) != 1 || res.Total != 4 {
		t.Fatalf("result %+v", res)
	}
	s := res.Items[0]
	if s.CoverURL != "https://d.musicapp.migu.cn/data/oss/resource/x.webp" || s.Extra["copyrightId"] != "11111111111" ||
		s.Extra["lrcUrl"] != "https://d.musicapp.migu.cn/lrc" || s.Extra["mrcUrl"] != "" || fmt.Sprint(s.Qualities) != "[{128k 4.12 MB } {flac 30.45 MB }]" {
		t.Fatalf("song %+v", s)
	}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestSearchValidates(t *testing.T) {
	s := &Service{fetch: &http.Client{Transport: &fakeAPI{}}}
	for _, c := range []struct{ platform, q string }{{"xm", "a"}, {Kuwo, "  "}, {Kuwo, strings.Repeat("长", MaxQueryLength+1)}} {
		if _, err := s.Search(context.Background(), c.platform, c.q, 1, 10); err == nil {
			t.Errorf("search %q %q accepted", c.platform, c.q)
		}
	}
}
