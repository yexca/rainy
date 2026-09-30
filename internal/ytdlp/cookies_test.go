package ytdlp

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"rainy/internal/auth"
)

// Synthetic cookie values; they must never show up in stored files or API answers.
const (
	valueSess  = "synthetic-sessdata-value"
	valueOther = "synthetic-unrelated-value"
)

func cookieFile(lines ...string) string {
	return "# Netscape HTTP Cookie File\n# exported for a test\n\n" + strings.Join(lines, "\r\n") + "\n"
}

func future() string { return strconv.FormatInt(time.Now().Add(24*time.Hour).Unix(), 10) }

func newTestService(t *testing.T) *Service {
	t.Helper()
	dir := t.TempDir()
	s := New(Options{
		Dir: filepath.Join(dir, "ytdlp"), TmpDir: filepath.Join(dir, "tmp"),
		Cipher: auth.NewCrypto([]byte("synthetic test key")),
	})
	t.Cleanup(s.Close)
	return s
}

func TestParseCookiesKeepsOnlyTheSite(t *testing.T) {
	text := cookieFile(
		".bilibili.com\tTRUE\t/\tFALSE\t"+future()+"\tSESSDATA\t"+valueSess,
		"#HttpOnly_.bilibili.com\tTRUE\t/\tTRUE\t0\tbili_jct\tsynthetic-csrf",
		"www.bilibili.com\tFALSE\t/\tFALSE\t0\tempty_value", // value dropped by the exporter
		".mail.example.com\tTRUE\t/\tTRUE\t0\tSID\t"+valueOther,
		".notbilibili.com\tTRUE\t/\tTRUE\t0\tSESSDATA\t"+valueOther,
		"garbage line",
	)
	kept, dropped, err := parseCookies(SiteBilibili, text)
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 3 || dropped != 2 {
		t.Fatalf("kept %d, dropped %d; want 3 and 2", len(kept), dropped)
	}
	out := formatCookies(SiteBilibili, kept)
	if !strings.HasPrefix(out, cookieHeader+"\n") {
		t.Errorf("stored text does not start with the Netscape header: %q", out)
	}
	if strings.Contains(out, valueOther) || !strings.Contains(out, "#HttpOnly_.bilibili.com\tTRUE\t/\tTRUE\t0\tbili_jct\t") {
		t.Errorf("unexpected stored text:\n%s", out)
	}
	info := summarize(SiteBilibili, kept, time.Now())
	if !info.SignedIn || info.ExpiresAt == 0 || info.Count != 3 {
		t.Errorf("summary %+v", info)
	}
}

func TestParseCookiesRejects(t *testing.T) {
	cases := map[string]string{
		"empty":      "",
		"json":       `[{"domain":".bilibili.com","name":"SESSDATA","value":"x"}]`,
		"no cookies": "# Netscape HTTP Cookie File\n",
		"other site": cookieFile(".youtube.com\tTRUE\t/\tTRUE\t0\tSID\t" + valueOther),
		"too large":  strings.Repeat("x", maxCookieText+1),
	}
	for name, text := range cases {
		_, _, err := parseCookies(SiteBilibili, text)
		if !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
			continue
		}
		if strings.Contains(err.Error(), valueOther) {
			t.Errorf("%s: the error quotes the input: %v", name, err)
		}
	}
	if _, _, err := parseCookies("example", cookieFile(".example.com\tTRUE\t/\tTRUE\t0\ta\tb")); !errors.Is(err, ErrInvalid) {
		t.Errorf("unknown site: %v", err)
	}
}

func TestExpiredLoginCookieIsNotSignedIn(t *testing.T) {
	past := strconv.FormatInt(time.Now().Add(-time.Hour).Unix(), 10)
	kept, _, err := parseCookies(SiteYouTube, cookieFile(
		".youtube.com\tTRUE\t/\tTRUE\t"+past+"\tLOGIN_INFO\tsynthetic",
		".youtube.com\tTRUE\t/\tTRUE\t0\tPREF\tsynthetic",
	))
	if err != nil {
		t.Fatal(err)
	}
	if info := summarize(SiteYouTube, kept, time.Now()); info.SignedIn {
		t.Errorf("expired sign-in cookie reported as signed in: %+v", info)
	}
}

func TestCookieStorageIsEncryptedAndPrivate(t *testing.T) {
	s := newTestService(t)
	res, err := s.SetCookies(SiteBilibili, cookieFile(
		".bilibili.com\tTRUE\t/\tFALSE\t"+future()+"\tSESSDATA\t"+valueSess,
		".mail.example.com\tTRUE\t/\tTRUE\t0\tSID\t"+valueOther,
	))
	if err != nil {
		t.Fatal(err)
	}
	if res.Count != 1 || res.Dropped != 1 || !res.SignedIn {
		t.Fatalf("save result %+v", res)
	}
	stored, err := os.ReadFile(s.cookiePath(SiteBilibili))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(stored), valueSess) || strings.Contains(string(stored), "SESSDATA") {
		t.Fatal("stored cookies are not encrypted")
	}
	if runtime.GOOS != "windows" {
		for _, p := range []string{s.cookiePath(SiteBilibili), filepath.Dir(s.cookiePath(SiteBilibili))} {
			fi, err := os.Stat(p)
			if err != nil {
				t.Fatal(err)
			}
			if fi.Mode().Perm()&0o077 != 0 {
				t.Errorf("%s is readable by others: %v", p, fi.Mode().Perm())
			}
		}
	}

	infos := s.Cookies()
	if len(infos) != 2 || infos[0].Site != SiteYouTube || infos[0].Configured || !infos[1].Configured || infos[1].Count != 1 {
		t.Fatalf("Cookies() = %+v", infos)
	}
	if !s.HasCookies(SiteBilibili) || s.HasCookies(SiteYouTube) {
		t.Fatal("HasCookies mismatch")
	}

	// A run gets a private plain-text copy in its own work directory.
	work := t.TempDir()
	p, err := s.writeRunCookies(SiteBilibili, work)
	if err != nil || p == "" {
		t.Fatalf("writeRunCookies = %q, %v", p, err)
	}
	copyText, _ := os.ReadFile(p)
	if !strings.Contains(string(copyText), valueSess) || strings.Contains(string(copyText), valueOther) {
		t.Fatalf("run copy:\n%s", copyText)
	}
	if p, err := s.writeRunCookies(SiteYouTube, work); p != "" || err != nil {
		t.Fatalf("no YouTube cookies expected, got %q, %v", p, err)
	}

	// A different secret key cannot read them: they are ignored instead of failing.
	other := New(Options{Dir: s.opt.Dir, TmpDir: s.opt.TmpDir, Cipher: auth.NewCrypto([]byte("another key"))})
	defer other.Close()
	if other.HasCookies(SiteBilibili) {
		t.Fatal("cookies readable with another key")
	}

	if err := s.DeleteCookies(SiteBilibili); err != nil {
		t.Fatal(err)
	}
	if s.HasCookies(SiteBilibili) {
		t.Fatal("cookies still present after delete")
	}
	if err := s.DeleteCookies(SiteBilibili); err != nil {
		t.Fatalf("deleting twice: %v", err)
	}
}
