package manage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/tags"
	"rainy/internal/ytdlp"
)

const testLink = "https://www.youtube.com/watch?v=synthetic01"

// fakeDownloader copies a library file into the work directory as yt-dlp would.
type fakeDownloader struct {
	src      string // audio file to "download"
	thumb    []byte
	items    []ytdlp.Item // metadata; Path/Thumbnail are filled in
	err      error
	block    chan struct{} // when set, Download waits for it (or cancellation)
	notReady error
	got      []ytdlp.Request
}

func (f *fakeDownloader) Ready(context.Context) error { return f.notReady }

func (f *fakeDownloader) Download(ctx context.Context, req ytdlp.Request, progress func(ytdlp.Progress)) ([]ytdlp.Item, error) {
	f.got = append(f.got, req)
	progress(ytdlp.Progress{Phase: ytdlp.PhaseDownloading, Title: "Synthetic", Fraction: 0.5, ETA: 3})
	if f.block != nil {
		select {
		case <-f.block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	var out []ytdlp.Item
	for i, it := range f.items {
		it.Path = filepath.Join(req.Dir, "out", it.ID+filepath.Ext(f.src))
		if err := os.MkdirAll(filepath.Dir(it.Path), 0o700); err != nil {
			return nil, err
		}
		if err := os.WriteFile(it.Path, mustReadFile(f.src), 0o600); err != nil {
			return nil, err
		}
		if f.thumb != nil && i == 0 {
			it.Thumbnail = filepath.Join(req.Dir, "out", it.ID+".jpg")
			_ = os.WriteFile(it.Thumbnail, f.thumb, 0o600)
		}
		out = append(out, it)
	}
	return out, f.err
}

func mustReadFile(p string) []byte {
	b, err := os.ReadFile(p)
	if err != nil {
		panic(err)
	}
	return b
}

func (e *env) enableDownloads(on bool) {
	e.t.Helper()
	set := model.DefaultSettings(e.cfg.ScanInterval)
	set.YtdlpEnabled = on
	if err := e.st.SaveSettings(e.ctx, set); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) waitJob(id string) DownloadJob {
	e.t.Helper()
	for i := 0; i < 400; i++ {
		for _, j := range e.svc.DownloadJobs() {
			if j.ID == id && !j.active() {
				return j
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	e.t.Fatalf("job %s did not finish", id)
	return DownloadJob{}
}

func wideJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestDownloadImportsLikeAnUpload(t *testing.T) {
	e := newEnv(t)
	src := e.abs("林雨晴/夏日微风 (2021)/01 夏日微风.mp3")
	requireTags(t, src)
	e.enableDownloads(true)
	dl := &fakeDownloader{src: src, thumb: wideJPEG(t, 320, 180), items: []ytdlp.Item{{
		ID: "synthetic01", Title: "Synthetic: Song?", Artists: []string{"Synthetic Artist"}, Date: "2024-05-06", WebpageURL: testLink,
	}}}
	e.svc.SetDownloader(dl)

	job, err := e.svc.StartDownload(e.ctx, e.user, DownloadRequest{URL: " " + testLink + " ", LibraryID: e.lib.ID, Dir: "Downloads"})
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != JobQueued || job.Site != ytdlp.SiteYouTube || job.Format != ytdlp.FormatBest || job.CreatedBy != "editor" {
		t.Fatalf("job %+v", job)
	}
	done := e.waitJob(job.ID)
	if done.Status != JobDone || len(done.TrackIDs) != 1 || done.Error != "" || done.Progress != 1 {
		t.Fatalf("finished job %+v", done)
	}
	if len(dl.got) != 1 || dl.got[0].Target.URL != testLink || dl.got[0].Playlist {
		t.Fatalf("download requests %+v", dl.got)
	}

	rel := "Downloads/Synthetic_ Song_.mp3"
	if !e.exists(rel) {
		t.Fatalf("%s was not placed in the library", rel)
	}
	raw, err := tags.ReadRaw(e.abs(rel))
	if err != nil {
		t.Fatal(err)
	}
	if raw["TITLE"][0] != "Synthetic: Song?" || raw["ARTIST"][0] != "Synthetic Artist" || raw["COMMENT"][0] != testLink {
		t.Errorf("tags written through TagLib: %v", raw)
	}
	pic, err := tags.ReadPicture(e.abs(rel))
	if err != nil || pic == nil {
		t.Fatalf("no cover embedded: %v", err)
	}
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(pic)); err != nil || cfg.Width != cfg.Height {
		t.Errorf("cover is not square: %+v %v", cfg, err)
	}

	entries, _, err := e.st.ListEditLog(e.ctx, done.TrackIDs[0], 0, 0)
	if err != nil || len(entries) != 1 || entries[0].Action != "download" {
		t.Fatalf("edit log %+v %v", entries, err)
	}
	var details map[string]any
	_ = json.Unmarshal(entries[0].Details, &details)
	if details["source"] != testLink || details["title"] != "Synthetic: Song?" {
		t.Errorf("edit log details %v", details)
	}
	if left, _ := filepath.Glob(filepath.Join(e.cfg.TmpDir(), "ytdlp-job-*")); len(left) != 0 {
		t.Errorf("work directories left behind: %v", left)
	}

	// Downloading again never overwrites the first file.
	job2, err := e.svc.StartDownload(e.ctx, e.user, DownloadRequest{URL: testLink, LibraryID: e.lib.ID, Dir: "Downloads"})
	if err != nil {
		t.Fatal(err)
	}
	if j := e.waitJob(job2.ID); j.Status != JobDone || !e.exists("Downloads/Synthetic_ Song_ (1).mp3") {
		t.Fatalf("second download %+v", j)
	}
}

func TestDownloadPlaylistNames(t *testing.T) {
	it := ytdlp.Item{Path: "/w/out/a.opus", Title: "Song / Part", PlaylistTitle: "My: List", PlaylistIndex: 3}
	if got := downloadName(it, true); got != "My_ List/03 Song _ Part.opus" {
		t.Errorf("playlist name %q", got)
	}
	if got := downloadName(it, false); got != "Song _ Part.opus" {
		t.Errorf("single name %q", got)
	}
	if got := downloadName(ytdlp.Item{Path: "/w/out/x.m4a", Title: "..", ID: "BV1Synthetic"}, false); got != "download.m4a" {
		t.Errorf("fallback name %q", got)
	}
}

func TestStartDownloadValidates(t *testing.T) {
	e := newEnvAt(t, t.TempDir(), false)
	e.enableDownloads(true)
	if _, err := e.svc.StartDownload(e.ctx, e.user, DownloadRequest{URL: testLink, LibraryID: e.lib.ID}); !errors.Is(err, errors.ErrUnsupported) {
		t.Fatalf("without a downloader: %v", err)
	}
	dl := &fakeDownloader{}
	e.svc.SetDownloader(dl)
	cases := map[string]DownloadRequest{
		"other host":   {URL: "https://evil.example.com/x", LibraryID: e.lib.ID},
		"format":       {URL: testLink, LibraryID: e.lib.ID, Format: "wav"},
		"no library":   {URL: testLink},
		"traversal":    {URL: testLink, LibraryID: e.lib.ID, Dir: "../outside"},
		"hidden dir":   {URL: testLink, LibraryID: e.lib.ID, Dir: ".rainy"},
		"bad library":  {URL: testLink, LibraryID: 999},
		"garbage link": {URL: "hello world", LibraryID: e.lib.ID},
	}
	for name, req := range cases {
		if _, err := e.svc.StartDownload(e.ctx, e.user, req); err == nil {
			t.Errorf("%s: accepted", name)
		} else if !errors.Is(err, store.ErrInvalid) && name != "traversal" {
			t.Errorf("%s: %v, want ErrInvalid", name, err)
		}
	}
	dl.notReady = ytdlp.ErrNotInstalled
	if _, err := e.svc.StartDownload(e.ctx, e.user, DownloadRequest{URL: testLink, LibraryID: e.lib.ID}); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("not installed: %v, want ErrConflict", err)
	}
	if len(dl.got) != 0 {
		t.Fatal("yt-dlp ran for a refused request")
	}
}

func TestDownloadCancelAndDisable(t *testing.T) {
	e := newEnvAt(t, t.TempDir(), false)
	e.enableDownloads(true)
	dl := &fakeDownloader{block: make(chan struct{})}
	e.svc.SetDownloader(dl)
	job, err := e.svc.StartDownload(e.ctx, e.user, DownloadRequest{URL: testLink, LibraryID: e.lib.ID})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 200 && len(e.svc.DownloadJobs()) > 0 && e.svc.DownloadJobs()[0].Status != JobRunning; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if j := e.svc.DownloadJobs()[0]; j.Status != JobRunning || j.Progress != 0.5 || j.Title != "Synthetic" {
		t.Fatalf("running job %+v", j)
	}
	if err := e.svc.RemoveDownload(job.ID); err != nil {
		t.Fatal(err)
	}
	if j := e.waitJob(job.ID); j.Status != JobCanceled {
		t.Fatalf("canceled job %+v", j)
	}
	// A finished job is removed from the list.
	if err := e.svc.RemoveDownload(job.ID); err != nil || len(e.svc.DownloadJobs()) != 0 {
		t.Fatalf("remove finished: %v, %d jobs", err, len(e.svc.DownloadJobs()))
	}
	if err := e.svc.RemoveDownload("unknown"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown job: %v", err)
	}

	// Turning the feature off stops queued jobs before yt-dlp runs.
	e.enableDownloads(false)
	close(dl.block)
	dl.got = nil
	job, err = e.svc.StartDownload(e.ctx, e.user, DownloadRequest{URL: testLink, LibraryID: e.lib.ID})
	if err != nil {
		t.Fatal(err)
	}
	if j := e.waitJob(job.ID); j.Status != JobError || len(dl.got) != 0 {
		t.Fatalf("job while disabled %+v, %d runs", j, len(dl.got))
	}
	e.svc.CloseDownloads()
}

func TestSquareCover(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 300, 100))
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	p := filepath.Join(t.TempDir(), "thumb.png")
	_ = os.WriteFile(p, b.Bytes(), 0o600)
	out, err := squareCover(p)
	if err != nil {
		t.Fatal(err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil || format != "jpeg" || cfg.Width != 100 || cfg.Height != 100 {
		t.Fatalf("cover %s %dx%d %v", format, cfg.Width, cfg.Height, err)
	}
	_ = os.WriteFile(p, []byte("not an image"), 0o600)
	if _, err := squareCover(p); err == nil {
		t.Fatal("garbage accepted")
	}
}

func TestDownloadFailureShowsYtdlpMessage(t *testing.T) {
	e := newEnvAt(t, t.TempDir(), false)
	e.enableDownloads(true)
	e.svc.SetDownloader(&fakeDownloader{err: fmt.Errorf("%w: [youtube] synthetic01: Video unavailable", ytdlp.ErrUpstream)})
	job, err := e.svc.StartDownload(e.ctx, e.user, DownloadRequest{URL: testLink, LibraryID: e.lib.ID})
	if err != nil {
		t.Fatal(err)
	}
	if j := e.waitJob(job.ID); j.Status != JobError || j.Error != "[youtube] synthetic01: Video unavailable" {
		t.Fatalf("failed job %+v", j)
	}
}
