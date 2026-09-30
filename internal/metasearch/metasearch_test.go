package metasearch

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

// fakeWeb answers requests by host without touching the network and records them.
type fakeWeb struct {
	handlers map[string]http.HandlerFunc
	requests []*http.Request
	bodies   []string
}

func (f *fakeWeb) RoundTrip(r *http.Request) (*http.Response, error) {
	var body string
	if r.Body != nil {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		r.Body = io.NopCloser(bytes.NewReader(b))
	}
	f.requests = append(f.requests, r)
	f.bodies = append(f.bodies, body)
	h, ok := f.handlers[r.URL.Host]
	if !ok {
		return nil, errors.New("fakeWeb: unexpected host " + r.URL.Host)
	}
	rec := httptest.NewRecorder()
	h(rec, r)
	resp := rec.Result()
	resp.Request = r
	return resp, nil
}

func newFake(handlers map[string]http.HandlerFunc) (*Service, *fakeWeb) {
	f := &fakeWeb{handlers: handlers}
	return New(&http.Client{Transport: f}), f
}

func text(s string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, s) }
}

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

func TestProviders(t *testing.T) {
	s := New(nil)
	var ids []string
	for _, p := range s.Providers() {
		ids = append(ids, p.ID)
		if p.Regions == nil {
			t.Errorf("%s: regions must be a list", p.ID)
		}
	}
	if want := []string{"netease", "qq", "kugou", "kuwo", "itunes"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("providers = %v, want %v", ids, want)
	}
}

func TestSearchNetease(t *testing.T) {
	s, f := newFake(map[string]http.HandlerFunc{
		"music.163.com": text(`{"code":200,"result":{"songs":[
			{"id":101,"name":"Synthetic &amp; Song","no":3,"cd":"02","dt":201500,"publishTime":1577808000000,
			 "ar":[{"name":"Artist A"},{"name":"Artist B"}],
			 "al":{"name":"Test Album","picUrl":"http://p1.music.126.net/abc==/1.jpg"}},
			{"id":102,"name":"","ar":[]}
		]}}`),
	})
	got, err := s.Search(context.Background(), "netease", "  synthetic   song ", 5, "", Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := []Result{{
		Provider: "netease", ID: "101", Title: "Synthetic & Song", Artists: []string{"Artist A", "Artist B"},
		Album: "Test Album", TrackNumber: 3, DiscNumber: 2, Date: "2020-01-01", Duration: 201.5,
		CoverURL: "https://p1.music.126.net/abc==/1.jpg", ThumbURL: "https://p1.music.126.net/abc==/1.jpg?param=150y150",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
	r := f.requests[0]
	if r.Method != http.MethodPost || r.URL.Path != "/api/cloudsearch/pc" || r.Header.Get("Referer") == "" {
		t.Fatalf("request = %s %s (referer %q)", r.Method, r.URL, r.Header.Get("Referer"))
	}
	if !strings.Contains(f.bodies[0], "s=synthetic+song") || !strings.Contains(f.bodies[0], "limit=5") {
		t.Fatalf("body = %q", f.bodies[0])
	}
}

func TestSearchQQ(t *testing.T) {
	s, f := newFake(map[string]http.HandlerFunc{
		"u.y.qq.com": text(`{"code":0,"req":{"code":0,"data":{"body":{"song":{"list":[
			{"mid":"000aaaaaaaaaaa","name":"Name","title":"Title","interval":180,"index_album":4,"index_cd":1,
			 "time_public":"2019-05-06","singer":[{"name":"Singer"}],"album":{"mid":"000bbbbbbbbbbb","name":"Album"}},
			{"mid":"000ccccccccccc","title":"No Album","singer":[],"album":{"mid":"","name":""}}
		]}}}}}`),
	})
	got, err := s.Search(context.Background(), "qq", "query", 0, "", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d results", len(got))
	}
	want := Result{
		Provider: "qq", ID: "000aaaaaaaaaaa", Title: "Title", Artists: []string{"Singer"}, Album: "Album",
		TrackNumber: 4, DiscNumber: 2, Date: "2019-05-06", Duration: 180,
		CoverURL: "https://y.gtimg.cn/music/photo_new/T002R800x800M000000bbbbbbbbbbb.jpg",
		ThumbURL: "https://y.gtimg.cn/music/photo_new/T002R150x150M000000bbbbbbbbbbb.jpg",
	}
	if !reflect.DeepEqual(got[0], want) {
		t.Fatalf("got  %+v\nwant %+v", got[0], want)
	}
	if got[1].CoverURL != "" || got[1].DiscNumber != 0 || got[1].Artists == nil {
		t.Fatalf("result without album = %+v", got[1])
	}
	// The query travels as JSON, never spliced into a URL or format string.
	if !strings.Contains(f.bodies[0], `"query":"query"`) || !strings.Contains(f.bodies[0], `"num_per_page":20`) {
		t.Fatalf("body = %s", f.bodies[0])
	}
}

func TestSearchQQFallsBackWhenRefused(t *testing.T) {
	s, f := newFake(map[string]http.HandlerFunc{
		"u.y.qq.com": text(`{"code":0,"req":{"code":2001}}`),
		"shc.y.qq.com": text(`{"code":0,"data":{"song":{"list":[{"songmid":"000aaaaaaaaaaa","songname":"Song",
			"albummid":"000bbbbbbbbbbb","albumname":"Album","interval":200,"pubtime":1577808000,"singer":[{"name":"Singer"}]}]}}}`),
	})
	got, err := s.Search(context.Background(), "qq", "q", 5, "", Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := []Result{{
		Provider: "qq", ID: "000aaaaaaaaaaa", Title: "Song", Artists: []string{"Singer"}, Album: "Album",
		Date: "2020-01-01", Duration: 200,
		CoverURL: "https://y.gtimg.cn/music/photo_new/T002R800x800M000000bbbbbbbbbbb.jpg",
		ThumbURL: "https://y.gtimg.cn/music/photo_new/T002R150x150M000000bbbbbbbbbbb.jpg",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
	if len(f.requests) != 2 || f.requests[1].URL.Query().Get("w") != "q" || f.requests[1].URL.Query().Get("n") != "5" {
		t.Fatalf("requests = %v", f.requests)
	}

	// Both refusing is an upstream error.
	s, _ = newFake(map[string]http.HandlerFunc{
		"u.y.qq.com":   text(`{"code":0,"req":{"code":2001}}`),
		"shc.y.qq.com": text(`{"code":-1}`),
	})
	if _, err := s.Search(context.Background(), "qq", "q", 5, "", Options{}); !errors.Is(err, ErrUpstream) {
		t.Fatalf("err = %v, want ErrUpstream", err)
	}
}

func TestSearchKugouAndKuwo(t *testing.T) {
	s, _ := newFake(map[string]http.HandlerFunc{
		"mobilecdn.kugou.com": text(`{"status":1,"data":{"info":[
			{"hash":"0123456789ABCDEF0123456789ABCDEF","songname":"<em>Song</em>","singername":"A、B","album_name":"Alb","duration":200,
			 "trans_param":{"union_cover":"http://imge.kugou.com/stdmusic/{size}/1/2.jpg"}},
			{"hash":"not-a-hash","songname":"skipped"}
		]}}`),
		"search.kuwo.cn": text(`{"abslist":[
			{"DC_TARGETID":"42","SONGNAME":"Song&nbsp;Two","ARTIST":"X&amp;Y","ALBUM":"Alb2","DURATION":"99",
			 "web_albumpic_short":"120/s3s94/93/1.jpg"},
			{"MUSICRID":"MUSIC_43","NAME":"Fallback","ARTIST":"Z","web_albumpic_short":"../../etc/passwd"}
		]}`),
	})
	kg, err := s.Search(context.Background(), "kugou", "q", 10, "", Options{})
	if err != nil {
		t.Fatal(err)
	}
	wantKg := []Result{{
		Provider: "kugou", ID: "0123456789abcdef0123456789abcdef", Title: "Song", Artists: []string{"A", "B"},
		Album: "Alb", Duration: 200,
		CoverURL: "https://imge.kugou.com/stdmusic/480/1/2.jpg", ThumbURL: "https://imge.kugou.com/stdmusic/150/1/2.jpg",
	}}
	if !reflect.DeepEqual(kg, wantKg) {
		t.Fatalf("kugou got  %+v\nwant %+v", kg, wantKg)
	}
	kw, err := s.Search(context.Background(), "kuwo", "q", 10, "", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(kw) != 2 {
		t.Fatalf("kuwo got %d results", len(kw))
	}
	wantKw := Result{
		Provider: "kuwo", ID: "42", Title: "Song Two", Artists: []string{"X", "Y"}, Album: "Alb2", Duration: 99,
		CoverURL: "https://img2.kuwo.cn/star/albumcover/500/s3s94/93/1.jpg",
		ThumbURL: "https://img2.kuwo.cn/star/albumcover/120/s3s94/93/1.jpg",
	}
	if !reflect.DeepEqual(kw[0], wantKw) {
		t.Fatalf("kuwo got  %+v\nwant %+v", kw[0], wantKw)
	}
	if kw[1].ID != "43" || kw[1].Title != "Fallback" || kw[1].CoverURL != "" {
		t.Fatalf("kuwo fallback = %+v", kw[1])
	}
}

func TestSearchItunes(t *testing.T) {
	s, f := newFake(map[string]http.HandlerFunc{
		"itunes.apple.com": text(`{"results":[{"kind":"song","trackId":7,"trackName":"Track","artistName":"Solo",
			"collectionName":"Record","trackNumber":2,"trackCount":11,"discNumber":1,"discCount":2,
			"releaseDate":"2008-09-12T07:00:00Z","primaryGenreName":"Pop","trackTimeMillis":180000,
			"artworkUrl100":"https://is1-ssl.mzstatic.com/image/thumb/a/100x100bb.jpg"},
			{"kind":"music-video","trackId":8,"trackName":"Video"}]}`),
	})
	got, err := s.Search(context.Background(), "itunes", "q", 5, "JP", Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := []Result{{
		Provider: "itunes", ID: "7", Title: "Track", Artists: []string{"Solo"}, Album: "Record", AlbumArtist: "Solo",
		TrackNumber: 2, TrackTotal: 11, DiscNumber: 1, DiscTotal: 2, Date: "2008-09-12", Genre: "Pop", Duration: 180,
		CoverURL: "https://is1-ssl.mzstatic.com/image/thumb/a/1200x1200bb.jpg",
		ThumbURL: "https://is1-ssl.mzstatic.com/image/thumb/a/150x150bb.jpg",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %+v\nwant %+v", got, want)
	}
	if c := f.requests[0].URL.Query().Get("country"); c != "jp" {
		t.Fatalf("country = %q", c)
	}
	if _, err := s.Search(context.Background(), "itunes", "q", 5, "zz", Options{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown region err = %v", err)
	}
}

func TestSearchValidation(t *testing.T) {
	s, f := newFake(nil)
	ctx := context.Background()
	cases := []struct {
		provider, query string
		want            error
	}{
		{"nope", "q", ErrUnknownProvider},
		{"qq", "   ", ErrInvalid},
		{"qq", strings.Repeat("字", MaxQueryLength+1), ErrInvalid},
	}
	for _, c := range cases {
		if _, err := s.Search(ctx, c.provider, c.query, 1, "", Options{}); !errors.Is(err, c.want) {
			t.Errorf("Search(%q, %.10q) err = %v, want %v", c.provider, c.query, err, c.want)
		}
	}
	if len(f.requests) != 0 {
		t.Fatalf("invalid searches sent %d requests", len(f.requests))
	}
}

func TestSearchLimitIsCapped(t *testing.T) {
	s, f := newFake(map[string]http.HandlerFunc{"search.kuwo.cn": text(`{"abslist":[]}`)})
	if _, err := s.Search(context.Background(), "kuwo", "q", 1000, "", Options{}); err != nil {
		t.Fatal(err)
	}
	if rn := f.requests[0].URL.Query().Get("rn"); rn != "30" {
		t.Fatalf("rn = %q, want 30", rn)
	}
}

func TestLyrics(t *testing.T) {
	s, f := newFake(map[string]http.HandlerFunc{
		"music.163.com": text(`{"lrc":{"lyric":"[00:01.00]Hello\r\n[00:02.00]World"},"tlyric":{"lyric":"[by:x]\n[00:01.00]\n[00:02.00]//"}}`),
		"c.y.qq.com":    text(`{"code":0,"lyric":"` + b64("[00:01.00]QQ line") + `","trans":"` + b64("[00:01.00]译文") + `"}`),
		"lyrics.kugou.com": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/search" {
				_, _ = io.WriteString(w, `{"candidates":[{"id":"55","accesskey":"ABC123"}]}`)
				return
			}
			if r.URL.Query().Get("id") != "55" || r.URL.Query().Get("accesskey") != "ABC123" {
				http.Error(w, "bad", http.StatusBadRequest)
				return
			}
			_, _ = io.WriteString(w, `{"content":"`+b64("[00:03.00]Kugou line")+`"}`)
		},
		"kuwo.cn": text(`{"code":200,"data":{"lrclist":[{"lineLyric":"First","time":"0.5"},{"lineLyric":"Second","time":"65.25"}]}}`),
	})
	ctx := context.Background()
	cases := []struct {
		provider, id string
		want         Lyrics
	}{
		// A translation made only of empty lines and "//" counts as none.
		{"netease", "101", Lyrics{Text: "[00:01.00]Hello\n[00:02.00]World"}},
		{"qq", "000aaaaaaaaaaa", Lyrics{Text: "[00:01.00]QQ line", Translation: "[00:01.00]译文"}},
		{"kugou", "0123456789abcdef0123456789abcdef", Lyrics{Text: "[00:03.00]Kugou line"}},
		{"kuwo", "42", Lyrics{Text: "[00:00.50]First\n[01:05.25]Second"}},
	}
	for _, c := range cases {
		got, err := s.Lyrics(ctx, c.provider, c.id, Options{})
		if err != nil {
			t.Errorf("%s: %v", c.provider, err)
			continue
		}
		if *got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.provider, *got, c.want)
		}
	}
	n := len(f.requests)
	for _, bad := range []struct{ provider, id string }{
		{"netease", "1&x=2"}, {"qq", "../x"}, {"kugou", "short"}, {"kuwo", "abc"},
	} {
		if _, err := s.Lyrics(ctx, bad.provider, bad.id, Options{}); !errors.Is(err, ErrInvalid) {
			t.Errorf("Lyrics(%s, %q) err = %v, want ErrInvalid", bad.provider, bad.id, err)
		}
	}
	if len(f.requests) != n {
		t.Fatal("invalid ids reached the network")
	}
	if _, err := s.Lyrics(ctx, "itunes", "7", Options{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("itunes lyrics err = %v", err)
	}
}

func TestLyricsNotFound(t *testing.T) {
	s, _ := newFake(map[string]http.HandlerFunc{
		"music.163.com":    text(`{"nolyric":true}`),
		"c.y.qq.com":       text(`{"code":-1901}`),
		"lyrics.kugou.com": text(`{"candidates":[]}`),
		"kuwo.cn":          text(`{"code":200,"data":null}`),
	})
	for _, c := range []struct{ provider, id string }{
		{"netease", "1"}, {"qq", "000aaaaaaaaaaa"}, {"kugou", "0123456789abcdef0123456789abcdef"}, {"kuwo", "1"},
	} {
		if _, err := s.Lyrics(context.Background(), c.provider, c.id, Options{}); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: err = %v, want ErrNotFound", c.provider, err)
		}
	}
}

func TestCoverAllowed(t *testing.T) {
	// Built from the allow-list itself so every provider host is covered.
	build := func(scheme, host string, user *url.Userinfo) string {
		return (&url.URL{Scheme: scheme, Host: host, User: user, Path: "/cover.jpg"}).String()
	}
	for _, h := range coverHosts {
		allowed := []string{build("https", h, nil), build("http", h, nil), build("https", "img."+h, nil), build("https", h+":443", nil)}
		refused := []string{
			build("https", "evil"+h, nil),                     // suffix without a dot boundary
			build("https", h+".example.com", nil),             // provider host as a prefix
			build("https", h, url.UserPassword("user", "pw")), // credentials
			build("https", h+":8443", nil),                    // unexpected port
			build("ftp", h, nil),                              // scheme
			"//" + h + "/cover.jpg",                           // no scheme
			build("https", "example.com", nil) + "?u=" + url.QueryEscape(build("https", h, nil)),
		}
		for _, u := range allowed {
			if !CoverAllowed(u) {
				t.Errorf("CoverAllowed(%q) = false, want true", u)
			}
		}
		for _, u := range refused {
			if CoverAllowed(u) {
				t.Errorf("CoverAllowed(%q) = true, want false", u)
			}
		}
	}
	for _, u := range []string{"", "file:///etc/passwd", "https://192.0.2.1/cover.jpg", "https://example.com/cover.jpg"} {
		if CoverAllowed(u) {
			t.Errorf("CoverAllowed(%q) = true, want false", u)
		}
	}
}

var jpegBytes = []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00synthetic")

func TestCover(t *testing.T) {
	s, f := newFake(map[string]http.HandlerFunc{
		"p1.music.126.net": func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/img.jpg":
				_, _ = w.Write(jpegBytes)
			case "/page.jpg":
				_, _ = io.WriteString(w, "<html><script>alert(1)</script></html>")
			case "/elsewhere.jpg":
				http.Redirect(w, r, "https://example.com/x.jpg", http.StatusFound)
			default:
				http.NotFound(w, r)
			}
		},
	})
	ctx := context.Background()
	data, ctype, err := s.Cover(ctx, "http://p1.music.126.net/img.jpg")
	if err != nil || ctype != "image/jpeg" || !bytes.Equal(data, jpegBytes) {
		t.Fatalf("Cover = %d bytes, %q, %v", len(data), ctype, err)
	}
	if f.requests[0].URL.Scheme != "https" {
		t.Fatalf("cover fetched over %s", f.requests[0].URL.Scheme)
	}
	if _, _, err := s.Cover(ctx, "https://p1.music.126.net/page.jpg"); !errors.Is(err, ErrUpstream) {
		t.Fatalf("non-image err = %v", err)
	}
	if _, _, err := s.Cover(ctx, "https://p1.music.126.net/elsewhere.jpg"); !errors.Is(err, ErrUpstream) {
		t.Fatalf("cross-host redirect err = %v", err)
	}
	if _, _, err := s.Cover(ctx, "https://p1.music.126.net/missing.jpg"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing err = %v", err)
	}
	n := len(f.requests)
	if _, _, err := s.Cover(ctx, "https://example.com/x.jpg"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("foreign host err = %v", err)
	}
	for _, r := range f.requests[n:] {
		t.Fatalf("foreign cover requested %s", r.URL)
	}
	for _, r := range f.requests {
		if r.URL.Host == "example.com" {
			t.Fatal("redirect was followed to example.com")
		}
	}
}

func TestResponseSizeIsBounded(t *testing.T) {
	big := strings.Repeat(" ", maxJSONSize+1)
	s, _ := newFake(map[string]http.HandlerFunc{"search.kuwo.cn": text(big)})
	if _, err := s.Search(context.Background(), "kuwo", "q", 1, "", Options{}); !errors.Is(err, ErrUpstream) {
		t.Fatalf("err = %v, want ErrUpstream", err)
	}
}

func TestErrorsDoNotEchoTheQuery(t *testing.T) {
	s := New(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	})})
	_, err := s.Search(context.Background(), "kuwo", "secret-query", 1, "", Options{})
	if !errors.Is(err, ErrUpstream) || strings.Contains(err.Error(), "secret-query") {
		t.Fatalf("err = %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestFlexIntAndDates(t *testing.T) {
	var v struct {
		A, B, C, D, E flexInt
	}
	if err := decode([]byte(`{"A":3,"B":"07","C":"1/2","D":null,"E":{"x":1}}`), "test", &v); err != nil {
		t.Fatal(err)
	}
	if v.A != 3 || v.B != 7 || v.C != 1 || v.D != 0 || v.E != 0 {
		t.Fatalf("flexInt = %+v", v)
	}
	for in, want := range map[string]string{
		"2003-07-31": "2003-07-31", "2021-02-11T08:00:00Z": "2021-02-11", "2019": "2019",
		"2019-13-40": "2019", "2019-05": "2019-05", "abc": "", "": "", "0000-00-00": "",
	} {
		if got := normalizeDate(in); got != want {
			t.Errorf("normalizeDate(%q) = %q, want %q", in, got, want)
		}
	}
	if got := dateFromMillis(0); got != "" {
		t.Errorf("dateFromMillis(0) = %q", got)
	}
}

func TestChinaIPHeader(t *testing.T) {
	answers := map[string]http.HandlerFunc{
		"music.163.com":       text(`{"code":200,"result":{"songs":[]},"lrc":{"lyric":"[00:01.00]x"}}`),
		"u.y.qq.com":          text(`{"code":0,"req":{"code":0}}`),
		"c.y.qq.com":          text(`{"code":0,"lyric":"` + b64("[00:01.00]x") + `"}`),
		"mobilecdn.kugou.com": text(`{"status":1,"data":{"info":[]}}`),
		"search.kuwo.cn":      text(`{"abslist":[]}`),
		"itunes.apple.com":    text(`{"results":[]}`),
		"p1.music.126.net":    func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(jpegBytes) },
	}
	ctx := context.Background()
	s, f := newFake(answers)
	for _, p := range []string{"netease", "qq", "kugou", "kuwo", "itunes"} {
		if _, err := s.Search(ctx, p, "q", 1, "", Options{}); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
	}
	for _, r := range f.requests {
		if r.Header.Get("X-Real-IP") != "" {
			t.Fatalf("header sent to %s while off", r.URL.Host)
		}
	}

	s, f = newFake(answers)
	on := Options{ChinaIP: true}
	for _, p := range []string{"netease", "qq", "kugou", "kuwo", "itunes"} {
		if _, err := s.Search(ctx, p, "q", 1, "", on); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
	}
	for p, id := range map[string]string{"netease": "101", "qq": "000aaaaaaaaaaa"} {
		if _, err := s.Lyrics(ctx, p, id, on); err != nil {
			t.Fatalf("%s lyrics: %v", p, err)
		}
	}
	if _, _, err := s.Cover(ctx, "https://p1.music.126.net/img.jpg"); err != nil {
		t.Fatal(err)
	}
	for _, r := range f.requests {
		ip := r.Header.Get("X-Real-IP")
		switch r.URL.Host {
		case "itunes.apple.com", "p1.music.126.net":
			if ip != "" {
				t.Errorf("header sent to %s", r.URL.Host)
			}
		default:
			if !inChinaPrefixes(ip) {
				t.Errorf("%s got X-Real-IP %q", r.URL.Host, ip)
			}
		}
	}
}

func inChinaPrefixes(s string) bool {
	a, err := netip.ParseAddr(s)
	if err != nil || !a.Is4() {
		return false
	}
	b := a.As4()
	for _, p := range chinaPrefixes {
		if b[0] == p[0] && b[1] == p[1] && b[3] != 0 && b[3] != 255 {
			return true
		}
	}
	return false
}

func TestChinaIPIsPublic(t *testing.T) {
	for range 1000 {
		ip := chinaIP()
		a := netip.MustParseAddr(ip)
		if !inChinaPrefixes(ip) || a.IsPrivate() || a.IsLoopback() || !a.IsGlobalUnicast() {
			t.Fatalf("chinaIP() = %s", ip)
		}
	}
}
