// Package util contains small dependency-free helpers shared by every layer: identifiers,
// time, text normalisation, cover-art ids, MIME types and safe path handling.
//
// This package must not import other internal packages.
package util

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"
)

const base62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// IDLength is the length of ids produced by NewID and HashID.
const IDLength = 22

// NewID returns a random 22-character base62 identifier (≈131 bits of entropy).
func NewID() string {
	var out [IDLength]byte
	var buf [64]byte
	n := 0
	for n < IDLength {
		if _, err := rand.Read(buf[:]); err != nil {
			panic("util.NewID: crypto/rand failed: " + err.Error())
		}
		for _, b := range buf {
			// Rejection sampling keeps the distribution uniform: 248 = 4*62.
			if b >= 248 {
				continue
			}
			out[n] = base62[b%62]
			n++
			if n == IDLength {
				break
			}
		}
	}
	return string(out[:])
}

// HashID returns a deterministic id: the first 22 lower-hex characters of the md5 of the
// parts joined with "\x00". Every part is trimmed and lower-cased first, so
// HashID("album", "Artist ", "Name") == HashID("album", "artist", "name").
// Used for albums (HashID("album", albumArtist, name)), artists (HashID("artist", name))
// and genres (HashID("genre", name)).
func HashID(parts ...string) string {
	norm := make([]string, len(parts))
	for i, p := range parts {
		norm[i] = strings.ToLower(strings.TrimSpace(p))
	}
	sum := md5.Sum([]byte(strings.Join(norm, "\x00")))
	return hex.EncodeToString(sum[:])[:IDLength]
}

// AlbumID is HashID("album", albumArtist, name).
func AlbumID(albumArtist, name string) string { return HashID("album", albumArtist, name) }

// ArtistID is HashID("artist", name).
func ArtistID(name string) string { return HashID("artist", name) }

// GenreID is HashID("genre", name).
func GenreID(name string) string { return HashID("genre", name) }

// NowMs returns the current time in unix milliseconds.
func NowMs() int64 { return time.Now().UnixMilli() }

// FromMs converts unix milliseconds to time.Time (zero time for 0).
func FromMs(ms int64) time.Time {
	if ms == 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}

// RFC3339Ms formats unix milliseconds as RFC3339 in UTC ("" for 0) — the Subsonic format.
func RFC3339Ms(ms int64) string {
	if ms == 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}
