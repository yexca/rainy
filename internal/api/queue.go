package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"rainy/internal/model"
)

// maxQueueTracks bounds the saved play queue.
const maxQueueTracks = 10000

// queueChangedBy is the PlayQueue.ChangedBy value for queues saved by the web UI.
const queueChangedBy = "web"

// routesQueue registers GET/PUT /queue (docs/architecture/contract.md §7.4); mounted behind
// auth.RequireUser. The queue is shared with Subsonic clients (getPlayQueue/savePlayQueue).
func (a *API) routesQueue(r chi.Router) {
	r.Get("/queue", a.queueGet)
	r.Put("/queue", a.queuePut)
}

// queueResponse is `{trackIds, currentId, positionMs, updatedAt, tracks}` (plus changedBy).
type queueResponse struct {
	model.PlayQueue
	Tracks []model.Track `json:"tracks"`
}

func (a *API) queueGet(w http.ResponseWriter, r *http.Request) {
	ctx, uid := r.Context(), userFrom(r).ID
	q, err := a.app.Store.GetPlayQueue(ctx, uid)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	tracks, err := a.app.Store.GetTracks(ctx, q.TrackIDs, uid)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	// Tracks whose files disappeared cannot be played; unknown ids are already skipped.
	playable := make([]model.Track, 0, len(tracks))
	for _, t := range tracks {
		if !t.Missing {
			playable = append(playable, t)
		}
	}
	writeJSON(w, http.StatusOK, queueResponse{PlayQueue: *q, Tracks: playable})
}

type queueBody struct {
	TrackIDs   []string `json:"trackIds"`
	CurrentID  string   `json:"currentId"`
	PositionMs int64    `json:"positionMs"`
	// CurrentIndex (optional) disambiguates a current song that is queued more than once.
	CurrentIndex *int `json:"currentIndex"`
}

func (a *API) queuePut(w http.ResponseWriter, r *http.Request) {
	var body queueBody
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if len(body.TrackIDs) > maxQueueTracks {
		writeErr(w, r, badRequest("the queue may hold at most %d tracks", maxQueueTracks))
		return
	}
	if body.PositionMs < 0 {
		body.PositionMs = 0
	}
	ids := make([]string, 0, len(body.TrackIDs))
	index := -1
	for i, id := range body.TrackIDs {
		if id != "" {
			if body.CurrentIndex != nil && *body.CurrentIndex == i {
				index = len(ids)
			}
			ids = append(ids, id)
		}
	}
	// The store checks that the index points at currentId (else: its first occurrence).
	q := &model.PlayQueue{TrackIDs: ids, CurrentID: body.CurrentID, PositionMs: body.PositionMs, ChangedBy: queueChangedBy,
		CurrentIndex: index}
	if err := a.app.Store.SavePlayQueue(r.Context(), userFrom(r).ID, q); err != nil {
		writeErr(w, r, err)
		return
	}
	writeNoContent(w)
}
