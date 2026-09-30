package ytdlp

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"rainy/internal/auth"
)

const testTag = "2026.08.19"

// fakeGitHub answers the release API and the release downloads; nothing leaves the test.
type fakeGitHub struct {
	tag    string
	files  map[string][]byte // asset name → content
	sums   string            // SHA2-256SUMS override ("" = computed from files)
	calls  []string
	status int
}

func (f *fakeGitHub) RoundTrip(r *http.Request) (*http.Response, error) {
	f.calls = append(f.calls, r.URL.String())
	rec := httptest.NewRecorder()
	switch {
	case f.status != 0:
		rec.WriteHeader(f.status)
	case r.URL.String() == releaseAPI:
		_, _ = fmt.Fprintf(rec, `{"tag_name":%q,"assets":[{"browser_download_url":"https://evil.example.com/x"}]}`, f.tag)
	case r.URL.String() == fmt.Sprintf(downloadURL, f.tag, sumsAsset):
		if f.sums != "" {
			_, _ = rec.WriteString(f.sums)
			break
		}
		for name, b := range f.files {
			sum := sha256.Sum256(b)
			_, _ = fmt.Fprintf(rec, "%s  %s\n", hex.EncodeToString(sum[:]), name)
		}
	default:
		name := strings.TrimPrefix(r.URL.Path, "/yt-dlp/yt-dlp/releases/download/"+f.tag+"/")
		b, ok := f.files[name]
		if !ok || r.URL.Host != "github.com" {
			rec.WriteHeader(http.StatusNotFound)
			break
		}
		_, _ = rec.Write(b)
	}
	resp := rec.Result()
	resp.Request = r
	return resp, nil
}

func newUpdateService(t *testing.T, gh *fakeGitHub) *Service {
	t.Helper()
	dir := t.TempDir()
	s := New(Options{
		Dir: filepath.Join(dir, "ytdlp"), TmpDir: filepath.Join(dir, "tmp"),
		Cipher: auth.NewCrypto([]byte("synthetic test key")), Client: &http.Client{Transport: gh},
	})
	t.Cleanup(s.Close)
	return s
}

func waitInstall(t *testing.T, s *Service) InstallState {
	t.Helper()
	for i := 0; i < 400; i++ {
		s.mu.Lock()
		st := s.install
		s.mu.Unlock()
		if !st.Running {
			return st
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("install did not finish")
	return InstallState{}
}

func TestAssetName(t *testing.T) {
	cases := []struct {
		goos, arch string
		musl       bool
		want       string
	}{
		{"linux", "amd64", true, "yt-dlp_musllinux"},
		{"linux", "arm64", true, "yt-dlp_musllinux_aarch64"},
		{"linux", "amd64", false, "yt-dlp_linux"},
		{"linux", "arm64", false, "yt-dlp_linux_aarch64"},
		{"windows", "amd64", false, "yt-dlp.exe"},
		{"darwin", "arm64", false, "yt-dlp_macos"},
	}
	for _, c := range cases {
		if got, err := assetName(c.goos, c.arch, c.musl); err != nil || got != c.want {
			t.Errorf("assetName(%s, %s, %v) = %q, %v; want %q", c.goos, c.arch, c.musl, got, err, c.want)
		}
	}
	if _, err := assetName("linux", "riscv64", true); !errors.Is(err, ErrUnsupportedPlatform) {
		t.Errorf("riscv64: %v", err)
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"2026.08.19", "2026.08.19", 0},
		{"2026.08.19", "2026.09.01", -1},
		{"2026.10.01", "2026.09.30", 1},
		{"2026.08.19.1", "2026.08.19", 1},
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%s, %s) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestCheckLatest(t *testing.T) {
	gh := &fakeGitHub{tag: testTag}
	s := newUpdateService(t, gh)
	if v, err := s.CheckLatest(t.Context()); err != nil || v != testTag {
		t.Fatalf("CheckLatest = %q, %v", v, err)
	}
	if st := s.Status(t.Context()); st.Latest != testTag || st.CheckedAt == 0 || st.Installed {
		t.Fatalf("status %+v", st)
	}
	// A tag that is not a plain release version is refused (it becomes part of a URL).
	gh.tag = "../../evil"
	if _, err := s.CheckLatest(t.Context()); !errors.Is(err, ErrUpstream) {
		t.Fatalf("bad tag: %v", err)
	}
	gh.status = http.StatusForbidden
	if _, err := s.CheckLatest(t.Context()); !errors.Is(err, ErrUpstream) {
		t.Fatalf("rate limited: %v", err)
	}
}

func TestInstallRejectsChecksumMismatch(t *testing.T) {
	asset, err := platformAsset()
	if err != nil {
		t.Skip(err)
	}
	gh := &fakeGitHub{tag: testTag, files: map[string][]byte{asset: []byte("tampered")}}
	gh.sums = strings.Repeat("0", 64) + "  " + asset + "\n"
	s := newUpdateService(t, gh)
	if err := s.StartInstall(); err != nil {
		t.Fatal(err)
	}
	st := waitInstall(t, s)
	if !strings.Contains(st.Error, "checksum") {
		t.Fatalf("install error %q, want a checksum error", st.Error)
	}
	if _, err := os.Stat(s.binaryPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a binary was installed despite the mismatch: %v", err)
	}
	for _, c := range gh.calls {
		if strings.Contains(c, "evil.example.com") {
			t.Fatal("followed a URL from the API response")
		}
	}
}

func TestInstall(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake yt-dlp is a shell script")
	}
	asset, err := platformAsset()
	if err != nil {
		t.Skip(err)
	}
	script := []byte("#!/bin/sh\necho " + testTag + "\n")
	gh := &fakeGitHub{tag: testTag, files: map[string][]byte{asset: script}}
	s := newUpdateService(t, gh)
	if err := s.StartInstall(); err != nil {
		t.Fatal(err)
	}
	if st := waitInstall(t, s); st.Error != "" {
		t.Fatalf("install failed: %s", st.Error)
	}
	st := s.Status(t.Context())
	if !st.Installed || st.Version != testTag || st.Latest != testTag {
		t.Fatalf("status after install %+v", st)
	}
	if fi, err := os.Stat(s.binaryPath()); err != nil || fi.Mode().Perm()&0o100 == 0 {
		t.Fatalf("binary not executable: %v", err)
	}
	left, _ := filepath.Glob(filepath.Join(s.opt.Dir, ".yt-dlp-download-*"))
	if len(left) != 0 {
		t.Fatalf("temporary downloads left behind: %v", left)
	}
}

func TestInstallRefusedForOperatorBinary(t *testing.T) {
	s := New(Options{Dir: t.TempDir(), TmpDir: t.TempDir(), BinaryPath: filepath.Join(t.TempDir(), "yt-dlp")})
	defer s.Close()
	if err := s.StartInstall(); !errors.Is(err, ErrUnmanaged) {
		t.Fatalf("StartInstall = %v, want ErrUnmanaged", err)
	}
	if st := s.Status(t.Context()); st.Managed || st.Installed {
		t.Fatalf("status %+v", st)
	}
}

func TestRedirectsStayOnGitHub(t *testing.T) {
	for host, want := range map[string]bool{
		"github.com": true, "release-assets.githubusercontent.com": true, "objects.githubusercontent.com": true,
		"evil.example.com": false, "githubusercontent.com.example.com": false,
	} {
		if got := githubHost(host); got != want {
			t.Errorf("githubHost(%q) = %v", host, got)
		}
	}
}
