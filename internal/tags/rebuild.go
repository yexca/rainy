package tags

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"go.senan.xyz/taglib"
)

// Rebuild copies an audio file, reads its existing tags (including the read-only ffprobe
// fallback), and writes them through TagLib to the copy. If no tags can be read, fallback
// supplies the indexed fields. The original is replaced only after TagLib can read the
// result. WAV INFO remains an active tag container and is saved normally by TagLib;
// rebuilding does not convert INFO to JUNK. Lossy fields need a readable fallback.
// The caller must hold the library lock and check the path's write permissions.
func Rebuild(path string, fallback map[string][]string) (source string, err error) {
	if err := checkFile(path); err != nil {
		return "", err
	}
	entry, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !entry.Mode().IsRegular() {
		return "", fmt.Errorf("rebuilding %s: not a regular file: %w", filepath.Base(path), fs.ErrInvalid)
	}
	beforeProps, _ := ReadProperties(path)
	raw, readErr := ReadRaw(path)
	source = "file"
	if readErr != nil {
		if len(fallback) == 0 {
			return "", fmt.Errorf("reading existing tags: %w", readErr)
		}
		raw, source = map[string][]string{}, "index"
	}
	if raw == nil {
		raw = map[string][]string{}
	}
	// A successful read can still contain replacement characters from a legacy
	// container. Do not write those lossy values back over usable indexed fields.
	var unreadable []string
	for key, values := range raw {
		var readable []string
		for _, value := range values {
			if !strings.ContainsRune(value, utf8.RuneError) {
				readable = append(readable, value)
			}
		}
		if len(readable) == 0 {
			delete(raw, key)
			if len(values) > 0 {
				unreadable = append(unreadable, key)
			}
		} else {
			raw[key] = readable
		}
	}
	if len(raw) == 0 {
		source = "index"
	}
	for key, values := range fallback {
		if len(raw[key]) != 0 {
			continue
		}
		var readable []string
		for _, value := range cleanValues(values) {
			if utf8.ValidString(value) && !strings.ContainsRune(value, utf8.RuneError) {
				readable = append(readable, value)
			}
		}
		if len(readable) > 0 {
			raw[key] = readable
			if source == "file" {
				source = "file+index"
			}
		}
	}
	for _, key := range unreadable {
		if len(raw[key]) == 0 {
			return "", fmt.Errorf("no readable fallback for tag %s: %w", key, ErrWriteFailed)
		}
	}
	if len(raw) == 0 {
		return "", fmt.Errorf("no tags available to rebuild: %w", ErrUnsupported)
	}

	in, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = in.Close() }()
	fi, err := in.Stat()
	if err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".rainy-tag-rebuild-*."+strings.TrimPrefix(filepath.Ext(path), "."))
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err = io.Copy(tmp, in); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err = tmp.Close(); err != nil {
		return "", err
	}
	if err = in.Close(); err != nil {
		return "", err
	}
	if err = os.Chmod(tmpPath, fi.Mode().Perm()|0o200); err != nil {
		return "", err
	}
	if err = Write(tmpPath, raw); err != nil {
		return "", err
	}
	if err = restoreTags(tmpPath, raw); err != nil {
		return "", fmt.Errorf("verifying preserved tags: %w", err)
	}
	if _, err = taglib.ReadTags(tmpPath); err != nil {
		return "", wrapTaglib("verifying rebuilt tags of", path, err)
	}
	afterProps, err := taglib.ReadProperties(tmpPath)
	if err != nil {
		return "", wrapTaglib("verifying rebuilt audio of", path, err)
	}
	if beforeProps != nil {
		if beforeProps.Duration > 0 && (afterProps.Length.Seconds() < beforeProps.Duration-1 || afterProps.Length.Seconds() > beforeProps.Duration+1) ||
			beforeProps.SampleRate > 0 && int(afterProps.SampleRate) != beforeProps.SampleRate ||
			beforeProps.Channels > 0 && int(afterProps.Channels) != beforeProps.Channels ||
			len(afterProps.Images) < len(beforeProps.Pictures) {
			return "", fmt.Errorf("rebuilt file did not preserve audio or pictures: %w", ErrWriteFailed)
		}
	}
	if err = os.Chmod(tmpPath, fi.Mode().Perm()); err != nil {
		return "", err
	}
	if err = os.Rename(tmpPath, path); err != nil {
		return "", err
	}
	return source, nil
}
