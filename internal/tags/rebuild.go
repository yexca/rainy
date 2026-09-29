package tags

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.senan.xyz/taglib"
)

// Rebuild copies an audio file, reads its existing tags (including the read-only ffprobe
// fallback), and writes them through TagLib to the copy. If no tags can be read, fallback
// supplies the indexed fields. The original is replaced only after TagLib can read the
// result. For WAV, legacy LIST/INFO chunks are retained byte-for-byte as inert JUNK
// chunks: TagLib cannot parse some of them, while the audio and ID3 chunks remain intact.
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
	if len(raw) == 0 {
		source = "index"
	}
	for key, values := range fallback {
		if len(raw[key]) == 0 && len(values) > 0 {
			raw[key] = values
			if source == "file" {
				source = "file+index"
			}
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
	if strings.EqualFold(filepath.Ext(path), ".wav") {
		if err = neutralizeWAVInfo(tmpPath); err != nil {
			return "", err
		}
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

// neutralizeWAVInfo changes only the four-byte chunk id of LIST/INFO chunks.
// RIFF readers ignore JUNK, so the legacy bytes stay recoverable in the file.
func neutralizeWAVInfo(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	var header [12]byte
	if _, err := io.ReadFull(f, header[:]); err != nil {
		return fmt.Errorf("reading WAV header: %w", err)
	}
	if string(header[:4]) != "RIFF" || string(header[8:]) != "WAVE" {
		return fmt.Errorf("invalid WAV header: %w", ErrUnsupported)
	}
	limit := int64(binary.LittleEndian.Uint32(header[4:8])) + 8
	if limit > fi.Size() || limit < 12 {
		return fmt.Errorf("invalid WAV size: %w", ErrUnsupported)
	}
	for pos := int64(12); pos+8 <= limit; {
		var chunk [8]byte
		if _, err := f.ReadAt(chunk[:], pos); err != nil {
			return err
		}
		size := int64(binary.LittleEndian.Uint32(chunk[4:]))
		end := pos + 8 + size + size%2
		if end > limit {
			return fmt.Errorf("invalid WAV chunk size: %w", ErrUnsupported)
		}
		if string(chunk[:4]) == "LIST" && size >= 4 {
			var kind [4]byte
			if _, err := f.ReadAt(kind[:], pos+8); err != nil {
				return err
			}
			if string(kind[:]) == "INFO" {
				if _, err := f.WriteAt([]byte("JUNK"), pos); err != nil {
					return err
				}
			}
		}
		pos = end
	}
	return f.Sync()
}
