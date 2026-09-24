package subsonic

import (
	"strconv"

	"rainy/internal/events"
	"rainy/internal/nowplaying"
	"rainy/internal/util"
)

// starTargets groups the star/unstar ids by item type: "id" may be a song, album or
// artist id; "albumId" and "artistId" are typed. Unknown ids yield error 70.
func (a *API) starTargets(q *request) (map[string][]string, error) {
	targets := map[string][]string{}
	for _, id := range q.strs("id") {
		kind, err := a.itemKind(q.ctx, id)
		if err != nil {
			return nil, notFoundAs(err, "Item "+id)
		}
		targets[kind] = append(targets[kind], id)
	}
	for param, want := range map[string]string{"albumId": kindAlbum, "artistId": kindArtist} {
		for _, id := range q.strs(param) {
			if kind, err := a.itemKind(q.ctx, id); err != nil || kind != want {
				return nil, errNotFound("Item " + id)
			}
			targets[want] = append(targets[want], id)
		}
	}
	n := 0
	for _, ids := range targets {
		n += len(ids)
	}
	if n == 0 {
		return nil, errMissing("id, albumId or artistId")
	}
	return targets, nil
}

func (a *API) setStarred(q *request, starred bool) (*Response, error) {
	targets, err := a.starTargets(q)
	if err != nil {
		return nil, err
	}
	for kind, ids := range targets {
		if len(ids) == 0 {
			continue
		}
		if err := a.app.Store.SetStarred(q.ctx, q.user.ID, kind, ids, starred); err != nil {
			return nil, err
		}
	}
	return newResponse(), nil
}

func (a *API) star(q *request) (*Response, error)   { return a.setStarred(q, true) }
func (a *API) unstar(q *request) (*Response, error) { return a.setStarred(q, false) }

// setRating rates a song, album or artist 1–5; 0 removes the rating.
func (a *API) setRating(q *request) (*Response, error) {
	id, err := q.requiredStr("id")
	if err != nil {
		return nil, err
	}
	if !q.has("rating") {
		return nil, errMissing("rating")
	}
	rating, err := q.intParam("rating", 0)
	if err != nil {
		return nil, err
	}
	if rating < 0 || rating > 5 {
		return nil, newError(codeGeneric, "Rating must be between 0 and 5")
	}
	kind, err := a.itemKind(q.ctx, id)
	if err != nil {
		return nil, notFoundAs(err, "Item")
	}
	if err := a.app.Store.SetRating(q.ctx, q.user.ID, kind, id, rating); err != nil {
		return nil, err
	}
	return newResponse(), nil
}

// scrobble registers plays (submission=true, the default; optional time[] in ms per id)
// or "now playing" notifications (submission=false).
func (a *API) scrobble(q *request) (*Response, error) {
	ids := q.strs("id")
	if len(ids) == 0 {
		return nil, errMissing("id")
	}
	submission, err := q.boolParam("submission", true)
	if err != nil {
		return nil, err
	}
	times := q.strs("time")
	if !submission {
		id := ids[len(ids)-1]
		if _, err := a.app.Store.GetTrack(q.ctx, id, ""); err != nil {
			return nil, notFoundAs(err, "Song")
		}
		a.app.NowPlaying.Set(nowplaying.Entry{UserID: q.user.ID, Username: q.user.Username, TrackID: id, Player: q.client})
		a.app.Bus.Publish(events.Event{Type: events.TypeNowPlaying, Data: a.app.NowPlaying.List()})
		return newResponse(), nil
	}
	for i, id := range ids {
		at := util.NowMs()
		if i < len(times) {
			ms, err := strconv.ParseInt(times[i], 10, 64)
			if err != nil || ms <= 0 {
				return nil, newError(codeGeneric, "Invalid value for parameter time: %q", times[i])
			}
			at = min(ms, at) // never in the future
		}
		if err := a.app.Store.RecordPlay(q.ctx, q.user.ID, id, at, q.client); err != nil {
			return nil, notFoundAs(err, "Song")
		}
	}
	return newResponse(), nil
}
