package api

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"rainy/internal/events"
	"rainy/internal/nowplaying"
)

// sseReader reads server-sent event blocks from a stream.
type sseReader struct {
	t     *testing.T
	lines chan string
}

func newSSEReader(t *testing.T, resp *http.Response) *sseReader {
	r := &sseReader{t: t, lines: make(chan string, 64)}
	go func() {
		defer close(r.lines)
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			r.lines <- sc.Text()
		}
	}()
	return r
}

// next returns the next block (lines up to a blank line).
func (r *sseReader) next() []string {
	r.t.Helper()
	var block []string
	timeout := time.After(5 * time.Second)
	for {
		select {
		case line, ok := <-r.lines:
			if !ok {
				r.t.Fatalf("stream closed; partial block %q", block)
			}
			if line == "" {
				if len(block) > 0 {
					return block
				}
				continue
			}
			block = append(block, line)
		case <-timeout:
			r.t.Fatalf("timed out waiting for an event; partial block %q", block)
		}
	}
}

func TestNativeEvents(t *testing.T) {
	e := newNativeEnv(t)
	old := sseKeepAlive
	sseKeepAlive = 100 * time.Millisecond
	t.Cleanup(func() { sseKeepAlive = old })

	srv := httptest.NewServer(e.h)
	defer srv.Close()

	// Unauthenticated.
	resp, err := http.Get(srv.URL + "/events")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("anonymous events: %d", resp.StatusCode)
	}

	e.app.NowPlaying.Set(nowplaying.Entry{UserID: e.bob.ID, Username: "bob", TrackID: e.id("tone"), Player: "web"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/events", nil)
	req.Header.Set("Authorization", "Bearer "+e.aliceTok)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "text/event-stream; charset=utf-8" ||
		resp.Header.Get("X-Accel-Buffering") != "no" || !strings.HasPrefix(resp.Header.Get("Cache-Control"), "no-cache") {
		t.Fatalf("events response %d %v", resp.StatusCode, resp.Header)
	}
	r := newSSEReader(t, resp)
	if b := r.next(); b[0] != "retry: 5000" {
		t.Fatalf("preamble %q", b)
	}
	// Initial now-playing snapshot.
	b := r.next()
	if b[0] != "event: nowPlaying" || !strings.HasPrefix(b[1], "data: [") || !strings.Contains(b[1], `"username":"bob"`) {
		t.Fatalf("snapshot %q", b)
	}

	if n := e.app.Bus.Subscribers(); n != 1 {
		t.Fatalf("subscribers %d", n)
	}
	e.app.Bus.Publish(events.Library("tags"))
	for {
		b = r.next()
		if b[0] != ": ping" {
			break
		}
	}
	if len(b) != 2 || b[0] != "event: library" || b[1] != `data: {"reason":"tags"}` {
		t.Fatalf("library event %q", b)
	}
	// Keep-alive pings.
	if b = r.next(); b[0] != ": ping" {
		t.Fatalf("ping %q", b)
	}

	// Disconnecting unsubscribes.
	cancel()
	deadline := time.Now().Add(5 * time.Second)
	for e.app.Bus.Subscribers() != 0 {
		if time.Now().After(deadline) {
			t.Fatal("subscriber not removed after disconnect")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestWriteSSE(t *testing.T) {
	var sb strings.Builder
	if err := writeSSE(&sb, events.Event{Type: "scan", Data: map[string]int{"filesSeen": 3}}); err != nil {
		t.Fatal(err)
	}
	if sb.String() != "event: scan\ndata: {\"filesSeen\":3}\n\n" {
		t.Fatalf("%q", sb.String())
	}
	if err := writeSSE(&sb, events.Event{Type: "x", Data: func() {}}); err == nil {
		t.Fatal("unencodable data must fail")
	}
}
