package api

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"testing"

	"rainy/internal/model"
	"rainy/internal/store"
)

func TestNativeLibraryRequiresAuth(t *testing.T) {
	e := newNativeEnv(t)
	for _, p := range []string{"/home", "/albums", "/albums/x", "/artists", "/artists/x", "/tracks", "/tracks/x",
		"/genres", "/search?q=a", "/starred", "/random", "/recent-tracks"} {
		if code := nativeErrorCode(t, e.do("GET", p, "", nil), 401); code != CodeUnauthorized {
			t.Errorf("%s: %s", p, code)
		}
		nativeErrorCode(t, e.do("GET", p, "not-a-token", nil), 401)
	}
}

func TestNativeHome(t *testing.T) {
	e := newNativeEnv(t)
	h := nativeJSON[home](t, e.do("GET", "/home", e.aliceTok, nil), 200)
	if h.Stats != (homeStats{Tracks: 6, Albums: 3, Artists: 3}) {
		t.Fatalf("stats %+v", h.Stats)
	}
	if len(h.RecentlyAdded) != 3 || h.RecentlyAdded[0].ID != e.hits {
		t.Fatalf("recently added %v", nativeAlbumIDs(h.RecentlyAdded))
	}
	if len(h.Random) != 3 || len(h.RecentlyPlayed) != 0 || len(h.MostPlayed) != 0 || len(h.Starred) != 0 {
		t.Fatalf("home %+v", h)
	}
	ctx := t.Context()
	for i, k := range []string{"tone", "tone", "one"} {
		if err := e.app.Store.RecordPlay(ctx, e.alice.ID, e.id(k), int64(1_000_000+i), "test"); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.app.Store.SetStarred(ctx, e.alice.ID, "album", []string{e.hits}, true); err != nil {
		t.Fatal(err)
	}
	h = nativeJSON[home](t, e.do("GET", "/home", e.aliceTok, nil), 200)
	if got := nativeAlbumIDs(h.RecentlyPlayed); !slices.Equal(got, []string{e.blueSkies, e.tones}) {
		t.Errorf("recently played %v", got)
	}
	if got := nativeAlbumIDs(h.MostPlayed); !slices.Equal(got, []string{e.tones, e.blueSkies}) {
		t.Errorf("most played %v", got)
	}
	if got := nativeAlbumIDs(h.Starred); !slices.Equal(got, []string{e.hits}) || !h.Starred[0].Starred {
		t.Errorf("starred %v", got)
	}
	// Per-user: bob has played nothing.
	if h = nativeJSON[home](t, e.do("GET", "/home", e.bobTok, nil), 200); len(h.RecentlyPlayed) != 0 || len(h.Starred) != 0 {
		t.Error("annotations leaked across users")
	}
	// JSON arrays, never null.
	body := e.do("GET", "/home", e.bobTok, nil).Body.String()
	for _, k := range []string{`"recentlyPlayed":[]`, `"mostPlayed":[]`, `"starred":[]`} {
		if !strings.Contains(body, k) {
			t.Errorf("%s lacks %s", body, k)
		}
	}
}

func TestNativeAlbums(t *testing.T) {
	e := newNativeEnv(t)
	list := func(q string) Page[model.Album] {
		return nativeJSON[Page[model.Album]](t, e.do("GET", "/albums"+q, e.aliceTok, nil), 200)
	}
	p := list("")
	if p.Total != 3 || !slices.Equal(nativeAlbumIDs(p.Items), []string{e.blueSkies, e.hits, e.tones}) {
		t.Fatalf("albums by name %v", nativeAlbumIDs(p.Items))
	}
	if p.Items[0].CoverArt == "" || !strings.HasPrefix(p.Items[0].CoverArt, "al-"+e.blueSkies+"_") || p.Items[0].SongCount != 3 {
		t.Fatalf("album fields %+v", p.Items[0])
	}
	if p = list("?sort=year&order=desc&limit=2"); p.Total != 3 || !slices.Equal(nativeAlbumIDs(p.Items), []string{e.hits, e.blueSkies}) {
		t.Errorf("year desc %v", nativeAlbumIDs(p.Items))
	}
	if p = list("?sort=name&offset=2&limit=5"); len(p.Items) != 1 || p.Items[0].ID != e.tones {
		t.Errorf("offset %v", nativeAlbumIDs(p.Items))
	}
	if p = list("?artistId=" + e.rainyDays); !slices.Equal(nativeAlbumIDs(p.Items), []string{e.blueSkies}) {
		t.Errorf("artist filter %v", nativeAlbumIDs(p.Items))
	}
	if p = list("?genre=Rock"); !slices.Equal(nativeAlbumIDs(p.Items), []string{e.blueSkies}) {
		t.Errorf("genre filter %v", nativeAlbumIDs(p.Items))
	}
	if p = list("?fromYear=2020&toYear=2024"); p.Total != 2 {
		t.Errorf("years %v", nativeAlbumIDs(p.Items))
	}
	if p = list("?q=blue%20ski"); !slices.Equal(nativeAlbumIDs(p.Items), []string{e.blueSkies}) {
		t.Errorf("query %v", nativeAlbumIDs(p.Items))
	}
	if p = list("?starred=true"); p.Total != 0 || p.Items == nil {
		t.Errorf("starred %+v", p)
	}
	if p = list(fmt.Sprintf("?libraryId=%d", e.lib.ID+99)); p.Total != 0 {
		t.Error("library filter")
	}
	if p = list("?limit=100000"); len(p.Items) != 3 {
		t.Error("limit clamp")
	}
}

func TestNativeAlbumDetail(t *testing.T) {
	e := newNativeEnv(t)
	type detail struct {
		model.Album
		Tracks []model.Track `json:"tracks"`
		Discs  []int         `json:"discs"`
	}
	d := nativeJSON[detail](t, e.do("GET", "/albums/"+e.blueSkies, e.aliceTok, nil), 200)
	if d.ID != e.blueSkies || d.Name != "Blue Skies" || d.AlbumArtist != "The Rainy Days" {
		t.Fatalf("album %+v", d.Album)
	}
	if got := nativeTrackIDs(d.Tracks); !slices.Equal(got, []string{e.id("one"), e.id("umbrella"), e.id("puddles")}) {
		t.Fatalf("tracks %v (missing track must not be listed)", got)
	}
	if !slices.Equal(d.Discs, []int{1, 2}) {
		t.Fatalf("discs %v", d.Discs)
	}
	if !d.Tracks[0].HasLyrics || d.Tracks[0].ContentType != "audio/mpeg" || len(d.Tracks[1].Genres) != 2 {
		t.Fatalf("track fields %+v", d.Tracks[0])
	}
	if code := nativeErrorCode(t, e.do("GET", "/albums/nope", e.aliceTok, nil), 404); code != CodeNotFound {
		t.Fatal(code)
	}
	if got := discNumbers(nil); got == nil || len(got) != 0 {
		t.Fatal("discNumbers(nil)")
	}
}

func TestNativeArtists(t *testing.T) {
	e := newNativeEnv(t)
	p := nativeJSON[Page[model.Artist]](t, e.do("GET", "/artists", e.aliceTok, nil), 200)
	names := []string{}
	for _, a := range p.Items {
		names = append(names, a.Name)
	}
	if p.Total != 3 || !slices.Equal(names, []string{"The Rainy Days", "Sine Wave", "Various Artists"}) {
		// Sorted by sort name, which ignores the leading article.
		t.Fatalf("artists %v", names)
	}
	if p = nativeJSON[Page[model.Artist]](t, e.do("GET", "/artists?all=1", e.aliceTok, nil), 200); p.Total != 4 {
		t.Fatalf("all artists %d", p.Total)
	}
	if p = nativeJSON[Page[model.Artist]](t, e.do("GET", "/artists?q=guest&all=true", e.aliceTok, nil), 200); p.Total != 1 || p.Items[0].ID != e.guest {
		t.Fatalf("artist query %+v", p)
	}
	if p = nativeJSON[Page[model.Artist]](t, e.do("GET", "/artists?starred=1", e.aliceTok, nil), 200); p.Total != 0 {
		t.Fatal("starred artists")
	}
}

func TestNativeArtistDetail(t *testing.T) {
	e := newNativeEnv(t)
	d := nativeJSON[artistDetail](t, e.do("GET", "/artists/"+e.rainyDays, e.aliceTok, nil), 200)
	if d.Name != "The Rainy Days" || !slices.Equal(nativeAlbumIDs(d.Albums), []string{e.blueSkies}) ||
		!slices.Equal(nativeAlbumIDs(d.AppearsOn), []string{e.hits}) {
		t.Fatalf("artist %+v albums %v appears %v", d.Artist, nativeAlbumIDs(d.Albums), nativeAlbumIDs(d.AppearsOn))
	}
	// No plays yet: random fill with all four available tracks (the missing one excluded).
	if got := nativeTrackIDs(d.TopTracks); len(got) != 4 || slices.Contains(got, e.id("gone")) {
		t.Fatalf("top tracks %v", got)
	}
	ctx := t.Context()
	for _, k := range []string{"umbrella", "umbrella", "umbrella", "feature", "feature", "one"} {
		if err := e.app.Store.RecordPlay(ctx, e.alice.ID, e.id(k), 0, "test"); err != nil {
			t.Fatal(err)
		}
	}
	d = nativeJSON[artistDetail](t, e.do("GET", "/artists/"+e.rainyDays, e.aliceTok, nil), 200)
	got := nativeTrackIDs(d.TopTracks)
	if len(got) != 4 || !slices.Equal(got[:3], []string{e.id("umbrella"), e.id("feature"), e.id("one")}) || got[3] != e.id("puddles") {
		t.Fatalf("top tracks by plays %v", got)
	}
	// Artists without their own albums still resolve (appears-on only).
	d = nativeJSON[artistDetail](t, e.do("GET", "/artists/"+e.guest, e.aliceTok, nil), 200)
	if len(d.Albums) != 0 || d.Albums == nil || !slices.Equal(nativeAlbumIDs(d.AppearsOn), []string{e.hits}) || len(d.TopTracks) != 1 {
		t.Fatalf("guest %+v", d)
	}
	nativeErrorCode(t, e.do("GET", "/artists/unknown", e.aliceTok, nil), 404)
}

func TestNativeTracks(t *testing.T) {
	e := newNativeEnv(t)
	list := func(tok, q string) Page[model.Track] {
		return nativeJSON[Page[model.Track]](t, e.do("GET", "/tracks"+q, tok, nil), 200)
	}
	if p := list(e.aliceTok, ""); p.Total != 6 || len(p.Items) != 6 {
		t.Fatalf("tracks total %d", p.Total)
	}
	if p := list(e.aliceTok, "?albumId="+e.blueSkies+"&sort=track"); !slices.Equal(nativeTrackIDs(p.Items), []string{e.id("one"), e.id("umbrella"), e.id("puddles")}) {
		t.Errorf("album tracks %v", nativeTrackIDs(p.Items))
	}
	if p := list(e.aliceTok, "?artistId="+e.va); p.Total != 2 {
		t.Errorf("album artist filter %d", p.Total)
	}
	if p := list(e.aliceTok, "?genre=Rock"); p.Total != 2 {
		t.Errorf("genre %d", p.Total)
	}
	if p := list(e.aliceTok, "?dir="+url.QueryEscape("Sine Wave/Tones")); p.Total != 1 || p.Items[0].ID != e.id("tone") {
		t.Errorf("dir %v", nativeTrackIDs(p.Items))
	}
	if p := list(e.aliceTok, "?dirPrefix="+url.QueryEscape("The Rainy Days")); p.Total != 3 {
		t.Errorf("dirPrefix %d", p.Total)
	}
	if p := list(e.aliceTok, "?fromYear=2024"); p.Total != 2 {
		t.Errorf("fromYear %d", p.Total)
	}
	if p := list(e.aliceTok, "?q=umbrella"); p.Total != 1 {
		t.Errorf("q %d", p.Total)
	}
	if p := list(e.aliceTok, "?sort=title&order=desc&limit=2&offset=1"); p.Total != 6 || len(p.Items) != 2 || p.Items[0].Title != "Tone" || p.Items[1].Title != "Rain One" {
		t.Errorf("paging %+v", nativeTrackIDs(p.Items))
	}

	// Missing tracks: ignored for normal users, honoured for managers.
	if p := list(e.aliceTok, "?missing=only"); p.Total != 6 {
		t.Errorf("missing filter must be ignored for non-managers: %d", p.Total)
	}
	if p := list(e.adminTok, "?missing=only"); p.Total != 1 || p.Items[0].ID != e.id("gone") || !p.Items[0].Missing {
		t.Errorf("missing=only %+v", nativeTrackIDs(p.Items))
	}
	if p := list(e.adminTok, "?missing=include"); p.Total != 7 {
		t.Errorf("missing=include %d", p.Total)
	}

	// ids keep their order unless a sort is given; unknown ids are skipped.
	ids := []string{e.id("tone"), "nope", e.id("one"), e.id("feature")}
	p := list(e.aliceTok, "?ids="+strings.Join(ids, ","))
	if !slices.Equal(nativeTrackIDs(p.Items), []string{e.id("tone"), e.id("one"), e.id("feature")}) || p.Total != 3 {
		t.Errorf("ids order %v", nativeTrackIDs(p.Items))
	}
	p = list(e.aliceTok, "?ids="+strings.Join(ids, ",")+"&offset=1&limit=1")
	if !slices.Equal(nativeTrackIDs(p.Items), []string{e.id("one")}) || p.Total != 3 {
		t.Errorf("ids paging %v", nativeTrackIDs(p.Items))
	}
	p = list(e.aliceTok, "?ids="+strings.Join(ids, ",")+"&sort=title")
	if !slices.Equal(nativeTrackIDs(p.Items), []string{e.id("feature"), e.id("one"), e.id("tone")}) {
		t.Errorf("ids sorted %v", nativeTrackIDs(p.Items))
	}
	if p = list(e.aliceTok, "?ids="); p.Total != 0 {
		t.Errorf("empty ids must match nothing: %d", p.Total)
	}
	many := strings.Repeat("x,", maxIDsPerRequest+1)
	nativeErrorCode(t, e.do("GET", "/tracks?ids="+many, e.aliceTok, nil), 400)

	// Single track (missing ones included, with the flag).
	tr := nativeJSON[model.Track](t, e.do("GET", "/tracks/"+e.id("gone"), e.aliceTok, nil), 200)
	if !tr.Missing || tr.Title != "Gone Missing" {
		t.Errorf("track %+v", tr)
	}
	nativeErrorCode(t, e.do("GET", "/tracks/nope", e.aliceTok, nil), 404)
}

func TestNativeGenresSearchStarredRandomRecent(t *testing.T) {
	e := newNativeEnv(t)
	genres := nativeJSON[[]model.Genre](t, e.do("GET", "/genres", e.aliceTok, nil), 200)
	byName := map[string]model.Genre{}
	for _, g := range genres {
		byName[g.Name] = g
	}
	if len(genres) != 3 || byName["Pop"].SongCount != 4 || byName["Rock"].SongCount != 2 || byName["Electronic"].AlbumCount != 1 {
		t.Fatalf("genres %+v", genres)
	}

	res := nativeJSON[store.SearchResult](t, e.do("GET", "/search?q=rainy", e.aliceTok, nil), 200)
	if len(res.Artists) != 1 || len(res.Albums) != 1 || len(res.Tracks) != 4 {
		t.Fatalf("search %d %d %d", len(res.Artists), len(res.Albums), len(res.Tracks))
	}
	res = nativeJSON[store.SearchResult](t, e.do("GET", "/search?q=rainy&tracks=2&albums=0", e.aliceTok, nil), 200)
	if len(res.Tracks) != 2 || len(res.Albums) != 0 || res.Albums == nil {
		t.Fatalf("search limits %+v", res)
	}
	res = nativeJSON[store.SearchResult](t, e.do("GET", "/search?q=%20", e.aliceTok, nil), 200)
	if len(res.Tracks)+len(res.Albums)+len(res.Artists) != 0 || res.Tracks == nil {
		t.Fatal("blank query must return nothing")
	}

	st := nativeJSON[store.SearchResult](t, e.do("GET", "/starred", e.aliceTok, nil), 200)
	if st.Tracks == nil || st.Albums == nil || st.Artists == nil || len(st.Tracks) != 0 {
		t.Fatal("starred empty")
	}
	ctx := t.Context()
	_ = e.app.Store.SetStarred(ctx, e.alice.ID, "track", []string{e.id("tone"), e.id("one")}, true)
	_ = e.app.Store.SetStarred(ctx, e.alice.ID, "artist", []string{e.sineWave}, true)
	_ = e.app.Store.SetStarred(ctx, e.alice.ID, "album", []string{e.tones}, true)
	st = nativeJSON[store.SearchResult](t, e.do("GET", "/starred", e.aliceTok, nil), 200)
	if len(st.Tracks) != 2 || len(st.Artists) != 1 || len(st.Albums) != 1 || !st.Tracks[0].Starred {
		t.Fatalf("starred %+v", st)
	}
	if st = nativeJSON[store.SearchResult](t, e.do("GET", "/starred", e.bobTok, nil), 200); len(st.Tracks) != 0 {
		t.Fatal("stars are per user")
	}

	rnd := nativeJSON[[]model.Track](t, e.do("GET", "/random?size=3", e.aliceTok, nil), 200)
	if len(rnd) != 3 {
		t.Fatalf("random %d", len(rnd))
	}
	if rnd = nativeJSON[[]model.Track](t, e.do("GET", "/random?genre=Electronic", e.aliceTok, nil), 200); len(rnd) != 1 {
		t.Fatalf("random genre %d", len(rnd))
	}
	if rnd = nativeJSON[[]model.Track](t, e.do("GET", "/random?fromYear=2030", e.aliceTok, nil), 200); rnd == nil || len(rnd) != 0 {
		t.Fatal("random empty")
	}

	rec := nativeJSON[[]model.Track](t, e.do("GET", "/recent-tracks", e.aliceTok, nil), 200)
	if rec == nil || len(rec) != 0 {
		t.Fatal("recent empty")
	}
	for i, k := range []string{"one", "tone", "one"} {
		if err := e.app.Store.RecordPlay(ctx, e.alice.ID, e.id(k), int64(1_000+i), "test"); err != nil {
			t.Fatal(err)
		}
	}
	rec = nativeJSON[[]model.Track](t, e.do("GET", "/recent-tracks?limit=5", e.aliceTok, nil), 200)
	if !slices.Equal(nativeTrackIDs(rec), []string{e.id("one"), e.id("tone")}) {
		t.Fatalf("recent %v", nativeTrackIDs(rec))
	}
	if rec = nativeJSON[[]model.Track](t, e.do("GET", "/recent-tracks?limit=1", e.aliceTok, nil), 200); len(rec) != 1 {
		t.Fatal("recent limit")
	}
}

func TestNativeOrderByIDs(t *testing.T) {
	ts := []model.Track{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	got := nativeTrackIDs(orderByIDs(ts, []string{"c", "x", "a", "c"}))
	if !slices.Equal(got, []string{"c", "a"}) {
		t.Fatal(got)
	}
}
