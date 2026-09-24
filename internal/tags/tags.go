// Package tags wraps go.senan.xyz/taglib: reading and writing tags, audio properties and
// embedded pictures, plus mojibake repair for legacy CJK encodings.
//
// Tag keys are TagLib property names, upper-case (TITLE, ARTIST, ALBUMARTIST, TRACKNUMBER,
// DISCNUMBER, DATE, GENRE, LYRICS, REPLAYGAIN_TRACK_GAIN, …). Every call instantiates a
// fresh TagLib WASM module, so the functions are safe for concurrent use.
//
// docs/architecture/contract.md §5.7.
package tags

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	_ "image/gif" // register decoders used when converting pictures to JPEG
	_ "image/png"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"

	"go.senan.xyz/taglib"
)

var (
	// ErrUnsupported is returned (wrapped) for files TagLib cannot parse (unknown format or
	// corrupt file).
	ErrUnsupported = errors.New("unsupported or corrupt audio file")
	// ErrWriteFailed is returned (wrapped) when TagLib could not save the file (read-only
	// file or file system, or a format that cannot store the given tags).
	ErrWriteFailed = errors.New("could not save tags")
	// ErrInvalidKey is returned (wrapped) by Write for empty or malformed tag keys.
	ErrInvalidKey = errors.New("invalid tag key")
	// ErrInvalidImage is returned (wrapped) by WritePicture for data that is not an image.
	ErrInvalidImage = errors.New("invalid image")
)

// AudioExtensions lists the audio file extensions (lower-case, without dot) the scanner
// considers.
var AudioExtensions = map[string]bool{
	"mp3": true, "flac": true, "m4a": true, "m4b": true, "mp4": true, "aac": true, "alac": true,
	"ogg": true, "oga": true, "opus": true, "wav": true, "aif": true, "aiff": true, "wma": true,
	"ape": true, "wv": true, "mpc": true, "dsf": true, "dff": true, "tta": true, "spx": true,
}

// IsAudioFile reports whether name has an audio extension.
func IsAudioFile(name string) bool {
	return AudioExtensions[strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))]
}

// Metadata is everything read from one audio file.
type Metadata struct {
	Title, Album, Artist, AlbumArtist string

	TrackNumber, TrackTotal, DiscNumber, DiscTotal int
	DiscSubtitle                                   string

	Year         int
	Date         string
	OriginalYear int

	Genres                    []string
	Composer, Comment, Lyrics string
	BPM                       int
	Compilation               bool

	RGTrackGain, RGTrackPeak, RGAlbumGain, RGAlbumPeak *float64

	MbzTrackID, MbzAlbumID, MbzArtistID, MbzAlbumArtistID string
	SortTitle, SortAlbum, SortArtist, SortAlbumArtist     string

	Duration                                float64
	Bitrate, SampleRate, BitDepth, Channels int
	Codec                                   string
	HasPicture                              bool
	Raw                                     map[string][]string // all tags as returned by taglib (keys upper-case)
}

// ReadOptions controls Read.
type ReadOptions struct {
	FixEncoding bool // repair mojibake in text fields (not Raw)
}

// Picture describes one embedded picture (without its data).
type Picture struct {
	Type        string `json:"type"`        // e.g. "Front Cover"
	Description string `json:"description"` //
	MimeType    string `json:"mimeType"`    // e.g. "image/jpeg"
}

// Properties are the audio properties of a file.
type Properties struct {
	Format     string    // TagLib format: mpeg, flac, mp4, ogg, wav, aiff, asf, ape, wavpack, …
	InnerCodec string    // codec inside a container (aac, alac, vorbis, opus, pcm, …) or ""
	Codec      string    // display codec: "mp3", "flac", "mp4/aac", "ogg/opus", …
	Duration   float64   // seconds
	Bitrate    int       // kbit/s
	SampleRate int       // Hz
	BitDepth   int       // 0 for lossy formats
	Channels   int       //
	Pictures   []Picture // embedded pictures in file order
}

// checkFile returns a descriptive error when path is not a readable regular file, so
// callers can distinguish "gone" (fs.ErrNotExist) from "unparseable".
func checkFile(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("%s: not a regular file: %w", path, fs.ErrInvalid)
	}
	return nil
}

func wrapTaglib(op, path string, err error) error {
	if errors.Is(err, taglib.ErrInvalidFile) {
		return fmt.Errorf("%s %s: %w", op, filepath.Base(path), ErrUnsupported)
	}
	if errors.Is(err, taglib.ErrSavingFile) {
		return fmt.Errorf("%s %s: %w", op, filepath.Base(path), ErrWriteFailed)
	}
	return fmt.Errorf("%s %s: %w", op, filepath.Base(path), err)
}

// Read returns tags and audio properties of path (no fallbacks applied). Files TagLib
// cannot handle are read with ffprobe when SetFFmpeg enabled that fallback.
func Read(path string, opt ReadOptions) (*Metadata, error) {
	raw, err := ReadRaw(path)
	if err != nil {
		return nil, err
	}
	props, err := ReadProperties(path)
	if err != nil {
		return nil, err
	}
	m := parse(raw)
	m.Duration = props.Duration
	m.Bitrate = props.Bitrate
	m.SampleRate = props.SampleRate
	m.BitDepth = props.BitDepth
	m.Channels = props.Channels
	m.Codec = props.Codec
	m.HasPicture = len(props.Pictures) > 0
	if opt.FixEncoding {
		m.fixEncoding()
	}
	return m, nil
}

// ReadRaw returns all tags of path (keys upper-case, values in file order; keys with
// several spellings are merged).
func ReadRaw(path string) (map[string][]string, error) {
	if err := checkFile(path); err != nil {
		return nil, err
	}
	t, err := taglib.ReadTags(path)
	if err != nil {
		if raw, _, perr := probe(path); perr == nil {
			return raw, nil
		}
		return nil, wrapTaglib("reading tags of", path, err)
	}
	out := make(map[string][]string, len(t))
	keys := make([]string, 0, len(t))
	for k := range t {
		keys = append(keys, k)
	}
	sort.Strings(keys) // deterministic merge order
	for _, k := range keys {
		uk := strings.ToUpper(strings.TrimSpace(k))
		if uk == "" {
			continue
		}
		for _, v := range t[k] {
			out[uk] = append(out[uk], strings.ReplaceAll(v, "\x00", ""))
		}
	}
	return out, nil
}

// ReadProperties returns the audio properties and embedded picture descriptions of path.
func ReadProperties(path string) (*Properties, error) {
	if err := checkFile(path); err != nil {
		return nil, err
	}
	p, err := taglib.ReadProperties(path)
	if err != nil {
		if _, props, perr := probe(path); perr == nil {
			return props, nil
		}
		return nil, wrapTaglib("reading properties of", path, err)
	}
	// TagLib happily "opens" arbitrary bytes with an audio extension but finds no stream.
	if p.SampleRate == 0 && p.Channels == 0 && p.Length == 0 {
		return nil, fmt.Errorf("reading properties of %s: no audio stream: %w", filepath.Base(path), ErrUnsupported)
	}
	out := &Properties{
		Format:     p.Format,
		InnerCodec: p.InnerCodec,
		Codec:      codecName(p.Format, p.InnerCodec),
		Duration:   p.Length.Seconds(),
		Bitrate:    int(p.BitRate),
		SampleRate: int(p.SampleRate),
		BitDepth:   int(p.BitDepth),
		Channels:   int(p.Channels),
		Pictures:   make([]Picture, 0, len(p.Images)),
	}
	for _, im := range p.Images {
		out.Pictures = append(out.Pictures, Picture{Type: im.Type, Description: im.Description, MimeType: im.MIMEType})
	}
	return out, nil
}

// codecName renders a compact codec label from TagLib's format and inner codec.
func codecName(format, inner string) string {
	switch format {
	case "":
		return inner
	case "mpeg":
		format = "mp3"
	}
	if inner == "" {
		return format
	}
	return format + "/" + inner
}

// Write merges changes into the file's tags: listed keys are replaced, an empty slice (or
// one holding only empty strings) deletes the key, other keys are untouched. Keys are
// upper-cased.
func Write(path string, changes map[string][]string) error {
	if len(changes) == 0 {
		return nil
	}
	if err := checkFile(path); err != nil {
		return err
	}
	clean := make(map[string][]string, len(changes))
	for k, vs := range changes {
		key := strings.ToUpper(strings.TrimSpace(k))
		if !validKey(key) {
			return fmt.Errorf("%w: %q", ErrInvalidKey, k)
		}
		clean[key] = append(clean[key], cleanValues(vs)...)
	}
	if err := taglib.WriteTags(path, clean, 0); err != nil {
		return wrapTaglib("writing tags to", path, err)
	}
	return nil
}

// ReadPicture returns the front cover (or, when none is marked as such, the first embedded
// picture); nil if the file has no picture.
func ReadPicture(path string) ([]byte, error) {
	props, err := ReadProperties(path)
	if err != nil {
		return nil, err
	}
	if len(props.Pictures) == 0 {
		return nil, nil
	}
	idx := 0
	for i, p := range props.Pictures {
		if strings.EqualFold(p.Type, "Front Cover") {
			idx = i
			break
		}
	}
	img, err := taglib.ReadImageOptions(path, idx)
	if err != nil {
		if pic, perr := probePicture(path); perr == nil {
			return pic, nil
		}
		return nil, wrapTaglib("reading picture of", path, err)
	}
	if len(img) == 0 {
		return nil, nil
	}
	return img, nil
}

// WritePicture sets the front cover (index 0); nil removes all pictures. JPEG and PNG are
// embedded as-is; other decodable formats (WebP, GIF, BMP) are converted to JPEG because
// most players cannot display them.
//
// The text tags are preserved: in formats with two tag containers (WAV: RIFF INFO and
// ID3v2) embedding a picture creates an ID3v2 tag holding only the picture, which TagLib
// then prefers when reading, so the text tags read before are written back into it.
func WritePicture(path string, img []byte) error {
	if err := checkFile(path); err != nil {
		return err
	}
	before, err := ReadRaw(path)
	if err != nil {
		return err
	}
	if len(img) == 0 {
		props, err := ReadProperties(path)
		if err != nil {
			return err
		}
		for range props.Pictures {
			if err := taglib.WriteImageOptions(path, nil, 0, "", "", ""); err != nil {
				return wrapTaglib("removing picture from", path, err)
			}
		}
		return restoreTags(path, before)
	}
	data, mime, err := embeddable(img)
	if err != nil {
		return err
	}
	if err := taglib.WriteImageOptions(path, data, 0, "Front Cover", "", mime); err != nil {
		return wrapTaglib("writing picture to", path, err)
	}
	return restoreTags(path, before)
}

// restoreTags writes back the tags of before that path no longer reports (merge: other
// tags are untouched) and verifies the result.
func restoreTags(path string, before map[string][]string) error {
	lost := func() (map[string][]string, error) {
		after, err := ReadRaw(path)
		if err != nil {
			return nil, err
		}
		out := map[string][]string{}
		for k, vs := range before {
			if want := cleanValues(vs); validKey(k) && len(want) > 0 && !slices.Equal(want, cleanValues(after[k])) {
				out[k] = vs
			}
		}
		return out, nil
	}
	missing, err := lost()
	if err != nil || len(missing) == 0 {
		return err
	}
	if err := Write(path, missing); err != nil {
		return fmt.Errorf("restoring tags after changing the picture: %w", err)
	}
	if missing, err = lost(); err != nil {
		return err
	}
	if len(missing) > 0 {
		keys := make([]string, 0, len(missing))
		for k := range missing {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return fmt.Errorf("writing picture to %s: tags %v could not be preserved: %w", filepath.Base(path), keys, ErrWriteFailed)
	}
	return nil
}

// cleanValues drops blank values and the characters the TagLib bridge cannot carry (\v
// separates values, \x00 terminates strings).
func cleanValues(vs []string) []string {
	out := make([]string, 0, len(vs))
	for _, v := range vs {
		v = strings.NewReplacer("\v", " ", "\x00", "").Replace(v)
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}

func validKey(k string) bool {
	return k != "" && !strings.ContainsAny(k, "\t\v\n\r\x00=")
}

// embeddable returns img as JPEG or PNG together with its MIME type.
func embeddable(img []byte) ([]byte, string, error) {
	switch {
	case bytes.HasPrefix(img, []byte("\xFF\xD8\xFF")):
		return img, "image/jpeg", nil
	case bytes.HasPrefix(img, []byte("\x89PNG\r\n\x1a\n")):
		return img, "image/png", nil
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(img))
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrInvalidImage, err)
	}
	if cfg.Width*cfg.Height > 100_000_000 {
		return nil, "", fmt.Errorf("%w: %dx%d is too large", ErrInvalidImage, cfg.Width, cfg.Height)
	}
	decoded, _, err := image.Decode(bytes.NewReader(img))
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrInvalidImage, err)
	}
	// Flatten transparency onto white (JPEG has no alpha channel).
	flat := image.NewRGBA(decoded.Bounds())
	draw.Draw(flat, flat.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(flat, flat.Bounds(), decoded, decoded.Bounds().Min, draw.Over)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, flat, &jpeg.Options{Quality: 92}); err != nil {
		return nil, "", fmt.Errorf("converting picture to JPEG: %w", err)
	}
	return buf.Bytes(), "image/jpeg", nil
}
