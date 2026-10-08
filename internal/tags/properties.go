package tags

import (
	"encoding/binary"
	"io"
	"os"
	"strings"

	"go.senan.xyz/taglib"
)

// Keep Rainy's container/codec labels independent of the TagLib bridge's API.
func taglibFormatCodec(format taglib.FileFormat, codec string) (string, string) {
	switch format {
	case taglib.FormatMPEG:
		if codec == "AAC" {
			return "aac", ""
		}
		return "mpeg", ""
	case taglib.FormatMP4:
		return "mp4", strings.ToLower(codec)
	case taglib.FormatASF:
		return "asf", strings.ToLower(codec)
	case taglib.FormatOggVorbis:
		return "ogg", "vorbis"
	case taglib.FormatOggOpus:
		return "ogg", "opus"
	case taglib.FormatOggSpeex:
		return "ogg", "speex"
	case taglib.FormatOggFLAC, taglib.FormatFLAC:
		return "flac", ""
	case taglib.FormatWAV:
		return "wav", ""
	case taglib.FormatAIFF:
		return "aiff", ""
	case taglib.FormatAPE:
		return "ape", ""
	case taglib.FormatWavPack:
		return "wavpack", ""
	case taglib.FormatDSF:
		return "dsf", "dsd"
	case taglib.FormatDSDIFF:
		return "dsdiff", "dsd"
	case taglib.FormatTrueAudio:
		return "tta", ""
	case taglib.FormatMPC:
		return "musepack", ""
	case taglib.FormatShorten:
		return "shorten", ""
	default:
		return "", ""
	}
}

// The bridge does not expose WAV/AIFF compression types. Read just the container
// headers so compressed WAV and AIFF-C are never labelled as PCM by assumption.
func pcmContainerCodec(path string, format taglib.FileFormat) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	fi, err := f.Stat()
	if err != nil {
		return ""
	}
	var header [12]byte
	if _, err := io.ReadFull(f, header[:]); err != nil {
		return ""
	}
	var order binary.ByteOrder = binary.LittleEndian
	wantChunk := "fmt "
	if format == taglib.FormatAIFF {
		if string(header[:4]) != "FORM" {
			return ""
		}
		switch string(header[8:]) {
		case "AIFF":
			return "pcm"
		case "AIFC":
			order, wantChunk = binary.BigEndian, "COMM"
		default:
			return ""
		}
	} else if string(header[:4]) != "RIFF" || string(header[8:]) != "WAVE" {
		return ""
	}
	limit := int64(order.Uint32(header[4:8])) + 8
	if limit < 12 || limit > fi.Size() {
		return ""
	}
	for pos := int64(12); pos+8 <= limit; {
		var chunk [8]byte
		if _, err := f.ReadAt(chunk[:], pos); err != nil {
			return ""
		}
		size := int64(order.Uint32(chunk[4:]))
		end := pos + 8 + size + size%2
		if end > limit {
			return ""
		}
		if string(chunk[:4]) == wantChunk {
			if format == taglib.FormatWAV && size >= 2 {
				var code [2]byte
				if _, err := f.ReadAt(code[:], pos+8); err == nil {
					if n := order.Uint16(code[:]); n == 1 || n == 3 {
						return "pcm"
					}
				}
			} else if format == taglib.FormatAIFF && size >= 22 {
				var code [4]byte
				if _, err := f.ReadAt(code[:], pos+8+18); err == nil {
					switch string(code[:]) {
					case "NONE", "sowt", "twos", "fl32", "fl64":
						return "pcm"
					}
				}
			}
			return ""
		}
		pos = end
	}
	return ""
}
