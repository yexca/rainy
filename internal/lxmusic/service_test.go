package lxmusic

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"rainy/internal/db/dbtest"
	"rainy/internal/model"
	"rainy/internal/store"
)

func newTestService(t *testing.T) (*Service, *store.Store) {
	t.Helper()
	st := store.New(dbtest.New(t))
	s := New(Options{Store: st, AllowPrivate: true})
	t.Cleanup(s.Close)
	return s, st
}

// sourceScript builds a script providing platform kw that answers with link (or rejects
// with reason when link is empty).
func sourceScript(name, link, reason string) string {
	answer := fmt.Sprintf("Promise.resolve(%q + '?q=' + info.type + '&id=' + info.musicInfo.songmid)", link)
	if link == "" {
		answer = fmt.Sprintf("Promise.reject(new Error(%q))", reason)
	}
	return fmt.Sprintf(`/**
 * @name %s
 * @description synthetic test source
 * @version 1.2.3
 * @author tester
 * @homepage javascript:alert(1)
 */
lx.on(lx.EVENT_NAMES.request, ({ info }) => %s)
lx.send(lx.EVENT_NAMES.inited, { sources: { kw: { type: 'music', actions: ['musicUrl'], qualitys: ['128k', '320k', 'flac'] } } })
`, name, answer)
}

var testSong = Song{
	Platform: Kuwo, ID: "123", Title: "Synthetic Song", Artists: []string{"Synthetic Artist"}, Album: "Synthetic Album",
	Duration: 200, Qualities: []Quality{{Type: "128k", Size: "3.00 MB"}, {Type: "320k", Size: "7.00 MB"}, {Type: "flac24bit"}},
	Extra: map[string]string{},
}

func TestServiceImportAndFallback(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestService(t)

	bad, err := s.Import(ctx, sourceScript("Broken", "", "服务器繁忙"), "", true)
	if err != nil {
		t.Fatal(err)
	}
	if bad.Status != StatusReady || bad.Version != "1.2.3" || bad.Homepage != "" || len(bad.Platforms) != 1 {
		t.Fatalf("imported %+v", bad)
	}
	if _, err := s.Import(ctx, sourceScript("Broken", "", "服务器繁忙"), "", false); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("duplicate import: %v", err)
	}
	good, err := s.Import(ctx, byteOrderMark+sourceScript("Working", "https://cdn.example.com/a.mp3", ""), "", false)
	if err != nil {
		t.Fatal(err)
	}
	if good.Status != StatusIdle || good.Position != bad.Position+1 {
		t.Fatalf("second source %+v", good)
	}
	if _, err := s.Import(ctx, "const x = 1", "", false); !errors.Is(err, ErrInvalid) {
		t.Fatalf("headerless script: %v", err)
	}

	auto := Selection{Mode: model.LxSourceModeAuto}
	cands, err := s.Candidates(ctx, Kuwo, auto)
	if err != nil || len(cands) != 2 || cands[0].Name != "Broken" || cands[1].Name != "Working" {
		t.Fatalf("candidates %+v %v", cands, err)
	}
	if _, _, err := s.MusicURL(ctx, cands[0].ID, testSong, "flac"); err == nil || !strings.Contains(err.Error(), "服务器繁忙") {
		t.Fatalf("broken source: %v", err)
	}
	link, quality, err := s.MusicURL(ctx, cands[1].ID, testSong, "flac")
	if err != nil || quality != "320k" || link != "https://cdn.example.com/a.mp3?q=320k&id=123" {
		t.Fatalf("link %q quality %q err %v", link, quality, err)
	}
	// Loading the second source recorded what it provides.
	if info, _ := s.Get(ctx, good.ID); info.Status != StatusReady || info.Platforms[0].Platform != Kuwo {
		t.Fatalf("after use %+v", info)
	}

	fixed, err := s.Candidates(ctx, Kuwo, Selection{Mode: model.LxSourceModeFixed, SourceID: good.ID})
	if err != nil || len(fixed) != 1 || fixed[0].ID != good.ID {
		t.Fatalf("fixed candidates %+v %v", fixed, err)
	}
	if other, _ := s.Candidates(ctx, NetEase, auto); len(other) != 0 {
		t.Fatalf("sources without wy offered for wy: %+v", other)
	}
	avail, _ := s.Available(ctx, auto)
	if fmt.Sprint(avail) != "map[kw:[128k 320k flac]]" {
		t.Fatalf("available %v", avail)
	}

	off := false
	if _, err := s.Update(ctx, bad.ID, &off, nil); err != nil {
		t.Fatal(err)
	}
	if cands, _ := s.Candidates(ctx, Kuwo, auto); len(cands) != 1 || cands[0].ID != good.ID {
		t.Fatalf("disabled source offered: %+v", cands)
	}
	if err := s.Reorder(ctx, []string{good.ID, bad.ID}); err != nil {
		t.Fatal(err)
	}
	if err := s.Reorder(ctx, []string{good.ID}); !errors.Is(err, store.ErrInvalid) {
		t.Fatalf("partial order: %v", err)
	}
	list, _ := s.List(ctx)
	if len(list) != 2 || list[0].ID != good.ID || list[1].Enabled {
		t.Fatalf("list %+v", list)
	}
	if err := s.Delete(ctx, bad.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, bad.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("deleted source: %v", err)
	}
}

func TestServiceStartFailureIsRecorded(t *testing.T) {
	ctx := context.Background()
	s, st := newTestService(t)
	info, err := s.Import(ctx, "/**\n * @name Fails\n */\nthrow new Error('cannot start')", "", true)
	if err != nil {
		t.Fatal(err)
	}
	if info.Status != StatusError || info.Error != "cannot start" {
		t.Fatalf("info %+v", info)
	}
	rows, _ := st.ListLxSources(ctx)
	if rows[0].LastError != "cannot start" || rows[0].LoadedAt == 0 {
		t.Fatalf("row %+v", rows[0])
	}
	if _, _, err := s.MusicURL(ctx, info.ID, testSong, "128k"); err == nil || !strings.Contains(err.Error(), "cannot start") {
		t.Fatalf("resolve through a broken source: %v", err)
	}
}

func TestServiceRefreshFromURL(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestService(t)
	var version atomic.Value
	version.Store(sourceScript("Remote", "https://cdn.example.com/v1.mp3", ""))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(version.Load().(string)))
	}))
	defer srv.Close()

	text, err := s.FetchScript(ctx, srv.URL+"/source.js")
	if err != nil {
		t.Fatal(err)
	}
	info, err := s.Import(ctx, text, srv.URL+"/source.js", true)
	if err != nil {
		t.Fatal(err)
	}
	if link, _, err := s.MusicURL(ctx, info.ID, testSong, "128k"); err != nil || !strings.HasPrefix(link, "https://cdn.example.com/v1.mp3") {
		t.Fatalf("v1 link %q %v", link, err)
	}
	version.Store(strings.Replace(sourceScript("Remote", "https://cdn.example.com/v2.mp3", ""), "1.2.3", "2.0.0", 1))
	info, err = s.Refresh(ctx, info.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if info.Version != "2.0.0" || info.Status != StatusReady {
		t.Fatalf("refreshed %+v", info)
	}
	if link, _, err := s.MusicURL(ctx, info.ID, testSong, "128k"); err != nil || !strings.HasPrefix(link, "https://cdn.example.com/v2.mp3") {
		t.Fatalf("v2 link %q %v", link, err)
	}

	guarded := New(Options{Store: store.New(dbtest.New(t))})
	defer guarded.Close()
	if _, err := guarded.FetchScript(ctx, srv.URL+"/source.js"); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("loopback import: %v", err)
	}
}

func TestDownloadReportsExpiredLinks(t *testing.T) {
	status := http.StatusGone
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	}))
	defer srv.Close()
	s, _ := newTestService(t)
	for code, expired := range map[int]bool{http.StatusGone: true, http.StatusForbidden: true, http.StatusUnauthorized: true, http.StatusNotFound: false, http.StatusBadGateway: false} {
		status = code
		_, err := s.Download(context.Background(), srv.URL+"/song.mp3", io.Discard, 1<<20, nil)
		if !errors.Is(err, ErrUpstream) || LinkExpired(err) != expired {
			t.Errorf("HTTP %d: err %v, expired %v", code, err, LinkExpired(err))
		}
	}
	unknown := requestError("gone.invalid", &url.Error{Op: "Get", URL: "https://gone.invalid/", Err: &net.OpError{Op: "dial", Err: &net.DNSError{Name: "gone.invalid", IsNotFound: true}}})
	if !LinkExpired(unknown) || !errors.Is(unknown, ErrUpstream) {
		t.Errorf("unknown host: %v", unknown)
	}
	if LinkExpired(errors.New("reset")) {
		t.Error("other errors do not count as expired links")
	}
}

func TestPickQuality(t *testing.T) {
	all := []string{"128k", "320k", "flac", "flac24bit"}
	cases := []struct {
		song   []string
		source []string
		want   string
		pick   string
	}{
		{[]string{"128k", "320k", "flac"}, all, "flac24bit", "flac"},
		{[]string{"128k", "320k", "flac"}, all, "320k", "320k"},
		{[]string{"128k", "320k", "flac"}, []string{"128k", "320k"}, "flac", "320k"},
		{[]string{"flac"}, all, "320k", "flac"},         // nothing at or below: the lowest above
		{nil, []string{"128k", "320k"}, "flac", "320k"}, // the song lists nothing: trust the source
		{[]string{"flac24bit"}, []string{"128k"}, "flac", ""},
		{[]string{"128k"}, all, "bogus", "128k"},
	}
	for _, c := range cases {
		var qs []Quality
		for _, q := range c.song {
			qs = append(qs, Quality{Type: q})
		}
		if got := pickQuality(qs, c.source, c.want); got != c.pick {
			t.Errorf("pickQuality(%v, %v, %s) = %q, want %q", c.song, c.source, c.want, got, c.pick)
		}
	}
}

func TestSongValidateAndMusicInfo(t *testing.T) {
	kg := Song{
		Platform: Kugou, ID: "101", Title: "Synthetic", Artists: []string{"A", "B"}, Album: "Album", AlbumID: "201",
		Duration: 269, CoverURL: "https://imge.kugou.com/stdmusic/480/x.jpg",
		Qualities: []Quality{{Type: "128k", Size: "4.12 MB", Hash: "0123456789ABCDEF0123456789ABCDEF"}},
		Extra:     map[string]string{"hash": "0123456789ABCDEF0123456789ABCDEF", "albumAudioId": "301"},
	}
	if err := kg.Validate(); err != nil {
		t.Fatal(err)
	}
	info := kg.MusicInfo()
	if info["songmid"] != int64(101) || info["singer"] != "A、B" || info["interval"] != "04:29" || info["hash"] != kg.Extra["hash"] {
		t.Fatalf("musicInfo %v", info)
	}
	if types := info["_types"].(map[string]any); types["128k"].(map[string]any)["hash"] != kg.Qualities[0].Hash {
		t.Fatalf("_types %v", types)
	}

	mutate := []func(s *Song){
		func(s *Song) { s.Platform = "xm" },
		func(s *Song) { s.ID = "1; drop" },
		func(s *Song) { s.Title = " " },
		func(s *Song) { s.Extra = map[string]string{"albumAudioId": "1"} },  // hash required
		func(s *Song) { s.Extra["hash"] = "not-a-hash" },                    // bad hash
		func(s *Song) { s.Extra["evil"] = "1" },                             // unknown key
		func(s *Song) { s.CoverURL = "https://example.com/x.jpg" },          // not an image host
		func(s *Song) { s.Qualities = append(s.Qualities, s.Qualities[0]) }, // repeated quality
		func(s *Song) { s.Qualities = []Quality{{Type: "999k"}} },           // unknown quality
		func(s *Song) { s.AlbumID = "../x" },
	}
	for i, fn := range mutate {
		s := kg
		s.Extra = map[string]string{"hash": kg.Extra["hash"], "albumAudioId": kg.Extra["albumAudioId"]}
		s.Qualities = append([]Quality{}, kg.Qualities...)
		fn(&s)
		if err := s.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("case %d: %v", i, err)
		}
	}
}

func TestScriptHeader(t *testing.T) {
	info, err := ParseScript("/**\r\n * @name  A very long source name that goes on and on\r\n * @description d\r\n * @homepage https://example.com/x\r\n * @unknown x\r\n */\ncode()")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "A very long source name ..." || info.Description != "d" || info.Homepage != "https://example.com/x" {
		t.Fatalf("%+v", info)
	}
	if info, _ := ParseScript("/** plain comment */"); info.Name != "Source" {
		t.Fatalf("default name %+v", info)
	}
	for _, s := range []string{"", "code()\n/** @name late */", strings.Repeat("x", MaxScriptSize+1), "a\x00b"} {
		if _, err := NormalizeScript(s); err == nil {
			if _, err := ParseScript(s); err == nil {
				t.Errorf("accepted %.20q", s)
			}
		}
	}
}

func TestRSAEncryptNoPadding(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	pemKey := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
	for _, k := range []string{pemKey, strings.ReplaceAll(pemKey, "\n", "")} {
		out, err := rsaEncrypt([]byte("secret"), k)
		if err != nil {
			t.Fatal(err)
		}
		m := new(big.Int).Exp(new(big.Int).SetBytes(out), key.D, key.N)
		if got := m.FillBytes(make([]byte, 128)); string(got[122:]) != "secret" || got[0] != 0 {
			t.Fatalf("decrypted %x", got)
		}
	}
	if _, err := rsaEncrypt(make([]byte, 129), pemKey); err == nil {
		t.Fatal("accepted more than 128 bytes")
	}
}

func TestBlockedAddresses(t *testing.T) {
	v4 := func(a, b, c, d byte) netip.Addr { return netip.AddrFrom4([4]byte{a, b, c, d}) }
	cases := map[netip.Addr]bool{
		v4(127, 0, 0, 1): true, v4(10, 1, 2, 3): true, v4(172, 17, 0, 1): true, v4(192, 168, 1, 1): true,
		v4(169, 254, 169, 254): true, v4(100, 64, 0, 1): true, v4(0, 0, 0, 0): true, v4(224, 0, 0, 1): true,
		v4(192, 0, 2, 1): true, v4(1, 2, 3, 4): false,
		mustAddr(t, "::1"): true, mustAddr(t, "fd00::1"): true, mustAddr(t, "fe80::1"): true,
		mustAddr(t, "::ffff:7f00:1"): true, mustAddr(t, "64:ff9b::a01:203"): true, // mapped loopback, NAT64 of a private address
		mustAddr(t, "2400::1"): false, mustAddr(t, "64:ff9b::102:304"): false, // public, NAT64 of a public address
	}
	for addr, blocked := range cases {
		if got := blockedAddr(addr); got != blocked {
			t.Errorf("blockedAddr(%s) = %v, want %v", addr, got, blocked)
		}
	}
}
