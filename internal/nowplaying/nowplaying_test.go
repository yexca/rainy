package nowplaying

import (
	"testing"
	"time"
)

func TestTracker(t *testing.T) {
	tr := New()
	now := time.Now()
	tr.now = func() time.Time { return now }
	if l := tr.List(); l == nil || len(l) != 0 {
		t.Fatal("empty list must be non-nil")
	}
	tr.Set(Entry{UserID: "u1", Username: "a", TrackID: "t1", Player: "web"})
	now = now.Add(time.Second)
	tr.Set(Entry{UserID: "u2", Username: "b", TrackID: "t2", Player: "dsub"})
	tr.Set(Entry{UserID: "u1", Username: "a", TrackID: "t3", Player: "web"}) // replaces t1
	l := tr.List()
	if len(l) != 2 || l[0].TrackID != "t3" || l[1].TrackID != "t2" {
		t.Fatalf("%+v", l)
	}
	now = now.Add(TTL + time.Second)
	tr.Set(Entry{UserID: "u3", TrackID: "t4", Player: "web"})
	if l := tr.List(); len(l) != 1 || l[0].UserID != "u3" {
		t.Fatalf("stale entries must drop: %+v", l)
	}
	tr.Remove("u3", "web")
	if len(tr.List()) != 0 {
		t.Fatal("remove")
	}
}
