// Package lxmusic brings the online part of lx-music to Rainy (docs/architecture/contract.md
// §5.15): it searches the five catalogues lx-music knows (Kuwo, Kugou, QQ Music, NetEase Cloud
// Music, Migu) and runs lx-music "custom source" scripts, which turn a search result into a
// download link. Manage uses both to download songs into a library.
//
// Source scripts are third-party JavaScript. They run inside a goja interpreter (pure Go, no
// file system, process or module access) that offers only the lx-music script API: lx.request
// reaches public internet addresses only (loopback, private, link-local and other special
// ranges are refused after DNS resolution), responses and inputs are bounded, and every call
// into the script has a time limit after which the interpreter is discarded. Memory use is not
// bounded, so administrators should import only scripts they trust.
//
// Everything here contacts third-party services, so the API refuses to use it unless an
// administrator turned on settings.lxSourcesEnabled.
package lxmusic

import (
	"errors"
	"slices"
)

// Errors. Callers map ErrInvalid to 400, ErrNotFound to 404 and ErrUpstream / ErrUnavailable
// to 503.
var (
	ErrInvalid     = errors.New("invalid request")
	ErrNotFound    = errors.New("not found")
	ErrUpstream    = errors.New("online request failed")
	ErrUnavailable = errors.New("no music source can provide this song")
	ErrScript      = errors.New("source script failed")
	ErrBlocked     = errors.New("address not allowed")
	ErrClosed      = errors.New("music sources are shut down")
)

// Platform ids (lx-music source keys).
const (
	Kuwo    = "kw"
	Kugou   = "kg"
	QQ      = "tx"
	NetEase = "wy"
	Migu    = "mg"
)

// Platforms lists the platform ids in display order.
var Platforms = []string{Kuwo, Kugou, QQ, NetEase, Migu}

// ValidPlatform reports whether id is a platform id.
func ValidPlatform(id string) bool { return slices.Contains(Platforms, id) }

// Qualities lists the qualities a source can provide, lowest first.
var Qualities = []string{"128k", "320k", "flac", "flac24bit"}

// ValidQuality reports whether q is a quality id.
func ValidQuality(q string) bool { return slices.Contains(Qualities, q) }

// qualityRank returns the position of q in Qualities (-1 when unknown).
func qualityRank(q string) int { return slices.Index(Qualities, q) }
