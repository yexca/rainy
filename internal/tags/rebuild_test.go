package tags

import (
	"bytes"
	"encoding/binary"
	"image/color"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func wavChunk(id string, data []byte) []byte {
	var b bytes.Buffer
	b.WriteString(id)
	_ = binary.Write(&b, binary.LittleEndian, uint32(len(data)))
	b.Write(data)
	if len(data)%2 != 0 {
		b.WriteByte(0)
	}
	return b.Bytes()
}

func syntheticWAV(info []byte) []byte {
	// 1 second of silent 8 kHz, 16-bit mono PCM.
	fmtData := []byte{1, 0, 1, 0, 0x40, 0x1f, 0, 0, 0x80, 0x3e, 0, 0, 2, 0, 16, 0}
	data := make([]byte, 16000)
	chunks := append(wavChunk("fmt ", fmtData), wavChunk("data", data)...)
	if info != nil {
		chunks = append(chunks, wavChunk("LIST", append([]byte("INFO"), wavChunk("INAM", info)...))...)
	}
	var b bytes.Buffer
	b.WriteString("RIFF")
	_ = binary.Write(&b, binary.LittleEndian, uint32(len(chunks)+4))
	b.WriteString("WAVE")
	b.Write(chunks)
	return b.Bytes()
}

func TestRebuildWAV(t *testing.T) {
	for _, tc := range []struct {
		name, wantSource, wantTitle string
		info                        []byte
	}{
		{name: "untagged", info: nil, wantSource: "index", wantTitle: "Sample title"},
		{name: "readable info", info: []byte("Existing title\x00"), wantSource: "file", wantTitle: "Existing title"},
		{name: "western info", info: []byte("Exämplö – Café\x00"), wantSource: "file", wantTitle: "Exämplö – Café"},
		{name: "legacy info", info: []byte{0xff, 0xfe, 0x81, 0}, wantSource: "index", wantTitle: "Sample title"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sample.wav")
			original := syntheticWAV(tc.info)
			if err := os.WriteFile(path, original, 0o644); err != nil {
				t.Fatal(err)
			}
			source, err := Rebuild(path, map[string][]string{"TITLE": {"Sample title"}})
			if err != nil {
				t.Fatal(err)
			}
			if source != tc.wantSource {
				t.Fatalf("source = %q, want %q", source, tc.wantSource)
			}
			raw, err := ReadRaw(path)
			if err != nil || !slices.Equal(raw["TITLE"], []string{tc.wantTitle}) {
				t.Fatalf("rebuilt tags = %v, %v", raw, err)
			}
			props, err := ReadProperties(path)
			if err != nil || props.Duration < 0.9 || props.Duration > 1.1 {
				t.Fatalf("rebuilt audio = %+v, %v", props, err)
			}
			result, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(result, wavChunk("data", make([]byte, 16000))) {
				t.Fatal("audio data changed")
			}
			chunks := wavChunks(t, result)
			if len(chunks["JUNK"]) != 0 || len(chunks["LIST:INFO"]) != 1 {
				t.Fatal("rebuild must keep one active INFO container without creating JUNK")
			}
			if _, err := Rebuild(path, nil); err != nil {
				t.Fatal(err)
			}
			again, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(wavChunks(t, again)["JUNK"]) != 0 || len(again) != len(result) {
				t.Fatal("repeated rebuild accumulated padding or changed file size")
			}
		})
	}
}

func wavChunks(t *testing.T, data []byte) map[string][][]byte {
	t.Helper()
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" ||
		uint64(binary.LittleEndian.Uint32(data[4:8]))+8 != uint64(len(data)) {
		t.Fatal("invalid RIFF/WAVE file")
	}
	chunks := map[string][][]byte{}
	for pos := 12; pos < len(data); {
		if len(data)-pos < 8 {
			t.Fatal("truncated chunk header")
		}
		n := int(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		if n < 0 || n+n%2 > len(data)-pos-8 {
			t.Fatal("truncated chunk payload")
		}
		id := string(data[pos : pos+4])
		payload := data[pos+8 : pos+8+n]
		if id == "LIST" && len(payload) >= 4 {
			id += ":" + string(payload[:4])
		}
		chunks[id] = append(chunks[id], payload)
		pos += 8 + n + n%2
	}
	return chunks
}

func TestRebuildWAVPreservesOtherChunks(t *testing.T) {
	var info bytes.Buffer
	info.WriteString("INFO")
	info.Write(wavChunk("INAM", []byte("Exämplö – Café\x00")))
	unknown := wavChunk("IZZZ", []byte("Synthetic vendor field\x00"))
	info.Write(unknown)
	adtl := append([]byte("adtl"), wavChunk("labl", []byte("\x01\x00\x00\x00Synthetic cue\x00"))...)
	padding := []byte("Existing padding")
	original := syntheticWAV(nil)
	original = append(original, wavChunk("LIST", info.Bytes())...)
	original = append(original, wavChunk("LIST", adtl)...)
	original = append(original, wavChunk("JUNK", padding)...)
	binary.LittleEndian.PutUint32(original[4:8], uint32(len(original)-8))
	path := filepath.Join(t.TempDir(), "sample.wav")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := Rebuild(path, nil); err != nil {
			t.Fatal(err)
		}
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		chunks := wavChunks(t, after)
		if len(chunks["LIST:INFO"]) != 1 || !bytes.Contains(chunks["LIST:INFO"][0], unknown) {
			t.Fatal("unknown INFO field was not retained as metadata")
		}
		if len(chunks["LIST:adtl"]) != 1 || !bytes.Equal(chunks["LIST:adtl"][0], adtl) ||
			len(chunks["JUNK"]) != 1 || !bytes.Equal(chunks["JUNK"][0], padding) {
			t.Fatal("existing cue data or padding changed")
		}
	}
}

func TestRebuildWAVWithLegacyInfoKeepsID3(t *testing.T) {
	picture := testPNG(t, color.RGBA{80, 160, 200, 255})
	original := syntheticWAV([]byte{0xb2, 0xe2, 0xca, 0xd4, 0}) // GBK INFO title
	original = append(original, wavChunk("id3 ", syntheticID3("测试歌曲", "Example Artist", "Example Album", picture))...)
	binary.LittleEndian.PutUint32(original[4:8], uint32(len(original)-8))
	path := filepath.Join(t.TempDir(), "sample.wav")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	// A stale, lossy index must not overwrite the correctly declared ID3 tags.
	if _, err := Rebuild(path, map[string][]string{"TITLE": {"����"}}); err != nil {
		t.Fatal(err)
	}
	m, err := Read(path, ReadOptions{})
	if err != nil || m.Title != "测试歌曲" || m.Artist != "Example Artist" || m.Album != "Example Album" ||
		m.Lyrics != "[00:00.00]Synthetic lyrics" || !m.HasPicture {
		t.Fatalf("ID3 metadata changed: %+v, %v", m, err)
	}
	gotPicture, err := ReadPicture(path)
	if err != nil || !bytes.Equal(gotPicture, picture) {
		t.Fatalf("embedded cover changed: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	chunks := wavChunks(t, after)
	if len(chunks["JUNK"]) != 0 || len(chunks["LIST:INFO"]) != 1 ||
		!bytes.Contains(chunks["LIST:INFO"][0], []byte("测试歌曲")) ||
		len(chunks["data"]) != 1 || !bytes.Equal(chunks["data"][0], make([]byte, 16000)) {
		t.Fatal("rebuilt INFO or audio data changed unexpectedly")
	}
}

func TestRebuildRejectsUnreadableFallback(t *testing.T) {
	for _, tc := range []struct {
		name     string
		info     []byte
		fallback map[string][]string
	}{
		{"lossy index", nil, map[string][]string{"TITLE": {"����"}}},
		{"invalid UTF-8 index", nil, map[string][]string{"TITLE": {string([]byte{0xff})}}},
		{"lossy file and index", []byte{0xff, 0xfe, 0x81, 0}, map[string][]string{"TITLE": {"����"}}},
		{"missing fallback for lossy field", []byte{0xff, 0xfe, 0x81, 0}, map[string][]string{"ALBUM": {"Example Album"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sample.wav")
			original := syntheticWAV(tc.info)
			if err := os.WriteFile(path, original, 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := Rebuild(path, tc.fallback); err == nil {
				t.Fatal("unreadable tags were accepted")
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(after, original) {
				t.Fatalf("original changed after failure: %v", err)
			}
		})
	}
}

func TestRebuildDoesNotReplaceOnFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.wav")
	before := []byte("not a WAV file")
	if err := os.WriteFile(path, before, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Rebuild(path, map[string][]string{"TITLE": {"Sample"}}); err == nil {
		t.Fatal("invalid WAV was accepted")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, before) {
		t.Fatalf("original changed after failure: %v", err)
	}
}
