package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"rainy/internal/listening"
	"rainy/internal/recommend"
)

// routesRecommend registers the infinite-mode mix and the daily mix
// (docs/architecture/contract.md §7.9); mounted behind auth.RequireUser. Both only read the
// signed-in user's own plays and annotations.
func (a *API) routesRecommend(r chi.Router) {
	r.Post("/recommend/mix", a.recommendMix)
	r.Get("/recommend/daily", a.recommendDaily)
}

type mixRequest struct {
	Seeds   []string `json:"seeds"`
	Exclude []string `json:"exclude"`
	Limit   int      `json:"limit"`
}

func (a *API) recommendMix(w http.ResponseWriter, r *http.Request) {
	var req mixRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeErr(w, r, err)
		return
	}
	if len(req.Seeds) > recommend.MaxSeeds || len(req.Exclude) > recommend.MaxExclude ||
		req.Limit < 0 || req.Limit > recommend.MaxMixSize {
		writeErr(w, r, badRequest("at most %d seeds, %d excluded tracks and a limit of %d",
			recommend.MaxSeeds, recommend.MaxExclude, recommend.MaxMixSize))
		return
	}
	tracks, err := a.app.Recommend.Mix(r.Context(), userFrom(r).ID, recommend.MixQuery{
		Seeds: req.Seeds, Exclude: req.Exclude, Limit: req.Limit,
	})
	if errors.Is(err, recommend.ErrInvalid) {
		err = badRequest("%s", err.Error())
	}
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, nonNil(tracks))
}

func (a *API) recommendDaily(w http.ResponseWriter, r *http.Request) {
	loc := listening.LoadLocation(strings.TrimSpace(r.URL.Query().Get("tz")))
	mix, err := a.app.Recommend.Daily(r.Context(), userFrom(r).ID, loc)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, mix)
}
