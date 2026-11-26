package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"rainy/internal/events"
	"rainy/internal/nowplaying"
	"rainy/internal/store"
)

// sseKeepAlive is the interval of ": ping" comments that keep proxies and browsers from
// closing idle event streams.
var sseKeepAlive = 25 * time.Second

// sseWriteTimeout bounds each write (and flush) to an event stream.
const sseWriteTimeout = 30 * time.Second

// sseRetryMs is the reconnection delay suggested to EventSource clients.
const sseRetryMs = 5000

// routesEvents registers GET /events (server-sent events, docs/architecture/contract.md §7.5); mounted
// behind auth.RequireUser.
func (a *API) routesEvents(r chi.Router) {
	r.Get("/events", a.eventsStream)
}

// writeSSE writes one event ("event: <type>" + "data: <json of e.Data>").
func writeSSE(w io.Writer, e events.Event) error {
	data, err := json.Marshal(e.Data)
	if err != nil {
		return fmt.Errorf("encoding %s event: %w", e.Type, err)
	}
	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.Type, data)
	return err
}

// eventsStream streams bus events to the client until it disconnects (or the server shuts
// down). The current now-playing list is sent right after connecting.
func (a *API) eventsStream(w http.ResponseWriter, r *http.Request) {
	rc := http.NewResponseController(w)
	ch, unsubscribe := a.app.Bus.Subscribe()
	defer unsubscribe()

	h := w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-cache, no-transform")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// Every write gets a deadline so a client that stops reading (half-open connection, full
	// TCP window) cannot pin this goroutine and its bus subscription forever. The deadline is
	// cleared on return so it cannot leak into a later request on the same connection.
	// Recorders that do not support deadlines simply skip this.
	defer func() { _ = rc.SetWriteDeadline(time.Time{}) }()
	flush := func() bool {
		if err := rc.Flush(); err != nil {
			slog.Debug("events: flush failed", "err", err)
			return false
		}
		return true
	}
	deadline := func() { _ = rc.SetWriteDeadline(time.Now().Add(sseWriteTimeout)) }

	deadline()
	if _, err := fmt.Fprintf(w, "retry: %d\n: connected\n\n", sseRetryMs); err != nil {
		return
	}
	if err := writeSSE(w, a.visibleEvent(r.Context(), events.Event{Type: events.TypeNowPlaying, Data: a.app.NowPlaying.List()})); err != nil || !flush() {
		return
	}

	ping := time.NewTicker(sseKeepAlive)
	defer ping.Stop()
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-ch:
			if !ok {
				return
			}
			deadline()
			if err := writeSSE(w, a.visibleEvent(ctx, e)); err != nil {
				slog.Debug("events: write failed", "err", err)
				return
			}
			if !flush() {
				return
			}
		case <-ping.C:
			deadline()
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil || !flush() {
				return
			}
		}
	}
}

// visibleEvent drops the now-playing entries whose track is outside the viewer's library
// scope, so a user limited to some libraries never learns what is played elsewhere.
func (a *API) visibleEvent(ctx context.Context, e events.Event) events.Event {
	entries, ok := e.Data.([]nowplaying.Entry)
	if _, restricted := store.LibraryScope(ctx); !restricted || !ok {
		return e
	}
	ids := make([]string, len(entries))
	for i, en := range entries {
		ids[i] = en.TrackID
	}
	visible := map[string]bool{}
	if tracks, err := a.app.Store.GetTracks(ctx, ids, ""); err == nil {
		for _, t := range tracks {
			visible[t.ID] = true
		}
	} else {
		slog.Debug("events: filtering now playing", "err", err)
	}
	out := make([]nowplaying.Entry, 0, len(entries))
	for _, en := range entries {
		if visible[en.TrackID] {
			out = append(out, en)
		}
	}
	return events.Event{Type: e.Type, Data: out}
}
