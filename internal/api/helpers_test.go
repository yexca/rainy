package api

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"rainy/internal/auth"
	"rainy/internal/scanner"
	"rainy/internal/store"
	"rainy/internal/util"
)

func TestErrorFrom(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{badRequest("x %d", 1), 400, CodeBadRequest},
		{fmt.Errorf("wrapped: %w", forbidden("no")), 403, CodeForbidden},
		{fmt.Errorf("get: %w", store.ErrNotFound), 404, CodeNotFound},
		{store.ErrConflict, 409, CodeConflict},
		{scanner.ErrScanInProgress, 409, CodeConflict},
		{&fs.PathError{Op: "open", Path: "/music/x", Err: fs.ErrPermission}, 409, CodeReadonly},
		{fmt.Errorf("%w: rating", store.ErrInvalid), 400, CodeBadRequest},
		{auth.ErrWeakPassword, 400, CodeBadRequest},
		{util.ErrUnsafePath, 400, CodeBadRequest},
		{auth.ErrInvalidCredentials, 401, CodeUnauthorized},
		{auth.ErrRateLimited, 429, CodeRateLimited},
		{fmt.Errorf("x: %w", errors.ErrUnsupported), 501, CodeNotImplemented},
		{context.Canceled, 503, CodeUnavailable},
		{errors.New("secret db detail"), 500, CodeInternal},
	}
	for _, c := range cases {
		status, code, msg := errorFrom(c.err)
		if status != c.status || code != c.code {
			t.Errorf("errorFrom(%v) = %d %s, want %d %s", c.err, status, code, c.status, c.code)
		}
		if c.status == 500 && strings.Contains(msg, "secret") {
			t.Error("internal errors must not leak details")
		}
	}
}

func TestDecodeJSON(t *testing.T) {
	type body struct {
		Name string `json:"name"`
	}
	req := func(s string) (*httptest.ResponseRecorder, *http.Request) {
		return httptest.NewRecorder(), httptest.NewRequest("POST", "/", strings.NewReader(s))
	}
	var b body
	w, r := req(`{"name":"x","extra":1}`)
	if err := decodeJSON(w, r, &b); err != nil || b.Name != "x" {
		t.Fatal(err, b)
	}
	w, r = req(``)
	if err := decodeJSON(w, r, &b); err != nil {
		t.Fatal("empty body must be accepted", err)
	}
	for _, bad := range []string{`{`, `{"name":1}`, `{} {}`} {
		w, r = req(bad)
		var ae *Error
		if err := decodeJSON(w, r, &b); !errors.As(err, &ae) || ae.Status != 400 {
			t.Errorf("%q: %v", bad, err)
		}
	}
	w, r = req(`{"name":"` + strings.Repeat("a", maxJSONBody) + `"}`)
	var ae *Error
	if err := decodeJSON(w, r, &b); !errors.As(err, &ae) || ae.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("too large: %v", err)
	}
}

func TestQueryHelpers(t *testing.T) {
	r := httptest.NewRequest("GET", "/?offset=-5&limit=5000&sort=title&order=DESC&b=true&n=x&i=7&big=9000000000&ids=a,%20b,,c&ids=d", nil)
	off, lim, sort, order := pageParams(r, 50, 1000)
	if off != 0 || lim != 1000 || sort != "title" || order != "desc" {
		t.Fatal(off, lim, sort, order)
	}
	off, lim, _, order = pageParams(httptest.NewRequest("GET", "/?limit=0&order=up&offset=20", nil), 50, 1000)
	if off != 20 || lim != 50 || order != "" {
		t.Fatal(off, lim, order)
	}
	if !queryBool(r, "b") || queryBool(r, "n") || queryBool(r, "missing") {
		t.Fatal("queryBool")
	}
	if queryInt(r, "i", 0) != 7 || queryInt(r, "n", 3) != 3 || queryInt64(r, "big", 0) != 9000000000 {
		t.Fatal("queryInt")
	}
	if got := queryList(r, "ids"); !reflect.DeepEqual(got, []string{"a", "b", "c", "d"}) {
		t.Fatal(got)
	}
	if queryList(r, "none") != nil {
		t.Fatal("absent list must be nil")
	}
}

func TestWriteHelpers(t *testing.T) {
	w := httptest.NewRecorder()
	writePage[string](w, nil, 0)
	if strings.TrimSpace(w.Body.String()) != `{"items":[],"total":0}` || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal(w.Body.String())
	}
	w = httptest.NewRecorder()
	writeErr(w, httptest.NewRequest("GET", "/x", nil), store.ErrNotFound)
	if w.Code != 404 || strings.TrimSpace(w.Body.String()) != `{"error":{"code":"not_found","message":"not found"}}` {
		t.Fatal(w.Code, w.Body.String())
	}
	if nonNil[int](nil) == nil {
		t.Fatal("nonNil")
	}
}
