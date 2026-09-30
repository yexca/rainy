package ytdlp

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// The official release channel. Rainy asks the GitHub API for the latest tag, then builds
// the download URLs itself from that validated tag (it never follows a URL taken from the
// API response) and verifies the binary against the release's SHA2-256SUMS.
const (
	releaseAPI  = "https://api.github.com/repos/yt-dlp/yt-dlp/releases/latest"
	downloadURL = "https://github.com/yt-dlp/yt-dlp/releases/download/%s/%s"
	sumsAsset   = "SHA2-256SUMS"

	maxReleaseJSON = 2 << 20
	maxSums        = 64 << 10
	maxBinary      = 256 << 20
	installTimeout = 15 * time.Minute
)

// tagPattern matches yt-dlp release tags such as "2026.08.19" or "2026.08.19.1".
var tagPattern = regexp.MustCompile(`^\d{4}\.\d{2}\.\d{2}(?:\.\d+)?$`)

// assetName returns the release file for a platform. Alpine (the Docker image) needs the
// musl build; glibc systems use the plain Linux build.
func assetName(goos, goarch string, musl bool) (string, error) {
	switch goos + "/" + goarch {
	case "linux/amd64":
		if musl {
			return "yt-dlp_musllinux", nil
		}
		return "yt-dlp_linux", nil
	case "linux/arm64":
		if musl {
			return "yt-dlp_musllinux_aarch64", nil
		}
		return "yt-dlp_linux_aarch64", nil
	case "darwin/amd64", "darwin/arm64":
		return "yt-dlp_macos", nil
	case "windows/amd64":
		return "yt-dlp.exe", nil
	case "windows/arm64":
		return "yt-dlp_arm64.exe", nil
	}
	return "", fmt.Errorf("%w: no yt-dlp build for %s/%s; set RAINY_YTDLP_PATH to a yt-dlp you installed yourself", ErrUnsupportedPlatform, goos, goarch)
}

// isMusl reports whether the process runs on a musl libc system (Alpine).
func isMusl() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	m, _ := filepath.Glob("/lib/ld-musl-*.so.1")
	return len(m) > 0
}

func platformAsset() (string, error) { return assetName(runtime.GOOS, runtime.GOARCH, isMusl()) }

// compareVersions compares two yt-dlp versions ("2026.08.19", "2026.08.19.1") numerically.
func compareVersions(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < max(len(pa), len(pb)); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

// sumFor returns the SHA-256 hex digest of asset listed in a SHA2-256SUMS file.
func sumFor(sums []byte, asset string) (string, error) {
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == asset {
			if len(f[0]) == 64 {
				if _, err := hex.DecodeString(f[0]); err == nil {
					return strings.ToLower(f[0]), nil
				}
			}
		}
	}
	return "", fmt.Errorf("%w: %s is not listed in %s", ErrUpstream, asset, sumsAsset)
}

// CheckLatest asks GitHub for the latest yt-dlp release and remembers it for Status.
func (s *Service) CheckLatest(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	tag, err := s.latestTag(ctx)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	s.latest, s.checkedAt = tag, time.Now().UnixMilli()
	s.mu.Unlock()
	return tag, nil
}

func (s *Service) latestTag(ctx context.Context) (string, error) {
	body, err := s.get(ctx, releaseAPI, maxReleaseJSON, "application/vnd.github+json")
	if err != nil {
		return "", err
	}
	var rel struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &rel); err != nil || !tagPattern.MatchString(rel.TagName) {
		return "", fmt.Errorf("%w: unexpected answer from GitHub", ErrUpstream)
	}
	return rel.TagName, nil
}

// get fetches url with a size cap.
func (s *Service) get(ctx context.Context, target string, limit int64, accept string) ([]byte, error) {
	resp, err := s.request(ctx, target, accept)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("%w: reading %s: %v", ErrUpstream, hostOf(target), err)
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("%w: the answer from %s is too large", ErrUpstream, hostOf(target))
	}
	return b, nil
}

func (s *Service) request(ctx context.Context, target, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", s.opt.UserAgent)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w: cannot reach %s: %v", ErrUpstream, hostOf(target), unwrapURLError(err))
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("%w: %s answered %s", ErrUpstream, hostOf(target), resp.Status)
	}
	return resp, nil
}

// StartInstall installs (or updates to) the latest yt-dlp release in the background.
// Progress is reported by Status().Install.
func (s *Service) StartInstall() error {
	if !s.Managed() {
		return ErrUnmanaged
	}
	if _, err := platformAsset(); err != nil {
		return err
	}
	s.mu.Lock()
	if s.install.Running {
		s.mu.Unlock()
		return ErrBusy
	}
	s.install = InstallState{Running: true}
	s.mu.Unlock()
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ctx, cancel := context.WithTimeout(s.ctx, installTimeout)
		defer cancel()
		version, err := s.install1(ctx)
		s.mu.Lock()
		defer s.mu.Unlock()
		s.install = InstallState{FinishedAt: time.Now().UnixMilli()}
		if err != nil {
			s.install.Error = err.Error()
			slog.Warn("ytdlp: installing yt-dlp failed", "err", err)
			return
		}
		s.latest, s.checkedAt = version, time.Now().UnixMilli()
		slog.Info("ytdlp: installed yt-dlp", "version", version)
	}()
	return nil
}

// install1 downloads the latest release for this platform, verifies its checksum and that
// it runs, then atomically replaces the managed binary.
func (s *Service) install1(ctx context.Context) (string, error) {
	asset, err := platformAsset()
	if err != nil {
		return "", err
	}
	tag, err := s.latestTag(ctx)
	if err != nil {
		return "", err
	}
	sums, err := s.get(ctx, fmt.Sprintf(downloadURL, tag, sumsAsset), maxSums, "")
	if err != nil {
		return "", err
	}
	want, err := sumFor(sums, asset)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(s.opt.Dir, 0o755); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(s.opt.Dir, ".yt-dlp-download-*")
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	defer func() { _ = os.Remove(tmp) }()
	resp, err := s.request(ctx, fmt.Sprintf(downloadURL, tag, asset), "application/octet-stream")
	if err != nil {
		_ = f.Close()
		return "", err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, maxBinary+1))
	_ = resp.Body.Close()
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	switch {
	case err != nil && ctx.Err() != nil:
		return "", ctx.Err()
	case err != nil:
		return "", fmt.Errorf("%w: downloading %s: %v", ErrUpstream, asset, err)
	case n > maxBinary:
		return "", fmt.Errorf("%w: %s is unexpectedly large", ErrUpstream, asset)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return "", fmt.Errorf("%w: the checksum of the downloaded %s does not match %s; nothing was installed", ErrUpstream, asset, sumsAsset)
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		return "", err
	}
	v, err := s.probe(ctx, tmp)
	if err != nil {
		return "", fmt.Errorf("the downloaded yt-dlp cannot run on this server (%v); if the data folder is mounted noexec, install yt-dlp yourself and set RAINY_YTDLP_PATH", err)
	}
	if v != tag {
		return "", fmt.Errorf("%w: the downloaded yt-dlp reports version %q instead of %q", ErrUpstream, v, tag)
	}
	if err := os.Rename(tmp, s.binaryPath()); err != nil {
		return "", fmt.Errorf("replacing yt-dlp: %w", err)
	}
	s.forgetVersion()
	return tag, nil
}

// newHTTPClient returns the client for GitHub requests: proxies from the environment,
// bounded timeouts, HTTPS only, and redirects only to GitHub's own download hosts.
func newHTTPClient() *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = http.ProxyFromEnvironment
	tr.ResponseHeaderTimeout = 30 * time.Second
	return &http.Client{
		Transport: tr,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			if req.URL.Scheme != "https" || !githubHost(req.URL.Hostname()) {
				return fmt.Errorf("refusing redirect to %s", req.URL.Hostname())
			}
			return nil
		},
	}
}

func githubHost(h string) bool {
	h = strings.ToLower(h)
	return h == "github.com" || h == "api.github.com" || strings.HasSuffix(h, ".githubusercontent.com")
}

func hostOf(u string) string {
	if i := strings.Index(u, "://"); i >= 0 {
		u = u[i+3:]
	}
	if i := strings.IndexAny(u, "/?#"); i >= 0 {
		u = u[:i]
	}
	return u
}

// unwrapURLError drops the URL from *url.Error messages.
func unwrapURLError(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}
