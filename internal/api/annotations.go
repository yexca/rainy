package api

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"rainy/internal/events"
	"rainy/internal/nowplaying"
	"rainy/internal/util"
)

// defaultPlayer is the now-playing player name of the web UI.
const defaultPlayer = "web"

// routesAnnotations registers /star, /rating and /scrobble (docs/architecture/contract.md §7.3); mounted
// behind auth.RequireUser.
func (a *API) routesAnnotations(r chi.Router) {
	r.Post("/star", a.annStar)
	r.Post("/rating", a.annRating)
	r.Post("/scrobble", a.annScrobble)
}

// validStarType reports whether t is an annotatable item type.
func validStarType(t string) bool { return t == "track" || t == "album" || t == "artist" }

type starBody struct {
	Type    string   `json:"type"`
	IDs     []string `json:"ids"`
	Starred bool     `json:"starred"`
}

func (a *API) annStar(w http.ResponseWriter, r *http.Request) {
	var body starBody
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if !validStarType(body.Type) {
		writeErr(w, r, badRequest("type must be track, album or artist"))
		return
	}
	if len(body.IDs) > maxIDsPerRequest {
		writeErr(w, r, badRequest("at most %d ids per request", maxIDsPerRequest))
		return
	}
	ids := make([]string, 0, len(body.IDs))
	for _, id := range body.IDs {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	if err := a.app.Store.SetStarred(r.Context(), userFrom(r).ID, body.Type, ids, body.Starred); err != nil {
		writeErr(w, r, err)
		return
	}
	writeNoContent(w)
}

type ratingBody struct {
	Type   string `json:"type"`
	ID     string `json:"id"`
	Rating int    `json:"rating"`
}

func (a *API) annRating(w http.ResponseWriter, r *http.Request) {
	var body ratingBody
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	switch {
	case !validStarType(body.Type):
		writeErr(w, r, badRequest("type must be track, album or artist"))
		return
	case strings.TrimSpace(body.ID) == "":
		writeErr(w, r, badRequest("id is required"))
		return
	case body.Rating < 0 || body.Rating > 5:
		writeErr(w, r, badRequest("rating must be between 0 and 5"))
		return
	}
	if err := a.app.Store.SetRating(r.Context(), userFrom(r).ID, body.Type, body.ID, body.Rating); err != nil {
		writeErr(w, r, err)
		return
	}
	writeNoContent(w)
}

type scrobbleBody struct {
	TrackID    string `json:"trackId"`
	Submission bool   `json:"submission"`
	Time       int64  `json:"time"`   // unix ms; 0 = now
	Player     string `json:"player"` // optional player name for now playing (default "web")
}

// annScrobble records "now playing" (submission=false) or a completed play (true).
func (a *API) annScrobble(w http.ResponseWriter, r *http.Request) {
	var body scrobbleBody
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if strings.TrimSpace(body.TrackID) == "" {
		writeErr(w, r, badRequest("trackId is required"))
		return
	}
	u := userFrom(r)
	ctx := r.Context()
	player := strings.TrimSpace(body.Player)
	if player == "" || len(player) > 64 {
		player = defaultPlayer
	}
	if !body.Submission {
		t, err := a.app.Store.GetTrack(ctx, body.TrackID, u.ID)
		if err != nil {
			writeErr(w, r, err)
			return
		}
		a.app.NowPlaying.Set(nowplaying.Entry{UserID: u.ID, Username: u.Username, TrackID: t.ID, Player: player})
		a.app.Bus.Publish(events.Event{Type: events.TypeNowPlaying, Data: a.app.NowPlaying.List()})
		writeNoContent(w)
		return
	}
	at := body.Time
	if now := util.NowMs(); at <= 0 || at > now {
		at = now
	}
	if err := a.app.Store.RecordPlay(ctx, u.ID, body.TrackID, at, player); err != nil {
		writeErr(w, r, err)
		return
	}
	writeNoContent(w)
}
