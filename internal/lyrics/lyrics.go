// Package lyrics parses LRC / plain-text lyrics and loads a track's lyrics from a sidecar
// .lrc file or its embedded tag (docs/architecture/contract.md §5.10).
//
// Supported LRC features: [mm:ss], [mm:ss.x], [mm:ss.xx], [mm:ss.xxx] and [mm:ss:xx] time
// tags; several time tags on one line ("[00:12.00][01:30.00]chorus"); the [offset:±ms]
// tag; metadata tags ([ti:], [ar:], [al:], [by:], [la:], [length:], …) which are dropped
// from the lines; and enhanced (A2) word time stamps ("<00:12.50>word") which are removed
// from the text. In synced lyrics, untimed text following a timed line (e.g. a translation)
// takes that line's time. Anything without time tags is treated as plain text.
package lyrics

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"

	"rainy/internal/model"
)

// Sources.
const (
	SourceLRC      = "lrc"
	SourceEmbedded = "embedded"
	SourceNone     = "none"
)

// maxSidecarSize bounds the .lrc files Load reads.
const maxSidecarSize = 1 << 20

// Line is one lyrics line; Start is in ms, -1 when unsynced.
type Line struct {
	Start int64  `json:"start"`
	Text  string `json:"text"`
}

// Lyrics is the parsed result.
//
// For synced lyrics, Line.Start already has the file's [offset:] applied (a positive offset
// makes lines appear earlier, as in the LRC convention); Offset reports the value found in
// the file for information only and must not be applied again.
type Lyrics struct {
	Synced bool   `json:"synced"`
	Lines  []Line `json:"lines"`  // never nil
	Source string `json:"source"` // lrc|embedded|none
	Raw    string `json:"raw"`    // the original text (BOM removed)
	Offset int64  `json:"offset"` // ms, from [offset:]
	Lang   string `json:"lang"`   // from [la:] / [lang:] ("" if absent)
}

var (
	// timeTag matches one leading LRC time tag: [mm:ss], [mm:ss.xx], [mm:ss:xx], [mm:ss.xxx].
	timeTag = regexp.MustCompile(`^\[\s*(\d{1,4}):(\d{1,2})(?:[.:](\d{1,3}))?\s*\]`)
	// metaTag matches an ID tag line such as [ar: Artist] or [offset:+250].
	metaTag = regexp.MustCompile(`^\[\s*([A-Za-z#]+)\s*:(.*)\]$`)
	// wordStamp matches enhanced-LRC word time stamps.
	wordStamp = regexp.MustCompile(`<\s*\d{1,4}:\d{1,2}(?:[.:]\d{1,3})?\s*>`)
	// spaces collapses runs left behind by removed word stamps.
	spaces = regexp.MustCompile(`[ \t]{2,}`)
)

// metaKeys are the LRC ID tags consumed as metadata (lower-case).
var metaKeys = map[string]bool{
	"ti": true, "ar": true, "al": true, "au": true, "by": true, "re": true, "ve": true,
	"tool": true, "length": true, "offset": true, "la": true, "lang": true, "language": true, "#": true,
}

// parseTime converts the groups of a time tag to milliseconds.
func parseTime(min, sec, frac string) (int64, bool) {
	m, err1 := strconv.ParseInt(min, 10, 64)
	s, err2 := strconv.ParseInt(sec, 10, 64)
	if err1 != nil || err2 != nil || s >= 60 {
		return 0, false
	}
	ms := (m*60 + s) * 1000
	switch len(frac) {
	case 0:
	case 1:
		f, _ := strconv.ParseInt(frac, 10, 64)
		ms += f * 100
	case 2:
		f, _ := strconv.ParseInt(frac, 10, 64)
		ms += f * 10
	default:
		f, _ := strconv.ParseInt(frac, 10, 64)
		ms += f
	}
	return ms, true
}

// cleanText removes word stamps and normalises whitespace of a lyric line.
func cleanText(s string) string {
	if strings.IndexByte(s, '<') >= 0 {
		s = wordStamp.ReplaceAllString(s, "")
		s = spaces.ReplaceAllString(s, " ")
	}
	return strings.TrimSpace(s)
}

// Parse parses LRC or plain text. The result's Source is "none" when there are no lines,
// otherwise "embedded" (Load sets "lrc" for sidecar files).
func Parse(text string) *Lyrics {
	text = strings.TrimPrefix(text, string(rune(0xFEFF)))
	l := &Lyrics{Lines: []Line{}, Raw: text, Source: SourceNone}

	var timed, plain []Line
	// prevStamps are the time tags of the last timed line with text: untimed text after it
	// (a translation or a wrapped line) is shown together with it instead of being lost.
	var prevStamps []int64
	for _, raw := range splitLines(text) {
		line := strings.TrimSpace(raw)
		var stamps []int64
		tagged := false
		for {
			m := timeTag.FindStringSubmatch(line)
			if m == nil {
				break
			}
			tagged = true
			if ms, ok := parseTime(m[1], m[2], m[3]); ok {
				stamps = append(stamps, ms)
			}
			line = line[len(m[0]):]
		}
		if tagged && len(stamps) == 0 {
			continue // only invalid time tags ("[00:61.00]"): drop the line
		}
		if len(stamps) > 0 {
			t := cleanText(line)
			for _, ms := range stamps {
				timed = append(timed, Line{Start: ms, Text: t})
			}
			prevStamps = stamps
			if t == "" {
				prevStamps = nil // an instrumental break has nothing to continue
			}
			continue
		}
		if m := metaTag.FindStringSubmatch(line); m != nil && metaKeys[strings.ToLower(m[1])] {
			value := strings.TrimSpace(m[2])
			switch strings.ToLower(m[1]) {
			case "offset":
				if v, err := strconv.ParseInt(strings.TrimPrefix(value, "+"), 10, 64); err == nil {
					l.Offset = v
				}
			case "la", "lang", "language":
				l.Lang = value
			}
			continue
		}
		t := cleanText(line)
		plain = append(plain, Line{Start: -1, Text: t})
		if t != "" {
			for _, ms := range prevStamps {
				timed = append(timed, Line{Start: ms, Text: t})
			}
		}
	}

	if len(timed) > 0 {
		sort.SliceStable(timed, func(i, j int) bool { return timed[i].Start < timed[j].Start })
		for i := range timed {
			timed[i].Start = max(timed[i].Start-l.Offset, 0)
		}
		l.Synced, l.Lines = true, timed
	} else {
		l.Lines = compactPlain(plain)
	}
	if len(l.Lines) > 0 {
		l.Source = SourceEmbedded
	}
	return l
}

// splitLines splits on \n, \r\n and lone \r.
func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.Split(s, "\n")
}

// compactPlain drops leading/trailing blank lines and collapses runs of blank lines (stanza
// breaks) to one.
func compactPlain(lines []Line) []Line {
	out := make([]Line, 0, len(lines))
	for _, ln := range lines {
		if ln.Text == "" && (len(out) == 0 || out[len(out)-1].Text == "") {
			continue
		}
		out = append(out, ln)
	}
	for len(out) > 0 && out[len(out)-1].Text == "" {
		out = out[:len(out)-1]
	}
	return out
}

// LrcPath returns the sidecar .lrc path for an audio file ("dir/song.flac" → "dir/song.lrc").
func LrcPath(audioAbsPath string) string {
	return strings.TrimSuffix(audioAbsPath, filepath.Ext(audioAbsPath)) + ".lrc"
}

// Load returns the lyrics of a track: the sidecar .lrc next to audioAbsPath first (".lrc",
// then ".LRC"), then the embedded t.Lyrics; Source "none" with empty Lines if neither has
// any content. Unreadable sidecars are skipped. audioAbsPath may be "" (embedded only).
func Load(t *model.Track, audioAbsPath string) (*Lyrics, error) {
	if audioAbsPath != "" {
		base := strings.TrimSuffix(audioAbsPath, filepath.Ext(audioAbsPath))
		for _, p := range []string{base + ".lrc", base + ".LRC"} {
			text, err := readSidecar(p)
			if err != nil || strings.TrimSpace(text) == "" {
				continue
			}
			if l := Parse(text); len(l.Lines) > 0 {
				l.Source = SourceLRC
				return l, nil
			}
		}
	}
	if t != nil && strings.TrimSpace(t.Lyrics) != "" {
		if l := Parse(t.Lyrics); len(l.Lines) > 0 {
			l.Source = SourceEmbedded
			return l, nil
		}
	}
	return &Lyrics{Lines: []Line{}, Source: SourceNone}, nil
}

// errTooLarge is returned by readSidecar for oversized files.
var errTooLarge = errors.New("lyrics: sidecar file too large")

// readSidecar reads and decodes a text file (at most maxSidecarSize bytes).
func readSidecar(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	fi, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !fi.Mode().IsRegular() {
		return "", fs.ErrNotExist
	}
	b, err := io.ReadAll(io.LimitReader(f, maxSidecarSize+1))
	if err != nil {
		return "", err
	}
	if len(b) > maxSidecarSize {
		return "", errTooLarge
	}
	return DecodeText(b), nil
}

// DecodeText converts the bytes of a text file to a string: UTF-8 (BOM stripped), UTF-16
// with a BOM, or — for invalid UTF-8 — the legacy CJK encoding (GB18030, Big5, Shift-JIS)
// that decodes with the fewest errors.
func DecodeText(b []byte) string {
	switch {
	case bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}):
		return string(b[3:])
	case bytes.HasPrefix(b, []byte{0xFF, 0xFE}):
		return decodeUTF16(b[2:], false)
	case bytes.HasPrefix(b, []byte{0xFE, 0xFF}):
		return decodeUTF16(b[2:], true)
	case utf8.Valid(b):
		return string(b)
	}
	replacement := string(utf8.RuneError)
	// Shift-JIS first: text with kana that decodes cleanly as Shift-JIS is Japanese (GB18030
	// accepts almost any byte sequence, so it cannot be the first judge).
	if out, err := japanese.ShiftJIS.NewDecoder().Bytes(b); err == nil {
		if s := string(out); !strings.Contains(s, replacement) && hasKana(s) {
			return s
		}
	}
	best, bestBad := strings.ToValidUTF8(string(b), replacement), -1
	for _, enc := range []encoding.Encoding{simplifiedchinese.GB18030, traditionalchinese.Big5, japanese.ShiftJIS} {
		out, err := enc.NewDecoder().Bytes(b)
		if err != nil {
			continue
		}
		bad := strings.Count(string(out), replacement)
		if bestBad < 0 || bad < bestBad {
			best, bestBad = string(out), bad
		}
	}
	return best
}

// hasKana reports whether s contains full-width hiragana or katakana.
func hasKana(s string) bool {
	for _, r := range s {
		if r >= 0x3041 && r <= 0x30FF {
			return true
		}
	}
	return false
}

func decodeUTF16(b []byte, bigEndian bool) string {
	u := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		if bigEndian {
			u = append(u, uint16(b[i])<<8|uint16(b[i+1]))
		} else {
			u = append(u, uint16(b[i+1])<<8|uint16(b[i]))
		}
	}
	return string(utf16.Decode(u))
}
