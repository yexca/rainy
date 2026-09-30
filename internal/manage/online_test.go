package manage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"rainy/internal/lxmusic"
	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/tags"
)

// fakeOnline serves links per source id; a link is either "audio" (the test file) or
// "html" (an error page).
type fakeOnline struct {
	mu        sync.Mutex
	src       string // audio file to serve
	cands     []lxmusic.Candidate
	links     map[string]string   // source id → "audio" | "html" | "expired" | "" (MusicURL fails)
	seq       map[string][]string // source id → links handed out in turn (overrides links)
	cover     []byte
	lyrics    [2]string
	asked     []string // source ids asked for a link
	selection lxmusic.Selection
}

func (f *fakeOnline) Candidates(_ context.Context, platform string, sel lxmusic.Selection) ([]lxmusic.Candidate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.selection = sel
	return f.cands, nil
}

func (f *fakeOnline) MusicURL(_ context.Context, id string, song lxmusic.Song, want string) (string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, id)
	if next := f.seq[id]; len(next) > 0 {
		f.seq[id] = next[1:]
		return "https://cdn.example.com/" + next[0], "320k", nil
	}
	if f.links[id] == "" {
		return "", "", fmt.Errorf("%w: 服务器繁忙", lxmusic.ErrScript)
	}
	return "https://cdn.example.com/" + f.links[id], "320k", nil
}

func (f *fakeOnline) Download(_ context.Context, link string, w io.Writer, limit int64, progress func(done, total int64)) (int64, error) {
	var data []byte
	if strings.HasSuffix(link, "/expired") {
		return 0, &lxmusic.LinkError{Host: "cdn.example.com", Status: 410}
	}
	if strings.HasSuffix(link, "/audio") {
		data = mustReadFile(f.src)
	} else {
		data = []byte("<!doctype html><title>Forbidden</title>")
	}
	n, err := w.Write(data)
	progress(int64(n), int64(len(data)))
	return int64(n), err
}

func (f *fakeOnline) Lyrics(context.Context, lxmusic.Song) (string, string, error) {
	if f.lyrics[0] == "" {
		return "", "", lxmusic.ErrNotFound
	}
	return f.lyrics[0], f.lyrics[1], nil
}

func (f *fakeOnline) Cover(context.Context, lxmusic.Song) ([]byte, error) {
	if f.cover == nil {
		return nil, lxmusic.ErrNotFound
	}
	return f.cover, nil
}

func (e *env) enableOnline(on bool, mode, id string) {
	e.t.Helper()
	set := model.DefaultSettings(e.cfg.ScanInterval)
	set.LxSourcesEnabled, set.LxSourceMode, set.LxSourceID = on, mode, id
	if err := e.st.SaveSettings(e.ctx, set); err != nil {
		e.t.Fatal(err)
	}
}

var onlineSong = lxmusic.Song{
	Platform: lxmusic.QQ, ID: "0039MnYb0qxYhV", Title: "Synthetic: Rain?", Artists: []string{"Singer A", "Singer B"},
	Album: "Synthetic Album", AlbumID: "000MkMni19ClKG", Duration: 200,
	Qualities: []lxmusic.Quality{{Type: "128k"}, {Type: "320k"}},
	Extra:     map[string]string{"strMediaMid": "003Qui1q2u1Zho", "albumMid": "000MkMni19ClKG", "songId": "97773"},
}

func TestOnlineDownloadFallsBackAndImports(t *testing.T) {
	e := newEnv(t)
	src := e.abs("林雨晴/夏日微风 (2021)/01 夏日微风.mp3")
	requireTags(t, src)
	e.enableOnline(true, model.LxSourceModeAuto, "")
	online := &fakeOnline{
		src: src, cover: wideJPEG(t, 300, 300),
		cands:  []lxmusic.Candidate{{ID: "a", Name: "Busy"}, {ID: "b", Name: "Broken link"}, {ID: "c", Name: "Working"}},
		links:  map[string]string{"b": "html", "c": "audio"},
		lyrics: [2]string{"[00:01.00]Hello\n[00:02.50]World", "[00:01.00]你好\n[00:02.5]世界"},
	}
	e.svc.SetOnlineSource(online)

	jobs, err := e.svc.StartOnlineDownload(e.ctx, e.user, OnlineDownloadRequest{
		Songs: []lxmusic.Song{onlineSong}, Quality: "flac", LibraryID: e.lib.ID, Dir: "Online", Lyrics: true, Cover: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].Kind != JobKindOnline || jobs[0].Online.Song.PageURL != "https://y.qq.com/n/yqq/song/0039MnYb0qxYhV.html" {
		t.Fatalf("jobs %+v", jobs)
	}
	done := e.waitJob(jobs[0].ID)
	if done.Status != JobDone || len(done.TrackIDs) != 1 || done.Online.Source != "Working" || done.Online.Got != "320k" {
		t.Fatalf("finished job %+v %+v", done, done.Online)
	}
	if fmt.Sprint(online.asked) != "[a b c]" {
		t.Fatalf("sources asked %v", online.asked)
	}

	rel := "Online/Synthetic_ Rain_ - Singer A、Singer B.mp3"
	if !e.exists(rel) {
		t.Fatalf("%s was not placed in the library", rel)
	}
	raw, err := tags.ReadRaw(e.abs(rel))
	if err != nil {
		t.Fatal(err)
	}
	if raw["TITLE"][0] != "Synthetic: Rain?" || strings.Join(raw["ARTIST"], "|") != "Singer A|Singer B" || raw["ALBUM"][0] != "Synthetic Album" {
		t.Errorf("tags %v", raw)
	}
	if lyrics := strings.Join(raw["LYRICS"], ""); lyrics != "[00:01.00]Hello\n[00:01.00]你好\n[00:02.50]World\n[00:02.50]世界" {
		t.Errorf("lyrics %q", lyrics)
	}
	if pic, err := tags.ReadPicture(e.abs(rel)); err != nil || pic == nil {
		t.Errorf("no cover: %v", err)
	}
	entries, _, _ := e.st.ListEditLog(e.ctx, done.TrackIDs[0], 0, 0)
	if len(entries) != 1 || entries[0].Action != "download" {
		t.Fatalf("edit log %+v", entries)
	}
	var details map[string]any
	_ = json.Unmarshal(entries[0].Details, &details)
	if details["via"] != "Working" || details["quality"] != "320k" || details["source"] != "https://y.qq.com/n/yqq/song/0039MnYb0qxYhV.html" {
		t.Errorf("details %v", details)
	}
	if left, _ := filepath.Glob(filepath.Join(e.cfg.TmpDir(), "online-job-*")); len(left) != 0 {
		t.Errorf("work directories left behind: %v", left)
	}
}

func TestOnlineDownloadFailures(t *testing.T) {
	e := newEnv(t)
	src := e.abs("林雨晴/夏日微风 (2021)/01 夏日微风.mp3")
	online := &fakeOnline{src: src, cands: []lxmusic.Candidate{{ID: "a", Name: "Busy"}, {ID: "b", Name: "HTML"}}, links: map[string]string{"b": "html"}}
	e.svc.SetOnlineSource(online)
	req := OnlineDownloadRequest{Songs: []lxmusic.Song{onlineSong}, LibraryID: e.lib.ID}

	e.enableOnline(true, model.LxSourceModeFixed, "b")
	jobs, err := e.svc.StartOnlineDownload(e.ctx, e.user, req)
	if err != nil {
		t.Fatal(err)
	}
	done := e.waitJob(jobs[0].ID)
	if done.Status != JobError || !strings.Contains(done.Error, "Busy: 服务器繁忙") || !strings.Contains(done.Error, "HTML: the link did not lead to a playable audio file") {
		t.Fatalf("job %+v", done)
	}
	if online.selection.Mode != model.LxSourceModeFixed || online.selection.SourceID != "b" || jobs[0].Online.Quality != "flac" {
		t.Fatalf("selection %+v quality %s", online.selection, jobs[0].Online.Quality)
	}

	e.enableOnline(false, model.LxSourceModeAuto, "")
	jobs, _ = e.svc.StartOnlineDownload(e.ctx, e.user, req)
	if done := e.waitJob(jobs[0].ID); done.Status != JobError || !strings.Contains(done.Error, "turned off") {
		t.Fatalf("disabled: %+v", done)
	}

	bad := onlineSong
	bad.Extra = map[string]string{"songId": "1"}
	cases := []OnlineDownloadRequest{
		{LibraryID: e.lib.ID},
		{Songs: []lxmusic.Song{bad}, LibraryID: e.lib.ID},
		{Songs: []lxmusic.Song{onlineSong}, LibraryID: e.lib.ID, Quality: "999k"},
		{Songs: []lxmusic.Song{onlineSong}, LibraryID: e.lib.ID, Dir: "../outside"},
		{Songs: make([]lxmusic.Song, maxOnlineSongs+1), LibraryID: e.lib.ID},
	}
	for i, c := range cases {
		if _, err := e.svc.StartOnlineDownload(e.ctx, e.user, c); !errors.Is(err, store.ErrInvalid) {
			t.Errorf("case %d: %v", i, err)
		}
	}
}

func TestOnlineDownloadRefreshesExpiredLinks(t *testing.T) {
	e := newEnv(t)
	src := e.abs("林雨晴/夏日微风 (2021)/01 夏日微风.mp3")
	requireTags(t, src)
	e.enableOnline(true, model.LxSourceModeAuto, "")
	req := OnlineDownloadRequest{Songs: []lxmusic.Song{onlineSong}, LibraryID: e.lib.ID, Dir: "Refresh"}

	// An expired link is asked for again from the same source (like lx-music), once.
	online := &fakeOnline{src: src, cands: []lxmusic.Candidate{{ID: "a", Name: "Expiring"}}, seq: map[string][]string{"a": {"expired", "audio"}}}
	e.svc.SetOnlineSource(online)
	jobs, err := e.svc.StartOnlineDownload(e.ctx, e.user, req)
	if err != nil {
		t.Fatal(err)
	}
	if done := e.waitJob(jobs[0].ID); done.Status != JobDone || fmt.Sprint(online.asked) != "[a a]" {
		t.Fatalf("job %+v asked %v", done, online.asked)
	}

	// A source whose second link expires too is given up; the next one is tried.
	online = &fakeOnline{
		src: src, cands: []lxmusic.Candidate{{ID: "a", Name: "Expiring"}, {ID: "b", Name: "Working"}},
		seq: map[string][]string{"a": {"expired", "expired", "audio"}}, links: map[string]string{"b": "audio"},
	}
	e.svc.SetOnlineSource(online)
	jobs, err = e.svc.StartOnlineDownload(e.ctx, e.user, req)
	if err != nil {
		t.Fatal(err)
	}
	if done := e.waitJob(jobs[0].ID); done.Status != JobDone || done.Online.Source != "Working" || fmt.Sprint(online.asked) != "[a a b]" {
		t.Fatalf("job %+v asked %v", done, online.asked)
	}
}

func TestSniffAudio(t *testing.T) {
	dir := t.TempDir()
	// An ID3v2 tag with a 2-byte body in front of rest (a fresh slice every time).
	id3 := func(rest ...byte) []byte { return append([]byte("ID3\x04\x00\x00\x00\x00\x00\x02\x00\x00"), rest...) }
	for name, c := range map[string]struct {
		data []byte
		ext  string
	}{
		"flac":     {[]byte("fLaC\x00\x00\x00\x22"), ".flac"},
		"id3 flac": {id3([]byte("fLaCxxxx")...), ".flac"},
		"id3 mp3":  {id3(0xff, 0xfb, 0x90, 0x00), ".mp3"},
		"mpeg":     {[]byte{0xff, 0xfb, 0x90, 0x44, 0, 0}, ".mp3"},
		"m4a":      {[]byte("\x00\x00\x00\x20ftypM4A \x00\x00\x00\x00"), ".m4a"},
		"opus":     {[]byte("OggS\x00\x02\x00\x00\x00\x00\x00\x00\x00\x00\x01\x02\x03\x04\x00\x00\x00\x00\x00\x00\x00\x00\x01\x13OpusHead"), ".opus"},
		"wav":      {[]byte("RIFF\x24\x00\x00\x00WAVEfmt "), ".wav"},
		"html":     {[]byte("<!doctype html>"), ""},
		"adts":     {[]byte{0xff, 0xf1, 0x50, 0x80}, ""},
		"empty":    {nil, ""},
	} {
		p := filepath.Join(dir, strings.ReplaceAll(name, " ", "_"))
		if err := os.WriteFile(p, c.data, 0o600); err != nil {
			t.Fatal(err)
		}
		ext, err := sniffAudio(p)
		if ext != c.ext || (c.ext == "") != (err != nil) {
			t.Errorf("%s: %q %v", name, ext, err)
		}
	}
}

func TestPairTranslation(t *testing.T) {
	cases := []struct{ text, trans, want string }{
		{"[00:01.00]A\n[00:02.00]B", "", "[00:01.00]A\n[00:02.00]B"},
		{"[00:01.00]A\n[00:02.00]B\n[00:03.00]", "[00:01.000]甲\n[00:02]乙\n[00:03.00]\n[00:09.00]多余", "[00:01.00]A\n[00:01.00]甲\n[00:02.00]B\n[00:02.00]乙\n[00:03.00]"},
		{"[ti:Song]\n[00:01.00]Same", "[00:01.00]Same", "[ti:Song]\n[00:01.00]Same"},
		{"plain text", "[00:01.00]译", "plain text"},
	}
	for _, c := range cases {
		if got := pairTranslation(c.text, c.trans); got != c.want {
			t.Errorf("pairTranslation(%q, %q)\n got %q\nwant %q", c.text, c.trans, got, c.want)
		}
	}
}
