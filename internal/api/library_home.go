package api

import (
	"net/http"

	"golang.org/x/sync/errgroup"

	"rainy/internal/model"
	"rainy/internal/store"
)

// homeShelfSize is the number of albums per home shelf.
const homeShelfSize = 12

// homeStats are the library totals shown on the home page.
type homeStats struct {
	Tracks  int `json:"tracks"`
	Albums  int `json:"albums"`
	Artists int `json:"artists"`
}

// home is the GET /api/home response.
type home struct {
	RecentlyAdded  []model.Album `json:"recentlyAdded"`
	RecentlyPlayed []model.Album `json:"recentlyPlayed"`
	MostPlayed     []model.Album `json:"mostPlayed"`
	Random         []model.Album `json:"random"`
	Starred        []model.Album `json:"starred"`
	Stats          homeStats     `json:"stats"`
}

// libHome serves the home page shelves; the queries run concurrently on the reader pool.
func (a *API) libHome(w http.ResponseWriter, r *http.Request) {
	uid := userFrom(r).ID
	st := a.app.Store
	g, ctx := errgroup.WithContext(r.Context())
	var h home

	shelf := func(dst *[]model.Album, total *int, q store.AlbumQuery) {
		g.Go(func() error {
			q.UserID, q.Limit = uid, homeShelfSize
			albums, n, err := st.ListAlbums(ctx, q)
			if err != nil {
				return err
			}
			*dst = nonNil(albums)
			if total != nil {
				*total = n
			}
			return nil
		})
	}
	shelf(&h.RecentlyAdded, &h.Stats.Albums, store.AlbumQuery{Sort: "recent"})
	shelf(&h.RecentlyPlayed, nil, store.AlbumQuery{Played: true, Sort: "played"})
	shelf(&h.MostPlayed, nil, store.AlbumQuery{Played: true, Sort: "frequent"})
	shelf(&h.Random, nil, store.AlbumQuery{Sort: "random"})
	shelf(&h.Starred, nil, store.AlbumQuery{Starred: true, Sort: "starred"})
	g.Go(func() error {
		_, n, err := st.ListTracks(ctx, store.TrackQuery{Limit: 1})
		h.Stats.Tracks = n
		return err
	})
	g.Go(func() error {
		_, n, err := st.ListArtists(ctx, store.ArtistQuery{AlbumArtistsOnly: true, Limit: 1})
		h.Stats.Artists = n
		return err
	})
	if err := g.Wait(); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, h)
}
