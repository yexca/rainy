package outbound

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

// hops serves /hop/N, redirecting to /hop/N-1 until /hop/0 answers "done".
func hops(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n, _ := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/hop/"))
		if n > 0 {
			http.Redirect(w, r, "/hop/"+strconv.Itoa(n-1), http.StatusFound)
			return
		}
		_, _ = io.WriteString(w, "done")
	}))
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, c *http.Client, target string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Do(req)
	if err == nil {
		t.Cleanup(func() { _ = resp.Body.Close() })
	}
	return resp, err
}

func TestZeroRedirectsReturnsTheRedirect(t *testing.T) {
	srv := hops(t)
	resp, err := get(t, NewClient(Options{}), srv.URL+"/hop/1")
	if err != nil || resp.StatusCode != http.StatusFound {
		t.Fatalf("got %v, %v; want the 302 itself", resp, err)
	}
}

func TestMaxRedirects(t *testing.T) {
	srv := hops(t)
	c := NewClient(Options{Redirects: Redirects{Max: 2}})
	if resp, err := get(t, c, srv.URL+"/hop/2"); err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("two redirects: %v, %v", resp, err)
	}
	if _, err := get(t, c, srv.URL+"/hop/3"); !errors.Is(err, ErrRedirect) {
		t.Fatalf("three redirects: err = %v, want ErrRedirect", err)
	}
}

func TestRedirectTargetsAreChecked(t *testing.T) {
	var target string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/go" {
			http.Redirect(w, r, target, http.StatusFound)
			return
		}
		_, _ = io.WriteString(w, "ok")
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	sameHost := Redirects{Max: 3, Allow: SameHost}
	for _, tc := range []struct {
		name   string
		policy Redirects
		target string
		ok     bool
	}{
		{"same host", sameHost, srv.URL + "/end", true},
		{"other host", sameHost, "http://other.invalid/end", false},
		{"user info", sameHost, (&url.URL{Scheme: "http", User: url.User("someone"), Host: u.Host, Path: "/end"}).String(), false},
		{"other scheme", Redirects{Max: 3}, "ftp://" + u.Host + "/end", false},
		{"plain http when HTTPS only", Redirects{Max: 3, HTTPSOnly: true}, srv.URL + "/end", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target = tc.target
			_, err := get(t, NewClient(Options{Redirects: tc.policy}), srv.URL+"/go")
			if tc.ok && err != nil {
				t.Fatalf("err = %v, want success", err)
			}
			if !tc.ok {
				if !errors.Is(err, ErrRedirect) {
					t.Fatalf("err = %v, want ErrRedirect", err)
				}
				// Callers drop the *url.Error wrapper; what remains names only the host.
				var ue *url.Error
				if !errors.As(err, &ue) {
					t.Fatalf("err = %T, want *url.Error", err)
				}
				if msg := ue.Err.Error(); strings.Contains(msg, "someone") || strings.Contains(msg, "/end") {
					t.Fatalf("policy error repeats the target URL: %v", msg)
				}
			}
		})
	}
}

func TestReadAllLimit(t *testing.T) {
	if b, err := ReadAll(strings.NewReader("12345"), 5); err != nil || string(b) != "12345" {
		t.Fatalf("at the limit: %q, %v", b, err)
	}
	if b, err := ReadAll(strings.NewReader("123456"), 5); !errors.Is(err, ErrTooLarge) || b != nil {
		t.Fatalf("over the limit: %q, %v", b, err)
	}
}

// block serves a response that never finishes until the test ends.
func block(t *testing.T) *httptest.Server {
	t.Helper()
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		select {
		case <-done:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() { close(done); srv.Close() })
	return srv
}

func TestContextCancelStopsTheRequest(t *testing.T) {
	srv := block(t)
	ctx, cancel := context.WithCancel(t.Context())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	resp, err := NewClient(Options{}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	time.AfterFunc(20*time.Millisecond, cancel)
	if _, err := ReadAll(resp.Body, 1<<20); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestTimeoutBoundsTheBody(t *testing.T) {
	srv := block(t)
	resp, err := get(t, NewClient(Options{Timeout: 50 * time.Millisecond}), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if _, err := ReadAll(resp.Body, 1<<20); err == nil {
		t.Fatal("reading a stalled body succeeded")
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("timeout took %v", d)
	}
}

func TestCustomTransportIsUsed(t *testing.T) {
	refused := errors.New("refused by the transport")
	rt := roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, refused })
	if _, err := get(t, NewClient(Options{Transport: rt}), "http://example.invalid/"); !errors.Is(err, refused) {
		t.Fatalf("err = %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
