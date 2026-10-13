package manage

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rainy/internal/model"
	"rainy/internal/store"
)

type upFile struct {
	name string
	data []byte
}

// multipartBody builds a request body like the web UI sends (files first, then fields).
func multipartBody(t *testing.T, files []upFile, fields map[string]string) *multipart.Reader {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for _, f := range files {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="files"; filename="%s"`, f.name))
		h.Set("Content-Type", "application/octet-stream")
		w, err := mw.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(f.data)
	}
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	return multipart.NewReader(&buf, mw.Boundary())
}

// keepNames is the explicit "organize": false of import requests that keep the given names
// (without it settings.OrganizeByTags, on by default, decides).
var keepNames = new(bool)

func TestUploadToDirNeverOverwrites(t *testing.T) {
	e := newEnv(t)
	mp3, err := os.ReadFile(e.abs("林雨晴/夏日微风 (2021)/01 夏日微风.mp3"))
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]string{"libraryId": fmt.Sprint(e.lib.ID), "dir": "Uploads", "organize": "false"}
	files := []upFile{{`My Album\01 夏日微风.mp3`, mp3}, {"notes.txt", []byte("hi")}, {"../../escape.mp3", mp3}, {"empty.mp3", nil}}
	res, err := e.svc.Upload(e.ctx, e.user, multipartBody(t, files, fields))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Updated) != 2 || len(res.Errors) != 2 {
		t.Fatalf("result %+v", res)
	}
	if !e.exists("Uploads/My Album/01 夏日微风.mp3") || !e.exists("Uploads/escape.mp3") {
		t.Fatal("files not placed")
	}
	if b, _ := os.ReadFile(e.abs("Uploads/My Album/01 夏日微风.mp3")); !bytes.Equal(b, mp3) {
		t.Fatal("content differs")
	}
	// Same upload again: a numbered copy, never an overwrite.
	res, err = e.svc.Upload(e.ctx, e.user, multipartBody(t, files[:1], fields))
	if err != nil || len(res.Updated) != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	if res.Updated[0].Path != "Uploads/My Album/01 夏日微风 (1).mp3" {
		t.Fatalf("path %q", res.Updated[0].Path)
	}
	if acts := e.logActions(res.Updated[0].ID); len(acts) != 1 || acts[0] != "upload" {
		t.Fatalf("log %v", acts)
	}
	// Staging files are cleaned up.
	if entries, _ := os.ReadDir(e.cfg.TmpDir()); len(entries) != 0 {
		t.Fatalf("tmp not cleaned: %v", entries)
	}
}

func TestUploadValidation(t *testing.T) {
	e := newEnv(t)
	body := func(fields map[string]string) *multipart.Reader {
		return multipartBody(t, []upFile{{"a.mp3", []byte("data")}}, fields)
	}
	for _, fields := range []map[string]string{
		{},                                  // no library
		{"libraryId": "999"},                // unknown library
		{"libraryId": "x"},                  // invalid
		{"libraryId": "1", "dir": "../out"}, // traversal
		{"libraryId": "1", "organize": "maybe"},
	} {
		if _, err := e.svc.Upload(e.ctx, e.user, body(fields)); !errors.Is(err, store.ErrInvalid) && !errors.Is(err, store.ErrNotFound) && !isUnsafe(err) {
			t.Errorf("%v: %v", fields, err)
		}
	}
	if entries, _ := os.ReadDir(e.cfg.TmpDir()); len(entries) != 0 {
		t.Fatalf("tmp not cleaned: %v", entries)
	}
}

func isUnsafe(err error) bool { return err != nil && strings.Contains(err.Error(), "unsafe path") }

func TestUploadOrganize(t *testing.T) {
	e := newEnv(t)
	src := e.abs("林雨晴/夏日微风 (2021)/01 夏日微风.mp3")
	requireTags(t, src)
	mp3, _ := os.ReadFile(src)
	fields := map[string]string{"libraryId": fmt.Sprint(e.lib.ID), "organize": "true"}
	res, err := e.svc.Upload(e.ctx, e.user, multipartBody(t, []upFile{{"whatever.mp3", mp3}}, fields))
	if err != nil || len(res.Errors) != 0 || len(res.Updated) != 1 {
		t.Fatalf("%+v %v", res, err)
	}
	want := filepath.ToSlash("林雨晴/夏日微风/01 夏日微风.mp3")
	if res.Updated[0].Path != want || !e.exists(want) {
		t.Fatalf("organized to %q", res.Updated[0].Path)
	}
}

// Without an "organize" field the setting decides (on by default), so every way of adding
// music is placed the same way; an explicit field still wins.
func TestUploadOrganizeFollowsSetting(t *testing.T) {
	e := newEnv(t)
	src := e.abs("林雨晴/夏日微风 (2021)/01 夏日微风.mp3")
	requireTags(t, src)
	mp3, _ := os.ReadFile(src)
	upload := func(name string, fields map[string]string) string {
		t.Helper()
		fields["libraryId"] = fmt.Sprint(e.lib.ID)
		res, err := e.svc.Upload(e.ctx, e.user, multipartBody(t, []upFile{{name, mp3}}, fields))
		if err != nil || len(res.Errors) != 0 || len(res.Updated) != 1 {
			t.Fatalf("%s: %+v %v", name, res, err)
		}
		return res.Updated[0].Path
	}

	if got := upload("a.mp3", map[string]string{"dir": "In"}); got != "In/林雨晴/夏日微风/01 夏日微风.mp3" {
		t.Errorf("default: placed at %q", got)
	}
	if got := upload("b.mp3", map[string]string{"dir": "In", "organize": "false"}); got != "In/b.mp3" {
		t.Errorf("explicit false: placed at %q", got)
	}

	set := model.DefaultSettings(e.cfg.ScanInterval)
	set.OrganizeByTags = false
	if err := e.st.SaveSettings(e.ctx, set); err != nil {
		t.Fatal(err)
	}
	if got := upload("c.mp3", map[string]string{"dir": "In"}); got != "In/c.mp3" {
		t.Errorf("setting off: placed at %q", got)
	}
	if got := upload("d.mp3", map[string]string{"dir": "Out", "organize": "true"}); got != "Out/林雨晴/夏日微风/01 夏日微风.mp3" {
		t.Errorf("explicit true: placed at %q", got)
	}
}

// silentWAV is one second of silent 8 kHz, 16-bit mono PCM without any tags.
func silentWAV() []byte {
	var wav bytes.Buffer
	wav.WriteString("RIFF")
	_ = binary.Write(&wav, binary.LittleEndian, uint32(36+16000))
	wav.WriteString("WAVEfmt ")
	wav.Write([]byte{16, 0, 0, 0, 1, 0, 1, 0, 0x40, 0x1f, 0, 0, 0x80, 0x3e, 0, 0, 2, 0, 16, 0})
	wav.WriteString("data")
	_ = binary.Write(&wav, binary.LittleEndian, uint32(16000))
	wav.Write(make([]byte, 16000))
	return wav.Bytes()
}
