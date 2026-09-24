package artwork

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"path"
	"sort"
	"strings"

	_ "image/gif" // registered decoders
	_ "image/png"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"rainy/internal/util"
)

// maxDecodePixels guards against decompression bombs.
const maxDecodePixels = 64_000_000

// errBadImage marks source bytes that are not a decodable image (corrupt cover file or
// embedded picture); Get then tries the next candidate and finally reports ErrNotFound.
var errBadImage = errors.New("not a decodable image")

// MatchFolderImage returns the first file name matching patterns (glob patterns such as
// "cover.*" in priority order, case-insensitive) that has an image suffix Rainy serves
// (jpg, jpeg, png, webp), or "". Within one pattern names are tried alphabetically.
func MatchFolderImage(names []string, patterns []string) string {
	sorted := make([]string, 0, len(names))
	for _, n := range names {
		if util.IsImageSuffix(path.Ext(n)) && !strings.HasPrefix(n, ".") {
			sorted = append(sorted, n)
		}
	}
	if len(sorted) == 0 {
		return ""
	}
	sort.Strings(sorted)
	for _, p := range patterns {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		for _, n := range sorted {
			if ok, err := path.Match(p, strings.ToLower(n)); err == nil && ok {
				return n
			}
		}
	}
	return ""
}

// decode decodes an image, refusing absurd dimensions.
func decode(data []byte) (image.Image, string, error) {
	if err := checkImage(data); err != nil {
		return nil, "", err
	}
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", errBadImage, err)
	}
	return img, format, nil
}

// checkImage validates the header of an image (cheap: no pixel decoding).
func checkImage(data []byte) error {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("%w: %v", errBadImage, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxDecodePixels {
		return fmt.Errorf("%w: unsupported dimensions %dx%d", errBadImage, cfg.Width, cfg.Height)
	}
	return nil
}

// resize fits data within size×size and encodes it as JPEG. JPEG/PNG images that are
// already small enough are returned unchanged (no generation loss).
func resize(data []byte, size int) ([]byte, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err == nil && max(cfg.Width, cfg.Height) <= size && (format == "jpeg" || format == "png") {
		return data, nil
	}
	img, _, err := decode(data)
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > size || h > size {
		if w >= h {
			w, h = size, max(1, (h*size+w/2)/w)
		} else {
			w, h = max(1, (w*size+h/2)/h), size
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src) // flatten alpha on white
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
	return encodeJPEG(dst)
}

// mosaic renders four covers as a 2×2 grid of size×size (each cover centre-cropped to a
// square). Covers that cannot be loaded become neutral tiles.
func mosaic(srcs []*source, size int) ([]byte, error) {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.RGBA{0xd9, 0xdc, 0xe3, 0xff}), image.Point{}, draw.Src)
	half := size / 2
	tiles := []image.Rectangle{
		image.Rect(0, 0, half, half), image.Rect(half, 0, size, half),
		image.Rect(0, half, half, size), image.Rect(half, half, size, size),
	}
	loaded := 0
	for i, src := range srcs {
		if i >= len(tiles) {
			break
		}
		img, err := decodeSource(src)
		if err != nil {
			continue
		}
		draw.CatmullRom.Scale(dst, tiles[i], img, squareCrop(img.Bounds()), draw.Over, nil)
		loaded++
	}
	if loaded == 0 {
		return nil, ErrNotFound
	}
	return encodeJPEG(dst)
}

// decodeSource decodes the image of src, moving on to its fallback candidates while a
// candidate is unusable.
func decodeSource(src *source) (image.Image, error) {
	for {
		data, err := src.load()
		if err == nil {
			var img image.Image
			if img, _, err = decode(data); err == nil {
				return img, nil
			}
		}
		if src.fallback == nil || !unusable(err) {
			return nil, err
		}
		if src, err = src.fallback(); err != nil {
			return nil, err
		}
	}
}

// squareCrop returns the centred square of r.
func squareCrop(r image.Rectangle) image.Rectangle {
	w, h := r.Dx(), r.Dy()
	if w > h {
		off := (w - h) / 2
		return image.Rect(r.Min.X+off, r.Min.Y, r.Min.X+off+h, r.Max.Y)
	}
	off := (h - w) / 2
	return image.Rect(r.Min.X, r.Min.Y+off, r.Max.X, r.Min.Y+off+w)
}

func encodeJPEG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, fmt.Errorf("encoding JPEG: %w", err)
	}
	if buf.Len() == 0 {
		return nil, errors.New("encoding JPEG: empty output")
	}
	return buf.Bytes(), nil
}
