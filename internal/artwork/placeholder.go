package artwork

import (
	"image"
	"image/color"
	"math"
	"time"

	"golang.org/x/image/vector"
)

// Placeholder gradient colours (calm, neutral blue-grey) and glyph colour.
var (
	placeholderFrom  = [3]float64{0xEE, 0xF1, 0xF6}
	placeholderTo    = [3]float64{0xC8, 0xCF, 0xDD}
	placeholderGlyph = color.NRGBA{0xFF, 0xFF, 0xFF, 0xE6}
)

// Placeholder returns a neutral placeholder image (soft diagonal gradient with a music-note
// glyph, JPEG) for clients that cannot handle a missing cover. size ≤ 0 means 512; sizes
// are clamped to [16, 1024]. Never nil; images are generated once per size.
func (s *Service) Placeholder(size int) *Image {
	if size <= 0 {
		size = 512
	}
	size = min(max(size, MinSize), 1024)
	if v, ok := s.placeholders.Load(size); ok {
		return v.(*Image)
	}
	img := renderPlaceholder(size)
	data, err := encodeJPEG(img)
	if err != nil {
		// Encoding an in-memory RGBA image cannot realistically fail; keep the contract.
		data = nil
	}
	v, _ := s.placeholders.LoadOrStore(size, &Image{Data: data, ContentType: "image/jpeg", ModTime: time.Unix(0, 0).UTC()})
	return v.(*Image)
}

// renderPlaceholder draws the gradient and the note glyph.
func renderPlaceholder(size int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	n := float64(2*size - 2)
	if n <= 0 {
		n = 1
	}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			t := float64(x+y) / n
			i := dst.PixOffset(x, y)
			for c := 0; c < 3; c++ {
				dst.Pix[i+c] = uint8(math.Round(placeholderFrom[c] + (placeholderTo[c]-placeholderFrom[c])*t))
			}
			dst.Pix[i+3] = 0xFF
		}
	}

	f := float32(size)
	z := vector.NewRasterizer(size, size)
	// Note head: an ellipse tilted by -20°, approximated by a 48-gon.
	cx, cy, rx, ry := 0.44*f, 0.665*f, 0.105*f, 0.078*f
	rot := -20 * math.Pi / 180
	const seg = 48
	for i := 0; i <= seg; i++ {
		a := 2 * math.Pi * float64(i) / seg
		x, y := float64(rx)*math.Cos(a), float64(ry)*math.Sin(a)
		px := cx + float32(x*math.Cos(rot)-y*math.Sin(rot))
		py := cy + float32(x*math.Sin(rot)+y*math.Cos(rot))
		if i == 0 {
			z.MoveTo(px, py)
		} else {
			z.LineTo(px, py)
		}
	}
	z.ClosePath()
	// Stem.
	sx0, sx1 := 0.508*f, 0.542*f
	z.MoveTo(sx0, 0.27*f)
	z.LineTo(sx1, 0.27*f)
	z.LineTo(sx1, 0.645*f)
	z.LineTo(sx0, 0.66*f)
	z.ClosePath()
	// Flag.
	z.MoveTo(sx1-0.002*f, 0.27*f)
	z.CubeTo(0.56*f, 0.34*f, 0.66*f, 0.36*f, 0.64*f, 0.49*f)
	z.CubeTo(0.625*f, 0.415*f, 0.585*f, 0.395*f, sx1-0.002*f, 0.385*f)
	z.ClosePath()
	z.Draw(dst, dst.Bounds(), image.NewUniform(placeholderGlyph), image.Point{})
	return dst
}
