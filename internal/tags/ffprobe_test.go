package tags

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// A WAV with an "id3 " chunk (UTF-8) and a GBK LIST/INFO chunk: ffprobe reports every key
// twice and the INFO copies are undecodable (U+FFFD). The clean copies must win.
const dualContainerProbe = `{
  "streams": [
    {"codec_type": "audio", "codec_name": "pcm_s16le", "sample_rate": "44100", "channels": 2, "bits_per_sample": 16, "disposition": {"attached_pic": 0}},
    {"codec_type": "video", "codec_name": "mjpeg", "disposition": {"attached_pic": 1}}
  ],
  "format": {
    "format_name": "wav", "duration": "165.000000", "bit_rate": "1429568",
    "tags": {
      "title": "我们快出发", "artist": "泠鸢yousa/嘉然Diana", "album": "我们快出发",
      "lyrics-XXX": "[00:00.00]作词 : 泠鸢yousa\n[00:09.41]我坐过了站",
      "album_artist": "泠鸢yousa", "track": "1", "disc": "2", "date": "2022",
      "artist": "����yousa", "title": "����",
      "IPRO": "��yousa", "encoder": "Lavf59.6.100"
    }
  }
}`

func TestParseProbePrefersCleanDuplicates(t *testing.T) {
	raw, props, err := parseProbe([]byte(dualContainerProbe))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"TITLE": "我们快出发", "ARTIST": "泠鸢yousa/嘉然Diana", "ALBUM": "我们快出发",
		"ALBUMARTIST": "泠鸢yousa", "TRACKNUMBER": "1", "DISCNUMBER": "2", "DATE": "2022",
	}
	for k, v := range want {
		if got := raw[k]; len(got) != 1 || got[0] != v {
			t.Errorf("%s = %q, want [%q]", k, got, v)
		}
	}
	if _, ok := raw["ENCODER"]; ok {
		t.Error("encoder tag should be dropped")
	}
	if got := raw["IPRO"]; len(got) != 1 {
		t.Errorf("undecodable-only value should still be kept as a last resort, got %q", got)
	}
	m := parse(raw)
	if m.Lyrics == "" || m.TrackNumber != 1 || m.DiscNumber != 2 || m.Year != 2022 {
		t.Errorf("parsed metadata = %+v", m)
	}
	if props.Duration != 165 || props.SampleRate != 44100 || props.Channels != 2 || props.BitDepth != 16 ||
		props.Bitrate != 1429 || props.Codec != "wav/pcm" || len(props.Pictures) != 1 {
		t.Errorf("props = %+v", props)
	}
}

func TestParseProbeRejectsNoAudio(t *testing.T) {
	if _, _, err := parseProbe([]byte(`{"streams":[{"codec_type":"video","codec_name":"mjpeg"}],"format":{"format_name":"image2"}}`)); err == nil {
		t.Fatal("expected an error for a file without audio")
	}
}

func TestProbeRealFile(t *testing.T) {
	ff, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	SetFFmpeg(ff)
	t.Cleanup(func() { SetFFmpeg("") })
	if loadPath(&ffprobePath) == "" {
		t.Skip("ffprobe not installed")
	}
	dir := t.TempDir()
	cover := filepath.Join(dir, "c.jpg")
	wav := filepath.Join(dir, "t.mp3")
	run := func(args ...string) {
		if out, err := exec.Command(ff, append([]string{"-v", "error", "-y"}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("ffmpeg: %v: %s", err, out)
		}
	}
	run("-f", "lavfi", "-i", "color=c=red:s=64x64", "-frames:v", "1", cover)
	run("-f", "lavfi", "-i", "sine=f=440:d=1", "-i", cover, "-map", "0", "-map", "1", "-c:v", "copy",
		"-disposition:v", "attached_pic", "-metadata", "title=雨の歌", "-metadata", "artist=Tester", "-id3v2_version", "3",
		"-f", "mp3", wav)

	raw, props, err := probe(wav)
	if err != nil {
		t.Fatal(err)
	}
	if got := raw["TITLE"]; len(got) != 1 || got[0] != "雨の歌" {
		t.Errorf("TITLE = %q", got)
	}
	if props.Duration < 0.9 || props.Duration > 1.1 || props.SampleRate == 0 {
		t.Errorf("props = %+v", props)
	}
	if len(props.Pictures) == 1 {
		pic, err := probePicture(wav)
		if err != nil || len(pic) < 3 || pic[0] != 0xFF || pic[1] != 0xD8 {
			t.Errorf("picture: %v (%d bytes)", err, len(pic))
		}
	}
	if _, _, err := probe(filepath.Join(dir, "missing.wav")); err == nil {
		t.Error("expected an error for a missing file")
	}
	_ = os.Remove(wav)
}
