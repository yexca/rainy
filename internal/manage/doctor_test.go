package manage

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"os"
	"slices"
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"

	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/tags"
)

// gbkMojibake returns s encoded as GBK and wrongly decoded as Latin-1.
func gbkMojibake(t *testing.T, s string) string {
	t.Helper()
	b, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	r := make([]rune, len(b))
	for i, c := range b {
		r[i] = rune(c)
	}
	return string(r)
}

func TestIssues(t *testing.T) {
	e := newEnv(t)
	sum, err := e.svc.IssueSummary(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, typ := range IssueTypes {
		if _, ok := sum[typ]; !ok {
			t.Errorf("summary lacks %s", typ)
		}
	}
	if sum[IssueMissingTags] != 1 || sum[IssueDuplicates] != 0 || sum[IssueMissingFiles] != 0 {
		t.Fatalf("summary %v", sum)
	}
	items, total, err := e.svc.Issues(e.ctx, e.user.ID, IssueMissingTags, 0, 10)
	if err != nil || total != 1 || len(items) != 1 || items[0].Tracks[0].Path != "Unsorted/untagged demo.mp3" ||
		!strings.Contains(items[0].Message, "artist") {
		t.Fatalf("missing tags %+v %d %v", items, total, err)
	}

	// No cover: every seeded album; giving one a folder image removes it from the list.
	albums, n, _ := e.st.ListAlbums(e.ctx, store.AlbumQuery{})
	if sum[IssueNoCover] != n || n == 0 {
		t.Fatalf("no_cover %d of %d albums", sum[IssueNoCover], n)
	}
	if err := e.st.SetAlbumCoverPath(e.ctx, albums[0].ID, e.abs("x/cover.jpg")); err != nil {
		t.Fatal(err)
	}
	items, total, err = e.svc.Issues(e.ctx, e.user.ID, IssueNoCover, 0, 100)
	if err != nil || total != n-1 || len(items) != n-1 || items[0].Album == nil || len(items[0].Tracks) == 0 {
		t.Fatalf("no cover %d %v", total, err)
	}

	// Duplicates: same normalized artist + title within 2 s.
	orig := e.track("The Rainy Days/Blue Skies/01 Blue Skies.opus")
	dup := model.Track{LibraryID: e.lib.ID, Path: "Dupes/blue skies.mp3", Title: "BLUE  SKIES", Artist: "the rainy days",
		Album: "Dupes", AlbumArtist: "x", AlbumID: "d", ArtistID: "a", AlbumArtistID: "x", Duration: orig.Duration + 1.5}
	far := dup
	far.Path, far.Duration = "Dupes/long.mp3", orig.Duration+30
	if err := e.st.UpsertTracks(e.ctx, []model.Track{dup, far}); err != nil {
		t.Fatal(err)
	}
	items, total, err = e.svc.Issues(e.ctx, e.user.ID, IssueDuplicates, 0, 10)
	if err != nil || total != 1 || len(items[0].Tracks) != 2 {
		t.Fatalf("duplicates %+v %d %v", items, total, err)
	}
	if _, _, err := e.svc.Issues(e.ctx, e.user.ID, "bogus", 0, 10); !errors.Is(err, store.ErrInvalid) {
		t.Fatalf("unknown type: %v", err)
	}
}

func TestEncodingFromDBFields(t *testing.T) {
	e := newEnv(t)
	tr := e.track("Unsorted/untagged demo.mp3")
	tr.Title = gbkMojibake(t, "你好世界")
	if err := e.st.UpsertTracks(e.ctx, []model.Track{*tr}); err != nil {
		t.Fatal(err)
	}
	items, total, err := e.svc.Issues(e.ctx, e.user.ID, IssueEncoding, 0, 10)
	if err != nil || total != 1 || items[0].Tracks[0].ID != tr.ID || !strings.Contains(items[0].Message, "TITLE") {
		t.Fatalf("encoding issues %+v %d %v", items, total, err)
	}
}

func TestEncodingFixRawTags(t *testing.T) {
	e := newEnv(t)
	rel := "林雨晴/夏日微风 (2021)/02 海边的约定.mp3"
	requireTags(t, e.abs(rel))
	tr := e.track(rel)
	bad := gbkMojibake(t, "海边的约定")
	if err := tags.Write(e.abs(rel), map[string][]string{"TITLE": {bad}}); err != nil {
		t.Fatal(err)
	}
	if raw, _ := tags.ReadRaw(e.abs(rel)); !slices.Equal(raw["TITLE"], []string{bad}) {
		t.Skipf("tag round trip changed the mojibake: %q", raw["TITLE"])
	}
	sum, err := e.svc.IssueSummary(e.ctx)
	if err != nil || sum[IssueEncoding] != 1 {
		t.Fatalf("summary %v %v", sum, err)
	}
	prev, err := e.svc.Encoding(e.ctx, e.user, []string{tr.ID}, false)
	if err != nil || len(prev.Items) != 1 || prev.Result != nil {
		t.Fatalf("preview %+v %v", prev, err)
	}
	if c := prev.Items[0].Changes["TITLE"]; prev.Items[0].Encoding != "gbk" || !slices.Equal(c.New, []string{"海边的约定"}) {
		t.Fatalf("fix %+v", prev.Items[0])
	}
	if raw, _ := tags.ReadRaw(e.abs(rel)); raw["TITLE"][0] != bad {
		t.Fatal("preview wrote the file")
	}
	res, err := e.svc.Encoding(e.ctx, e.user, nil, true) // nil = every detected issue
	if err != nil || res.Result == nil || len(res.Result.Updated) != 1 || len(res.Result.Errors) != 0 {
		t.Fatalf("apply %+v %v", res, err)
	}
	if raw, _ := tags.ReadRaw(e.abs(rel)); raw["TITLE"][0] != "海边的约定" {
		t.Fatalf("not fixed: %q", raw["TITLE"])
	}
	if acts := e.logActions(tr.ID); len(acts) != 1 || acts[0] != "encoding" {
		t.Fatalf("log %v", acts)
	}
	if sum, _ := e.svc.IssueSummary(e.ctx); sum[IssueEncoding] != 0 {
		t.Fatalf("still reported: %v", sum)
	}
}

func TestListFolder(t *testing.T) {
	e := newEnv(t)
	root, err := e.svc.ListFolder(e.ctx, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if root.Parent != nil || root.Dir != "" || !root.Writable || len(root.Folders) != 8 {
		t.Fatalf("root %+v", root)
	}
	l, err := e.svc.ListFolder(e.ctx, e.lib.ID, "/林雨晴/夏日微风 (2021)/")
	if err != nil {
		t.Fatal(err)
	}
	if l.Parent == nil || *l.Parent != "林雨晴" || len(l.Folders) != 0 || len(l.Files) != 6 {
		t.Fatalf("listing %+v", l)
	}
	if f := l.Files[0]; f.Name != "01 夏日微风.mp3" || !f.IsAudio || f.TrackID == nil || f.Path != "林雨晴/夏日微风 (2021)/01 夏日微风.mp3" {
		t.Fatalf("file %+v", f)
	}
	art, _ := e.svc.ListFolder(e.ctx, e.lib.ID, "林雨晴")
	if len(art.Files) != 1 || !art.Files[0].IsImage || art.Files[0].TrackID != nil {
		t.Fatalf("artist dir %+v", art.Files)
	}
	for _, bad := range []string{"../x", "nope"} {
		if _, err := e.svc.ListFolder(e.ctx, e.lib.ID, bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if err := e.svc.RescanFolder(e.ctx, e.lib.ID, "林雨晴"); err != nil {
		t.Fatal(err)
	}
}

func pngImage(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = uint8(i)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPrepareCover(t *testing.T) {
	small := pngImage(t, 300, 300)
	pc, err := prepareCover(small)
	if err != nil || pc.ext != "png" || !bytes.Equal(pc.data, small) {
		t.Fatalf("small png must be kept: %v", err)
	}
	big := pngImage(t, 2000, 1000)
	pc, err = prepareCover(big)
	if err != nil || pc.ext != "jpg" {
		t.Fatalf("big: %v", err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(pc.data))
	if err != nil || format != "jpeg" || cfg.Width != 1400 || cfg.Height != 700 {
		t.Fatalf("resized %v %s %dx%d", err, format, cfg.Width, cfg.Height)
	}
	for _, bad := range [][]byte{nil, []byte("not an image")} {
		if _, err := prepareCover(bad); !errors.Is(err, store.ErrInvalid) {
			t.Errorf("bad image accepted: %v", err)
		}
	}
}

func TestSetAndRemoveCover(t *testing.T) {
	e := newEnv(t)
	dir := "Northern Echo/Aurora Lights/CD1"
	rel := dir + "/1-01 Polar Night.flac"
	requireTags(t, e.abs(rel))
	tr := e.track(rel)
	if err := os.WriteFile(e.abs(dir+"/Cover.PNG"), pngImage(t, 10, 10), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := e.svc.SetCover(e.ctx, e.user, CoverRequest{TrackIDs: []string{tr.ID}, Embed: true, SaveToFolder: true, Image: pngImage(t, 1800, 1800)})
	if err != nil || len(res.Errors) != 0 || len(res.Updated) != 1 {
		t.Fatalf("set cover %+v %v", res, err)
	}
	pic, err := tags.ReadPicture(e.abs(rel))
	if err != nil || len(pic) == 0 {
		t.Fatalf("picture not embedded: %v", err)
	}
	if cfg, format, err := image.DecodeConfig(bytes.NewReader(pic)); err != nil || format != "jpeg" || cfg.Width != 1400 {
		t.Fatalf("embedded %s %d %v", format, cfg.Width, err)
	}
	if !e.exists(dir+"/cover.jpg") || e.exists(dir+"/Cover.PNG") {
		t.Fatal("folder image not saved / old variant not removed")
	}
	al, _ := e.st.GetAlbum(e.ctx, tr.AlbumID, "")
	if al.CoverPath != e.abs(dir+"/cover.jpg") {
		t.Fatalf("album cover path %q", al.CoverPath)
	}
	if data, ctype, err := e.svc.Picture(e.ctx, tr.ID); err != nil || ctype != "image/jpeg" || len(data) == 0 {
		t.Fatalf("picture endpoint %s %v", ctype, err)
	}

	res, err = e.svc.RemoveCover(e.ctx, e.user, RemoveCoverRequest{TrackIDs: []string{tr.ID}, RemoveFolderImage: true})
	if err != nil || len(res.Errors) != 0 {
		t.Fatalf("remove %+v %v", res, err)
	}
	if e.exists(dir + "/cover.jpg") {
		t.Fatal("folder image not removed")
	}
	// The trash holds Cover.PNG (replaced by SetCover) and the cover.jpg removed now.
	if trash, _ := e.svc.ListTrash(e.ctx); len(trash) != 2 || trash[0].OriginalPath != dir+"/cover.jpg" {
		t.Fatalf("folder image must go to the trash: %+v", trash)
	}
	if al, _ := e.st.GetAlbum(e.ctx, tr.AlbumID, ""); al.CoverPath != "" {
		t.Fatalf("album cover path not cleared: %q", al.CoverPath)
	}
	if pic, _ := tags.ReadPicture(e.abs(rel)); len(pic) != 0 {
		t.Fatal("embedded picture not removed")
	}
	if acts := e.logActions(tr.ID); len(acts) != 2 || acts[0] != "cover" {
		t.Fatalf("log %v", acts)
	}
}
