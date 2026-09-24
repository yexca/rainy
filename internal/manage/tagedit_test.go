package manage

import (
	"encoding/json"
	"errors"
	"os"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"

	"rainy/internal/store"
	"rainy/internal/tags"
)

func TestNormalizeTagMap(t *testing.T) {
	got, err := normalizeTagMap(TagMap{"title": {" New "}, "Genre": {"Pop", "", "Rock"}, "COMMENT": {" keep "}, "MUSICBRAINZ_TRACKID": {}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got["TITLE"], []string{"New"}) || !slices.Equal(got["GENRE"], []string{"Pop", "Rock"}) ||
		!slices.Equal(got["COMMENT"], []string{" keep "}) || len(got["MUSICBRAINZ_TRACKID"]) != 0 {
		t.Fatalf("%v", got)
	}
	for _, bad := range []TagMap{
		{"": {"x"}}, {"_X": {"x"}}, {"TI\"TLE": {"x"}}, {"TI\tTLE": {"x"}}, {strings.Repeat("A", 65): {"x"}},
		{"PICTURE": {"x"}}, {"TITLE": {"a\x00b"}}, {"TITLE": {"\xff"}}, {"title": {"a"}, "TITLE": {"b"}},
	} {
		if _, err := normalizeTagMap(bad); !errors.Is(err, store.ErrInvalid) {
			t.Errorf("%v accepted: %v", bad, err)
		}
	}
}

func TestTagDiff(t *testing.T) {
	old := map[string][]string{"TITLE": {"a"}, "ARTIST": {"x"}}
	write, diff := tagDiff(old, TagMap{"TITLE": {"a"}, "ARTIST": {"y"}, "GENRE": {}, "ALBUM": {"z"}})
	if len(write) != 2 || !slices.Equal(write["ARTIST"], []string{"y"}) || !slices.Equal(write["ALBUM"], []string{"z"}) {
		t.Fatalf("write %v", write)
	}
	if d := diff["ARTIST"]; !slices.Equal(d.Old, []string{"x"}) || !slices.Equal(d.New, []string{"y"}) {
		t.Fatalf("diff %v", diff)
	}
	if d := diff["ALBUM"]; d.Old == nil || len(d.Old) != 0 {
		t.Fatalf("old must be [] not null: %#v", d)
	}
}

func TestSaveTagsWritesFileAndLog(t *testing.T) {
	e := newEnv(t)
	rel := "Northern Echo/Aurora Lights/CD1/1-01 Polar Night.flac"
	requireTags(t, e.abs(rel))
	tr := e.track(rel)
	res, err := e.svc.SaveTags(e.ctx, e.user, []TagEdit{
		{TrackID: tr.ID, Tags: TagMap{"TITLE": {"Polar Night (Remaster)"}, "comment": {"edited"}, "DISCSUBTITLE": {}}},
		{TrackID: "does-not-exist", Tags: TagMap{"TITLE": {"x"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Updated) != 1 || len(res.Errors) != 1 || res.Errors[0].TrackID != "does-not-exist" {
		t.Fatalf("result %+v", res)
	}
	raw, err := tags.ReadRaw(e.abs(rel))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(raw["TITLE"], []string{"Polar Night (Remaster)"}) || !slices.Equal(raw["COMMENT"], []string{"edited"}) {
		t.Fatalf("tags not written: %v", raw)
	}
	if len(raw["DISCSUBTITLE"]) != 0 {
		t.Fatalf("DISCSUBTITLE not deleted: %v", raw["DISCSUBTITLE"])
	}
	if !slices.Equal(raw["ARTIST"], []string{"Northern Echo"}) {
		t.Fatalf("untouched key changed: %v", raw["ARTIST"])
	}
	entries, total, err := e.st.ListEditLog(e.ctx, tr.ID, 0, 10)
	if err != nil || total != 1 || entries[0].Action != "tags" || entries[0].Username != "editor" {
		t.Fatalf("log %+v %v", entries, err)
	}
	var details struct {
		Changes map[string]Change `json:"changes"`
	}
	if err := json.Unmarshal(entries[0].Details, &details); err != nil {
		t.Fatal(err)
	}
	if c := details.Changes["TITLE"]; !slices.Equal(c.Old, []string{"Polar Night"}) || !slices.Equal(c.New, []string{"Polar Night (Remaster)"}) {
		t.Fatalf("details %s", entries[0].Details)
	}
	if len(e.sc.calls) != 1 || e.sc.calls[0][0] != rel {
		t.Fatalf("rescan %v", e.sc.calls)
	}

	// Saving identical values writes nothing and logs nothing.
	if _, err := e.svc.SaveTags(e.ctx, e.user, []TagEdit{{TrackID: tr.ID, Tags: TagMap{"TITLE": {"Polar Night (Remaster)"}}}}); err != nil {
		t.Fatal(err)
	}
	if _, total, _ := e.st.ListEditLog(e.ctx, tr.ID, 0, 10); total != 1 {
		t.Fatal("no-op edit was logged")
	}

	tt, err := e.svc.TrackTags(e.ctx, tr.ID, e.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !tt.Writable || tt.File.Format != "flac" || tt.File.Size == 0 || !slices.Equal(tt.Tags["TITLE"], []string{"Polar Night (Remaster)"}) || tt.Lrc != nil {
		t.Fatalf("track tags %+v", tt)
	}
}

func TestSetLyricsSidecarAndEmbedded(t *testing.T) {
	e := newEnv(t)
	rel := "林雨晴/城市夜雨 (2019)/03 末班车.flac"
	tr := e.track(rel)
	lrc := "林雨晴/城市夜雨 (2019)/03 末班车.lrc"
	if _, err := e.svc.SetLyrics(e.ctx, e.user, tr.ID, "[00:01.00]末班车\r\n", "lrc"); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(e.abs(lrc)); err != nil || string(b) != "[00:01.00]末班车\n" {
		t.Fatalf("lrc %q %v", b, err)
	}
	if _, err := e.svc.SetLyrics(e.ctx, e.user, tr.ID, "  ", "lrc"); err != nil {
		t.Fatal(err)
	}
	if e.exists(lrc) {
		t.Fatal("empty text must remove the sidecar")
	}
	if acts := e.logActions(tr.ID); len(acts) != 2 || acts[0] != "lyrics" {
		t.Fatalf("log %v", acts)
	}
	if _, err := e.svc.SetLyrics(e.ctx, e.user, tr.ID, "x", "file"); !errors.Is(err, store.ErrInvalid) {
		t.Fatalf("bad target: %v", err)
	}

	requireTags(t, e.abs(rel))
	if _, err := e.svc.SetLyrics(e.ctx, e.user, tr.ID, "line one\nline two", "embedded"); err != nil {
		t.Fatal(err)
	}
	raw, _ := tags.ReadRaw(e.abs(rel))
	if strings.Join(raw["LYRICS"], "\n") != "line one\nline two" {
		t.Fatalf("embedded lyrics %v", raw["LYRICS"])
	}
	if _, err := e.svc.SetLyrics(e.ctx, e.user, tr.ID, "", "embedded"); err != nil {
		t.Fatal(err)
	}
	raw, _ = tags.ReadRaw(e.abs(rel))
	if len(raw["LYRICS"]) != 0 {
		t.Fatalf("embedded lyrics not removed: %v", raw["LYRICS"])
	}
}

func TestReadonlyErrors(t *testing.T) {
	e := newEnv(t)
	rel := "林雨晴/城市夜雨 (2019)/01 城市夜雨.flac"
	tr := e.track(rel)
	lrc := e.abs("林雨晴/城市夜雨 (2019)/01 城市夜雨.lrc")
	if err := os.Chmod(lrc, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(lrc, 0o644) })
	if fileWritable(lrc) {
		t.Skip("running with privileges that ignore file permissions")
	}
	_, err := e.svc.SetLyrics(e.ctx, e.user, tr.ID, "new text", "lrc")
	var re *ReadonlyError
	if !errors.As(err, &re) || !strings.Contains(re.Error(), "PUID/PGID") {
		t.Fatalf("want ReadonlyError, got %v", err)
	}

	// Tag writes on a read-only file: the whole batch fails with ReadonlyError.
	audio := e.abs(rel)
	if err := os.Chmod(audio, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(audio, 0o644) })
	_, err = e.svc.SaveTags(e.ctx, e.user, []TagEdit{{TrackID: tr.ID, Tags: TagMap{"TITLE": {"changed"}}}})
	if !errors.As(err, &re) || re.Path != rel {
		t.Fatalf("tag write on read-only file: %v", err)
	}
	if _, err := e.svc.RemoveCover(e.ctx, e.user, RemoveCoverRequest{TrackIDs: []string{tr.ID}}); err != nil && !errors.As(err, &re) {
		t.Fatalf("cover removal: %v", err)
	}

	for _, err := range []error{syscall.EROFS, syscall.EACCES, syscall.EPERM, &os.PathError{Op: "open", Path: "x", Err: os.ErrPermission}} {
		if !IsReadonly(err) {
			t.Errorf("IsReadonly(%v) = false", err)
		}
	}
	if IsReadonly(errors.New("boom")) || IsReadonly(nil) {
		t.Fatal("false positive")
	}
	res := newBatch()
	res.fail("a", "a.mp3", fsErr("a.mp3", syscall.EROFS))
	res.fail("b", "b.mp3", fsErr("b.mp3", &os.PathError{Op: "open", Path: "b", Err: os.ErrPermission}))
	if err := res.failure(); !errors.As(err, &re) {
		t.Fatalf("all-readonly batch must fail: %v", err)
	}
	res.fail("c", "c.mp3", errors.New("other"))
	if res.failure() != nil {
		t.Fatal("mixed failures must be reported per item")
	}
	if runtime.GOOS == "windows" && !IsReadonly(syscall.Errno(5)) { // ERROR_ACCESS_DENIED
		t.Fatal("windows access denied")
	}
}
