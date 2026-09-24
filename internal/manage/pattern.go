package manage

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/util"
)

// maxComponent is the maximum length in bytes of one path component produced by the
// rename engine (most file systems allow 255; the margin leaves room for " (n)" suffixes
// and sidecar extensions).
const maxComponent = 200

// Pattern is a parsed rename pattern (docs/architecture/contract.md §7.6):
//
//	{title} {artist} {album} {albumartist} {track} {track:N} {disc} {disc:N} {year} {genre} {composer}
//
// "/" separates directories, "[...]" is an optional segment dropped when every token
// inside it is empty. The original extension is appended by the caller.
type Pattern struct {
	src   string
	nodes []patNode
}

type patNode struct {
	lit      string    // literal text (may contain "/")
	token    string    // token name ("" for literals and groups)
	width    int       // zero-padding width for {track:N} / {disc:N}
	optional []patNode // non-nil for "[...]" groups
}

var patternTokens = map[string]bool{
	"title": true, "artist": true, "album": true, "albumartist": true, "track": true,
	"disc": true, "year": true, "genre": true, "composer": true,
}

// ParsePattern parses and validates a rename pattern. Errors wrap store.ErrInvalid.
func ParsePattern(src string) (*Pattern, error) {
	if strings.TrimSpace(src) == "" {
		return nil, fmt.Errorf("%w: the pattern is empty", store.ErrInvalid)
	}
	if len(src) > 1024 {
		return nil, fmt.Errorf("%w: the pattern is too long", store.ErrInvalid)
	}
	p := &Pattern{src: src}
	nodes, rest, err := parseNodes(src, false)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", store.ErrInvalid, err)
	}
	if rest != "" {
		return nil, fmt.Errorf("%w: unexpected %q", store.ErrInvalid, rest[:1])
	}
	p.nodes = nodes
	hasToken := false
	walkNodes(nodes, func(n patNode) {
		if n.token != "" {
			hasToken = true
		}
	})
	if !hasToken {
		return nil, fmt.Errorf("%w: the pattern must contain at least one token such as {title}", store.ErrInvalid)
	}
	return p, nil
}

// String returns the pattern source.
func (p *Pattern) String() string { return p.src }

func walkNodes(nodes []patNode, fn func(patNode)) {
	for _, n := range nodes {
		fn(n)
		walkNodes(n.optional, fn)
	}
}

// parseNodes parses until the end of s or, inside a group, until the closing "]".
func parseNodes(s string, inGroup bool) (nodes []patNode, rest string, err error) {
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			nodes = append(nodes, patNode{lit: lit.String()})
			lit.Reset()
		}
	}
	for len(s) > 0 {
		switch s[0] {
		case '{':
			end := strings.IndexByte(s, '}')
			if end < 0 {
				return nil, "", fmt.Errorf("unclosed \"{\"")
			}
			name, width, err := parseToken(s[1:end])
			if err != nil {
				return nil, "", err
			}
			flush()
			nodes = append(nodes, patNode{token: name, width: width})
			s = s[end+1:]
		case '}':
			return nil, "", fmt.Errorf("unexpected \"}\"")
		case '[':
			if inGroup {
				return nil, "", fmt.Errorf("optional segments cannot be nested")
			}
			flush()
			inner, r, err := parseNodes(s[1:], true)
			if err != nil {
				return nil, "", err
			}
			if !strings.HasPrefix(r, "]") {
				return nil, "", fmt.Errorf("unclosed \"[\"")
			}
			nodes = append(nodes, patNode{optional: append([]patNode{}, inner...)})
			s = r[1:]
		case ']':
			if !inGroup {
				return nil, "", fmt.Errorf("unexpected \"]\"")
			}
			flush()
			return nodes, s, nil
		default:
			r, size := utf8.DecodeRuneInString(s)
			lit.WriteRune(r)
			s = s[size:]
		}
	}
	if inGroup {
		return nil, "", fmt.Errorf("unclosed \"[\"")
	}
	flush()
	return nodes, "", nil
}

func parseToken(body string) (name string, width int, err error) {
	name, arg, hasArg := strings.Cut(strings.TrimSpace(body), ":")
	name = strings.ToLower(strings.TrimSpace(name))
	if !patternTokens[name] {
		return "", 0, fmt.Errorf("unknown token {%s}", body)
	}
	if hasArg {
		if name != "track" && name != "disc" {
			return "", 0, fmt.Errorf("{%s} does not take a width", name)
		}
		width, err = strconv.Atoi(strings.TrimSpace(arg))
		if err != nil || width < 1 || width > 9 {
			return "", 0, fmt.Errorf("invalid width in {%s} (1-9)", body)
		}
	}
	return name, width, nil
}

// Values are the inputs of a pattern.
type Values struct {
	Title, Artist, Album, AlbumArtist, Genre, Composer string
	Track, Disc, Year                                  int
	// MultiDisc makes {disc} render; for single-disc albums {disc} is empty so the
	// default pattern's "[{disc}-]" disappears.
	MultiDisc bool
}

// trackValues extracts pattern values from a track row. multiDisc should be true when
// the track's album spans more than one disc.
func trackValues(t *model.Track, multiDisc bool) Values {
	genre := ""
	if gs := model.SplitGenre(t.Genre); len(gs) > 0 {
		genre = gs[0]
	}
	return Values{
		Title: t.Title, Artist: t.Artist, Album: t.Album, AlbumArtist: t.AlbumArtist,
		Genre: genre, Composer: t.Composer,
		Track: t.TrackNumber, Disc: t.DiscNumber, Year: t.Year,
		MultiDisc: multiDisc || t.DiscTotal > 1,
	}
}

func (v Values) token(name string, width int) string {
	num := func(n int) string {
		if n <= 0 {
			return ""
		}
		s := strconv.Itoa(n)
		for len(s) < width {
			s = "0" + s
		}
		return s
	}
	switch name {
	case "title":
		return v.Title
	case "artist":
		return v.Artist
	case "album":
		return v.Album
	case "albumartist":
		return v.AlbumArtist
	case "genre":
		return v.Genre
	case "composer":
		return v.Composer
	case "track":
		return num(v.Track)
	case "disc":
		if !v.MultiDisc {
			return ""
		}
		return num(v.Disc)
	case "year":
		return num(v.Year)
	}
	return ""
}

// Render expands the pattern and returns the sanitized library-relative path without the
// extension ("Artist/Album/01 Title"). ext (without dot, may be "") is used to keep the
// final component within maxComponent bytes. Errors wrap store.ErrInvalid.
func (p *Pattern) Render(v Values, ext string) (string, error) {
	var b strings.Builder
	renderNodes(&b, p.nodes, v)
	raw := strings.Split(b.String(), "/")
	parts := make([]string, 0, len(raw))
	for i, c := range raw {
		last := i == len(raw)-1
		limit := maxComponent
		if last && ext != "" {
			limit -= len(ext) + 1
		}
		c = sanitizeComponent(c, limit)
		if c == "" {
			if last {
				return "", fmt.Errorf("%w: the pattern produces an empty file name", store.ErrInvalid)
			}
			continue // e.g. an empty {genre}/ directory level
		}
		parts = append(parts, c)
	}
	return strings.Join(parts, "/"), nil
}

func renderNodes(b *strings.Builder, nodes []patNode, v Values) {
	for _, n := range nodes {
		switch {
		case n.optional != nil:
			var inner strings.Builder
			tokens, filled := 0, 0
			for _, c := range n.optional {
				if c.token != "" {
					tokens++
					if v.token(c.token, c.width) != "" {
						filled++
					}
				}
			}
			if tokens > 0 && filled == 0 {
				continue
			}
			renderNodes(&inner, n.optional, v)
			b.WriteString(inner.String())
		case n.token != "":
			// Values never create directories: "EX/AMPLE" → "EX_AMPLE".
			b.WriteString(replaceIllegal(v.token(n.token, n.width)))
		default:
			b.WriteString(n.lit)
		}
	}
}

// replaceIllegal replaces characters that are invalid in file names on common file
// systems (<>:"/\|?* and control characters) with "_".
func replaceIllegal(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*':
			return '_'
		}
		if unicode.IsControl(r) || r == utf8.RuneError {
			return '_'
		}
		return r
	}, s)
}

// windowsReserved are device names Windows refuses as file names (with any extension).
var windowsReserved = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true, "com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true, "lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// sanitizeComponent makes one path component safe: illegal characters → "_", leading
// spaces and trailing dots/spaces trimmed, Windows device names suffixed with "_", names
// the scanner skips (hidden ".x", NAS system folders such as "@eaDir") prefixed with "_",
// at most limit bytes (cut on a UTF-8 boundary). "." and ".." become "".
func sanitizeComponent(c string, limit int) string {
	c = replaceIllegal(c)
	c = strings.TrimLeft(c, " ")
	c = strings.TrimRight(c, ". ")
	if c == "" {
		return ""
	}
	if util.IsHiddenOrSystem(c) {
		c = "_" + c
	}
	if windowsReserved[strings.ToLower(strings.SplitN(c, ".", 2)[0])] {
		c += "_"
	}
	c = truncateBytes(c, limit)
	return strings.TrimRight(c, ". ")
}

// truncateBytes cuts s to at most n bytes without splitting a UTF-8 sequence.
func truncateBytes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// renderPath renders p for a track and appends ext (the original extension, without dot).
func renderPath(p *Pattern, v Values, ext string) (string, error) {
	rel, err := p.Render(v, ext)
	if err != nil {
		return "", err
	}
	if ext != "" {
		rel += "." + ext
	}
	return rel, nil
}
