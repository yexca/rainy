package lxmusic

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// MaxScriptSize bounds a source script (lx-music accepts up to 9,000,000 characters).
const MaxScriptSize = 9_000_000

// ScriptInfo is the header of a source script:
//
//	/**
//	 * @name Example source
//	 * @description …
//	 * @version 1.0.0
//	 * @author …
//	 * @homepage https://example.com
//	 */
type ScriptInfo struct {
	Name        string
	Description string
	Version     string
	Author      string
	Homepage    string
}

// Header fields and their maximum length in characters (longer values are cut and end in
// "...", like lx-music does).
var infoLimits = []struct {
	key   string
	limit int
}{
	{"name", 24},
	{"description", 36},
	{"author", 56},
	{"homepage", 1024},
	{"version", 36},
}

var (
	headerComment = regexp.MustCompile(`^/\*[\s\S]+?\*/`)
	headerLine    = regexp.MustCompile(`^\s?\*\s?@(\w+)\s(.+)$`)
)

// NormalizeScript strips a UTF-8 byte order mark and checks the size and encoding.
func NormalizeScript(script string) (string, error) {
	script = strings.TrimPrefix(script, byteOrderMark)
	if strings.TrimSpace(script) == "" {
		return "", fmt.Errorf("%w: the script is empty", ErrInvalid)
	}
	if len(script) > MaxScriptSize {
		return "", fmt.Errorf("%w: the script is larger than %d bytes", ErrInvalid, MaxScriptSize)
	}
	if !utf8.ValidString(script) {
		return "", fmt.Errorf("%w: the script is not UTF-8 text", ErrInvalid)
	}
	if strings.ContainsRune(script, 0) {
		return "", fmt.Errorf("%w: the script contains NUL bytes", ErrInvalid)
	}
	return script, nil
}

// ParseScript reads the header comment, which must open the script. The name defaults to
// "Source" when the header has none.
func ParseScript(script string) (ScriptInfo, error) {
	header := headerComment.FindString(script)
	if header == "" {
		return ScriptInfo{}, fmt.Errorf("%w: not a source script (it must start with a /** @name … */ comment)", ErrInvalid)
	}
	values := map[string]string{}
	for _, line := range strings.Split(strings.ReplaceAll(header, "\r\n", "\n"), "\n") {
		m := headerLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		values[m[1]] = strings.TrimSpace(m[2])
	}
	for _, f := range infoLimits {
		v := values[f.key]
		if utf8.RuneCountInString(v) > f.limit {
			v = string([]rune(v)[:f.limit]) + "..."
		}
		values[f.key] = v
	}
	info := ScriptInfo{
		Name:        values["name"],
		Description: values["description"],
		Version:     values["version"],
		Author:      values["author"],
		Homepage:    values["homepage"],
	}
	if info.Name == "" {
		info.Name = "Source"
	}
	if !validHomepage(info.Homepage) {
		info.Homepage = ""
	}
	return info, nil
}

// validHomepage accepts only http(s) links, so the web app never renders a javascript: URL.
func validHomepage(s string) bool {
	lower := strings.ToLower(s)
	return (strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://")) &&
		!strings.ContainsAny(s, " \t\r\n\"'<>`")
}

// ScriptHash identifies a script's content (duplicate detection).
func ScriptHash(script string) string {
	sum := sha256.Sum256([]byte(script))
	return hex.EncodeToString(sum[:])
}
