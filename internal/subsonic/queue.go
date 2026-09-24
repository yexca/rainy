package subsonic

import (
	"errors"

	"rainy/internal/model"
	"rainy/internal/store"
)

// ---- bookmarks

func (a *API) getBookmarks(q *request) (*Response, error) {
	bms, err := a.app.Store.ListBookmarks(q.ctx, q.user.ID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(bms))
	for _, b := range bms {
		ids = append(ids, b.TrackID)
	}
	tracks, err := a.app.Store.GetTracks(q.ctx, ids, q.user.ID)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*model.Track, len(tracks))
	for i := range tracks {
		byID[tracks[i].ID] = &tracks[i]
	}
	out := make([]Bookmark, 0, len(bms))
	for _, b := range bms {
		t, ok := byID[b.TrackID]
		if !ok || t.Missing {
			continue
		}
		entry := q.child(t)
		entry.BookmarkPosition = b.PositionMs
		out = append(out, Bookmark{
			Position: b.PositionMs, Username: q.user.Username, Comment: b.Comment,
			Created: date(b.CreatedAt), Changed: date(b.UpdatedAt), Entry: entry,
		})
	}
	resp := newResponse()
	resp.Bookmarks = &Bookmarks{Bookmarks: out}
	return resp, nil
}

func (a *API) createBookmark(q *request) (*Response, error) {
	id, err := q.requiredStr("id")
	if err != nil {
		return nil, err
	}
	if !q.has("position") {
		return nil, errMissing("position")
	}
	pos, err := q.int64Param("position", 0)
	if err != nil {
		return nil, err
	}
	if pos < 0 {
		return nil, newError(codeGeneric, "Position must not be negative")
	}
	if _, err := a.app.Store.GetTrack(q.ctx, id, ""); err != nil {
		return nil, notFoundAs(err, "Song")
	}
	comment := q.params.Get("comment")
	if len(comment) > 2000 {
		comment = comment[:2000]
	}
	if err := a.app.Store.UpsertBookmark(q.ctx, q.user.ID, &model.Bookmark{TrackID: id, PositionMs: pos, Comment: comment}); err != nil {
		return nil, err
	}
	return newResponse(), nil
}

func (a *API) deleteBookmark(q *request) (*Response, error) {
	id, err := q.requiredStr("id")
	if err != nil {
		return nil, err
	}
	if err := a.app.Store.DeleteBookmark(q.ctx, q.user.ID, id); err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	return newResponse(), nil
}

// ---- play queue (shared with the web UI's /api/queue)

// maxQueueLength bounds a saved queue.
const maxQueueLength = 5000

// savedQueue loads the user's queue with its playable songs; ok is false when nothing has
// been saved.
func (a *API) savedQueue(q *request) (pq *model.PlayQueue, tracks []model.Track, ok bool, err error) {
	pq, err = a.app.Store.GetPlayQueue(q.ctx, q.user.ID)
	if err != nil {
		return nil, nil, false, err
	}
	if len(pq.TrackIDs) == 0 && pq.UpdatedAt == 0 {
		return pq, nil, false, nil
	}
	all, err := a.app.Store.GetTracks(q.ctx, pq.TrackIDs, q.user.ID)
	if err != nil {
		return nil, nil, false, err
	}
	tracks = make([]model.Track, 0, len(all))
	currentFound := false
	for _, t := range all {
		if !t.Missing {
			tracks = append(tracks, t)
			currentFound = currentFound || t.ID == pq.CurrentID
		}
	}
	if !currentFound {
		// The current song is gone (deleted or missing): point at the first remaining song
		// from its start, so clients never get a "current" that is not in the entries.
		pq.CurrentID, pq.PositionMs, pq.CurrentIndex = "", 0, -1
		if len(tracks) > 0 {
			pq.CurrentID = tracks[0].ID
		}
	}
	return pq, tracks, true, nil
}

func (a *API) getPlayQueue(q *request) (*Response, error) {
	pq, tracks, ok, err := a.savedQueue(q)
	if err != nil {
		return nil, err
	}
	resp := newResponse()
	if !ok {
		return resp, nil
	}
	resp.PlayQueue = &PlayQueue{
		Current: pq.CurrentID, Position: pq.PositionMs, Username: q.user.Username,
		Changed: date(pq.UpdatedAt), ChangedBy: pq.ChangedBy, Entries: q.children(tracks),
	}
	return resp, nil
}

func (a *API) getPlayQueueByIndex(q *request) (*Response, error) {
	pq, tracks, ok, err := a.savedQueue(q)
	if err != nil {
		return nil, err
	}
	resp := newResponse()
	if !ok {
		return resp, nil
	}
	// The stored index points into the saved ids; entries of unknown or missing songs
	// before it are not in the response.
	idx := 0
	if pos := pq.CurrentPos(); pos >= 0 {
		playable := make(map[string]bool, len(tracks))
		for i := range tracks {
			playable[tracks[i].ID] = true
		}
		for _, id := range pq.TrackIDs[:pos] {
			if playable[id] {
				idx++
			}
		}
	}
	resp.PlayQueueByIndex = &PlayQueueByIndex{
		CurrentIndex: idx, Position: pq.PositionMs, Username: q.user.Username,
		Changed: date(pq.UpdatedAt), ChangedBy: pq.ChangedBy, Entries: q.children(tracks),
	}
	return resp, nil
}

// queueIDs validates the "id" list of a queue save.
func queueIDs(q *request) ([]string, error) {
	ids := q.strs("id")
	if len(ids) > maxQueueLength {
		return nil, newError(codeGeneric, "Play queue too long (max %d songs)", maxQueueLength)
	}
	return ids, nil
}

func (a *API) savePlayQueue(q *request) (*Response, error) {
	ids, err := queueIDs(q)
	if err != nil {
		return nil, err
	}
	pos, err := q.int64Param("position", 0)
	if err != nil {
		return nil, err
	}
	current := q.str("current")
	if len(ids) == 0 {
		current, pos = "", 0
	} else if current == "" {
		current = ids[0]
	}
	pq := &model.PlayQueue{TrackIDs: ids, CurrentID: current, PositionMs: max(pos, 0), ChangedBy: q.client}
	if err := a.app.Store.SavePlayQueue(q.ctx, q.user.ID, pq); err != nil {
		return nil, err
	}
	return newResponse(), nil
}

func (a *API) savePlayQueueByIndex(q *request) (*Response, error) {
	ids, err := queueIDs(q)
	if err != nil {
		return nil, err
	}
	pos, err := q.int64Param("position", 0)
	if err != nil {
		return nil, err
	}
	pq := &model.PlayQueue{TrackIDs: ids, PositionMs: max(pos, 0), ChangedBy: q.client}
	if len(ids) == 0 {
		if q.str("currentIndex") != "" {
			return nil, newError(codeMissingParam, "currentIndex must not be set without id")
		}
		pq.PositionMs = 0
	} else {
		if q.str("currentIndex") == "" {
			return nil, errMissing("currentIndex")
		}
		idx, err := q.intParam("currentIndex", 0)
		if err != nil || idx < 0 || idx >= len(ids) {
			return nil, newError(codeMissingParam, "currentIndex out of range")
		}
		pq.CurrentID, pq.CurrentIndex = ids[idx], idx
	}
	if err := a.app.Store.SavePlayQueue(q.ctx, q.user.ID, pq); err != nil {
		return nil, err
	}
	return newResponse(), nil
}
