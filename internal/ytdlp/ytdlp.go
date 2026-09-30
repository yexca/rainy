// Package ytdlp downloads audio from YouTube and bilibili with yt-dlp
// (https://github.com/yt-dlp/yt-dlp) and manages the yt-dlp binary and the sign-in cookies
// it may use (docs/architecture/contract.md §5.14).
//
// Everything here is opt-in: the API refuses downloads, update checks and installs unless an
// administrator turned on settings.ytdlpEnabled. By default Rainy installs the official
// release build into <data>/ytdlp (checksum-verified, see update.go); an operator can point
// RAINY_YTDLP_PATH at a binary they manage themselves instead.
//
// yt-dlp is always started with a fixed argument list: --ignore-config and --no-plugin-dirs
// so no configuration file or plugin can add options, the generic extractor disabled so it
// only talks to the supported sites, the link last after "--", and its output confined to a
// work directory the caller owns. The package never writes into a library; manage imports the
// downloaded files like uploads.
package ytdlp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Errors.
var (
	ErrInvalid             = errors.New("invalid input")
	ErrNotInstalled        = errors.New("yt-dlp is not installed")
	ErrUnmanaged           = errors.New("yt-dlp is managed by the server administrator (RAINY_YTDLP_PATH); update it there")
	ErrBusy                = errors.New("yt-dlp is already being installed")
	ErrUpstream            = errors.New("upstream error")
	ErrUnsupportedPlatform = errors.New("unsupported platform")
	ErrNoFFmpeg            = errors.New("ffmpeg is not installed; yt-dlp needs it to extract audio")
)

// Cipher encrypts stored cookies (implemented by *auth.Crypto).
type Cipher interface {
	Encrypt(plain string) (string, error)
	Decrypt(enc string) (string, error)
}

// Options configures the service.
type Options struct {
	Dir        string // <data>/ytdlp: managed binary, cache and encrypted cookies
	TmpDir     string // <data>/tmp: short-lived work directories
	BinaryPath string // RAINY_YTDLP_PATH; "" = the managed binary in Dir
	FFmpegPath string // ffmpeg binary (name or path)
	Cipher     Cipher
	Client     *http.Client // GitHub requests; nil → proxies from the environment, GitHub redirects only
	UserAgent  string
}

// InstallState reports a background install.
type InstallState struct {
	Running    bool   `json:"running"`
	Error      string `json:"error"`
	FinishedAt int64  `json:"finishedAt"`
}

// Status describes the yt-dlp installation.
type Status struct {
	Managed   bool         `json:"managed"`   // Rainy installs and updates the binary
	Installed bool         `json:"installed"` // the binary exists and runs
	Version   string       `json:"version"`
	Error     string       `json:"error"`     // why an existing binary cannot be used
	Latest    string       `json:"latest"`    // latest release from the last check ("" = never checked)
	CheckedAt int64        `json:"checkedAt"` // ms
	Asset     string       `json:"asset"`     // release file for this platform ("" = none)
	JSRuntime string       `json:"jsRuntime"` // deno | node | quickjs | "" (YouTube needs one)
	FFmpeg    bool         `json:"ffmpeg"`
	Install   InstallState `json:"install"`
}

// Service runs yt-dlp and manages its binary and cookies.
type Service struct {
	opt    Options
	client *http.Client
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu        sync.Mutex
	ver       versionCache
	latest    string
	checkedAt int64
	install   InstallState

	cookieMu sync.Mutex
}

type versionCache struct {
	path    string
	size    int64
	mod     time.Time
	version string
	err     error
}

// New creates the service and removes work directories left behind by a crash.
func New(opt Options) *Service {
	if opt.Client == nil {
		opt.Client = newHTTPClient()
	}
	if opt.UserAgent == "" {
		opt.UserAgent = "Rainy"
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{opt: opt, client: opt.Client, ctx: ctx, cancel: cancel}
	for _, pattern := range []string{filepath.Join(opt.TmpDir, "ytdlp-*"), filepath.Join(opt.Dir, ".yt-dlp-download-*")} {
		if stale, _ := filepath.Glob(pattern); len(stale) > 0 {
			for _, p := range stale {
				_ = os.RemoveAll(p)
			}
		}
	}
	return s
}

// Close stops a running install and waits for it.
func (s *Service) Close() {
	s.cancel()
	s.wg.Wait()
}

// Managed reports whether Rainy installs and updates the binary itself.
func (s *Service) Managed() bool { return s.opt.BinaryPath == "" }

func (s *Service) binaryPath() string {
	if !s.Managed() {
		return s.opt.BinaryPath
	}
	name := "yt-dlp"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(s.opt.Dir, name)
}

// binary returns the path of a runnable yt-dlp and its version.
func (s *Service) binary(ctx context.Context) (string, string, error) {
	p := s.binaryPath()
	if !s.Managed() && !strings.ContainsAny(p, `/\`) {
		lp, err := exec.LookPath(p)
		if err != nil {
			return "", "", ErrNotInstalled
		}
		p = lp
	}
	fi, err := os.Stat(p)
	if err != nil || fi.IsDir() {
		return "", "", ErrNotInstalled
	}
	s.mu.Lock()
	c := s.ver
	s.mu.Unlock()
	if c.path == p && c.size == fi.Size() && c.mod.Equal(fi.ModTime()) {
		return p, c.version, c.err
	}
	v, err := s.probe(ctx, p)
	if err != nil {
		err = fmt.Errorf("yt-dlp cannot run: %w", err)
	}
	s.mu.Lock()
	s.ver = versionCache{path: p, size: fi.Size(), mod: fi.ModTime(), version: v, err: err}
	s.mu.Unlock()
	return p, v, err
}

func (s *Service) forgetVersion() {
	s.mu.Lock()
	s.ver = versionCache{}
	s.mu.Unlock()
}

// probe runs "<bin> --version".
func (s *Service) probe(ctx context.Context, bin string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	work, err := s.workDir("ytdlp-probe-*")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(work) }()
	cmd := exec.CommandContext(ctx, bin, "--ignore-config", "--no-plugin-dirs", "--version")
	cmd.Dir = work
	cmd.Env = childEnv(work)
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	setProcessGroup(cmd)
	err = cmd.Run()
	killProcessGroup(cmd)
	if err != nil {
		if msg := lastLine(stderr.String()); msg != "" {
			return "", fmt.Errorf("%v: %s", err, msg)
		}
		return "", err
	}
	v := strings.TrimSpace(out.String())
	if !tagPattern.MatchString(v) {
		return "", fmt.Errorf("unexpected version output %q", truncate(v, 60))
	}
	return v, nil
}

// workDir creates a private temporary directory under TmpDir.
func (s *Service) workDir(pattern string) (string, error) {
	if err := os.MkdirAll(s.opt.TmpDir, 0o755); err != nil {
		return "", err
	}
	return os.MkdirTemp(s.opt.TmpDir, pattern)
}

// childEnv is the environment of yt-dlp: the server's (for proxies and PATH) with the
// temporary directory (PyInstaller unpacks itself there) inside the work directory.
func childEnv(tmp string) []string {
	env := make([]string, 0, len(os.Environ())+4)
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		switch strings.ToUpper(k) {
		case "TMPDIR", "TMP", "TEMP", "PYTHONIOENCODING":
			continue
		}
		env = append(env, kv)
	}
	return append(env, "TMPDIR="+tmp, "TMP="+tmp, "TEMP="+tmp, "PYTHONIOENCODING=utf-8")
}

// Status reports the installation (running "yt-dlp --version" when the binary changed).
func (s *Service) Status(ctx context.Context) Status {
	st := Status{Managed: s.Managed()}
	if a, err := platformAsset(); err == nil {
		st.Asset = a
	}
	_, v, err := s.binary(ctx)
	switch {
	case errors.Is(err, ErrNotInstalled):
	case err != nil:
		st.Error = err.Error()
	default:
		st.Installed, st.Version = true, v
	}
	st.JSRuntime, _ = jsRuntime()
	_, ffErr := s.ffmpeg()
	st.FFmpeg = ffErr == nil
	s.mu.Lock()
	st.Latest, st.CheckedAt, st.Install = s.latest, s.checkedAt, s.install
	s.mu.Unlock()
	return st
}

// Ready returns nil when downloads can run: yt-dlp is installed and runs, and ffmpeg exists.
func (s *Service) Ready(ctx context.Context) error {
	if _, _, err := s.binary(ctx); err != nil {
		return err
	}
	_, err := s.ffmpeg()
	return err
}

func (s *Service) ffmpeg() (string, error) {
	name := s.opt.FFmpegPath
	if name == "" {
		name = "ffmpeg"
	}
	p, err := exec.LookPath(name)
	if err != nil {
		return "", ErrNoFFmpeg
	}
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	return p, nil
}

// jsRuntime finds the JavaScript runtime yt-dlp needs to solve YouTube's challenges, in
// yt-dlp's order of preference. The Docker image ships QuickJS; Deno, when present, runs
// the scripts in a sandbox and is preferred.
func jsRuntime() (name, path string) {
	for _, c := range []struct{ name, bin string }{{"deno", "deno"}, {"node", "node"}, {"quickjs", "qjs"}} {
		if p, err := exec.LookPath(c.bin); err == nil {
			if abs, err := filepath.Abs(p); err == nil {
				p = abs
			}
			return c.name, p
		}
	}
	return "", ""
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return truncate(strings.TrimSpace(lines[len(lines)-1]), 300)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
