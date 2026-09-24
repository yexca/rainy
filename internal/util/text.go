package util

import (
	"strings"
	"unicode"

	"github.com/mozillazg/go-pinyin"
	"golang.org/x/text/unicode/norm"
)

// NormalizeSearch builds the normalised form used for search_text columns and search
// queries. The parts are joined with a space, then:
//   - Unicode NFKD decomposition (full-width → ASCII, ligatures, compatibility forms),
//   - combining marks stripped ("Beyoncé" → "beyonce", "ガ" → "カ"),
//   - lower-cased,
//   - apostrophes removed ("Don't" → "dont"), other punctuation and symbols become spaces,
//   - whitespace collapsed and trimmed.
//
// CJK characters are kept as-is, so substring matching works for Chinese/Japanese text.
func NormalizeSearch(parts ...string) string {
	s := norm.NFKD.String(strings.Join(parts, " "))
	var b strings.Builder
	b.Grow(len(s))
	space := true // suppress leading spaces
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r):
			continue
		case isApostrophe(r):
			continue
		case unicode.IsSpace(r) || unicode.IsControl(r) || unicode.IsPunct(r) || unicode.IsSymbol(r):
			if !space {
				b.WriteByte(' ')
				space = true
			}
		default:
			b.WriteRune(unicode.ToLower(r))
			space = false
		}
	}
	return strings.TrimRight(b.String(), " ")
}

func isApostrophe(r rune) bool {
	switch r {
	case '\'', '’', '‘', '`', '´', 'ʼ':
		return true
	}
	return false
}

// SearchTokens returns the distinct tokens of NormalizeSearch(q); nil for an empty query.
func SearchTokens(q string) []string {
	fields := strings.Fields(NormalizeSearch(q))
	if len(fields) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(fields))
	out := fields[:0]
	for _, f := range fields {
		if !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	return out
}

// EscapeLike escapes %, _ and \ for use in a LIKE pattern with ESCAPE '\'.
func EscapeLike(s string) string {
	if !strings.ContainsAny(s, `%_\`) {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if r == '%' || r == '_' || r == '\\' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// StripArticle removes a leading article (case-insensitive, must be followed by a space)
// from name: StripArticle("The Beatles", {"The"}) == "Beatles". The name is returned
// (trimmed) unchanged when nothing would remain.
func StripArticle(name string, ignoredArticles []string) string {
	name = strings.TrimSpace(name)
	for _, a := range ignoredArticles {
		if a == "" || len(name) <= len(a)+1 {
			continue
		}
		if strings.EqualFold(name[:len(a)], a) && name[len(a)] == ' ' {
			if rest := strings.TrimSpace(name[len(a)+1:]); rest != "" {
				return rest
			}
		}
	}
	return name
}

// SortName returns the sort key for a name: leading article stripped, then NormalizeSearch
// (lower-case, accents removed). SortName("The Beatles", {"The"}) == "beatles".
func SortName(name string, ignoredArticles []string) string {
	return NormalizeSearch(StripArticle(name, ignoredArticles))
}

var pinyinArgs = func() pinyin.Args {
	a := pinyin.NewArgs()
	a.Style = pinyin.FirstLetter
	return a
}()

// IndexKey returns the alphabetical index bucket of a name: "A".."Z" or "#". A leading
// article is ignored, accents and full-width forms are folded, leading punctuation is
// skipped, and a Han character maps to the initial of its pinyin ("周杰伦" → "Z").
// Digits, kana, hangul and anything else map to "#".
func IndexKey(name string, ignoredArticles []string) string {
	s := norm.NFKD.String(StripArticle(name, ignoredArticles))
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Mn, r), unicode.IsSpace(r), unicode.IsPunct(r), unicode.IsSymbol(r), unicode.IsControl(r):
			continue
		case r >= 'a' && r <= 'z':
			return string(r - 'a' + 'A')
		case r >= 'A' && r <= 'Z':
			return string(r)
		case unicode.Is(unicode.Han, r):
			if py := pinyin.SinglePinyin(r, pinyinArgs); len(py) > 0 && py[0] != "" {
				if c := py[0][0]; c >= 'a' && c <= 'z' {
					return string(rune(c - 'a' + 'A'))
				}
			}
			return "#"
		default:
			return "#"
		}
	}
	return "#"
}

// SplitMulti splits each value on any rune contained in seps, trims the parts, drops empty
// ones and removes case-insensitive duplicates (first spelling wins, order preserved).
// With empty seps the values are only trimmed and de-duplicated. Never returns nil.
func SplitMulti(values []string, seps string) []string {
	out := []string{}
	seen := map[string]bool{}
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		k := strings.ToLower(p)
		if seen[k] {
			return
		}
		seen[k] = true
		out = append(out, p)
	}
	for _, v := range values {
		if seps == "" {
			add(v)
			continue
		}
		for _, p := range strings.FieldsFunc(v, func(r rune) bool { return strings.ContainsRune(seps, r) }) {
			add(p)
		}
	}
	return out
}

// FirstNonEmpty returns the first argument that is not empty after trimming.
func FirstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
