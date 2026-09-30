package ytdlp

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"rainy/internal/tags"
)

func box(typ string, children ...[]byte) []byte {
	var body []byte
	for _, c := range children {
		body = append(body, c...)
	}
	b := make([]byte, 8, 8+len(body))
	binary.BigEndian.PutUint32(b, uint32(8+len(body)))
	copy(b[4:], typ)
	return append(b, body...)
}

func TestIsFragmentedMP4(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]struct {
		data []byte
		want bool
	}{
		"regular":    {append(append(box("ftyp", []byte("M4A ")), box("moov", box("mvhd", make([]byte, 20)))...), box("mdat", []byte("xx"))...), false},
		"dash moov":  {append(box("ftyp", []byte("iso5")), box("moov", box("mvhd"), box("mvex", box("mehd")))...), true},
		"moof":       {append(append(box("ftyp", []byte("iso5")), box("moov", box("mvhd"))...), box("moof", box("mfhd"))...), true},
		"to the end": {append(box("ftyp"), 0, 0, 0, 0, 'm', 'd', 'a', 't', 1, 2, 3), false},
	}
	for name, c := range cases {
		p := filepath.Join(dir, name+".m4a")
		if err := os.WriteFile(p, c.data, 0o600); err != nil {
			t.Fatal(err)
		}
		if got, err := isFragmentedMP4(p); err != nil || got != c.want {
			t.Errorf("%s: fragmented = %v, %v; want %v", name, got, err, c.want)
		}
	}
	bad := filepath.Join(dir, "bad.m4a")
	_ = os.WriteFile(bad, []byte{0, 0, 0, 99, 'f', 't', 'y', 'p'}, 0o600)
	if _, err := isFragmentedMP4(bad); err == nil {
		t.Error("truncated box accepted")
	}
}

// TestDefragmentMP4 needs ffmpeg: it writes a fragmented AAC file like bilibili's and checks
// that TagLib reads a duration after the remux.
func TestDefragmentMP4(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	p := filepath.Join(t.TempDir(), "BV1Synthetic.m4a")
	gen := exec.Command(ffmpeg, "-nostdin", "-loglevel", "error", "-f", "lavfi", "-i", "sine=frequency=440:duration=2",
		"-c:a", "aac", "-movflags", "frag_keyframe+empty_moov", "file:"+p)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("generating: %v %s", err, out)
	}
	if frag, _ := isFragmentedMP4(p); !frag {
		t.Fatal("test file is not fragmented")
	}
	if err := defragmentMP4(t.Context(), ffmpeg, p); err != nil {
		t.Fatal(err)
	}
	if frag, err := isFragmentedMP4(p); err != nil || frag {
		t.Fatalf("still fragmented: %v", err)
	}
	md, err := tags.Read(p, tags.ReadOptions{})
	if err != nil || md.Duration < 1.5 {
		t.Fatalf("duration after remux %v, %v", md, err)
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(p), "*.remux.*")); len(left) != 0 {
		t.Fatalf("temporary files left: %v", left)
	}
}
