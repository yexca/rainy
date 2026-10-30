package util

import (
	"fmt"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

const MaxPlaylistEntries = 10000
const MaxPlaylistBytes = 512 << 10

// M3UEntry is a library-relative entry; TrackID is an optional Rainy export hint.
type M3UEntry struct {
	Line    int
	Path    string
	TrackID string
	Valid   bool
}

// ParseM3U accepts UTF-8 M3U/M3U8. URLs and absolute/traversing paths are invalid
// entries, never filesystem or network inputs (contract.md §7.4).
func ParseM3U(text, baseDir string) ([]M3UEntry, error) {
	if len(text) > MaxPlaylistBytes || !utf8.ValidString(text) {
		return nil, fmt.Errorf("playlist must be UTF-8 and at most 512 KiB")
	}
	baseDir = strings.ReplaceAll(strings.TrimSpace(baseDir), "\\", "/")
	if baseDir != "" && !playlistRelative(baseDir) {
		return nil, fmt.Errorf("playlist folder must be library-relative")
	}
	entries := []M3UEntry{}
	hint := ""
	for i, line := range strings.Split(strings.TrimPrefix(text, "\ufeff"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#RAINY-TRACK:") {
			hint = strings.TrimSpace(strings.TrimPrefix(line, "#RAINY-TRACK:"))
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		if len(entries) == MaxPlaylistEntries {
			return nil, fmt.Errorf("at most 10000 playlist entries")
		}
		rel := strings.ReplaceAll(line, "\\", "/")
		valid := playlistRelative(rel)
		if valid {
			rel = path.Join(baseDir, rel)
		}
		entries = append(entries, M3UEntry{Line: i + 1, Path: rel, TrackID: hint, Valid: valid})
		hint = ""
	}
	return entries, nil
}

// playlistRelative refuses URLs (`scheme:/…`), drive letters, absolute paths, parent
// traversal and control characters. Other colons are file-name characters ("Re:ZERO/01.flac").
func playlistRelative(p string) bool {
	if p == "" || strings.HasPrefix(p, "/") || hasDriveLetter(p) || hasURLScheme(p) || strings.ContainsFunc(p, unicode.IsControl) {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if part == ".." {
			return false
		}
	}
	return path.Clean(p) != "."
}

// hasURLScheme reports a leading RFC 3986 scheme followed by ":/", as in web, file and SMB links.
func hasURLScheme(p string) bool {
	i := strings.Index(p, ":/")
	if i < 1 {
		return false
	}
	for j, r := range p[:i] {
		letter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		if !letter && (j == 0 ||
			((r < '0' || r > '9') && r != '+' && r != '-' && r != '.')) {
			return false
		}
	}
	return true
}

type M3UTrack struct {
	ID, Path, Artist, Title string
	Duration                float64
}

// FormatM3U exports library-relative paths without credentials or server addresses.
// The ID hint disambiguates libraries on the same server; other players ignore it.
func FormatM3U(tracks []M3UTrack) (string, error) {
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	clean := func(s string) string {
		return strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return ' '
			}
			return r
		}, s)
	}
	for _, t := range tracks {
		// Checked as written: the "./" prefix keeps a root file such as "A:B.flac" from reading as a drive.
		if strings.HasPrefix(t.Path, "/") || path.Clean(t.Path) != t.Path || !playlistRelative("./"+t.Path) ||
			strings.Contains(t.Path, "\\") || strings.TrimSpace(t.Path) != t.Path {
			return "", fmt.Errorf("a playlist path cannot be represented in M3U")
		}
		fmt.Fprintf(&b, "#EXTINF:%.0f,%s - %s\n#RAINY-TRACK:%s\n./%s\n", t.Duration, clean(t.Artist), clean(t.Title), clean(t.ID), t.Path)
	}
	if b.Len() > MaxPlaylistBytes || len(tracks) > MaxPlaylistEntries {
		return "", fmt.Errorf("playlist is too large to export (512 KiB, 10000 entries)")
	}
	return b.String(), nil
}
