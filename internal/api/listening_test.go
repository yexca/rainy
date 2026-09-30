package api

import (
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"rainy/internal/listening"
	"rainy/internal/model"
	"rainy/internal/scrobble"
)

type playsPage struct {
	Items []model.Play `json:"items"`
	Total int          `json:"total"`
}

// Reports and history show only the signed-in user's own plays.
func TestNativeListening(t *testing.T) {
	e := newNativeEnv(t)
	nativeErrorCode(t, e.do("GET", "/listening/report", "", nil), 401)
	nativeErrorCode(t, e.do("GET", "/listening/history", "", nil), 401)

	now := time.Now()
	for i, key := range []string{"one", "one", "tone"} {
		at := now.Add(-time.Duration(i+1) * time.Hour).UnixMilli()
		nativeExpect(t, e.do("POST", "/scrobble", e.aliceTok, map[string]any{"trackId": e.id(key), "submission": true, "time": at}), 204)
	}
	nativeExpect(t, e.do("POST", "/scrobble", e.bobTok, map[string]any{"trackId": e.id("umbrella"), "submission": true}), 204)

	from := now.Add(-7 * 24 * time.Hour).UnixMilli()
	rep := nativeJSON[listening.Report](t, e.do("GET", "/listening/report?tz=Asia%2FShanghai&from="+itoa(from), e.aliceTok, nil), 200)
	if rep.Totals.Plays != 3 || rep.Totals.Tracks != 2 || len(rep.TopTracks) != 2 || rep.TopTracks[0].ID != e.id("one") {
		t.Fatalf("alice's report %+v tops %+v", rep.Totals, rep.TopTracks)
	}
	if rep.TopTracks[0].Track == nil || rep.TopTracks[0].Track.PlayCount != 2 || rep.Previous == nil || rep.Bucket != listening.BucketDay {
		t.Fatalf("report details %+v", rep)
	}
	if rep.TZ != "Asia/Shanghai" && rep.TZ != "UTC" { // UTC when the host has no zoneinfo
		t.Fatalf("tz %q", rep.TZ)
	}
	if len(rep.Clients) != 1 || rep.Clients[0].Name != "web" {
		t.Fatalf("clients %+v", rep.Clients)
	}
	bobs := nativeJSON[listening.Report](t, e.do("GET", "/listening/report", e.bobTok, nil), 200)
	if bobs.Totals.Plays != 1 || bobs.TopTracks[0].ID != e.id("umbrella") {
		t.Fatalf("bob's report %+v", bobs.Totals)
	}
	nativeErrorCode(t, e.do("GET", "/listening/report?from=10&to=5", e.aliceTok, nil), 400)

	page := nativeJSON[playsPage](t, e.do("GET", "/listening/history?limit=2", e.aliceTok, nil), 200)
	if page.Total != 3 || len(page.Items) != 2 || page.Items[0].TrackID != e.id("one") || page.Items[0].Track == nil {
		t.Fatalf("history %+v", page)
	}
	page = nativeJSON[playsPage](t, e.do("GET", "/listening/history?from="+itoa(now.Add(-90*time.Minute).UnixMilli()), e.aliceTok, nil), 200)
	if page.Total != 1 {
		t.Fatalf("history since 90 minutes %+v", page)
	}
}

func TestNativeScrobbling(t *testing.T) {
	e := newNativeEnv(t)
	nativeErrorCode(t, e.do("GET", "/me/scrobbling", "", nil), 401)
	st := nativeJSON[[]scrobble.AccountStatus](t, e.do("GET", "/me/scrobbling", e.aliceTok, nil), 200)
	if len(st) != 2 || st[0].Service != "lastfm" || st[0].Available || st[1].Service != "listenbrainz" || st[1].Linked {
		t.Fatalf("status %+v", st)
	}
	cb := map[string]any{"callback": "https://rainy.example.com/settings/lastfm"}
	nativeErrorCode(t, e.do("POST", "/me/scrobbling/lastfm/auth", e.aliceTok, cb), 403)
	nativeErrorCode(t, e.do("POST", "/me/scrobbling/listenbrainz", e.aliceTok, map[string]any{"token": "11111111-2222"}), 403)

	// Only administrators configure Last.fm; the secret is never returned.
	nativeErrorCode(t, e.do("GET", "/admin/scrobbling", e.aliceTok, nil), 403)
	nativeErrorCode(t, e.do("PUT", "/admin/scrobbling/lastfm", e.aliceTok, map[string]any{"apiKey": "x"}), 403)
	nativeExpect(t, e.do("PUT", "/admin/settings", e.adminTok, map[string]any{"lastfmEnabled": true}), 200)
	nativeErrorCode(t, e.do("POST", "/me/scrobbling/lastfm/auth", e.aliceTok, cb), 409)
	nativeErrorCode(t, e.do("PUT", "/admin/scrobbling/lastfm", e.adminTok, map[string]any{"secret": "short"}), 400)
	key, secret := strings.Repeat("01", 16), strings.Repeat("fe", 16) // synthetic API account
	rec := e.do("PUT", "/admin/scrobbling/lastfm", e.adminTok, map[string]any{"apiKey": key, "secret": secret})
	if strings.Contains(rec.Body.String(), secret) {
		t.Fatal("the shared secret was returned")
	}
	info := nativeJSON[scrobble.AdminInfo](t, rec, 200)
	if !info.Lastfm.Enabled || !info.Lastfm.Configured || !info.Lastfm.HasSecret || info.Lastfm.APIKey != key {
		t.Fatalf("admin info %+v", info)
	}
	if strings.Contains(e.do("GET", "/admin/settings", e.adminTok, nil).Body.String(), secret) {
		t.Fatal("the shared secret leaked into the settings")
	}

	// Starting the web sign-in does not contact Last.fm; it only builds the link.
	res := nativeJSON[map[string]string](t, e.do("POST", "/me/scrobbling/lastfm/auth", e.aliceTok, cb), 200)
	u, err := url.Parse(res["url"])
	if err != nil || u.Host != "www.last.fm" || u.Query().Get("api_key") != key || !strings.Contains(u.Query().Get("cb"), "state=") {
		t.Fatalf("auth url %q", res["url"])
	}
	nativeErrorCode(t, e.do("POST", "/me/scrobbling/lastfm/auth", e.aliceTok, map[string]any{"callback": "javascript:alert(1)"}), 400)
	nativeErrorCode(t, e.do("POST", "/me/scrobbling/lastfm", e.aliceTok, map[string]any{"token": "abc", "state": "wrong"}), 400)

	nativeErrorCode(t, e.do("PUT", "/me/scrobbling/lastfm", e.aliceTok, map[string]any{"enabled": false}), 404)
	nativeErrorCode(t, e.do("PUT", "/me/scrobbling/lastfm", e.aliceTok, map[string]any{}), 400)
	nativeErrorCode(t, e.do("PUT", "/me/scrobbling/myspace", e.aliceTok, map[string]any{"enabled": true}), 400)
	nativeErrorCode(t, e.do("DELETE", "/me/scrobbling/myspace", e.aliceTok, nil), 400)
	nativeExpect(t, e.do("DELETE", "/me/scrobbling/lastfm", e.aliceTok, nil), 204)
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
