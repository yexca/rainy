package util

import (
	"strconv"
	"strings"
)

// Cover-art id kinds.
const (
	CoverKindAlbum    = "al"
	CoverKindArtist   = "ar"
	CoverKindTrack    = "tr"
	CoverKindPlaylist = "pl"
)

// CoverArtID builds "<kind>-<id>_<base36(version)>", e.g. CoverArtID("al", "abc", 1700000000000).
// The version (usually an updated_at in ms) busts HTTP and disk caches when the art changes.
func CoverArtID(kind, id string, version int64) string {
	if id == "" {
		return ""
	}
	if version < 0 {
		version = 0
	}
	return kind + "-" + id + "_" + strconv.FormatInt(version, 36)
}

// ParseCoverArtID extracts kind and id from a cover-art id. It is tolerant: a missing or
// unknown prefix yields kind "" (the id may then be any kind), and a "_<version>" suffix is
// stripped when present.
func ParseCoverArtID(s string) (kind, id string) {
	kind, id, _ = SplitCoverArtID(s)
	return kind, id
}

// SplitCoverArtID is ParseCoverArtID that also returns the version suffix ("" if absent).
func SplitCoverArtID(s string) (kind, id, version string) {
	s = strings.TrimSpace(s)
	if len(s) > 3 && s[2] == '-' {
		switch k := s[:2]; k {
		case CoverKindAlbum, CoverKindArtist, CoverKindTrack, CoverKindPlaylist:
			kind, s = k, s[3:]
		}
	}
	if i := strings.LastIndexByte(s, '_'); i > 0 {
		s, version = s[:i], s[i+1:]
	}
	return kind, s, version
}
