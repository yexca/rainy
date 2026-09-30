package metasearch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"
)

// flexInt decodes a JSON number or numeric string ("3", "03", "1/2" → leading digits);
// anything else is 0. Providers are inconsistent about both.
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return nil
		}
		*f = flexInt(leadingInt(s))
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		*f = 0
		return nil
	}
	if i, err := n.Int64(); err == nil {
		*f = flexInt(i)
	} else if fl, err := n.Float64(); err == nil {
		*f = flexInt(fl)
	}
	return nil
}

// flexString decodes a JSON string or number as a string.
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*f = flexString(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err == nil {
		*f = flexString(n.String())
	}
	return nil
}

func leadingInt(s string) int {
	s = strings.TrimSpace(s)
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	n, _ := strconv.Atoi(s[:end])
	return n
}

func decode(data []byte, host string, v any) error {
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%w: unexpected answer from %s: %v", ErrUpstream, host, err)
	}
	return nil
}

// clean decodes HTML entities (Kuwo and QQ escape some names) and collapses whitespace.
func clean(s string) string {
	return strings.Join(strings.Fields(html.UnescapeString(s)), " ")
}

func cleanList(list []string) []string {
	out := make([]string, 0, len(list))
	seen := map[string]bool{}
	for _, v := range list {
		v = clean(v)
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// splitArtists splits a display string such as "A、B" or "A&B" into names.
func splitArtists(s string, seps string) []string {
	return strings.FieldsFunc(html.UnescapeString(s), func(r rune) bool { return strings.ContainsRune(seps, r) })
}

// china is the time zone the Chinese catalogues use for release dates.
var china = time.FixedZone("CST", 8*3600)

// dateFromMillis formats a release timestamp in ms as YYYY-MM-DD (0 or negative → "").
func dateFromMillis(ms int64) string {
	if ms <= 0 {
		return ""
	}
	return time.UnixMilli(ms).In(china).Format("2006-01-02")
}

// normalizeDate keeps YYYY, YYYY-MM or YYYY-MM-DD prefixes of a date string ("" when the
// string does not start with a plausible year).
func normalizeDate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 4 {
		return ""
	}
	year, err := strconv.Atoi(s[:4])
	if err != nil || year < 1000 {
		return ""
	}
	for _, n := range []int{10, 7} {
		if len(s) >= n {
			if _, err := time.Parse("2006-01-02"[:n], s[:n]); err == nil {
				return s[:n]
			}
		}
	}
	return s[:4]
}
