package artwork

import (
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	cacheExt = ".img"
	// touchAfter: cache hits refresh the file mtime at most this often (approximate LRU
	// without a write per hit).
	touchAfter = 24 * time.Hour
	// pruneTarget is the fraction of maxCache kept after pruning (hysteresis).
	pruneTarget = 0.9
	// staleTemp is the age after which leftover temp files are removed by pruning.
	staleTemp = time.Hour
)

func (s *Service) cachePath(key string) string {
	return filepath.Join(s.cacheDir, key[:2], key+cacheExt)
}

// readCache returns a cached image or nil.
func (s *Service) readCache(key string, modTime time.Time) *Image {
	if s.cacheDir == "" {
		return nil
	}
	p := s.cachePath(key)
	data, err := os.ReadFile(p)
	if err != nil || len(data) == 0 {
		return nil
	}
	if fi, err := os.Stat(p); err == nil && time.Since(fi.ModTime()) > touchAfter {
		now := time.Now()
		_ = os.Chtimes(p, now, now)
	}
	return &Image{Data: data, ContentType: sniff(data), ModTime: modTime}
}

// writeCache stores data atomically (temp file + rename) and schedules pruning when the
// cache grows beyond its limit. Failures are logged, never fatal.
func (s *Service) writeCache(key string, data []byte) {
	if s.cacheDir == "" {
		return
	}
	p := s.cachePath(key)
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.Warn("artwork cache: creating directory", "dir", dir, "err", err)
		return
	}
	f, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		slog.Warn("artwork cache: creating temp file", "err", err)
		return
	}
	tmp := f.Name()
	_, werr := f.Write(data)
	cerr := f.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(tmp)
		slog.Warn("artwork cache: writing", "err", errors.Join(werr, cerr))
		return
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		slog.Warn("artwork cache: renaming", "err", err)
		return
	}
	cur := s.cacheBytes.Load()
	if cur < 0 {
		s.startPrune() // first write since start: measure (and trim) in the background
		return
	}
	if s.cacheBytes.Add(int64(len(data))) > s.maxCache {
		s.startPrune()
	}
}

// startPrune runs prune in the background unless one is already running.
func (s *Service) startPrune() {
	if !s.pruning.CompareAndSwap(false, true) {
		return
	}
	s.pruneWG.Add(1)
	go func() {
		defer s.pruneWG.Done()
		defer s.pruning.Store(false)
		if err := s.prune(); err != nil {
			slog.Warn("artwork cache: pruning", "err", err)
		}
	}()
}

type cacheFile struct {
	path    string
	size    int64
	modTime time.Time
}

// prune measures the cache and, when it exceeds maxCache, deletes the least recently
// used files until it is below pruneTarget×maxCache. Stale temp files are removed.
func (s *Service) prune() error {
	var files []cacheFile
	var total int64
	err := filepath.WalkDir(s.cacheDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".tmp-") {
			if time.Since(fi.ModTime()) > staleTemp {
				_ = os.Remove(p)
			}
			return nil
		}
		files = append(files, cacheFile{p, fi.Size(), fi.ModTime()})
		total += fi.Size()
		return nil
	})
	if err != nil {
		return err
	}
	if total > s.maxCache {
		sort.Slice(files, func(i, j int) bool { return files[i].modTime.Before(files[j].modTime) })
		target := int64(float64(s.maxCache) * pruneTarget)
		removed := 0
		for _, f := range files {
			if total <= target {
				break
			}
			if err := os.Remove(f.path); err == nil || errors.Is(err, fs.ErrNotExist) {
				total -= f.size
				removed++
			}
		}
		slog.Info("artwork cache pruned", "removed", removed, "size", total)
	}
	s.cacheBytes.Store(total)
	return nil
}

// ClearCache deletes the resized-image cache and returns the bytes freed.
func (s *Service) ClearCache() (freed int64, err error) {
	if s.cacheDir == "" {
		return 0, nil
	}
	entries, err := os.ReadDir(s.cacheDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	var errs []error
	for _, e := range entries {
		p := filepath.Join(s.cacheDir, e.Name())
		n, _ := dirSize(p)
		if err := os.RemoveAll(p); err != nil {
			errs = append(errs, err)
			continue
		}
		freed += n
	}
	s.cacheBytes.Store(0)
	return freed, errors.Join(errs...)
}

// CacheSize returns the size of the resized-image cache in bytes.
func (s *Service) CacheSize() (int64, error) {
	if s.cacheDir == "" {
		return 0, nil
	}
	n, err := dirSize(s.cacheDir)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	return n, err
}

func dirSize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			if fi, err := d.Info(); err == nil {
				total += fi.Size()
			}
		}
		return nil
	})
	return total, err
}
