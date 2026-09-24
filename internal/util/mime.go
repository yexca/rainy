package util

import "strings"

var mimeTypes = map[string]string{
	// audio
	"mp3":  "audio/mpeg",
	"flac": "audio/flac",
	"m4a":  "audio/mp4",
	"m4b":  "audio/mp4",
	"mp4":  "audio/mp4",
	"alac": "audio/mp4",
	"aac":  "audio/aac",
	"ogg":  "audio/ogg",
	"oga":  "audio/ogg",
	"opus": "audio/ogg",
	"spx":  "audio/ogg",
	"wav":  "audio/wav",
	"aif":  "audio/aiff",
	"aiff": "audio/aiff",
	"wma":  "audio/x-ms-wma",
	"ape":  "audio/x-ape",
	"wv":   "audio/x-wavpack",
	"mpc":  "audio/x-musepack",
	"dsf":  "audio/x-dsf",
	"dff":  "audio/x-dff",
	"tta":  "audio/x-tta",
	// images
	"jpg":  "image/jpeg",
	"jpeg": "image/jpeg",
	"png":  "image/png",
	"webp": "image/webp",
	"gif":  "image/gif",
	"bmp":  "image/bmp",
	"svg":  "image/svg+xml",
	// text
	"lrc":  "text/plain; charset=utf-8",
	"txt":  "text/plain; charset=utf-8",
	"json": "application/json",
}

// MimeType returns the MIME type for a file suffix (with or without leading dot, any case);
// application/octet-stream when unknown.
func MimeType(suffix string) string {
	suffix = strings.ToLower(strings.TrimPrefix(suffix, "."))
	if m, ok := mimeTypes[suffix]; ok {
		return m
	}
	return "application/octet-stream"
}

// IsImageSuffix reports whether suffix is a cover image type Rainy can use (jpg, jpeg, png, webp).
func IsImageSuffix(suffix string) bool {
	switch strings.ToLower(strings.TrimPrefix(suffix, ".")) {
	case "jpg", "jpeg", "png", "webp":
		return true
	}
	return false
}
