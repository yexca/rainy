package subsonic

import (
	"context"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestAuthentication(t *testing.T) {
	f := newFixture(t)
	key, err := f.app.Auth.GenerateAPIKey(context.Background(), f.alice.ID)
	if err != nil {
		t.Fatal(err)
	}
	salt := "c19b2d"
	token := md5hex("alicepass" + salt)
	base := url.Values{"v": {"1.16.1"}, "c": {"test"}}

	tests := []struct {
		name string
		kv   []string
		code int // -1 = success
	}{
		{"no credentials", nil, codeMissingParam},
		{"username only", []string{"u", "alice"}, codeMissingParam},
		{"plain password", []string{"u", "alice", "p", "alicepass"}, -1},
		{"hex password", []string{"u", "alice", "p", "enc:" + hex.EncodeToString([]byte("alicepass"))}, -1},
		{"bad hex password", []string{"u", "alice", "p", "enc:zz"}, codeWrongCredentials},
		{"wrong password", []string{"u", "alice", "p", "nope"}, codeWrongCredentials},
		{"unknown user", []string{"u", "mallory", "p", "alicepass"}, codeWrongCredentials},
		{"username case-insensitive", []string{"u", "ALICE", "p", "alicepass"}, -1},
		{"token", []string{"u", "alice", "t", token, "s", salt}, -1},
		{"token upper-case", []string{"u", "alice", "t", strings.ToUpper(token), "s", salt}, -1},
		{"token without salt", []string{"u", "alice", "t", token}, codeMissingParam},
		{"wrong token", []string{"u", "alice", "t", md5hex("x" + salt), "s", salt}, codeWrongCredentials},
		{"password and token", []string{"u", "alice", "p", "alicepass", "t", token, "s", salt}, codeConflictingAuth},
		{"api key", []string{"apiKey", key}, -1},
		{"api key with username", []string{"apiKey", key, "u", "alice"}, codeConflictingAuth},
		{"api key with password", []string{"apiKey", key, "p", "alicepass"}, codeConflictingAuth},
		{"invalid api key", []string{"apiKey", "bogus"}, codeInvalidAPIKey},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f.t = t
			if tt.code < 0 {
				f.ok("ping", merge(base, tt.kv...))
				return
			}
			if got := f.failCode("ping", merge(base, tt.kv...)); got != tt.code {
				t.Fatalf("code = %d, want %d", got, tt.code)
			}
		})
	}
}

func TestTokenInfo(t *testing.T) {
	f := newFixture(t)
	key, err := f.app.Auth.GenerateAPIKey(context.Background(), f.bob.ID)
	if err != nil {
		t.Fatal(err)
	}
	env := f.ok("tokenInfo", url.Values{"apiKey": {key}})
	if got := jpath(t, env, "tokenInfo", "username"); got != "bob" {
		t.Fatalf("username = %v", got)
	}
}

func TestPublicExtensionsEndpoint(t *testing.T) {
	f := newFixture(t)
	env := f.ok("getOpenSubsonicExtensions", url.Values{})
	names := map[string]bool{}
	for _, e := range jpath(t, env, "openSubsonicExtensions").([]any) {
		names[e.(map[string]any)["name"].(string)] = true
	}
	for _, want := range []string{"formPost", "songLyrics", "transcodeOffset", "apiKeyAuthentication", "indexBasedQueue"} {
		if !names[want] {
			t.Errorf("extension %s not advertised", want)
		}
	}
	// Wrong credentials on the public endpoint are ignored rather than rejected.
	f.ok("getOpenSubsonicExtensions", creds("alice", "wrong"))
}

func TestAuthRateLimit(t *testing.T) {
	f := newFixture(t)
	for range limitMaxFailures {
		if code := f.failCode("ping", creds("bob", "wrong")); code != codeWrongCredentials {
			t.Fatalf("code %d", code)
		}
	}
	// Locked out: even the right password fails now.
	if code := f.failCode("ping", creds("bob", "bobpass")); code != codeWrongCredentials {
		t.Fatalf("expected lockout, got %d", code)
	}
}

func TestRoutingAndFormats(t *testing.T) {
	f := newFixture(t)
	p := f.aliceParams()

	// .view suffix
	rec := f.get("ping.view", p)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `status="ok"`) {
		t.Fatalf("ping.view: %d %s", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "xml") {
		t.Fatalf("content type %q", ct)
	}

	// POST form (OpenSubsonic formPost) merged with the query string.
	rec = f.post("ping", merge(p, "f", "json"))
	if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Fatalf("POST ping: %s", rec.Body)
	}

	// JSONP
	rec = f.get("ping", merge(p, "f", "jsonp", "callback", "cb"))
	if body := rec.Body.String(); !strings.HasPrefix(body, "/**/cb({") || !strings.HasSuffix(body, "});") {
		t.Fatalf("jsonp: %s", body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Fatalf("jsonp content type %q", ct)
	}
	// An unsafe callback falls back to plain JSON.
	rec = f.get("ping", merge(p, "f", "jsonp", "callback", "alert(1)//"))
	if body := rec.Body.String(); !strings.HasPrefix(body, `{"subsonic-response"`) {
		t.Fatalf("unsafe jsonp callback: %s", body)
	}

	// Unknown method
	rec = f.get("noSuchMethod", merge(p, "f", "json"))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"code":70`) {
		t.Fatalf("unknown method: %d %s", rec.Code, rec.Body)
	}

	// Errors keep HTTP 200.
	rec = f.get("getAlbum", merge(p, "f", "json"))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"code":10`) {
		t.Fatalf("missing id: %d %s", rec.Code, rec.Body)
	}
}
