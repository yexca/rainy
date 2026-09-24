package subsonic

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"rainy/internal/model"
	"rainy/internal/scanner"
)

// extensions are the advertised OpenSubsonic extensions.
var extensions = []Extension{
	{Name: "apiKeyAuthentication", Versions: []int{1}},
	{Name: "formPost", Versions: []int{1}},
	{Name: "indexBasedQueue", Versions: []int{1}},
	{Name: "songLyrics", Versions: []int{1}},
	{Name: "transcodeOffset", Versions: []int{1}},
}

func (a *API) ping(*request) (*Response, error) { return newResponse(), nil }

func (a *API) getLicense(*request) (*Response, error) {
	resp := newResponse()
	resp.License = &License{Valid: true}
	return resp, nil
}

func (a *API) getOpenSubsonicExtensions(*request) (*Response, error) {
	resp := newResponse()
	resp.OpenSubsonicExtensions = extensions
	return resp, nil
}

func (a *API) tokenInfo(q *request) (*Response, error) {
	resp := newResponse()
	resp.TokenInfo = &TokenInfo{Username: q.user.Username}
	return resp, nil
}

// ---- scanning

func (a *API) scanStatus(ctx context.Context) (*ScanStatus, error) {
	st := a.app.Scanner.Status()
	libs, err := a.app.Store.ListLibraries(ctx)
	if err != nil {
		return nil, err
	}
	out := &ScanStatus{Scanning: st.Scanning, FolderCount: len(libs)}
	if st.Scanning {
		out.Count = int64(st.FilesSeen)
	} else if err := a.app.Store.DB().R.GetContext(ctx, &out.Count, `SELECT COUNT(*) FROM tracks WHERE missing = 0`); err != nil {
		return nil, err
	}
	var last int64
	for _, l := range libs {
		last = max(last, l.LastScanAt)
	}
	last = max(last, st.FinishedAt)
	out.LastScan = date(last)
	return out, nil
}

func (a *API) getScanStatus(q *request) (*Response, error) {
	st, err := a.scanStatus(q.ctx)
	if err != nil {
		return nil, err
	}
	resp := newResponse()
	resp.ScanStatus = st
	return resp, nil
}

// startScan starts a quick scan of every library (fullScan=true re-reads every file).
// Admin only.
func (a *API) startScan(q *request) (*Response, error) {
	if !q.user.IsAdmin {
		return nil, errForbidden()
	}
	full, err := q.boolParam("fullScan", false)
	if err != nil {
		return nil, err
	}
	// The scan runs detached from the request (Start uses its own context).
	if err := a.app.Scanner.Start(context.WithoutCancel(q.ctx), scanner.Options{Full: full}); err != nil &&
		!errors.Is(err, scanner.ErrScanInProgress) {
		return nil, err
	}
	return a.getScanStatus(q)
}

// ---- internet radio

func radioOf(r *model.RadioStation) InternetRadioStation {
	return InternetRadioStation{ID: r.ID, Name: r.Name, StreamURL: r.StreamURL, HomePageURL: r.HomepageURL}
}

func (a *API) getInternetRadioStations(q *request) (*Response, error) {
	stations, err := a.app.Store.ListRadioStations(q.ctx)
	if err != nil {
		return nil, err
	}
	out := make([]InternetRadioStation, 0, len(stations))
	for i := range stations {
		out = append(out, radioOf(&stations[i]))
	}
	resp := newResponse()
	resp.InternetRadioStations = &InternetRadioStations{Stations: out}
	return resp, nil
}

// validURL accepts absolute http(s) URLs only.
func validURL(param, raw string, required bool) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if required {
			return "", errMissing(param)
		}
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", newError(codeGeneric, "Invalid URL for parameter %s", param)
	}
	return raw, nil
}

func (a *API) createInternetRadioStation(q *request) (*Response, error) {
	if !q.user.IsAdmin {
		return nil, errForbidden()
	}
	stream, err := validURL("streamUrl", q.str("streamUrl"), true)
	if err != nil {
		return nil, err
	}
	name, err := q.requiredStr("name")
	if err != nil {
		return nil, err
	}
	home, err := validURL("homepageUrl", q.str("homepageUrl"), false)
	if err != nil {
		return nil, err
	}
	st := &model.RadioStation{Name: name, StreamURL: stream, HomepageURL: home}
	if err := a.app.Store.CreateRadioStation(q.ctx, st); err != nil {
		return nil, err
	}
	return newResponse(), nil
}

func (a *API) updateInternetRadioStation(q *request) (*Response, error) {
	if !q.user.IsAdmin {
		return nil, errForbidden()
	}
	id, err := q.requiredStr("id")
	if err != nil {
		return nil, err
	}
	st, err := a.app.Store.GetRadioStation(q.ctx, id)
	if err != nil {
		return nil, err
	}
	if q.has("streamUrl") {
		if st.StreamURL, err = validURL("streamUrl", q.str("streamUrl"), true); err != nil {
			return nil, err
		}
	}
	if q.has("name") {
		if st.Name, err = q.requiredStr("name"); err != nil {
			return nil, err
		}
	}
	if q.has("homepageUrl") {
		if st.HomepageURL, err = validURL("homepageUrl", q.str("homepageUrl"), false); err != nil {
			return nil, err
		}
	}
	if err := a.app.Store.UpdateRadioStation(q.ctx, st); err != nil {
		return nil, err
	}
	return newResponse(), nil
}

func (a *API) deleteInternetRadioStation(q *request) (*Response, error) {
	if !q.user.IsAdmin {
		return nil, errForbidden()
	}
	id, err := q.requiredStr("id")
	if err != nil {
		return nil, err
	}
	if err := a.app.Store.DeleteRadioStation(q.ctx, id); err != nil {
		return nil, err
	}
	return newResponse(), nil
}

// ---- unsupported features

func (a *API) getPodcasts(*request) (*Response, error) {
	resp := newResponse()
	resp.Podcasts = &Podcasts{Channels: []struct{}{}}
	return resp, nil
}

func (a *API) getNewestPodcasts(*request) (*Response, error) {
	resp := newResponse()
	resp.NewestPodcasts = &NewestPodcasts{Episodes: []struct{}{}}
	return resp, nil
}

func (a *API) getShares(*request) (*Response, error) {
	resp := newResponse()
	resp.Shares = &Shares{Shares: []struct{}{}}
	return resp, nil
}

func (a *API) getChatMessages(*request) (*Response, error) {
	resp := newResponse()
	resp.ChatMessages = &ChatMessages{Messages: []struct{}{}}
	return resp, nil
}

func (a *API) getVideos(*request) (*Response, error) {
	resp := newResponse()
	resp.Videos = &Videos{Videos: []struct{}{}}
	return resp, nil
}

// unsupported answers error 0 for a feature Rainy does not implement.
func unsupported(feature string) handlerFunc {
	return func(*request) (*Response, error) {
		return nil, newError(codeGeneric, "%s is not supported by this server", feature)
	}
}

// notFoundStub answers error 70 (lookups of unsupported item kinds).
func notFoundStub(what string) handlerFunc {
	return func(*request) (*Response, error) { return nil, errNotFound(what) }
}
