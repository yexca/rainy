package ytdlp

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// bilibili serves its audio as fragmented (DASH) MP4, which yt-dlp keeps as it is. TagLib
// reads no duration from such files, so the library would show 0:00 and Subsonic clients
// could not seek. defragmentMP4 rewrites them as regular MP4 files with ffmpeg (stream copy,
// fixed arguments, absolute file: URLs); other files are left alone.

// isFragmentedMP4 reports whether the MP4 file at p has a top-level "moof" box or an "mvex"
// box in its "moov" box.
func isFragmentedMP4(p string) (bool, error) {
	f, err := os.Open(p)
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()
	fi, err := f.Stat()
	if err != nil {
		return false, err
	}
	return scanBoxes(f, 0, fi.Size(), true)
}

// scanBoxes walks the boxes in [start, end); inside "moov" it looks for "mvex".
func scanBoxes(r io.ReaderAt, start, end int64, top bool) (bool, error) {
	var hdr [16]byte
	for off, n := start, 0; off+8 <= end && n < 10000; n++ {
		if _, err := r.ReadAt(hdr[:8], off); err != nil {
			return false, err
		}
		size, typ, head := int64(binary.BigEndian.Uint32(hdr[:4])), string(hdr[4:8]), int64(8)
		switch size {
		case 0: // extends to the end of the file
			size = end - off
		case 1: // 64-bit size follows
			if _, err := r.ReadAt(hdr[8:16], off+8); err != nil {
				return false, err
			}
			size, head = int64(binary.BigEndian.Uint64(hdr[8:16])), 16
		}
		if size < head || off+size > end {
			return false, errors.New("malformed MP4 box")
		}
		switch {
		case top && typ == "moof":
			return true, nil
		case !top && typ == "mvex":
			return true, nil
		case top && typ == "moov":
			if frag, err := scanBoxes(r, off+head, off+size, false); err != nil || frag {
				return frag, err
			}
		}
		off += size
	}
	return false, nil
}

func defragmentMP4(ctx context.Context, ffmpeg, p string) error {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".m4a", ".mp4", ".m4b":
	default:
		return nil
	}
	frag, err := isFragmentedMP4(p)
	if err != nil || !frag {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	tmp := strings.TrimSuffix(p, filepath.Ext(p)) + ".remux" + filepath.Ext(p)
	defer func() { _ = os.Remove(tmp) }()
	cmd := exec.CommandContext(ctx, ffmpeg, "-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-i", "file:"+p, "-map", "0:a:0", "-c", "copy", "-map_metadata", "0", "-movflags", "+faststart", "file:"+tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg: %v: %s", err, lastLine(string(out)))
	}
	return os.Rename(tmp, p)
}
