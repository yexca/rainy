package manage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	_ "image/gif" // decoders registered for image.Decode
	_ "image/png"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"rainy/internal/artwork"
	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/tags"
	"rainy/internal/util"
)

const (
	// Covers larger than this (either side) or heavier than coverMaxBytes are re-encoded
	// as JPEG fitting coverTargetDim before embedding.
	coverMaxDim    = 1600
	coverMaxBytes  = 3 << 20
	coverTargetDim = 1400
	coverQuality   = 90
	// MaxCoverUpload bounds the uploaded image size.
	MaxCoverUpload = 32 << 20
	// maxCoverPixels guards against decompression bombs.
	maxCoverPixels = 100_000_000
)

// CoverRequest is POST /api/manage/cover.
type CoverRequest struct {
	TrackIDs     []string
	AlbumID      string
	Embed        bool
	SaveToFolder bool
	Image        []byte
}

// RemoveCoverRequest is DELETE /api/manage/cover.
type RemoveCoverRequest struct {
	TrackIDs          []string `json:"trackIds"`
	AlbumID           string   `json:"albumId"`
	RemoveFolderImage bool     `json:"removeFolderImage"`
}

// preparedCover is an image ready to embed.
type preparedCover struct {
	data []byte
	ext  string // "jpg" | "png"
}

// prepareCover validates an uploaded image. JPEG and PNG images within the limits are kept
// byte for byte; anything larger, heavier or in another format (WebP, GIF) is re-encoded
// as a JPEG fitting coverTargetDim.
func prepareCover(data []byte) (*preparedCover, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: the image is empty", store.ErrInvalid)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: unsupported image (use JPEG, PNG or WebP)", store.ErrInvalid)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxCoverPixels {
		return nil, fmt.Errorf("%w: image dimensions %dx%d are not supported", store.ErrInvalid, cfg.Width, cfg.Height)
	}
	small := cfg.Width <= coverMaxDim && cfg.Height <= coverMaxDim && len(data) <= coverMaxBytes
	switch {
	case small && format == "jpeg":
		return &preparedCover{data: data, ext: "jpg"}, nil
	case small && format == "png":
		return &preparedCover{data: data, ext: "png"}, nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: cannot decode image: %v", store.ErrInvalid, err)
	}
	out, err := encodeJPEG(fit(img, coverTargetDim))
	if err != nil {
		return nil, err
	}
	return &preparedCover{data: out, ext: "jpg"}, nil
}

// fit scales img down (never up) to fit within dim×dim, flattening transparency on white.
func fit(img image.Image, dim int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > dim || h > dim {
		if w >= h {
			h = max(1, h*dim/w)
			w = dim
		} else {
			w = max(1, w*dim/h)
			h = dim
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
	return dst
}

func encodeJPEG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: coverQuality}); err != nil {
		return nil, fmt.Errorf("encoding jpeg: %w", err)
	}
	return buf.Bytes(), nil
}

// writePicture sets (img != nil) or removes (nil) the embedded front cover of a file.
func writePicture(abs string, img []byte) error {
	if err := checkWritable("", abs); err != nil {
		return err
	}
	return tags.WritePicture(abs, img)
}

// coverTargets resolves the tracks of a cover request (explicit ids or a whole album).
func (s *Service) coverTargets(ctx context.Context, trackIDs []string, albumID string) ([]model.Track, error) {
	trackIDs = dedupe(trackIDs)
	switch {
	case albumID != "" && len(trackIDs) > 0:
		return nil, fmt.Errorf("%w: give either trackIds or albumId", store.ErrInvalid)
	case albumID != "":
		if _, err := s.st.GetAlbum(ctx, albumID, ""); err != nil {
			return nil, err
		}
		ts, _, err := s.st.ListTracks(ctx, store.TrackQuery{AlbumID: albumID, Sort: "track"})
		if err != nil {
			return nil, err
		}
		if len(ts) == 0 {
			return nil, fmt.Errorf("%w: the album has no tracks", store.ErrInvalid)
		}
		return ts, nil
	}
	if err := checkBatch(len(trackIDs), "trackIds"); err != nil {
		return nil, err
	}
	return s.st.GetTracks(ctx, trackIDs, "")
}

// SetCover embeds an image into tracks and/or saves it as the folder image.
func (s *Service) SetCover(ctx context.Context, u *model.User, req CoverRequest) (*BatchResult, error) {
	if !req.Embed && !req.SaveToFolder {
		return nil, fmt.Errorf("%w: nothing to do (embed and saveToFolder are both false)", store.ErrInvalid)
	}
	pc, err := prepareCover(req.Image)
	if err != nil {
		return nil, err
	}
	targets, err := s.coverTargets(ctx, req.TrackIDs, req.AlbumID)
	if err != nil {
		return nil, err
	}
	res := newBatch()
	if len(targets) == 0 {
		return res, nil
	}

	unlock, err := s.lock(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	ctx = context.WithoutCancel(ctx)
	libs := s.libs()
	var ok []model.Track
	folderImages := map[string]string{} // abs dir → saved image (abs)
	for _, t := range targets {
		_, abs, err := s.trackFile(ctx, libs, &t)
		if err != nil {
			res.fail(t.ID, t.Path, err)
			continue
		}
		if req.Embed {
			if err := writePicture(abs, pc.data); err != nil {
				res.fail(t.ID, t.Path, fsErr(t.Path, err))
				continue
			}
		}
		if req.SaveToFolder {
			dir := filepath.Dir(abs)
			if _, done := folderImages[dir]; !done {
				lib, _ := libs.get(ctx, t.LibraryID) // cached by trackFile
				p, err := s.saveFolderImage(ctx, u, lib, dir, pc)
				if err != nil {
					res.fail(t.ID, t.Path, fsErr(t.Dir+"/cover."+pc.ext, err))
					if !req.Embed {
						continue
					}
				} else {
					folderImages[dir] = p
				}
			}
		}
		ok = append(ok, t)
	}

	s.afterCoverChange(ctx, u, ok, req.Embed, "set", folderImages)
	res.Updated = s.tracksByID(ctx, trackIDsOf(ok), u.ID)
	if err := res.failure(); err != nil {
		return nil, err
	}
	return res, nil
}

// RemoveCover removes embedded pictures and optionally the folder images (moved to the
// trash so they can be restored).
func (s *Service) RemoveCover(ctx context.Context, u *model.User, req RemoveCoverRequest) (*BatchResult, error) {
	targets, err := s.coverTargets(ctx, req.TrackIDs, req.AlbumID)
	if err != nil {
		return nil, err
	}
	res := newBatch()
	if len(targets) == 0 {
		return res, nil
	}
	patterns := s.settings(ctx).CoverArtPatterns()

	unlock, err := s.lock(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	ctx = context.WithoutCancel(ctx)
	libs := s.libs()
	var ok []model.Track
	removedDirs := map[string]string{}
	for _, t := range targets {
		lib, err := libs.get(ctx, t.LibraryID)
		if err != nil {
			res.fail(t.ID, t.Path, err)
			continue
		}
		abs, err := libPath(lib.Path, t.Path)
		if err != nil {
			res.fail(t.ID, t.Path, err)
			continue
		}
		if pic, err := tags.ReadPicture(abs); err != nil || len(pic) > 0 {
			// Remove when a picture exists (or when reading failed, so the write reports
			// the real problem).
			if err := writePicture(abs, nil); err != nil {
				res.fail(t.ID, t.Path, fsErr(t.Path, err))
				continue
			}
		}
		if req.RemoveFolderImage {
			dir := filepath.Dir(abs)
			if _, done := removedDirs[dir]; !done {
				removedDirs[dir] = ""
				if err := s.trashFolderImages(ctx, u, lib, dir, patterns); err != nil {
					// The embedded picture is already gone: still rescan / log the track.
					res.fail(t.ID, t.Path, fsErr(t.Dir, err))
				}
			}
		}
		ok = append(ok, t)
	}

	s.afterCoverChange(ctx, u, ok, true, "remove", nil)
	res.Updated = s.tracksByID(ctx, trackIDsOf(ok), u.ID)
	if err := res.failure(); err != nil {
		return nil, err
	}
	return res, nil
}

// afterCoverChange rescans the tracks, fixes album cover paths, busts cover caches, logs
// and publishes.
func (s *Service) afterCoverChange(ctx context.Context, u *model.User, ok []model.Track, embedded bool, op string, folderImages map[string]string) {
	if len(ok) == 0 {
		return
	}
	byLib := map[int64][]string{}
	for _, t := range ok {
		byLib[t.LibraryID] = append(byLib[t.LibraryID], t.Path)
	}
	if embedded {
		for lib, paths := range byLib {
			s.rescan(ctx, lib, paths)
		}
	}
	// The rescan may already have updated cover paths; set them explicitly so the result
	// does not depend on the scanner's folder-image lookup.
	now := s.tracksByID(ctx, trackIDsOf(ok), "")
	patterns := s.settings(ctx).CoverArtPatterns()
	libs := s.libs()
	albumCover := map[string]string{}
	for _, t := range now {
		if _, done := albumCover[t.AlbumID]; done {
			continue
		}
		lib, err := libs.get(ctx, t.LibraryID)
		if err != nil {
			continue
		}
		abs, err := libPath(lib.Path, t.Path)
		if err != nil {
			continue
		}
		dir := filepath.Dir(abs)
		if p, saved := folderImages[dir]; saved {
			albumCover[t.AlbumID] = p
		} else {
			albumCover[t.AlbumID] = artwork.FindFolderImage(dir, patterns)
		}
	}
	albumIDs := make([]string, 0, len(albumCover))
	for id, p := range albumCover {
		albumIDs = append(albumIDs, id)
		if err := s.st.SetAlbumCoverPath(ctx, id, p); err != nil && !errors.Is(err, store.ErrNotFound) {
			slog.Warn("manage: setting album cover path", "album", id, "err", err)
		}
	}
	if err := s.st.TouchAlbums(ctx, albumIDs); err != nil {
		slog.Warn("manage: touching albums", "err", err)
	}
	for _, t := range ok {
		details := map[string]any{"op": op, "embedded": embedded}
		if lib, err := libs.get(ctx, t.LibraryID); err == nil {
			if abs, err := libPath(lib.Path, t.Path); err == nil {
				if p, saved := folderImages[filepath.Dir(abs)]; saved {
					if rel, err := util.ToRel(lib.Path, p); err == nil {
						details["folderImage"] = rel
					}
				}
			}
		}
		s.logEdit(ctx, u, "cover", t.ID, t.Path, details)
	}
	s.publish("cover")
}

// saveFolderImage writes cover.<ext> into dir so that it is the folder image picked up.
// Every existing "cover.*" image variant ("Cover.PNG", "cover.webp", the old cover.<ext>
// itself, …) is moved to the trash first (restorable, logged), so replacing a folder
// image never destroys one. Must be called with the library lock held.
func (s *Service) saveFolderImage(ctx context.Context, u *model.User, lib *model.Library, dir string, pc *preparedCover) (string, error) {
	if err := util.EnsureWithinRoot(lib.Path, dir); err != nil {
		return "", err
	}
	target := filepath.Join(dir, "cover."+pc.ext)
	tmp, err := os.CreateTemp(dir, ".rainy-cover-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	_, werr := tmp.Write(pc.data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(tmpName)
		return "", errors.Join(werr, cerr)
	}
	_ = os.Chmod(tmpName, 0o644)
	entries, err := os.ReadDir(dir)
	if err != nil {
		_ = os.Remove(tmpName)
		return "", err
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.EqualFold(stem(name), "cover") ||
			!util.IsImageSuffix(strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))) {
			continue
		}
		p := filepath.Join(dir, name)
		if !isRegularFile(p) {
			// A symlink: removing the link loses no data (and it is never followed).
			_ = os.Remove(p)
			continue
		}
		if name == "cover."+pc.ext {
			if old, err := os.ReadFile(p); err == nil && bytes.Equal(old, pc.data) {
				continue // same image: replacing it loses nothing
			}
		}
		rel, err := util.ToRel(lib.Path, p)
		if err == nil {
			entry := &model.TrashEntry{LibraryID: lib.ID, OriginalPath: rel, Title: name, DeletedBy: u.Username}
			if err = s.moveToTrash(ctx, lib, entry); err == nil {
				s.logEdit(ctx, u, "delete", "", rel, map[string]any{"trashId": entry.ID, "trashPath": entry.TrashPath, "replacedBy": "cover." + pc.ext})
				continue
			}
		}
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("moving the replaced folder image %s to the trash: %w", name, err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		_ = os.Remove(tmpName)
		return "", err
	}
	return target, nil
}

// trashFolderImages moves the images of dir matching the cover-art patterns to the trash.
func (s *Service) trashFolderImages(ctx context.Context, u *model.User, lib *model.Library, dir string, patterns []string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !util.IsImageSuffix(strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))) || !matchesAny(name, patterns) {
			continue
		}
		rel, err := util.ToRel(lib.Path, filepath.Join(dir, name))
		if err != nil {
			return err
		}
		entry := &model.TrashEntry{LibraryID: lib.ID, OriginalPath: rel, Title: name, DeletedBy: u.Username}
		if err := s.moveToTrash(ctx, lib, entry); err != nil {
			return err
		}
		s.logEdit(ctx, u, "delete", "", rel, map[string]any{"trashId": entry.ID, "trashPath": entry.TrashPath})
	}
	return nil
}

// matchesAny reports whether name matches one of the glob patterns (case-insensitive).
func matchesAny(name string, patterns []string) bool {
	lower := strings.ToLower(name)
	for _, p := range patterns {
		if ok, _ := filepath.Match(strings.ToLower(p), lower); ok {
			return true
		}
	}
	return false
}

func trackIDsOf(ts []model.Track) []string {
	ids := make([]string, len(ts))
	for i := range ts {
		ids[i] = ts[i].ID
	}
	return ids
}
