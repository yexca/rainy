// Package artwork resolves cover art (album folder images, embedded pictures, artist
// images, playlist mosaics), resizes it and caches the results on disk.
//
// Resized images are cached under the cache directory in two-level sharded folders
// (<cacheDir>/<k[0:2]>/<k>.img). The key k hashes the identity of the source image (file
// path + mtime + size, or track id + updated_at for embedded pictures) together with the
// requested size, so edited covers never serve stale bytes, and entries shared between an
// album and its tracks are stored once. The cache is pruned in the background to
// MaxCacheBytes, oldest first.
//
// docs/architecture/contract.md §5.9.
package artwork

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"

	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/tags"
	"rainy/internal/util"
)

// ErrNotFound is returned when no artwork exists for an id.
var ErrNotFound = errors.New("artwork not found")

const (
	// MaxCacheBytes bounds the on-disk cache of resized images.
	MaxCacheBytes int64 = 1 << 30
	// MinSize and MaxSize clamp requested sizes (size > 0).
	MinSize = 16
	MaxSize = 2048
	// mosaicDefaultSize is used for playlist mosaics requested with size 0.
	mosaicDefaultSize = 600
	// jpegQuality for resized output.
	jpegQuality = 85
	// maxSourceBytes guards against absurdly large "cover" files.
	maxSourceBytes = 64 << 20
)

// Image is an encoded image.
type Image struct {
	Data        []byte
	ContentType string
	ModTime     time.Time
}

// Service serves cover art. It is safe for concurrent use.
type Service struct {
	st       *store.Store
	cacheDir string
	maxCache int64

	group        singleflight.Group
	cacheBytes   atomic.Int64 // approximate cache size; -1 = not measured yet
	pruning      atomic.Bool
	pruneWG      sync.WaitGroup
	placeholders sync.Map // int size → *Image
}

// New creates the service; resized images are cached under cacheDir.
func New(st *store.Store, cacheDir string) *Service {
	s := &Service{st: st, cacheDir: cacheDir, maxCache: MaxCacheBytes}
	s.cacheBytes.Store(-1)
	return s
}

// source is one resolvable original image.
type source struct {
	key     string // identity including a version; changes whenever the bytes may change
	modTime time.Time
	load    func() ([]byte, error)
	// fallback, when set, resolves the next candidate used when this one turns out to be
	// unusable (file gone, corrupt image): folder image → embedded picture, track picture
	// → album cover, artist image → album covers.
	fallback func() (*source, error)
}

// Get returns the image for a cover-art id ("al-…", "tr-…", "ar-…", "pl-…" or a bare id,
// with or without "_<version>"). size > 0 fits the image within size×size (JPEG q85,
// cached on disk); 0 returns the original bytes (a playlist mosaic is rendered at 600px).
// ErrNotFound when the item does not exist or has no artwork.
func (s *Service) Get(ctx context.Context, coverArtID string, size int) (*Image, error) {
	kind, id, _ := util.SplitCoverArtID(coverArtID)
	if id == "" {
		return nil, ErrNotFound
	}
	srcs, err := s.resolve(ctx, kind, id)
	if err != nil {
		return nil, err
	}
	if size > 0 {
		size = min(max(size, MinSize), MaxSize)
	}
	var img *Image
	if len(srcs) == 1 {
		img, err = s.single(ctx, srcs[0], size)
	} else {
		img, err = s.cached(srcs, cmp.Or(size, mosaicDefaultSize))
	}
	if errors.Is(err, errBadImage) {
		slog.Warn("artwork: unusable image", "id", coverArtID, "err", err)
		return nil, fmt.Errorf("%w: %v", ErrNotFound, err)
	}
	return img, err
}

// single returns the original (size 0) or resized image of src, moving on to its fallback
// candidates while a candidate is unusable (vanished, unreadable picture, corrupt data).
func (s *Service) single(ctx context.Context, src *source, size int) (*Image, error) {
	for {
		var img *Image
		var err error
		if size <= 0 {
			var data []byte
			if data, err = src.load(); err == nil {
				if err = checkImage(data); err == nil {
					img = &Image{Data: data, ContentType: sniff(data), ModTime: src.modTime}
				}
			}
		} else {
			img, err = s.cached([]*source{src}, size)
		}
		if err == nil || src.fallback == nil || ctx.Err() != nil || !unusable(err) {
			return img, err
		}
		next, ferr := src.fallback()
		if ferr != nil {
			if errors.Is(ferr, ErrNotFound) {
				return nil, err // report why the last candidate failed
			}
			return nil, ferr
		}
		src = next
	}
}

// unusable reports whether err means "this candidate has no usable image" (as opposed to
// an I/O or database failure that should be reported).
func unusable(err error) bool {
	return errors.Is(err, ErrNotFound) || errors.Is(err, errBadImage) || errors.Is(err, tags.ErrUnsupported)
}

// resolve returns the source image(s) for an id: one source, or four for a mosaic.
func (s *Service) resolve(ctx context.Context, kind, id string) ([]*source, error) {
	one := func(src *source, err error) ([]*source, error) {
		if err != nil {
			return nil, err
		}
		return []*source{src}, nil
	}
	switch kind {
	case util.CoverKindAlbum:
		return one(s.albumSourceByID(ctx, id))
	case util.CoverKindTrack:
		return one(s.trackSource(ctx, id))
	case util.CoverKindArtist:
		return one(s.artistSource(ctx, id))
	case util.CoverKindPlaylist:
		return s.playlistSources(ctx, id)
	}
	// Bare id (Subsonic clients): try every kind.
	for _, k := range []string{util.CoverKindTrack, util.CoverKindAlbum, util.CoverKindArtist, util.CoverKindPlaylist} {
		srcs, err := s.resolve(ctx, k, id)
		if err == nil {
			return srcs, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}
	return nil, ErrNotFound
}

// lookupErr maps store.ErrNotFound to ErrNotFound.
func lookupErr(err error) error {
	if errors.Is(err, store.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

func (s *Service) albumSourceByID(ctx context.Context, id string) (*source, error) {
	a, err := s.st.GetAlbum(ctx, id, "")
	if err != nil {
		return nil, lookupErr(err)
	}
	return s.albumSource(ctx, a)
}

// albumSource: folder image (cover_path), else the embedded picture of cover_track_id.
func (s *Service) albumSource(ctx context.Context, a *model.Album) (*source, error) {
	if a.CoverPath != "" {
		if src, err := s.fileSource(ctx, a.CoverPath); err == nil {
			src.fallback = func() (*source, error) { return s.albumEmbeddedSource(ctx, a) }
			return src, nil
		}
	}
	return s.albumEmbeddedSource(ctx, a)
}

// albumEmbeddedSource: the embedded picture of the album's cover_track_id.
func (s *Service) albumEmbeddedSource(ctx context.Context, a *model.Album) (*source, error) {
	if a.CoverTrackID != "" {
		t, err := s.st.GetTrack(ctx, a.CoverTrackID, "")
		if err == nil && t.HasCover && !t.Missing {
			return s.embeddedSource(ctx, t)
		}
	}
	return nil, ErrNotFound
}

// trackSource: the track's embedded picture, else its album's cover.
func (s *Service) trackSource(ctx context.Context, id string) (*source, error) {
	t, err := s.st.GetTrack(ctx, id, "")
	if err != nil {
		return nil, lookupErr(err)
	}
	album := func() (*source, error) {
		if t.AlbumID == "" {
			return nil, ErrNotFound
		}
		return s.albumSourceByID(ctx, t.AlbumID)
	}
	if t.HasCover && !t.Missing {
		if src, err := s.embeddedSource(ctx, t); err == nil {
			src.fallback = album
			return src, nil
		}
	}
	return album()
}

// artistSource: the artist image (artist.* next to the album folders), else the cover of
// the artist's most recent album that has one.
func (s *Service) artistSource(ctx context.Context, id string) (*source, error) {
	a, err := s.st.GetArtist(ctx, id, "")
	if err != nil {
		return nil, lookupErr(err)
	}
	albums := func() (*source, error) { return s.artistAlbumSource(ctx, id) }
	if a.ImagePath != "" {
		if src, err := s.fileSource(ctx, a.ImagePath); err == nil {
			src.fallback = albums
			return src, nil
		}
	}
	return albums()
}

// artistAlbumSource: the cover of the artist's most recent album that has a usable one
// (an album whose cover turns out to be unusable falls back to the next album).
func (s *Service) artistAlbumSource(ctx context.Context, artistID string) (*source, error) {
	albums, _, err := s.st.ListAlbums(ctx, store.AlbumQuery{ArtistID: artistID, Sort: "year", Order: "desc", Limit: 20})
	if err != nil {
		return nil, err
	}
	var from func(i int) (*source, error)
	from = func(i int) (*source, error) {
		for ; i < len(albums); i++ {
			if src, err := s.albumSource(ctx, &albums[i]); err == nil {
				next := i + 1
				return withFallback(src, func() (*source, error) { return from(next) }), nil
			}
		}
		return nil, ErrNotFound
	}
	return from(0)
}

// withFallback appends next to the end of src's fallback chain: next is resolved once src
// and all of its own fallbacks turned out to be unusable.
func withFallback(src *source, next func() (*source, error)) *source {
	own := src.fallback
	if own == nil {
		src.fallback = next
		return src
	}
	src.fallback = func() (*source, error) {
		n, err := own()
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return next()
			}
			return nil, err
		}
		return withFallback(n, next), nil
	}
	return src
}

// playlistSources returns the covers of the first four distinct albums of a playlist (a
// 2×2 mosaic), or the first cover when there are fewer than four.
func (s *Service) playlistSources(ctx context.Context, id string) ([]*source, error) {
	if _, err := s.st.GetPlaylist(ctx, id); err != nil {
		return nil, lookupErr(err)
	}
	var albumIDs []string
	err := s.st.DB().R.SelectContext(ctx, &albumIDs, `SELECT t.album_id FROM playlist_tracks pt
		JOIN tracks t ON t.id = pt.track_id AND t.missing = 0
		WHERE pt.playlist_id = ? AND t.album_id != ''
		GROUP BY t.album_id ORDER BY MIN(pt.position) LIMIT 32`, id)
	if err != nil {
		return nil, fmt.Errorf("listing playlist albums: %w", err)
	}
	var srcs []*source
	for _, aid := range albumIDs {
		al, err := s.st.GetAlbum(ctx, aid, "")
		if err != nil {
			continue
		}
		src, err := s.albumSource(ctx, al)
		if err != nil {
			continue
		}
		// A mosaic tile may come from the source's fallback (decodeSource), whose identity is
		// not part of src.key: the album version (bumped whenever its tracks or cover change)
		// keeps the cached mosaic in step with it.
		src.key += "\x00album\x00" + al.ID + "\x00" + strconv.FormatInt(al.UpdatedAt, 10) + "\x00" + al.CoverTrackID
		srcs = append(srcs, src)
		if len(srcs) == 4 {
			return srcs, nil
		}
	}
	if len(srcs) == 0 {
		return nil, ErrNotFound
	}
	return srcs[:1], nil
}

// fileSource returns a source for an image file inside one of the libraries.
func (s *Service) fileSource(ctx context.Context, abs string) (*source, error) {
	if !s.insideLibrary(ctx, abs) {
		slog.Warn("artwork: ignoring image outside the libraries", "path", abs)
		return nil, ErrNotFound
	}
	fi, err := os.Stat(abs)
	if err != nil || !fi.Mode().IsRegular() || fi.Size() == 0 || fi.Size() > maxSourceBytes {
		return nil, ErrNotFound
	}
	return &source{
		key:     "file\x00" + abs + "\x00" + strconv.FormatInt(fi.ModTime().UnixNano(), 10) + "\x00" + strconv.FormatInt(fi.Size(), 10),
		modTime: fi.ModTime(),
		load: func() ([]byte, error) {
			data, err := os.ReadFile(abs)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return nil, ErrNotFound
				}
				return nil, fmt.Errorf("reading %s: %w", abs, err)
			}
			return data, nil
		},
	}, nil
}

// insideLibrary reports whether abs lies within one of the configured library roots.
func (s *Service) insideLibrary(ctx context.Context, abs string) bool {
	libs, err := s.st.ListLibraries(ctx)
	if err != nil {
		return false
	}
	for _, l := range libs {
		if util.IsWithin(l.Path, abs) {
			return true
		}
	}
	return false
}

// embeddedSource returns a source for the embedded picture of t.
func (s *Service) embeddedSource(ctx context.Context, t *model.Track) (*source, error) {
	lib, err := s.st.GetLibrary(ctx, t.LibraryID)
	if err != nil {
		return nil, lookupErr(err)
	}
	abs, err := util.SafeJoin(lib.Path, t.Path)
	if err != nil {
		return nil, ErrNotFound
	}
	return &source{
		key:     "embedded\x00" + t.ID + "\x00" + strconv.FormatInt(t.UpdatedAt, 10) + "\x00" + strconv.FormatInt(t.Mtime, 10),
		modTime: time.UnixMilli(t.Mtime),
		load: func() ([]byte, error) {
			data, err := tags.ReadPicture(abs)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return nil, ErrNotFound
				}
				return nil, err
			}
			if len(data) == 0 {
				return nil, ErrNotFound
			}
			return data, nil
		},
	}, nil
}

// cached returns the resized image (or mosaic) for srcs at size, generating it at most
// once concurrently and storing it in the disk cache.
func (s *Service) cached(srcs []*source, size int) (*Image, error) {
	h := sha1.New()
	for _, src := range srcs {
		h.Write([]byte(src.key))
		h.Write([]byte{0xff})
	}
	_, _ = fmt.Fprintf(h, "size=%d;v2", size)
	key := hex.EncodeToString(h.Sum(nil))
	modTime := srcs[0].modTime
	for _, src := range srcs[1:] {
		if src.modTime.After(modTime) {
			modTime = src.modTime
		}
	}

	if img := s.readCache(key, modTime); img != nil {
		return img, nil
	}
	v, err, _ := s.group.Do(key, func() (any, error) {
		if img := s.readCache(key, modTime); img != nil {
			return img, nil
		}
		var data []byte
		var err error
		if len(srcs) == 4 {
			data, err = mosaic(srcs, size)
		} else {
			var orig []byte
			if orig, err = srcs[0].load(); err == nil {
				data, err = resize(orig, size)
			}
		}
		if err != nil {
			return nil, err
		}
		s.writeCache(key, data)
		return &Image{Data: data, ContentType: sniff(data), ModTime: modTime}, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*Image), nil
}

// sniff returns the image MIME type of data.
func sniff(data []byte) string {
	ct := http.DetectContentType(data)
	switch ct {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/bmp":
		return ct
	}
	if bytes.HasPrefix(data, []byte("RIFF")) && len(data) > 12 && string(data[8:12]) == "WEBP" {
		return "image/webp"
	}
	return "application/octet-stream"
}

// FindFolderImage returns the absolute path of the first image in dir matching patterns
// (by pattern priority, case-insensitive; jpg/jpeg/png/webp), or "".
func FindFolderImage(dir string, patterns []string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		switch {
		case e.Type().IsRegular():
		case e.Type()&os.ModeSymlink != 0:
			fi, err := os.Stat(filepath.Join(dir, e.Name()))
			if err != nil || !fi.Mode().IsRegular() {
				continue
			}
		default:
			continue
		}
		names = append(names, e.Name())
	}
	if name := MatchFolderImage(names, patterns); name != "" {
		return filepath.Join(dir, name)
	}
	return ""
}
