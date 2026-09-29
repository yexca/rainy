package tags

import (
	"bytes"
	"encoding/binary"
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
			if tc.info != nil && !bytes.Contains(result, wavChunk("JUNK", append([]byte("INFO"), wavChunk("INAM", tc.info)...))) {
				t.Fatal("legacy INFO bytes were not retained")
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
