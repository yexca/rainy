package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"net/mail"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"rainy/internal/auth"
	"rainy/internal/buildinfo"
	"rainy/internal/config"
	"rainy/internal/events"
	"rainy/internal/manage"
	"rainy/internal/model"
	"rainy/internal/scanner"
	"rainy/internal/store"
	"rainy/internal/util"
)

// routesAdmin registers the admin endpoints under /api/admin (paths relative, e.g.
// "/users"); mounted behind auth.RequireAdmin.
//
// Owner: manage agent (D) (docs/architecture/contract.md §7.7).
func (a *API) routesAdmin(r chi.Router) {
	r.Get("/users", a.adminListUsers)
	r.Post("/users", a.adminCreateUser)
	r.Put("/users/{id}", a.adminUpdateUser)
	r.Delete("/users/{id}", a.adminDeleteUser)
	r.Get("/libraries", a.adminListLibraries)
	r.Post("/libraries", a.adminCreateLibrary)
	r.Put("/libraries/{id}", a.adminUpdateLibrary)
	r.Delete("/libraries/{id}", a.adminDeleteLibrary)
	r.Get("/scan", a.adminScanStatus)
	r.Post("/scan", a.adminStartScan)
	r.Get("/settings", a.adminGetSettings)
	r.Put("/settings", a.adminPutSettings)
	r.Get("/stats", a.adminStats)
	r.Get("/system", a.adminSystem)
	r.Post("/cache/clear", a.adminClearCache)
	r.Get("/ytdlp", a.adminYtdlp)
	r.Post("/ytdlp/check", a.adminYtdlpCheck)
	r.Post("/ytdlp/install", a.adminYtdlpInstall)
	r.Put("/ytdlp/cookies/{site}", a.adminSetCookies)
	r.Delete("/ytdlp/cookies/{site}", a.adminDeleteCookies)
}

// ---- users

type userBody struct {
	Username    *string `json:"username"`
	Password    *string `json:"password"`
	DisplayName *string `json:"displayName"`
	Email       *string `json:"email"`
	IsAdmin     *bool   `json:"isAdmin"`
	CanManage   *bool   `json:"canManage"`
	CanDownload *bool   `json:"canDownload"`
}

const maxProfileField = 200

// applyProfile validates and copies display name / e-mail from b into u.
func (b *userBody) applyProfile(u *model.User) error {
	if b.DisplayName != nil {
		v := strings.TrimSpace(*b.DisplayName)
		if utf8.RuneCountInString(v) > maxProfileField {
			return badRequest("display name is too long")
		}
		u.DisplayName = v
	}
	if b.Email != nil {
		v := strings.TrimSpace(*b.Email)
		if len(v) > maxProfileField {
			return badRequest("e-mail is too long")
		}
		if v != "" {
			if addr, err := mail.ParseAddress(v); err != nil || addr.Address != v {
				return badRequest("invalid e-mail address")
			}
		}
		u.Email = v
	}
	if b.IsAdmin != nil {
		u.IsAdmin = *b.IsAdmin
	}
	if b.CanManage != nil {
		u.CanManage = *b.CanManage
	}
	if b.CanDownload != nil {
		u.CanDownload = *b.CanDownload
	}
	return nil
}

func (a *API) adminListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.app.Store.ListUsers(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, nonNil(users))
}

func (a *API) adminCreateUser(w http.ResponseWriter, r *http.Request) {
	var body userBody
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if body.Username == nil || body.Password == nil {
		writeErr(w, r, badRequest("username and password are required"))
		return
	}
	u := &model.User{Username: *body.Username, CanDownload: true}
	if err := body.applyProfile(u); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := a.app.Auth.CreateUser(r.Context(), u, *body.Password); err != nil {
		writeErr(w, r, err)
		return
	}
	created, err := a.app.Store.GetUser(r.Context(), u.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// countAdmins returns the number of administrators.
func (a *API) countAdmins(ctx context.Context) (int, error) {
	users, err := a.app.Store.ListUsers(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, u := range users {
		if u.IsAdmin {
			n++
		}
	}
	return n, nil
}

// adminUsersMu serialises user updates and deletions so that two concurrent requests
// cannot both pass the "at least one administrator" check and remove the last admins.
var adminUsersMu sync.Mutex

func (a *API) adminUpdateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	me := userFrom(r)
	var body userBody
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	adminUsersMu.Lock()
	defer adminUsersMu.Unlock()
	u, err := a.app.Store.GetUser(ctx, chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	wasAdmin := u.IsAdmin
	if body.Username != nil {
		name := strings.TrimSpace(*body.Username)
		if err := auth.ValidateUsername(name); err != nil {
			writeErr(w, r, err)
			return
		}
		u.Username = name
	}
	if err := body.applyProfile(u); err != nil {
		writeErr(w, r, err)
		return
	}
	if wasAdmin && !u.IsAdmin {
		if u.ID == me.ID {
			writeErr(w, r, forbidden("you cannot remove your own administrator role"))
			return
		}
		n, err := a.countAdmins(ctx)
		if err != nil {
			writeErr(w, r, err)
			return
		}
		if n <= 1 {
			writeErr(w, r, newError(http.StatusConflict, CodeConflict, "at least one administrator is required"))
			return
		}
	}
	if body.Password != nil {
		if err := auth.ValidatePassword(*body.Password); err != nil {
			writeErr(w, r, err)
			return
		}
	}
	if err := a.app.Store.UpdateUser(ctx, u); err != nil {
		writeErr(w, r, err)
		return
	}
	if body.Password != nil {
		if err := a.app.Auth.ChangePassword(ctx, u.ID, *body.Password); err != nil {
			writeErr(w, r, err)
			return
		}
		if u.ID != me.ID {
			// Someone else's password was reset: sign them out everywhere.
			if err := a.app.Store.DeleteUserSessions(ctx, u.ID); err != nil {
				slog.Warn("admin: revoking sessions after password reset", "user", u.ID, "err", err)
			}
		}
	}
	updated, err := a.app.Store.GetUser(ctx, u.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *API) adminDeleteUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	adminUsersMu.Lock()
	defer adminUsersMu.Unlock()
	if id == userFrom(r).ID {
		writeErr(w, r, forbidden("you cannot delete your own account"))
		return
	}
	u, err := a.app.Store.GetUser(ctx, id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if u.IsAdmin {
		n, err := a.countAdmins(ctx)
		if err != nil {
			writeErr(w, r, err)
			return
		}
		if n <= 1 {
			writeErr(w, r, newError(http.StatusConflict, CodeConflict, "at least one administrator is required"))
			return
		}
	}
	if err := a.app.Store.DeleteUser(ctx, id); err != nil {
		writeErr(w, r, err)
		return
	}
	writeNoContent(w)
}

// ---- libraries

// LibraryInfo is a library with its status (GET /api/admin/libraries).
type LibraryInfo struct {
	model.Library
	TrackCount int  `json:"trackCount"`
	Exists     bool `json:"exists"`
	Writable   bool `json:"writable"`
}

func (a *API) libraryInfos(ctx context.Context) ([]LibraryInfo, error) {
	libs, err := a.app.Store.ListLibraries(ctx)
	if err != nil {
		return nil, err
	}
	counts, err := a.app.Store.LibraryTrackCounts(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]LibraryInfo, len(libs))
	for i, l := range libs {
		out[i] = libraryInfo(l, counts[l.ID])
	}
	return out, nil
}

func libraryInfo(l model.Library, count int) LibraryInfo {
	info := LibraryInfo{Library: l, TrackCount: count}
	if fi, err := os.Stat(l.Path); err == nil && fi.IsDir() {
		info.Exists = true
		info.Writable = manage.DirWritable(l.Path)
	}
	return info
}

func (a *API) adminListLibraries(w http.ResponseWriter, r *http.Request) {
	infos, err := a.libraryInfos(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, nonNil(infos))
}

type libraryBody struct {
	Name *string `json:"name"`
	Path *string `json:"path"`
}

// validateLibraryPath cleans p and checks that it is an existing directory that does not
// overlap another library (excluding id) or the data directory.
func (a *API) validateLibraryPath(ctx context.Context, p string, id int64) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", badRequest("path is required")
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", badRequest("invalid path: %v", err)
	}
	abs = filepath.Clean(abs)
	fi, err := os.Stat(abs)
	if err != nil || !fi.IsDir() {
		return "", badRequest("%s does not exist or is not a directory (inside docker, use the container path of a mounted volume, e.g. /music)", abs)
	}
	overlaps := func(x, y string) bool {
		x, y = canonicalPath(x), canonicalPath(y)
		return util.IsWithin(x, y) || util.IsWithin(y, x)
	}
	if overlaps(abs, a.app.Cfg.DataDir) {
		return "", badRequest("a library cannot overlap the data directory (%s)", a.app.Cfg.DataDir)
	}
	libs, err := a.app.Store.ListLibraries(ctx)
	if err != nil {
		return "", err
	}
	for _, l := range libs {
		if l.ID != id && overlaps(abs, l.Path) {
			return "", newError(http.StatusConflict, CodeConflict, "the folder overlaps library %q (%s); libraries cannot be nested", l.Name, l.Path)
		}
	}
	return abs, nil
}

// canonicalPath returns p absolute, cleaned and with symlinks resolved (as far as they
// exist), lower-cased on Windows where paths are case-insensitive — suitable for
// comparing library roots.
func canonicalPath(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		p = r
	}
	p = filepath.Clean(p)
	if runtime.GOOS == "windows" {
		p = strings.ToLower(p)
	}
	return p
}

func libraryName(name *string, path string) (string, error) {
	n := ""
	if name != nil {
		n = strings.TrimSpace(*name)
	}
	if n == "" {
		n = filepath.Base(path)
	}
	if utf8.RuneCountInString(n) > maxProfileField {
		return "", badRequest("name is too long")
	}
	return n, nil
}

// startLibraryScan starts a quick scan of a library (ignoring a running scan).
func (a *API) startLibraryScan(id int64) {
	err := a.app.Scanner.Start(context.Background(), scanner.Options{LibraryID: id})
	if err != nil && !errors.Is(err, scanner.ErrScanInProgress) {
		slog.Warn("admin: starting library scan", "library", id, "err", err)
	}
}

func (a *API) adminCreateLibrary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var body libraryBody
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if body.Path == nil {
		writeErr(w, r, badRequest("path is required"))
		return
	}
	path, err := a.validateLibraryPath(ctx, *body.Path, 0)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	name, err := libraryName(body.Name, path)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	lib := &model.Library{Name: name, Path: path}
	if err := a.app.Store.CreateLibrary(ctx, lib); err != nil {
		writeErr(w, r, err)
		return
	}
	a.startLibraryScan(lib.ID)
	a.app.Bus.Publish(events.Library("libraries"))
	writeJSON(w, http.StatusCreated, libraryInfo(*lib, 0))
}

func (a *API) libraryParam(r *http.Request) (*model.Library, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		return nil, notFound("library not found")
	}
	return a.app.Store.GetLibrary(r.Context(), id)
}

func (a *API) adminUpdateLibrary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	lib, err := a.libraryParam(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var body libraryBody
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	pathChanged := false
	if body.Path != nil {
		p, err := a.validateLibraryPath(ctx, *body.Path, lib.ID)
		if err != nil {
			writeErr(w, r, err)
			return
		}
		pathChanged = p != lib.Path
		lib.Path = p
	}
	if pathChanged {
		// A file operation or scan running on the old root must finish first: its
		// rescans would otherwise read the new root and mark the tracks missing.
		unlock, err := a.lockLibraries(r)
		if err != nil {
			writeErr(w, r, err)
			return
		}
		defer unlock()
		ctx = context.WithoutCancel(ctx)
	}
	if body.Name != nil {
		if lib.Name, err = libraryName(body.Name, lib.Path); err != nil {
			writeErr(w, r, err)
			return
		}
	}
	if err := a.app.Store.UpdateLibrary(ctx, lib); err != nil {
		writeErr(w, r, err)
		return
	}
	if pathChanged {
		a.startLibraryScan(lib.ID)
	}
	a.app.Bus.Publish(events.Library("libraries"))
	counts, _ := a.app.Store.LibraryTrackCounts(ctx)
	writeJSON(w, http.StatusOK, libraryInfo(*lib, counts[lib.ID]))
}

// adminLockTimeout bounds how long library changes wait for the library lock.
var adminLockTimeout = 15 * time.Second

// lockLibraries takes the scanner's library lock (keeping scans and file operations out
// while a library row changes), giving up with 409 when a scan or a file operation holds
// it for longer than adminLockTimeout.
func (a *API) lockLibraries(r *http.Request) (func(), error) {
	if a.app.Scanner.Status().Scanning {
		return nil, newError(http.StatusConflict, CodeConflict, "a scan is running; try again when it has finished")
	}
	ctx, cancel := context.WithTimeout(r.Context(), adminLockTimeout)
	defer cancel()
	unlock, err := a.app.Scanner.TryLockLibrary(ctx)
	if err != nil {
		if r.Context().Err() != nil {
			return nil, r.Context().Err()
		}
		return nil, newError(http.StatusConflict, CodeConflict, "the library is busy (a scan or a file operation is running); try again later")
	}
	return unlock, nil
}

func (a *API) adminDeleteLibrary(w http.ResponseWriter, r *http.Request) {
	lib, err := a.libraryParam(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	unlock, err := a.lockLibraries(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	err = a.app.Store.DeleteLibrary(context.WithoutCancel(r.Context()), lib.ID)
	unlock()
	if err != nil {
		writeErr(w, r, err)
		return
	}
	a.app.Bus.Publish(events.Library("libraries"))
	writeNoContent(w)
}

// ---- scan

func (a *API) adminScanStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.app.Scanner.Status())
}

func (a *API) adminStartScan(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Full      bool  `json:"full"`
		LibraryID int64 `json:"libraryId"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if body.LibraryID < 0 {
		writeErr(w, r, badRequest("invalid libraryId"))
		return
	}
	if body.LibraryID > 0 {
		if _, err := a.app.Store.GetLibrary(r.Context(), body.LibraryID); err != nil {
			writeErr(w, r, err)
			return
		}
	}
	if err := a.app.Scanner.Start(r.Context(), scanner.Options{Full: body.Full, LibraryID: body.LibraryID}); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, a.app.Scanner.Status())
}

// ---- settings

func (a *API) adminGetSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.app.Settings(r.Context()))
}

// mergeSettings applies a partial JSON object to cur (unknown keys and wrong types are
// rejected) and validates the result.
func mergeSettings(cur model.Settings, patch map[string]json.RawMessage) (model.Settings, error) {
	base, err := json.Marshal(cur)
	if err != nil {
		return cur, err
	}
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(base, &fields); err != nil {
		return cur, err
	}
	for k, v := range patch {
		if _, ok := fields[k]; !ok {
			return cur, badRequest("unknown setting %q", k)
		}
		fields[k] = v
	}
	merged, err := json.Marshal(fields)
	if err != nil {
		return cur, err
	}
	out := cur
	dec := json.NewDecoder(strings.NewReader(string(merged)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&out); err != nil {
		var te *json.UnmarshalTypeError
		if errors.As(err, &te) {
			return cur, badRequest("invalid value for %s", te.Field)
		}
		return cur, badRequest("invalid settings: %v", err)
	}
	return out, validateSettings(&out)
}

// validateSettings checks and normalizes settings in place.
func validateSettings(s *model.Settings) error {
	d, err := config.ParseDuration(s.ScanInterval)
	if err != nil || d < 0 {
		return badRequest("scanInterval must be a duration such as 30m, 1h or 1d (0 disables)")
	}
	if d > 0 && d < time.Minute {
		return badRequest("scanInterval must be at least 1m (or 0 to disable)")
	}
	s.ScanInterval = model.FormatDuration(d)
	s.TranscodeFormat = strings.ToLower(strings.TrimSpace(s.TranscodeFormat))
	switch s.TranscodeFormat {
	case "mp3", "opus", "aac":
	default:
		return badRequest("transcodeFormat must be mp3, opus or aac")
	}
	if s.TranscodeBitrate < 32 || s.TranscodeBitrate > 320 {
		return badRequest("transcodeBitrate must be between 32 and 320 kbps")
	}
	s.RenamePattern = strings.TrimSpace(s.RenamePattern)
	if _, err := manage.ParsePattern(s.RenamePattern); err != nil {
		return badRequest("renamePattern: %s", strings.TrimPrefix(err.Error(), store.ErrInvalid.Error()+": "))
	}
	if utf8.RuneCountInString(s.GenreSeparators) > 16 {
		return badRequest("genreSeparators is too long")
	}
	if len(s.IgnoredArticles) > 500 {
		return badRequest("ignoredArticles is too long")
	}
	s.IgnoredArticles = strings.Join(strings.Fields(s.IgnoredArticles), " ")
	var patterns []string
	for _, p := range strings.Split(s.CoverArtFiles, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.ContainsAny(p, `/\`) {
			return badRequest("coverArtFiles: %q must be a file name pattern without directories", p)
		}
		if _, err := filepath.Match(p, "x"); err != nil {
			return badRequest("coverArtFiles: invalid pattern %q", p)
		}
		patterns = append(patterns, p)
	}
	if len(patterns) == 0 {
		return badRequest("coverArtFiles needs at least one pattern")
	}
	s.CoverArtFiles = strings.Join(patterns, ",")
	return nil
}

func (a *API) adminPutSettings(w http.ResponseWriter, r *http.Request) {
	var patch map[string]json.RawMessage
	if err := decodeJSON(w, r, &patch); err != nil {
		writeErr(w, r, err)
		return
	}
	next, err := mergeSettings(a.app.Settings(r.Context()), patch)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if err := a.app.Store.SaveSettings(r.Context(), next); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, a.app.Settings(r.Context()))
}

// ---- stats & system

func (a *API) adminStats(w http.ResponseWriter, r *http.Request) {
	st, err := a.app.Store.Stats(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// FFmpegInfo describes the transcoder binary.
type FFmpegInfo struct {
	Available bool   `json:"available"`
	Version   string `json:"version"`
	Path      string `json:"path"`
}

// SystemInfo is the response of GET /api/admin/system.
type SystemInfo struct {
	Version   string        `json:"version"`
	Commit    string        `json:"commit"`
	BuildDate string        `json:"buildDate"`
	GoVersion string        `json:"goVersion"`
	OS        string        `json:"os"`
	Arch      string        `json:"arch"`
	UptimeSec int64         `json:"uptimeSec"`
	DataDir   string        `json:"dataDir"`
	DBSize    int64         `json:"dbSize"`
	CacheSize int64         `json:"cacheSize"`
	TrashSize int64         `json:"trashSize"`
	FFmpeg    FFmpegInfo    `json:"ffmpeg"`
	Libraries []LibraryInfo `json:"libraries"`
}

// dirSize sums the sizes of regular files under dir (0 when it does not exist).
func dirSize(dir string) int64 {
	var n int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.Type().IsRegular() {
			if fi, err := d.Info(); err == nil {
				n += fi.Size()
			}
		}
		return nil
	})
	return n
}

func fileSize(p string) int64 {
	if fi, err := os.Stat(p); err == nil {
		return fi.Size()
	}
	return 0
}

func (a *API) adminSystem(w http.ResponseWriter, r *http.Request) {
	cfg := a.app.Cfg
	libs, err := a.libraryInfos(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	db := cfg.DBPath()
	info := SystemInfo{
		Version: buildinfo.Version, Commit: buildinfo.Commit, BuildDate: buildinfo.BuildDate,
		GoVersion: buildinfo.GoVersion(), OS: runtime.GOOS, Arch: runtime.GOARCH,
		UptimeSec: int64(time.Since(a.app.StartedAt).Seconds()),
		DataDir:   cfg.DataDir,
		DBSize:    fileSize(db) + fileSize(db+"-wal") + fileSize(db+"-shm"),
		CacheSize: dirSize(cfg.CacheDir()),
		TrashSize: dirSize(cfg.TrashDir()),
		FFmpeg:    FFmpegInfo{Path: a.app.Transcoder.Path()},
		Libraries: nonNil(libs),
	}
	if a.app.Transcoder.Available() {
		info.FFmpeg.Available = true
		info.FFmpeg.Version = a.app.Transcoder.Version()
	}
	writeJSON(w, http.StatusOK, info)
}

func (a *API) adminClearCache(w http.ResponseWriter, r *http.Request) {
	freed, err := a.app.Artwork.ClearCache()
	if err != nil {
		writeErr(w, r, fmt.Errorf("clearing the artwork cache: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"freed": freed})
}
