package artwork

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPruneKeepsCacheBounded(t *testing.T) {
	dir := t.TempDir()
	s := New(nil, dir)
	s.maxCache = 10_000
	data := bytes.Repeat([]byte{0xAB}, 1000)
	var first string
	for i := 0; i < 30; i++ {
		sum := sha1.Sum([]byte(fmt.Sprint(i)))
		key := hex.EncodeToString(sum[:])
		if i == 0 {
			first = key
		}
		s.writeCache(key, data)
		// Make earlier entries older so they are evicted first.
		old := time.Now().Add(time.Duration(i-100) * time.Minute)
		_ = os.Chtimes(s.cachePath(key), old, old)
		s.pruneWG.Wait()
	}
	s.startPrune()
	s.pruneWG.Wait()
	size, err := s.CacheSize()
	if err != nil {
		t.Fatal(err)
	}
	if size > s.maxCache {
		t.Fatalf("cache size %d exceeds %d", size, s.maxCache)
	}
	if _, err := os.Stat(s.cachePath(first)); !os.IsNotExist(err) {
		t.Fatal("oldest entry not evicted")
	}
	if got := s.cacheBytes.Load(); got != size {
		t.Fatalf("tracked size %d != actual %d", got, size)
	}
	// Stale temp files are removed.
	tmp := filepath.Join(dir, "ab", ".tmp-123")
	_ = os.MkdirAll(filepath.Dir(tmp), 0o755)
	_ = os.WriteFile(tmp, []byte("x"), 0o644)
	old := time.Now().Add(-2 * time.Hour)
	_ = os.Chtimes(tmp, old, old)
	if err := s.prune(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Fatal("stale temp file kept")
	}
}

func TestResize(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1000, 500))
	for y := 0; y < 500; y++ {
		for x := 0; x < 1000; x++ {
			src.Set(x, y, color.RGBA{uint8(x), uint8(y), 100, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	out, err := resize(buf.Bytes(), 200)
	if err != nil {
		t.Fatal(err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil || format != "jpeg" || cfg.Width != 200 || cfg.Height != 100 {
		t.Fatalf("resize: %v %s %dx%d", err, format, cfg.Width, cfg.Height)
	}
	if _, err := resize([]byte("not an image"), 100); err == nil {
		t.Fatal("resize accepted garbage")
	}
	if r := squareCrop(image.Rect(0, 0, 100, 60)); r != image.Rect(20, 0, 80, 60) {
		t.Fatalf("squareCrop: %v", r)
	}
}
