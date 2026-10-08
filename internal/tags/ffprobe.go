package tags

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

// FFmpeg fallback for files TagLib cannot parse. The TagLib WASM build handles invalid
// UTF-8 without throwing, so a legacy WAV INFO chunk does not prevent reading its clean
// ID3 tags. Other unsupported files can still be read with ffprobe / ffmpeg instead.
// Writing always requires TagLib.

var (
	ffprobePath atomic.Value // string; "" disables the fallback
	ffmpegPath  atomic.Value // string
)

// SetFFmpeg enables the ffprobe/ffmpeg fallback for files TagLib cannot read. ffmpeg is the
// configured ffmpeg binary (name or path); ffprobe is looked up next to it. Missing binaries
// simply disable the fallback.
func SetFFmpeg(ffmpeg string) {
	probe := ""
	if ffmpeg != "" {
		dir, base := filepath.Split(ffmpeg)
		name := strings.Replace(base, "ffmpeg", "ffprobe", 1)
		if name == base {
			name = "ffprobe"
		}
		if p, err := exec.LookPath(filepath.Join(dir, name)); err == nil {
			probe = p
		}
		if p, err := exec.LookPath(ffmpeg); err == nil {
			ffmpeg = p
		} else {
			ffmpeg = ""
		}
	}
	ffprobePath.Store(probe)
	ffmpegPath.Store(ffmpeg)
}

func loadPath(v *atomic.Value) string {
	s, _ := v.Load().(string)
	return s
}

const probeTimeout = 30 * time.Second

// probeResult is the subset of `ffprobe -show_format -show_streams` output used here.
type probeResult struct {
	Streams []struct {
		CodecType        string          `json:"codec_type"`
		CodecName        string          `json:"codec_name"`
		SampleRate       string          `json:"sample_rate"`
		Channels         int             `json:"channels"`
		BitsPerSample    int             `json:"bits_per_sample"`
		BitsPerRawSample string          `json:"bits_per_raw_sample"`
		Disposition      map[string]int  `json:"disposition"`
		Tags             json.RawMessage `json:"tags"`
	} `json:"streams"`
	Format struct {
		FormatName string          `json:"format_name"`
		Duration   string          `json:"duration"`
		BitRate    string          `json:"bit_rate"`
		Tags       json.RawMessage `json:"tags"`
	} `json:"format"`
}

// probe runs ffprobe on path and returns its tags (TagLib key names) and properties.
func probe(path string) (map[string][]string, *Properties, error) {
	bin := loadPath(&ffprobePath)
	if bin == "" {
		return nil, nil, errors.New("ffprobe fallback not available")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	var out, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "-v", "error", "-print_format", "json",
		"-show_format", "-show_streams", "-i", "file:"+filepath.ToSlash(abs))
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		return nil, nil, fmt.Errorf("ffprobe %s: %w: %s", filepath.Base(path), err, strings.TrimSpace(stderr.String()))
	}
	return parseProbe(out.Bytes())
}

// parseProbe converts ffprobe JSON into TagLib-style tags and Properties.
func parseProbe(data []byte) (map[string][]string, *Properties, error) {
	var pr probeResult
	if err := json.Unmarshal(data, &pr); err != nil {
		return nil, nil, fmt.Errorf("parsing ffprobe output: %w", err)
	}
	props := &Properties{Pictures: []Picture{}}
	var tagSets []json.RawMessage
	tagSets = append(tagSets, pr.Format.Tags)
	audio := false
	for _, s := range pr.Streams {
		switch {
		case s.Disposition["attached_pic"] == 1:
			props.Pictures = append(props.Pictures, Picture{Type: "Front Cover", MimeType: pictureMime(s.CodecName)})
		case s.CodecType == "audio" && !audio:
			audio = true
			props.InnerCodec = s.CodecName
			props.SampleRate, _ = strconv.Atoi(s.SampleRate)
			props.Channels = s.Channels
			props.BitDepth = s.BitsPerSample
			if props.BitDepth == 0 {
				props.BitDepth, _ = strconv.Atoi(s.BitsPerRawSample)
			}
			tagSets = append(tagSets, s.Tags)
		}
	}
	if !audio {
		return nil, nil, fmt.Errorf("ffprobe: no audio stream: %w", ErrUnsupported)
	}
	props.Format = strings.SplitN(pr.Format.FormatName, ",", 2)[0]
	if strings.HasPrefix(props.InnerCodec, "pcm") {
		props.InnerCodec = "pcm"
	}
	if props.Format == props.InnerCodec || props.Format == "mp3" || props.Format == "flac" {
		props.Codec = props.Format
	} else {
		props.Codec = codecName(props.Format, props.InnerCodec)
	}
	if strings.Contains(props.InnerCodec, "mp3") || strings.Contains(props.InnerCodec, "aac") ||
		strings.Contains(props.InnerCodec, "vorbis") || strings.Contains(props.InnerCodec, "opus") {
		props.BitDepth = 0 // lossy: bit depth is meaningless
	}
	if d, err := strconv.ParseFloat(pr.Format.Duration, 64); err == nil {
		props.Duration = d
	}
	if br, err := strconv.Atoi(pr.Format.BitRate); err == nil {
		props.Bitrate = br / 1000
	}

	collected := map[string][]string{}
	var order []string
	for _, set := range tagSets {
		if err := collectTags(set, func(k, v string) {
			k = probeKey(k)
			if k == "" {
				return
			}
			if _, ok := collected[k]; !ok {
				order = append(order, k)
			}
			collected[k] = append(collected[k], v)
		}); err != nil {
			return nil, nil, err
		}
	}
	raw := make(map[string][]string, len(collected))
	for _, k := range order {
		if v, ok := bestValue(collected[k]); ok {
			raw[k] = []string{v}
		}
	}
	return raw, props, nil
}

// collectTags walks a JSON object in document order and reports every key/value pair —
// including duplicate keys, which ffprobe emits when a file has several tag containers.
func collectTags(obj json.RawMessage, fn func(k, v string)) error {
	if len(obj) == 0 || string(obj) == "null" {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(obj))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return fmt.Errorf("parsing ffprobe tags: expected object")
	}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return fmt.Errorf("parsing ffprobe tags: %w", err)
		}
		var v any
		if err := dec.Decode(&v); err != nil {
			return fmt.Errorf("parsing ffprobe tags: %w", err)
		}
		k, _ := kt.(string)
		switch v := v.(type) {
		case string:
			fn(k, v)
		case float64:
			fn(k, strconv.FormatFloat(v, 'f', -1, 64))
		}
	}
	if _, err := dec.Token(); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("parsing ffprobe tags: %w", err)
	}
	return nil
}

// probeKeys maps ffmpeg's generic metadata keys to TagLib property names.
var probeKeys = map[string]string{
	"album_artist":   "ALBUMARTIST",
	"albumartist":    "ALBUMARTIST",
	"track":          "TRACKNUMBER",
	"tracknumber":    "TRACKNUMBER",
	"disc":           "DISCNUMBER",
	"discnumber":     "DISCNUMBER",
	"tbpm":           "BPM",
	"tcmp":           "COMPILATION",
	"unsyncedlyrics": "LYRICS",
	"icmt":           "COMMENT",
	"iprd":           "ALBUM",
	"inam":           "TITLE",
	"iart":           "ARTIST",
	"ignr":           "GENRE",
	"icrd":           "DATE",
	"itrk":           "TRACKNUMBER",
}

func probeKey(k string) string {
	lk := strings.ToLower(strings.TrimSpace(k))
	switch {
	case lk == "" || lk == "encoder" || lk == "encoded_by" || lk == "creation_time":
		return ""
	case lk == "lyrics" || strings.HasPrefix(lk, "lyrics-") || strings.HasPrefix(lk, "lyrics_"):
		return "LYRICS"
	}
	if m, ok := probeKeys[lk]; ok {
		return m
	}
	return strings.ToUpper(lk)
}

// bestValue picks the first value that decoded cleanly; values containing U+FFFD come from
// legacy-encoded containers ffmpeg could not decode and are only used as a last resort.
func bestValue(vs []string) (string, bool) {
	fallback := ""
	for _, v := range vs {
		v = strings.ReplaceAll(v, "\x00", "")
		if strings.TrimSpace(v) == "" {
			continue
		}
		if strings.ContainsRune(v, utf8.RuneError) {
			if fallback == "" {
				fallback = v
			}
			continue
		}
		return v, true
	}
	return fallback, fallback != ""
}

func pictureMime(codec string) string {
	switch codec {
	case "mjpeg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "webp":
		return "image/webp"
	case "gif":
		return "image/gif"
	case "bmp":
		return "image/bmp"
	}
	return ""
}

// probePicture extracts the first attached picture with ffmpeg (stream copy).
func probePicture(path string) ([]byte, error) {
	bin := loadPath(&ffmpegPath)
	if bin == "" {
		return nil, errors.New("ffmpeg fallback not available")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	var out, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "-v", "error", "-nostdin", "-i", "file:"+filepath.ToSlash(abs),
		"-map", "0:v:0", "-frames:v", "1", "-c", "copy", "-f", "image2pipe", "-")
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg picture %s: %w: %s", filepath.Base(path), err, strings.TrimSpace(stderr.String()))
	}
	if out.Len() == 0 {
		return nil, nil
	}
	return out.Bytes(), nil
}
