package api

import (
	"testing"

	"rainy/internal/model"
)

func TestNativeRadios(t *testing.T) {
	e := newNativeEnv(t)
	nativeErrorCode(t, e.do("GET", "/radios", "", nil), 401)
	list := nativeJSON[[]model.RadioStation](t, e.do("GET", "/radios", e.aliceTok, nil), 200)
	if list == nil || len(list) != 0 {
		t.Fatal("empty list")
	}

	body := map[string]string{"name": " Jazz FM ", "streamUrl": "https://radio.example.com/jazz.mp3", "homepageUrl": "https://example.com"}
	if code := nativeErrorCode(t, e.do("POST", "/radios", e.aliceTok, body), 403); code != CodeForbidden {
		t.Fatal(code)
	}
	nativeErrorCode(t, e.do("POST", "/radios", "", body), 401)
	for _, bad := range []map[string]string{
		{"name": "", "streamUrl": "https://x.example/s"},
		{"name": "x", "streamUrl": "javascript:alert(1)"},
		{"name": "x", "streamUrl": "/relative"},
		{"name": "x", "streamUrl": "https://x.example/s", "homepageUrl": "ftp://x.example"},
	} {
		nativeErrorCode(t, e.do("POST", "/radios", e.adminTok, bad), 400)
	}
	st := nativeJSON[model.RadioStation](t, e.do("POST", "/radios", e.adminTok, body), 201)
	if st.ID == "" || st.Name != "Jazz FM" || st.StreamURL != body["streamUrl"] || st.HomepageURL != "https://example.com" || st.CreatedAt == 0 {
		t.Fatalf("created %+v", st)
	}
	if list = nativeJSON[[]model.RadioStation](t, e.do("GET", "/radios", e.bobTok, nil), 200); len(list) != 1 {
		t.Fatal("list after create")
	}

	upd := map[string]string{"name": "Jazz 24", "streamUrl": "http://radio.example.com/24"}
	nativeErrorCode(t, e.do("PUT", "/radios/"+st.ID, e.bobTok, upd), 403)
	got := nativeJSON[model.RadioStation](t, e.do("PUT", "/radios/"+st.ID, e.adminTok, upd), 200)
	if got.Name != "Jazz 24" || got.StreamURL != upd["streamUrl"] || got.HomepageURL != "" || got.CreatedAt != st.CreatedAt {
		t.Fatalf("updated %+v", got)
	}
	nativeErrorCode(t, e.do("PUT", "/radios/nope", e.adminTok, upd), 404)

	nativeErrorCode(t, e.do("DELETE", "/radios/"+st.ID, e.aliceTok, nil), 403)
	nativeExpect(t, e.do("DELETE", "/radios/"+st.ID, e.adminTok, nil), 204)
	nativeErrorCode(t, e.do("DELETE", "/radios/"+st.ID, e.adminTok, nil), 404)
}
