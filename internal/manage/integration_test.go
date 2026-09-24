package manage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"rainy/internal/artwork"
	"rainy/internal/scanner"
	"rainy/internal/store"
)

// TestWithRealScanner runs the main flows against the real scanner (tags are read from
// the files) to check the RescanFiles contract: ids survive rename, delete + restore and
// tag edits.
func TestWithRealScanner(t *testing.T) {
	music := filepath.Join(t.TempDir(), "music")
	copyTree(t, musicSource(t), music)
	e := newEnvAt(t, music, false)
	sc := scanner.New(e.st, e.bus, e.cfg)
	e.svc = New(e.st, sc, artwork.New(e.st, e.cfg.ArtworkCacheDir()), e.bus, e.cfg)
	if err := sc.Run(e.ctx, scanner.Options{Full: true}); err != nil {
		if errors.Is(err, errors.ErrUnsupported) {
			t.Skip("scanner not implemented yet")
		}
		t.Fatal(err)
	}
	ctx := context.Background()
	rel := "The Rainy Days/Blue Skies/01 Blue Skies.opus"
	tr := e.track(rel)
	if tr.Title != "Blue Skies" {
		t.Fatalf("scanned title %q", tr.Title)
	}

	// Tag edit → the rescan updates the row (same id).
	res, err := e.svc.SaveTags(ctx, e.user, []TagEdit{{TrackID: tr.ID, Tags: TagMap{"TITLE": {"Blue Skies (Edit)"}}}})
	if err != nil || len(res.Errors) != 0 || res.Updated[0].Title != "Blue Skies (Edit)" || res.Updated[0].ID != tr.ID {
		t.Fatalf("save tags %+v %v", res, err)
	}

	// Rename keeps the id.
	res, err = e.svc.Rename(ctx, e.user, []string{tr.ID}, "{albumartist}/{album}/{title}")
	if err != nil || len(res.Errors) != 0 || res.Updated[0].Path != "The Rainy Days/Blue Skies/Blue Skies (Edit).opus" || res.Updated[0].ID != tr.ID {
		t.Fatalf("rename %+v %v", res, err)
	}
	if _, err := e.st.GetTrackByPath(ctx, e.lib.ID, rel); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("old path still present: %v", err)
	}

	// Delete + restore keeps the id and clears the missing flag.
	if _, err := e.svc.Delete(ctx, e.user, []string{tr.ID}); err != nil {
		t.Fatal(err)
	}
	// A full scan while the file is in the trash must not drop the row.
	if err := sc.Run(ctx, scanner.Options{}); err != nil {
		t.Fatal(err)
	}
	if got, err := e.st.GetTrack(ctx, tr.ID, ""); err != nil || !got.Missing {
		t.Fatalf("trashed track %+v %v", got, err)
	}
	trash, _ := e.svc.ListTrash(ctx)
	res, err = e.svc.Restore(ctx, e.user, []string{trash[0].ID})
	if err != nil || len(res.Updated) != 1 || res.Updated[0].ID != tr.ID || res.Updated[0].Missing {
		t.Fatalf("restore %+v %v", res, err)
	}

	// Upload → scanned immediately with real metadata.
	up := multipartBody(t, []upFile{{"x.flac", mustRead(t, e.abs("Northern Echo/Aurora Lights/CD1/1-01 Polar Night.flac"))}},
		map[string]string{"libraryId": "1", "dir": "Incoming"})
	res, err = e.svc.Upload(ctx, e.user, up)
	if err != nil || len(res.Updated) != 1 || res.Updated[0].Title != "Polar Night" || res.Updated[0].Path != "Incoming/x.flac" {
		t.Fatalf("upload %+v %v", res, err)
	}
}
