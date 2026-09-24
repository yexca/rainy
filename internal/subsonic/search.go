package subsonic

import (
	"strings"

	"rainy/internal/store"
)

// maxSearchPage bounds each kind's page size (search3 full sync uses large pages).
const maxSearchPage = 10000

// runSearch executes search2/search3: query ("" or `""` = everything), artistCount,
// artistOffset, albumCount, albumOffset, songCount, songOffset, musicFolderId.
func (a *API) runSearch(q *request) (*store.SearchResult, error) {
	sq := store.SearchQuery{UserID: q.user.ID, Q: searchQuery(q.str("query"))}
	var err error
	if sq.ArtistLimit, sq.ArtistOffset, err = q.pageParams("artistCount", "artistOffset", 20, maxSearchPage); err != nil {
		return nil, err
	}
	if sq.AlbumLimit, sq.AlbumOffset, err = q.pageParams("albumCount", "albumOffset", 20, maxSearchPage); err != nil {
		return nil, err
	}
	if sq.TrackLimit, sq.TrackOffset, err = q.pageParams("songCount", "songOffset", 20, maxSearchPage); err != nil {
		return nil, err
	}
	if sq.LibraryID, err = q.musicFolder(); err != nil {
		return nil, err
	}
	return a.app.Store.Search(q.ctx, sq)
}

func (a *API) search2(q *request) (*Response, error) {
	res, err := a.runSearch(q)
	if err != nil {
		return nil, err
	}
	resp := newResponse()
	resp.SearchResult2 = &SearchResult2{
		Artists: folderArtists(res.Artists),
		Albums:  albumChildren(res.Albums),
		Songs:   q.children(res.Tracks),
	}
	return resp, nil
}

func (a *API) search3(q *request) (*Response, error) {
	res, err := a.runSearch(q)
	if err != nil {
		return nil, err
	}
	resp := newResponse()
	resp.SearchResult3 = &SearchResult3{
		Artists: artistsID3(res.Artists),
		Albums:  albumsID3(res.Albums),
		Songs:   q.children(res.Tracks),
	}
	return resp, nil
}

// search is the deprecated song search: artist, album, title and any are combined into
// one token query; count (default 20) and offset page the matches (newerThan is ignored).
func (a *API) search(q *request) (*Response, error) {
	var parts []string
	for _, p := range []string{"any", "artist", "album", "title"} {
		if v := searchQuery(q.str(p)); v != "" {
			parts = append(parts, v)
		}
	}
	count, offset, err := q.pageParams("count", "offset", 20, 500)
	if err != nil {
		return nil, err
	}
	resp := newResponse()
	resp.SearchResult = &SearchResult{Offset: offset, Matches: []Child{}}
	if count == 0 {
		return resp, nil
	}
	tracks, total, err := a.app.Store.ListTracks(q.ctx, store.TrackQuery{
		UserID: q.user.ID, Q: strings.Join(parts, " "), Sort: "title", Offset: offset, Limit: count,
	})
	if err != nil {
		return nil, err
	}
	resp.SearchResult.TotalHits = total
	resp.SearchResult.Matches = q.children(tracks)
	return resp, nil
}
