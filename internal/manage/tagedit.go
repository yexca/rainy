package manage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"

	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/tags"
)

// PictureInfo describes an embedded picture.
type PictureInfo struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType"`
}

// FileInfo describes the audio file behind a track.
type FileInfo struct {
	Path       string  `json:"path"`
	Size       int64   `json:"size"`
	Mtime      int64   `json:"mtime"`
	Format     string  `json:"format"`
	Codec      string  `json:"codec"`
	Duration   float64 `json:"duration"`
	Bitrate    int     `json:"bitrate"`
	SampleRate int     `json:"sampleRate"`
	BitDepth   int     `json:"bitDepth"`
	Channels   int     `json:"channels"`
}

// TrackTags is the response of GET /api/manage/tracks/{id}/tags.
type TrackTags struct {
	Track    model.Track   `json:"track"`
	Tags     TagMap        `json:"tags"`
	Pictures []PictureInfo `json:"pictures"`
	File     FileInfo      `json:"file"`
	Writable bool          `json:"writable"`
	Lyrics   string        `json:"lyrics"`
	Lrc      *string       `json:"lrc"`
}

// TagEdit is one track's tag changes (keys to replace; an empty list deletes the key).
type TagEdit struct {
	TrackID string `json:"trackId"`
	Tags    TagMap `json:"tags"`
}

// trackFile resolves the absolute path of a track.
func (s *Service) trackFile(ctx context.Context, libs *libraries, t *model.Track) (root, abs string, err error) {
	lib, err := libs.get(ctx, t.LibraryID)
	if err != nil {
		return "", "", err
	}
	abs, err = libPath(lib.Path, t.Path)
	if err != nil {
		return "", "", err
	}
	return lib.Path, abs, nil
}

// TrackTags returns everything the tag editor shows for one track.
func (s *Service) TrackTags(ctx context.Context, id, userID string) (*TrackTags, error) {
	t, err := s.st.GetTrack(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	_, abs, err := s.trackFile(ctx, s.libs(), t)
	if err != nil {
		return nil, err
	}
	fi, err := os.Stat(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: the file of this track is missing (%s)", store.ErrConflict, t.Path)
		}
		return nil, err
	}
	md, err := tags.Read(abs, tags.ReadOptions{})
	if err != nil {
		return nil, fmt.Errorf("reading tags of %s: %w", t.Path, err)
	}
	out := &TrackTags{
		Track:    *t,
		Tags:     cloneTags(md.Raw),
		Pictures: []PictureInfo{},
		Writable: fileWritable(abs),
		Lyrics:   md.Lyrics,
		File: FileInfo{
			Path: t.Path, Size: fi.Size(), Mtime: fi.ModTime().UnixMilli(),
			Format: strings.ToLower(strings.TrimPrefix(filepath.Ext(abs), ".")), Codec: md.Codec,
			Duration: md.Duration, Bitrate: md.Bitrate, SampleRate: md.SampleRate,
			BitDepth: md.BitDepth, Channels: md.Channels,
		},
	}
	if md.HasPicture {
		info := PictureInfo{Type: "Front Cover", MimeType: "image/jpeg"}
		if pic, err := tags.ReadPicture(abs); err == nil && len(pic) > 0 {
			info.MimeType = http.DetectContentType(pic)
		}
		out.Pictures = append(out.Pictures, info)
	}
	if lrc := findSidecar(abs); lrc != "" && isRegularFile(lrc) {
		if b, err := os.ReadFile(lrc); err == nil {
			text := decodeText(b)
			out.Lrc = &text
		}
	}
	return out, nil
}

// Picture returns the embedded picture of a track and its content type.
func (s *Service) Picture(ctx context.Context, id string) ([]byte, string, error) {
	t, err := s.st.GetTrack(ctx, id, "")
	if err != nil {
		return nil, "", err
	}
	_, abs, err := s.trackFile(ctx, s.libs(), t)
	if err != nil {
		return nil, "", err
	}
	pic, err := tags.ReadPicture(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, "", store.ErrNotFound
		}
		return nil, "", err
	}
	if len(pic) == 0 {
		return nil, "", store.ErrNotFound
	}
	return pic, http.DetectContentType(pic), nil
}

// decodeText returns UTF-8 text (BOM stripped; legacy GB18030 files are converted).
func decodeText(b []byte) string {
	b = bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF}) // UTF-8 BOM
	if utf8.Valid(b) {
		return string(b)
	}
	if out, err := simplifiedchinese.GB18030.NewDecoder().Bytes(b); err == nil && utf8.Valid(out) {
		return string(out)
	}
	return strings.ToValidUTF8(string(b), "�")
}

func cloneTags(m map[string][]string) TagMap {
	out := make(TagMap, len(m))
	for k, v := range m {
		out[k] = append([]string{}, v...)
	}
	return out
}

// ---- validation

const (
	maxTagKeyLen   = 64
	maxTagValues   = 100
	maxTagValueLen = 256 << 10 // lyrics can be long
	maxTagsPerEdit = 200
)

// forbiddenKeys are managed through dedicated endpoints (pictures) and must not be
// written as text properties.
var forbiddenKeys = map[string]bool{"PICTURE": true, "METADATA_BLOCK_PICTURE": true, "COVERART": true, "COVERARTMIME": true}

// normalizeTagKey upper-cases and validates a TagLib property name: A-Z, 0-9, space and
// "_-.:/" , starting with a letter or digit, at most 64 bytes.
func normalizeTagKey(k string) (string, error) {
	k = strings.ToUpper(strings.TrimSpace(k))
	if k == "" || len(k) > maxTagKeyLen {
		return "", fmt.Errorf("%w: invalid tag name %q", store.ErrInvalid, k)
	}
	for i, r := range k {
		ok := (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			(i > 0 && strings.ContainsRune(" _-.:/", r))
		if !ok {
			return "", fmt.Errorf("%w: invalid tag name %q (use letters, digits, space and _-.:/)", store.ErrInvalid, k)
		}
	}
	if forbiddenKeys[k] {
		return "", fmt.Errorf("%w: %s cannot be edited as text; use the cover endpoints", store.ErrInvalid, k)
	}
	return k, nil
}

// validateTagValue rejects invalid UTF-8, NUL bytes and control characters other than
// tab / newline / carriage return.
func validateTagValue(key, v string) error {
	if len(v) > maxTagValueLen {
		return fmt.Errorf("%w: value of %s is too long", store.ErrInvalid, key)
	}
	if !utf8.ValidString(v) {
		return fmt.Errorf("%w: value of %s is not valid UTF-8", store.ErrInvalid, key)
	}
	for _, r := range v {
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' || r == 0x7f {
			return fmt.Errorf("%w: value of %s contains control characters", store.ErrInvalid, key)
		}
	}
	return nil
}

// normalizeTagMap validates a whole edit and returns normalized keys and trimmed values
// (empty values dropped, so [""] deletes the key like []).
func normalizeTagMap(in TagMap) (TagMap, error) {
	if len(in) > maxTagsPerEdit {
		return nil, fmt.Errorf("%w: too many tags in one edit", store.ErrInvalid)
	}
	out := make(TagMap, len(in))
	for k, vals := range in {
		key, err := normalizeTagKey(k)
		if err != nil {
			return nil, err
		}
		if _, dup := out[key]; dup {
			return nil, fmt.Errorf("%w: tag %s given twice", store.ErrInvalid, key)
		}
		if len(vals) > maxTagValues {
			return nil, fmt.Errorf("%w: too many values for %s", store.ErrInvalid, key)
		}
		clean := []string{}
		for _, v := range vals {
			if err := validateTagValue(key, v); err != nil {
				return nil, err
			}
			if key != "LYRICS" && key != "UNSYNCEDLYRICS" && key != "COMMENT" {
				v = strings.TrimSpace(v)
			}
			if v != "" {
				clean = append(clean, v)
			}
		}
		out[key] = clean
	}
	return out, nil
}

// tagDiff returns the keys of changes whose values differ from old (the file's current
// raw tags), as changes to write and as a log diff.
func tagDiff(old map[string][]string, changes TagMap) (TagMap, map[string]Change) {
	write := TagMap{}
	diff := map[string]Change{}
	for k, nv := range changes {
		ov := old[k]
		if slices.Equal(ov, nv) || (len(ov) == 0 && len(nv) == 0) {
			continue
		}
		write[k] = nv
		diff[k] = Change{Old: nonNilStrings(ov), New: nonNilStrings(nv)}
	}
	return write, diff
}

func nonNilStrings(v []string) []string {
	if v == nil {
		return []string{}
	}
	return append([]string{}, v...)
}

// ---- tag writes

// SaveTags writes the given keys of each track (other keys untouched), rescans the files
// and logs the per-key diffs.
func (s *Service) SaveTags(ctx context.Context, u *model.User, edits []TagEdit) (*BatchResult, error) {
	if err := checkBatch(len(edits), "edits"); err != nil {
		return nil, err
	}
	norm := make([]TagMap, len(edits))
	seen := map[string]bool{}
	for i, e := range edits {
		if e.TrackID == "" {
			return nil, fmt.Errorf("%w: edit %d has no trackId", store.ErrInvalid, i)
		}
		if seen[e.TrackID] {
			return nil, fmt.Errorf("%w: track %s is edited twice", store.ErrInvalid, e.TrackID)
		}
		seen[e.TrackID] = true
		m, err := normalizeTagMap(e.Tags)
		if err != nil {
			return nil, err
		}
		norm[i] = m
	}

	unlock, err := s.lock(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	ctx = context.WithoutCancel(ctx)
	res := newBatch()
	libs := s.libs()
	type done struct {
		t    *model.Track
		diff map[string]Change
	}
	var ok []done
	for i, e := range edits {
		t, err := s.st.GetTrack(ctx, e.TrackID, "")
		if err != nil {
			res.fail(e.TrackID, "", notFoundTrack(err))
			continue
		}
		diff, err := s.writeTags(ctx, libs, t, norm[i])
		if err != nil {
			res.fail(t.ID, t.Path, err)
			continue
		}
		ok = append(ok, done{t, diff})
	}

	byLib := map[int64][]string{}
	var ids []string
	for _, d := range ok {
		ids = append(ids, d.t.ID)
		if len(d.diff) > 0 {
			byLib[d.t.LibraryID] = append(byLib[d.t.LibraryID], d.t.Path)
		}
	}
	for lib, paths := range byLib {
		s.rescan(ctx, lib, paths)
	}
	for _, d := range ok {
		if len(d.diff) > 0 {
			s.logEdit(ctx, u, "tags", d.t.ID, d.t.Path, map[string]any{"changes": d.diff})
		}
	}
	res.Updated = s.tracksByID(ctx, ids, u.ID)
	if len(byLib) > 0 {
		s.publish("tags")
	}
	if err := res.failure(); err != nil {
		return nil, err
	}
	return res, nil
}

// writeTags applies changes to one track's file and returns the effective diff.
func (s *Service) writeTags(ctx context.Context, libs *libraries, t *model.Track, changes TagMap) (map[string]Change, error) {
	_, abs, err := s.trackFile(ctx, libs, t)
	if err != nil {
		return nil, err
	}
	old, err := tags.ReadRaw(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%w: file not found", store.ErrNotFound)
		}
		return nil, fsErr(t.Path, fmt.Errorf("reading tags: %w", err))
	}
	write, diff := tagDiff(old, changes)
	if len(write) == 0 {
		return diff, nil
	}
	if err := checkWritable(t.Path, abs); err != nil {
		return nil, err
	}
	if err := tags.Write(abs, write); err != nil {
		return nil, fsErr(t.Path, fmt.Errorf("writing tags: %w", err))
	}
	s.forgetEncoding(abs)
	return diff, nil
}

func notFoundTrack(err error) error {
	if errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("%w: track not found", store.ErrNotFound)
	}
	return err
}

// ---- lyrics

// SetLyrics stores lyrics either embedded (LYRICS tag) or as a UTF-8 sidecar .lrc file.
// Empty text removes them from that target.
func (s *Service) SetLyrics(ctx context.Context, u *model.User, id, text, target string) (*model.Track, error) {
	if target != "embedded" && target != "lrc" {
		return nil, fmt.Errorf("%w: target must be \"embedded\" or \"lrc\"", store.ErrInvalid)
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if strings.TrimSpace(text) == "" {
		text = ""
	}
	if err := validateTagValue("LYRICS", text); err != nil {
		return nil, err
	}
	t, err := s.st.GetTrack(ctx, id, "")
	if err != nil {
		return nil, err
	}
	libs := s.libs()
	_, abs, err := s.trackFile(ctx, libs, t)
	if err != nil {
		return nil, err
	}

	unlock, err := s.lock(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	ctx = context.WithoutCancel(ctx)
	if fi, err := os.Stat(abs); err != nil || !fi.Mode().IsRegular() {
		// Never create an orphaned sidecar for a vanished file.
		return nil, fmt.Errorf("%w: the file of this track is missing (%s)", store.ErrConflict, t.Path)
	}
	var oldText string
	var changed bool
	if target == "embedded" {
		oldText, changed, err = writeEmbeddedLyrics(abs, text)
	} else {
		oldText, changed, err = writeSidecarLyrics(abs, text)
	}
	if err != nil {
		return nil, fsErr(t.Path, err)
	}
	if changed {
		if target == "embedded" {
			s.forgetEncoding(abs)
		}
		s.rescan(ctx, t.LibraryID, []string{t.Path})
		s.logEdit(ctx, u, "lyrics", t.ID, t.Path, map[string]any{
			"target":  target,
			"changes": map[string]Change{"LYRICS": {Old: textValues(clip(oldText)), New: textValues(clip(text))}},
		})
		s.publish("lyrics")
	}
	return s.st.GetTrack(ctx, id, u.ID)
}

func writeEmbeddedLyrics(abs, text string) (old string, changed bool, err error) {
	raw, err := tags.ReadRaw(abs)
	if err != nil {
		return "", false, fmt.Errorf("reading tags: %w", err)
	}
	old = strings.Join(raw["LYRICS"], "\n")
	if old == "" {
		old = strings.Join(raw["UNSYNCEDLYRICS"], "\n")
	}
	changes := TagMap{}
	if text == "" {
		if len(raw["LYRICS"]) > 0 {
			changes["LYRICS"] = []string{}
		}
		if len(raw["UNSYNCEDLYRICS"]) > 0 {
			changes["UNSYNCEDLYRICS"] = []string{}
		}
	} else if strings.Join(raw["LYRICS"], "\n") != text {
		changes["LYRICS"] = []string{text}
	}
	if len(changes) == 0 {
		return old, false, nil
	}
	if err := checkWritable("", abs); err != nil {
		return old, false, err
	}
	if err := tags.Write(abs, changes); err != nil {
		return old, false, fmt.Errorf("writing tags: %w", err)
	}
	return old, true, nil
}

func writeSidecarLyrics(abs, text string) (old string, changed bool, err error) {
	lrc := findSidecar(abs)
	if lrc != "" && !isRegularFile(lrc) {
		return "", false, fmt.Errorf("%w: the lyrics file %s is not a regular file (symlink?)", store.ErrConflict, filepath.Base(lrc))
	}
	if lrc != "" {
		b, err := os.ReadFile(lrc)
		if err != nil {
			return "", false, err
		}
		old = strings.ReplaceAll(decodeText(b), "\r\n", "\n")
	}
	switch {
	case text == "" && lrc == "":
		return "", false, nil
	case text == "":
		return old, true, os.Remove(lrc)
	case lrc != "" && strings.TrimRight(old, "\n") == strings.TrimRight(text, "\n"):
		return old, false, nil
	case lrc == "":
		lrc = strings.TrimSuffix(abs, filepath.Ext(abs)) + ".lrc"
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return old, true, writeFileAtomic(lrc, []byte(text))
}

// clip bounds text stored in the edit log.
func clip(s string) string {
	const max = 8 << 10
	if len(s) <= max {
		return s
	}
	return truncateBytes(s, max) + "…"
}

func textValues(s string) []string {
	if s == "" {
		return []string{}
	}
	return []string{s}
}
