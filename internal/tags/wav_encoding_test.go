package tags

import (
	"bytes"
	"encoding/binary"
	"image/color"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"

	"go.senan.xyz/taglib"
	"golang.org/x/text/encoding/simplifiedchinese"
)

func utf16Text(s string) []byte {
	var b bytes.Buffer
	b.Write([]byte{0xff, 0xfe})
	for _, r := range utf16.Encode([]rune(s)) {
		_ = binary.Write(&b, binary.LittleEndian, r)
	}
	return b.Bytes()
}

func id3v23Frame(key string, data []byte) []byte {
	var b bytes.Buffer
	b.WriteString(key)
	_ = binary.Write(&b, binary.BigEndian, uint32(len(data)))
	b.Write([]byte{0, 0})
	b.Write(data)
	return b.Bytes()
}

func syntheticID3(title, artist, album string, picture []byte) []byte {
	var frames bytes.Buffer
	for _, tag := range []struct{ key, value string }{
		{"TIT2", title}, {"TPE1", artist}, {"TALB", album},
		{"TPE2", "Example Album Artist"}, {"TRCK", "2/7"}, {"TCON", "Test Genre"},
	} {
		frames.Write(id3v23Frame(tag.key, append([]byte{1}, utf16Text(tag.value)...)))
	}
	lyrics := append([]byte{1, 'x', 'x', 'x'}, utf16Text("")...)
	lyrics = append(lyrics, 0, 0) // empty UTF-16 description
	lyrics = append(lyrics, utf16Text("[00:00.00]Synthetic lyrics")...)
	frames.Write(id3v23Frame("USLT", lyrics))
	if picture != nil {
		data := append([]byte{0}, []byte("image/png\x00\x03\x00")...)
		frames.Write(id3v23Frame("APIC", append(data, picture...)))
	}
	var tag bytes.Buffer
	tag.Write([]byte{'I', 'D', '3', 3, 0, 0})
	n := frames.Len()
	tag.Write([]byte{byte(n>>21) & 0x7f, byte(n>>14) & 0x7f, byte(n>>7) & 0x7f, byte(n) & 0x7f})
	tag.Write(frames.Bytes())
	return tag.Bytes()
}

// A WAV can contain correctly declared UTF-16 ID3 tags alongside invalid-UTF-8
// GBK INFO tags. Reading must keep the ID3 values, properties and cover without
// ffprobe, whose older versions overwrite the good values with lossy INFO text.
func TestReadWAVWithLegacyInfoKeepsID3(t *testing.T) {
	oldFFmpeg, oldProbe := loadPath(&ffmpegPath), loadPath(&ffprobePath)
	SetFFmpeg("")
	t.Cleanup(func() {
		ffmpegPath.Store(oldFFmpeg)
		ffprobePath.Store(oldProbe)
	})
	picture := testPNG(t, color.RGBA{80, 160, 200, 255})
	var info bytes.Buffer
	info.WriteString("INFO")
	for _, tag := range []struct{ key, value string }{
		{"INAM", "旧的信息"}, {"IART", "示例歌手"}, {"IPRD", "示例专辑"},
	} {
		encoded, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(tag.value))
		if err != nil {
			t.Fatal(err)
		}
		info.Write(wavChunk(tag.key, append(encoded, 0)))
	}
	for _, title := range []string{"测试歌曲", "仮の歌", "Exämplö – Café"} {
		for _, infoFirst := range []bool{false, true} {
			name := title + "/ID3-first"
			if infoFirst {
				name = title + "/INFO-first"
			}
			t.Run(name, func(t *testing.T) {
				id3Chunk := wavChunk("id3 ", syntheticID3(title, "Example Artist", "Example Album", picture))
				infoChunk := wavChunk("LIST", info.Bytes())
				original := syntheticWAV(nil)
				if infoFirst {
					original = append(original, infoChunk...)
					original = append(original, id3Chunk...)
				} else {
					original = append(original, id3Chunk...)
					original = append(original, infoChunk...)
				}
				binary.LittleEndian.PutUint32(original[4:8], uint32(len(original)-8))
				path := filepath.Join(t.TempDir(), "sample.wav")
				if err := os.WriteFile(path, original, 0o444); err != nil {
					t.Fatal(err)
				}
				for _, fixEncoding := range []bool{false, true} {
					m, err := Read(path, ReadOptions{FixEncoding: fixEncoding})
					if err != nil {
						t.Fatalf("Read without ffprobe: %v", err)
					}
					if m.Title != title || m.Artist != "Example Artist" || m.Album != "Example Album" ||
						m.AlbumArtist != "Example Album Artist" || m.TrackNumber != 2 || m.TrackTotal != 7 ||
						m.Lyrics != "[00:00.00]Synthetic lyrics" || !m.HasPicture {
						t.Fatalf("ID3 metadata lost: %+v", m)
					}
					if m.Codec != "wav/pcm" || m.SampleRate != 8000 || m.Channels != 1 || m.BitDepth != 16 || m.Duration != 1 {
						t.Fatalf("audio properties changed: %+v", m)
					}
				}
				raw, err := ReadRaw(path)
				if err != nil || len(raw["TITLE"]) != 1 || raw["TITLE"][0] != title {
					t.Fatalf("raw title = %q, %v", raw["TITLE"], err)
				}
				gotPicture, err := ReadPicture(path)
				if err != nil || !bytes.Equal(gotPicture, picture) {
					t.Fatalf("embedded cover changed: %d bytes, %v", len(gotPicture), err)
				}
				after, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(after, original) {
					t.Fatalf("reading changed the music file: %v", err)
				}
			})
		}
	}
}

func TestPCMContainerCodecDoesNotAssumePCM(t *testing.T) {
	for _, tc := range []struct {
		name string
		code uint16
		want string
	}{
		{"PCM", 1, "pcm"},
		{"float", 3, "pcm"},
		{"A-law", 6, ""},
		{"mu-law", 7, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := syntheticWAV(nil)
			binary.LittleEndian.PutUint16(data[20:22], tc.code)
			path := filepath.Join(t.TempDir(), "sample.wav")
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}
			if got := pcmContainerCodec(path, taglib.FormatWAV); got != tc.want {
				t.Fatalf("codec = %q; want %q", got, tc.want)
			}
		})
	}
}
