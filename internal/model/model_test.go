package model

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestTrackJSON(t *testing.T) {
	tr := Track{ID: "x", Genre: "Pop; Rock", Lyrics: "secret", SearchText: "s"}
	b, err := json.Marshal(tr)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"genres":["Pop","Rock"]`, `"libraryId":0`, `"albumArtistId":""`, `"rgTrackGain":null`,
		`"mbzTrackId":""`, `"starredAt":null`, `"hasLyrics":false`, `"contentType":""`, `"bpm":0`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s in %s", want, s)
		}
	}
	for _, bad := range []string{"secret", "searchText", "hasEmbeddedLyrics", "albumUpdatedAt", `"lyrics"`} {
		if strings.Contains(s, bad) {
			t.Errorf("leaked %s in %s", bad, s)
		}
	}
	empty, _ := json.Marshal(&Track{})
	if !strings.Contains(string(empty), `"genres":[]`) {
		t.Errorf("nil genres must marshal as []: %s", empty)
	}
}

func TestTrackFill(t *testing.T) {
	at := int64(5)
	tr := Track{Genre: " Pop ;; J-Pop/Anime ", StarredAt: &at, HasLrc: false, HasEmbeddedLyrics: true}
	tr.Fill()
	if len(tr.Genres) != 2 || tr.Genres[1] != "J-Pop/Anime" || !tr.Starred || !tr.HasLyrics {
		t.Fatalf("bad fill: %+v", tr)
	}
}

func TestAlbumUserJSONNames(t *testing.T) {
	b, _ := json.Marshal(Album{AlbumArtist: "A", AlbumArtistID: "1", CoverPath: "/x", CoverTrackID: "t"})
	s := string(b)
	if !strings.Contains(s, `"artist":"A"`) || !strings.Contains(s, `"artistId":"1"`) || strings.Contains(s, "/x") {
		t.Fatal(s)
	}
	h := "hash"
	u := User{PasswordEnc: "synthetic-enc", APIKeyHash: &h}
	u.Fill()
	b, _ = json.Marshal(u)
	s = string(b)
	if strings.Contains(s, "enc") || strings.Contains(s, "hash\"") || !strings.Contains(s, `"hasApiKey":true`) {
		t.Fatal(s)
	}
	if !(&User{CanManage: true}).CanEdit() || (&User{}).CanEdit() || (*User)(nil).CanEdit() {
		t.Fatal("CanEdit")
	}
}

func TestQueueAndLogJSON(t *testing.T) {
	// PlayQueue must stay embeddable (GET /api/queue = PlayQueue & {tracks}).
	b, _ := json.Marshal(struct {
		*PlayQueue
		Tracks []Track `json:"tracks"`
	}{&PlayQueue{TrackIDs: []string{}}, []Track{}})
	if !strings.Contains(string(b), `"trackIds":[]`) || !strings.Contains(string(b), `"tracks":[]`) {
		t.Fatal(string(b))
	}
	b, _ = json.Marshal([]EditLogEntry{{ID: 1}})
	if !strings.Contains(string(b), `"details":{}`) {
		t.Fatal(string(b))
	}
	b, _ = json.Marshal(RadioStation{StreamURL: "u", HomepageURL: "h"})
	if !strings.Contains(string(b), `"streamUrl":"u"`) || !strings.Contains(string(b), `"homepageUrl":"h"`) {
		t.Fatal(string(b))
	}
}

func TestSettings(t *testing.T) {
	s := DefaultSettings(time.Hour)
	if s.ScanInterval != "1h" || s.ScanIntervalDuration() != time.Hour || s.TranscodeBitrate != 192 || !s.EnableDownloads {
		t.Fatalf("%+v", s)
	}
	if got := DefaultSettings(0).ScanIntervalDuration(); got != 0 {
		t.Fatal(got)
	}
	if len(s.IgnoredArticleList()) != 7 || len(s.CoverArtPatterns()) != 5 {
		t.Fatal("lists")
	}
	for d, want := range map[time.Duration]string{0: "0", 90 * time.Minute: "1h30m", 45 * time.Second: "45s", 2 * time.Hour: "2h", time.Minute: "1m"} {
		if got := FormatDuration(d); got != want {
			t.Errorf("FormatDuration(%v) = %q, want %q", d, got, want)
		}
	}
}
