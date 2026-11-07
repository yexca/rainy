package manage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"rainy/internal/lxmusic"
	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/tags"
	"rainy/internal/util"
	"rainy/internal/ytdlp"
)

var reviewOn = func() *bool { v := true; return &v }()

// reviewEnv is a library without tracks and a downloader that "downloads" a silent WAV.
func reviewEnv(t *testing.T) (*env, *fakeDownloader) {
	t.Helper()
	e := newEnvAt(t, t.TempDir(), false)
	src := filepath.Join(t.TempDir(), "source.wav")
	if err := os.WriteFile(src, silentWAV(), 0o600); err != nil {
		t.Fatal(err)
	}
	requireTags(t, src)
	e.enableDownloads(true)
	dl := &fakeDownloader{src: src, items: []ytdlp.Item{{ID: "synthetic01", Title: "Synthetic Song", Artists: []string{"Synthetic Artist"}, WebpageURL: testLink}}}
	e.svc.SetDownloader(dl)
	return e, dl
}

func (e *env) stageDownload(req DownloadRequest) DownloadJob {
	e.t.Helper()
	job, err := e.svc.StartDownload(e.ctx, e.user, req)
	if err != nil {
		e.t.Fatal(err)
	}
	if !job.Review {
		e.t.Fatalf("job does not review: %+v", job)
	}
	done := e.waitJob(job.ID)
	if done.Status != JobDone || len(done.TrackIDs) != 0 || len(done.StagedIDs) != 1 {
		e.t.Fatalf("finished job %+v", done)
	}
	return done
}

func (e *env) staged() []StagedItem {
	e.t.Helper()
	items, err := e.svc.ListStaged(e.ctx)
	if err != nil {
		e.t.Fatal(err)
	}
	return items
}

func TestReviewHoldsDownloadUntilImported(t *testing.T) {
	e, _ := reviewEnv(t)
	done := e.stageDownload(DownloadRequest{URL: testLink, LibraryID: e.lib.ID, Dir: "Downloads", Organize: keepNames, Review: reviewOn})

	// Nothing reached the library: no file, no track, no edit-log row, no rescan.
	if e.exists("Downloads/Synthetic Song.wav") {
		t.Fatal("a reviewed download was placed in the library")
	}
	var n int
	if err := e.st.DB().R.GetContext(e.ctx, &n, `SELECT COUNT(*) FROM tracks`); err != nil || n != 0 {
		t.Fatalf("%d tracks after staging (%v)", n, err)
	}
	if acts := e.logActions(""); len(acts) != 0 {
		t.Fatalf("edit log before the import: %v", acts)
	}

	items := e.staged()
	if len(items) != 1 || items[0].ID != done.StagedIDs[0] {
		t.Fatalf("staged items %+v", items)
	}
	it := items[0]
	if it.Kind != JobKindLink || it.Source != testLink || it.Site != ytdlp.SiteYouTube || it.Format != "wav" || it.CreatedBy != "editor" ||
		it.Target != "Downloads/Synthetic Song.wav" || it.Error != "" || it.Conflict.Exists || it.Tags["TITLE"][0] != "Synthetic Song" {
		t.Fatalf("staged item %+v", it)
	}

	// Correct the tags and organize it: the target follows the tags.
	item, err := e.svc.UpdateStaged(e.ctx, it.ID, StagedUpdate{
		Tags:     TagMap{"title": {" Fixed "}, "ALBUM": {"Review Album"}, "ALBUMARTIST": {"Review Artist"}, "TRACKNUMBER": {"3"}},
		Organize: reviewOn,
	})
	if err != nil {
		t.Fatal(err)
	}
	if item.Target != "Downloads/Review Artist/Review Album/03 Fixed.wav" || !item.Organize || item.Tags["TITLE"][0] != "Fixed" {
		t.Fatalf("updated item %+v", item)
	}

	// The item survives a restart.
	restarted := newService(e.st, e.sc, nil, e.bus, e.cfg)
	if items, err := restarted.ListStaged(e.ctx); err != nil || len(items) != 1 || items[0].Target != item.Target {
		t.Fatalf("after a restart: %+v %v", items, err)
	}

	res, err := e.svc.ImportStaged(e.ctx, e.user, []string{it.ID}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Imported) != 1 || len(res.Updated) != 1 || len(res.Errors) != 0 || len(res.Skipped) != 0 {
		t.Fatalf("import %+v", res)
	}
	if !e.exists(item.Target) {
		t.Fatalf("%s was not imported", item.Target)
	}
	if raw, err := tags.ReadRaw(e.abs(item.Target)); err != nil || raw["TITLE"][0] != "Fixed" {
		t.Fatalf("imported tags %v %v", raw, err)
	}
	entries, _, _ := e.st.ListEditLog(e.ctx, res.Updated[0].ID, 0, 0)
	if len(entries) != 1 || entries[0].Action != "download" {
		t.Fatalf("edit log %+v", entries)
	}
	var details map[string]any
	_ = json.Unmarshal(entries[0].Details, &details)
	if details["source"] != testLink || details["title"] != "Synthetic Song" || details["reviewed"] != true || details["size"] == nil {
		t.Errorf("edit log details %v", details)
	}
	if items := e.staged(); len(items) != 0 {
		t.Fatalf("still staged after the import: %+v", items)
	}
	if _, err := os.Stat(filepath.Join(e.cfg.StagingDir(), it.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("staging directory left behind: %v", err)
	}
}

func TestReviewConflicts(t *testing.T) {
	e, _ := reviewEnv(t)
	req := DownloadRequest{URL: testLink, LibraryID: e.lib.ID, Dir: "Downloads", Organize: keepNames, Review: reviewOn}
	first := e.stageDownload(req)
	if _, err := e.svc.ImportStaged(e.ctx, e.user, first.StagedIDs, ConflictKeep); err != nil {
		t.Fatal(err)
	}
	imported := e.track("Downloads/Synthetic Song.wav")

	// The same link again: the target exists, and the earlier download is reported.
	second := e.stageDownload(req)
	it := e.staged()[0]
	if !it.Conflict.Exists || len(it.Conflict.SameSource) != 1 || it.Conflict.SameSource[0].TrackID != imported.ID ||
		it.Conflict.SameSource[0].Path != imported.Path {
		t.Fatalf("conflicts %+v", it.Conflict)
	}

	// A track with the same artist, title and duration elsewhere is similar (the imported
	// one is not: the fake scanner derives its row from the path).
	md, err := tags.Read(e.stagedFileOf(t, it.ID), tags.ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	other := trackFromPath(e.lib.ID, "Elsewhere/Album/01 Synthetic Song.wav")
	other.ID, other.Artist, other.Duration = util.NewID(), md.Artist, md.Duration
	if err := e.st.UpsertTracks(e.ctx, []model.Track{other}); err != nil {
		t.Fatal(err)
	}
	if it := e.staged()[0]; len(it.Conflict.Similar) != 1 || it.Conflict.Similar[0].TrackID != other.ID {
		t.Fatalf("similar %+v", it.Conflict.Similar)
	}

	// skip leaves the item staged; keep imports it next to the existing file.
	res, err := e.svc.ImportStaged(e.ctx, e.user, second.StagedIDs, ConflictSkip)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Skipped) != 1 || len(res.Imported) != 0 || len(e.staged()) != 1 {
		t.Fatalf("skip %+v", res)
	}
	res, err = e.svc.ImportStaged(e.ctx, e.user, second.StagedIDs, ConflictKeep)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Imported) != 1 || !e.exists("Downloads/Synthetic Song (1).wav") || len(e.staged()) != 0 {
		t.Fatalf("keep %+v", res)
	}
	if mustRead(t, e.abs("Downloads/Synthetic Song.wav")) == nil {
		t.Fatal("the first file was changed")
	}
}

func (e *env) stagedFileOf(t *testing.T, id string) string {
	t.Helper()
	m, err := e.svc.readStaged(id)
	if err != nil {
		t.Fatal(err)
	}
	return e.svc.stagedFile(m)
}

func TestReviewDiscardAndValidation(t *testing.T) {
	e, _ := reviewEnv(t)
	done := e.stageDownload(DownloadRequest{URL: testLink, LibraryID: e.lib.ID, Review: reviewOn})
	id := done.StagedIDs[0]

	for name, bad := range map[string]string{"traversal": "../" + id, "short": "abc", "unknown": util.NewID()} {
		if _, err := e.svc.UpdateStaged(e.ctx, bad, StagedUpdate{}); !errors.Is(err, store.ErrNotFound) {
			t.Errorf("%s id: %v, want ErrNotFound", name, err)
		}
	}
	if _, err := e.svc.UpdateStaged(e.ctx, id, StagedUpdate{Tags: TagMap{"PICTURE": {"x"}}}); !errors.Is(err, store.ErrInvalid) {
		t.Errorf("picture as text: %v", err)
	}
	dir := "../outside"
	if _, err := e.svc.UpdateStaged(e.ctx, id, StagedUpdate{Dir: &dir}); err == nil {
		t.Error("a destination outside the library was accepted")
	}
	hidden := ".rainy"
	if _, err := e.svc.UpdateStaged(e.ctx, id, StagedUpdate{Dir: &hidden}); !errors.Is(err, store.ErrInvalid) {
		t.Errorf("hidden destination: %v", err)
	}
	if _, err := e.svc.ImportStaged(e.ctx, e.user, nil, ""); !errors.Is(err, store.ErrInvalid) {
		t.Errorf("no ids: %v", err)
	}
	if _, err := e.svc.ImportStaged(e.ctx, e.user, []string{id}, "replace"); !errors.Is(err, store.ErrInvalid) {
		t.Errorf("unknown policy: %v", err)
	}
	if _, _, err := e.svc.StagedPicture(id); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("picture of a file without one: %v", err)
	}
	if res, err := e.svc.ImportStaged(e.ctx, e.user, []string{util.NewID()}, ""); err != nil || len(res.Errors) != 1 {
		t.Errorf("unknown id import: %+v %v", res, err)
	}

	if n, err := e.svc.DiscardStaged([]string{id, "../x", util.NewID()}); err != nil || n != 1 {
		t.Fatalf("discard: %d %v", n, err)
	}
	if items := e.staged(); len(items) != 0 {
		t.Fatalf("still staged: %+v", items)
	}
}

func TestReviewFollowsSettingAndOnlineDownloads(t *testing.T) {
	e, _ := reviewEnv(t)
	set := model.DefaultSettings(e.cfg.ScanInterval)
	set.YtdlpEnabled, set.LxSourcesEnabled, set.ReviewDownloads = true, true, true
	if err := e.st.SaveSettings(e.ctx, set); err != nil {
		t.Fatal(err)
	}
	e.stageDownload(DownloadRequest{URL: testLink, LibraryID: e.lib.ID}) // the setting decides

	src := filepath.Join(t.TempDir(), "online.wav")
	if err := os.WriteFile(src, silentWAV(), 0o600); err != nil {
		t.Fatal(err)
	}
	e.svc.SetOnlineSource(&fakeOnline{src: src, cands: []lxmusic.Candidate{{ID: "c", Name: "Working"}}, links: map[string]string{"c": "audio"}})
	jobs, err := e.svc.StartOnlineDownload(e.ctx, e.user, OnlineDownloadRequest{Songs: []lxmusic.Song{onlineSong}, LibraryID: e.lib.ID})
	if err != nil {
		t.Fatal(err)
	}
	done := e.waitJob(jobs[0].ID)
	if done.Status != JobDone || !done.Review || len(done.StagedIDs) != 1 || len(done.TrackIDs) != 0 {
		t.Fatalf("online job %+v", done)
	}
	var online *StagedItem
	for _, it := range e.staged() {
		if it.ID == done.StagedIDs[0] {
			online = &it
		}
	}
	if online == nil || online.Kind != JobKindOnline || online.Source != done.URL || online.Source == "" || online.Tags["ALBUM"][0] != "Synthetic Album" {
		t.Fatalf("online item %+v", online)
	}

	// An explicit review=false imports right away.
	job, err := e.svc.StartDownload(e.ctx, e.user, DownloadRequest{URL: testLink, LibraryID: e.lib.ID, Review: keepNames})
	if err != nil {
		t.Fatal(err)
	}
	if j := e.waitJob(job.ID); j.Review || len(j.TrackIDs) != 1 || len(j.StagedIDs) != 0 {
		t.Fatalf("job without review %+v", j)
	}
}

func TestNewRemovesHalfWrittenStagedItems(t *testing.T) {
	e, _ := reviewEnv(t)
	done := e.stageDownload(DownloadRequest{URL: testLink, LibraryID: e.lib.ID, Review: reviewOn})
	half := filepath.Join(e.cfg.StagingDir(), stagedNewPrefix+util.NewID())
	if err := os.MkdirAll(half, 0o700); err != nil {
		t.Fatal(err)
	}
	New(e.st, nil, nil, e.bus, e.cfg)
	if _, err := os.Stat(half); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("half-written item survived: %v", err)
	}
	if items := e.staged(); len(items) != 1 || items[0].ID != done.StagedIDs[0] {
		t.Fatalf("complete items must survive a restart: %+v", items)
	}
}
