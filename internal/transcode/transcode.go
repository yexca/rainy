// Package transcode streams audio through an external ffmpeg binary and decides when a
// track needs transcoding (docs/architecture/contract.md §5.10). It is used by the native API
// (/api/stream) and the Subsonic API (/rest/stream).
package transcode

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"rainy/internal/model"
)

// Supported output formats.
const (
	FormatMP3  = "mp3"
	FormatOpus = "opus"
	FormatAAC  = "aac"
	// FormatRaw means "the original file, untouched".
	FormatRaw = "raw"
)

// Bit rate bounds (kbit/s) applied to every transcode.
const (
	MinBitRate     = 32
	DefaultBitRate = 192
)

// maxBitRate is the highest bit rate each encoder accepts.
// Opus is capped at 256 kbps: libopus rejects higher rates for mono input and
// channel counts from tags are not reliable enough to decide per file.
var maxBitRate = map[string]int{FormatMP3: 320, FormatOpus: 256, FormatAAC: 320}

// ErrUnavailable is returned by Stream when the ffmpeg binary cannot be found.
var ErrUnavailable = errors.New("transcode: ffmpeg is not available")

// ErrUnsupportedFormat is returned by Stream for an unknown output format.
var ErrUnsupportedFormat = errors.New("transcode: unsupported format")

// stderrLimit bounds how much of ffmpeg's stderr is kept for error messages.
const stderrLimit = 4 << 10

// Service runs ffmpeg. It is safe for concurrent use.
type Service struct {
	path string

	mu        sync.Mutex
	checkedAt time.Time
	resolved  string // absolute binary path, "" when not found
	version   string
}

// availabilityTTL is how long a LookPath / version probe result is cached (so installing
// ffmpeg while the server runs is picked up without a restart).
const availabilityTTL = time.Minute

// New creates the service for the ffmpeg binary at ffmpegPath (a name looked up in PATH, or
// a path). An empty path means "ffmpeg".
func New(ffmpegPath string) *Service {
	if strings.TrimSpace(ffmpegPath) == "" {
		ffmpegPath = "ffmpeg"
	}
	return &Service{path: ffmpegPath}
}

// Path returns the configured ffmpeg path.
func (s *Service) Path() string { return s.path }

// probe resolves the binary and its version, caching the result for availabilityTTL.
func (s *Service) probe() (resolved, version string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.checkedAt.IsZero() && time.Since(s.checkedAt) < availabilityTTL {
		return s.resolved, s.version
	}
	s.checkedAt = time.Now()
	s.resolved, s.version = "", ""
	p, err := exec.LookPath(s.path)
	if err != nil {
		return "", ""
	}
	s.resolved = p
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, p, "-hide_banner", "-version").Output()
	if err == nil {
		s.version = parseVersion(string(out))
	}
	return s.resolved, s.version
}

// parseVersion extracts "7.1" from "ffmpeg version 7.1 Copyright (c) …" (the first line).
func parseVersion(out string) string {
	line, _, _ := strings.Cut(out, "\n")
	fields := strings.Fields(line)
	for i, f := range fields {
		if f == "version" && i+1 < len(fields) {
			return fields[i+1]
		}
	}
	return strings.TrimSpace(line)
}

// Available reports whether the ffmpeg binary can be found.
func (s *Service) Available() bool {
	p, _ := s.probe()
	return p != ""
}

// Version returns the ffmpeg version string ("" if unavailable).
func (s *Service) Version() string {
	_, v := s.probe()
	return v
}

// Options control a transcode.
type Options struct {
	Format  string  // mp3|opus|aac
	BitRate int     // kbps; <= 0 uses DefaultBitRate; clamped to the encoder's range
	Offset  float64 // seconds to skip from the start of the input
}

// IsFormat reports whether f is a supported transcode output format.
func IsFormat(f string) bool {
	_, ok := maxBitRate[f]
	return ok
}

// ClampBitRate bounds kbps to what the encoder of format accepts (DefaultBitRate when
// kbps <= 0).
func ClampBitRate(format string, kbps int) int {
	if kbps <= 0 {
		kbps = DefaultBitRate
	}
	hi, ok := maxBitRate[format]
	if !ok {
		hi = 320
	}
	return min(max(kbps, MinBitRate), hi)
}

// Args returns the ffmpeg arguments that transcode inputPath to stdout.
func Args(inputPath string, o Options) ([]string, error) {
	if !IsFormat(o.Format) {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedFormat, o.Format)
	}
	br := strconv.Itoa(ClampBitRate(o.Format, o.BitRate)) + "k"
	args := []string{"-hide_banner", "-loglevel", "error", "-nostdin"}
	if o.Offset > 0 {
		// Before -i: fast (demuxer-level) seek.
		args = append(args, "-ss", strconv.FormatFloat(o.Offset, 'f', 3, 64))
	}
	args = append(args, "-i", inputURL(inputPath),
		"-map", "0:a:0", // first audio stream only (skip embedded cover "video" streams)
		"-vn", "-sn", "-dn",
		"-map_metadata", "-1",
	)
	switch o.Format {
	case FormatMP3:
		args = append(args, "-c:a", "libmp3lame", "-b:a", br, "-write_xing", "0", "-id3v2_version", "0", "-f", "mp3")
	case FormatOpus:
		args = append(args, "-c:a", "libopus", "-b:a", br, "-vbr", "on", "-ar", "48000", "-f", "ogg")
	case FormatAAC:
		args = append(args, "-c:a", "aac", "-b:a", br, "-f", "adts")
	}
	return append(args, "pipe:1"), nil
}

// inputURL turns a file path into an explicit ffmpeg "file:" URL (made absolute), so a
// path can never be mistaken for an option ("-x.flac") or another protocol ("concat:…",
// "http:…") by ffmpeg.
func inputURL(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	return "file:" + p
}

// Stream transcodes inputPath to w until done or ctx is cancelled (ffmpeg is killed on
// cancellation or when writing to w fails). A failing ffmpeg run returns an error that
// includes the tail of its stderr.
func (s *Service) Stream(ctx context.Context, w io.Writer, inputPath string, o Options) error {
	args, err := Args(inputPath, o)
	if err != nil {
		return err
	}
	bin, _ := s.probe()
	if bin == "" {
		return ErrUnavailable
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.WaitDelay = 5 * time.Second
	stderr := &tailBuffer{limit: stderrLimit}
	cmd.Stderr = stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("transcode: stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("transcode: starting ffmpeg: %w", err)
	}

	_, copyErr := io.Copy(w, stdout)
	if copyErr != nil {
		// The client went away (or the writer failed): stop ffmpeg right away.
		cancel()
	}
	waitErr := cmd.Wait()

	switch {
	case copyErr != nil:
		return fmt.Errorf("transcode: writing output: %w", copyErr)
	case waitErr != nil:
		if err := ctx.Err(); err != nil {
			return err
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return fmt.Errorf("transcode: ffmpeg: %w", waitErr)
		}
		return fmt.Errorf("transcode: ffmpeg: %w: %s", waitErr, msg)
	}
	return nil
}

// tailBuffer keeps the last `limit` bytes written to it.
type tailBuffer struct {
	mu    sync.Mutex
	limit int
	buf   bytes.Buffer
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := len(p)
	if len(p) > t.limit {
		p = p[len(p)-t.limit:]
	}
	if over := t.buf.Len() + len(p) - t.limit; over > 0 {
		t.buf.Next(over)
	}
	t.buf.Write(p)
	return n, nil
}

func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.buf.String()
}

// ContentType returns the MIME type of a transcode format.
func ContentType(format string) string {
	switch format {
	case FormatMP3:
		return "audio/mpeg"
	case FormatOpus:
		return "audio/ogg"
	case FormatAAC:
		return "audio/aac"
	}
	return "application/octet-stream"
}

// Suffix returns the file suffix of a transcode format ("" for unknown formats).
func Suffix(format string) string {
	switch format {
	case FormatMP3, FormatOpus, FormatAAC:
		return format
	}
	return ""
}

// losslessSuffixes are containers that (practically) always hold lossless audio.
var losslessSuffixes = map[string]bool{
	"flac": true, "wav": true, "aif": true, "aiff": true, "ape": true, "wv": true,
	"alac": true, "dsf": true, "dff": true, "tta": true,
}

// losslessCodecs and lossyCodecs are substrings of (lower-cased) codec labels such as
// "flac", "mp4/alac", "mp4/aac", "ogg/opus".
var (
	losslessCodecs = []string{"flac", "alac", "pcm", "wav", "aiff", "ape", "monkey", "wavpack", "tta", "dsd"}
	lossyCodecs    = []string{"aac", "mp3", "mpeg", "vorbis", "opus", "speex", "musepack", "mpc"}
)

// IsLossless reports whether a track holds lossless audio (by suffix, codec or bit depth).
// A known lossy codec wins over the bit depth, because TagLib reports a nominal 16-bit
// depth for AAC in MP4 containers.
func IsLossless(t *model.Track) bool {
	if losslessSuffixes[strings.ToLower(t.Suffix)] {
		return true
	}
	codec := strings.ToLower(t.Codec)
	for _, c := range losslessCodecs {
		if strings.Contains(codec, c) {
			return true
		}
	}
	for _, c := range lossyCodecs {
		if strings.Contains(codec, c) {
			return false
		}
	}
	// Lossy codecs have no bit depth; lossless ones report 16/24/32.
	return t.BitDepth > 0
}

// sameFormat reports whether the track is already encoded in the transcode format f.
func sameFormat(t *model.Track, f string) bool {
	suffix, codec := strings.ToLower(t.Suffix), strings.ToLower(t.Codec)
	switch f {
	case FormatMP3:
		return suffix == "mp3"
	case FormatOpus:
		return suffix == "opus" || (suffix == "ogg" || suffix == "oga") && strings.Contains(codec, "opus")
	case FormatAAC:
		return suffix == "aac" || (suffix == "m4a" || suffix == "mp4" || suffix == "m4b") && strings.Contains(codec, "aac")
	}
	return false
}

// Decide chooses whether and how to transcode a track:
//
//   - requestedFormat "raw" → never transcode;
//   - a supported requestedFormat (mp3|opus|aac) → transcode to it at maxBitRate (or
//     defBitRate), unless the track already is in that format and within the bit rate;
//   - otherwise, maxBitRate > 0 and (the track is lossless or its bit rate is above
//     maxBitRate) → transcode to defFormat at maxBitRate;
//   - otherwise no transcode.
//
// When transcode is false, format is "raw" and bitRate 0. An invalid defFormat falls back
// to mp3, and bit rates are clamped to the encoder's range.
func Decide(t *model.Track, requestedFormat string, maxBitRate int, defFormat string, defBitRate int) (format string, bitRate int, transcode bool) {
	req := strings.ToLower(strings.TrimSpace(requestedFormat))
	if req == FormatRaw || t == nil {
		return FormatRaw, 0, false
	}
	if maxBitRate < 0 {
		maxBitRate = 0
	}
	if IsFormat(req) {
		if sameFormat(t, req) && (maxBitRate == 0 || (t.Bitrate > 0 && t.Bitrate <= maxBitRate)) {
			return FormatRaw, 0, false
		}
		br := maxBitRate
		if br == 0 {
			br = defBitRate
		}
		return req, ClampBitRate(req, br), true
	}
	if maxBitRate > 0 && (IsLossless(t) || t.Bitrate > maxBitRate) {
		f := strings.ToLower(defFormat)
		if !IsFormat(f) {
			f = FormatMP3
		}
		return f, ClampBitRate(f, maxBitRate), true
	}
	return FormatRaw, 0, false
}
