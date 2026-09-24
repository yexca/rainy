package tags

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// multiValueJoiner joins multi-valued ARTIST / ALBUMARTIST / COMPOSER tags.
const multiValueJoiner = " / "

// parse maps raw TagLib properties (upper-case keys) to Metadata (audio properties are
// filled by the caller). Rules (docs/architecture/contract.md §5.7):
//   - TRACKNUMBER "3/12" → 3, 12 (also TRACKTOTAL / TOTALTRACKS); same for discs
//   - DATE / YEAR → first 4-digit year; ORIGINALDATE / ORIGINALYEAR → original year
//   - COMPILATION (or the iTunes TCMP / CPIL frames) "1" / "true"
//   - ReplayGain "-6.50 dB" (Opus R128_* gains converted as a fallback)
//   - lyrics from LYRICS, then UNSYNCEDLYRICS (then LYRICS:<description> keys)
func parse(raw map[string][]string) *Metadata {
	get := func(keys ...string) string {
		for _, k := range keys {
			for _, v := range raw[k] {
				if v = clean(v); v != "" {
					return v
				}
			}
		}
		return ""
	}
	joined := func(key string) string {
		var parts []string
		seen := map[string]bool{}
		for _, v := range raw[key] {
			if v = clean(v); v != "" && !seen[strings.ToLower(v)] {
				seen[strings.ToLower(v)] = true
				parts = append(parts, v)
			}
		}
		return strings.Join(parts, multiValueJoiner)
	}

	m := &Metadata{
		Title:            get("TITLE"),
		Album:            get("ALBUM"),
		Artist:           joined("ARTIST"),
		AlbumArtist:      joined("ALBUMARTIST"),
		DiscSubtitle:     get("DISCSUBTITLE", "SETSUBTITLE"),
		Composer:         joined("COMPOSER"),
		Comment:          get("COMMENT", "DESCRIPTION"),
		MbzTrackID:       get("MUSICBRAINZ_TRACKID"),
		MbzAlbumID:       get("MUSICBRAINZ_ALBUMID"),
		MbzArtistID:      get("MUSICBRAINZ_ARTISTID"),
		MbzAlbumArtistID: get("MUSICBRAINZ_ALBUMARTISTID"),
		SortTitle:        get("TITLESORT"),
		SortAlbum:        get("ALBUMSORT"),
		SortArtist:       get("ARTISTSORT"),
		SortAlbumArtist:  get("ALBUMARTISTSORT"),
		Genres:           []string{},
		Raw:              raw,
	}
	if m.Raw == nil {
		m.Raw = map[string][]string{}
	}

	m.TrackNumber, m.TrackTotal = parsePair(get("TRACKNUMBER"))
	if m.TrackTotal == 0 {
		m.TrackTotal = parseLeadingInt(get("TRACKTOTAL", "TOTALTRACKS"))
	}
	m.DiscNumber, m.DiscTotal = parsePair(get("DISCNUMBER"))
	if m.DiscTotal == 0 {
		m.DiscTotal = parseLeadingInt(get("DISCTOTAL", "TOTALDISCS"))
	}

	m.Date = get("DATE", "YEAR")
	m.Year = parseYear(m.Date)
	if m.Year == 0 {
		m.Year = parseYear(get("YEAR", "RELEASEDATE"))
	}
	m.OriginalYear = parseYear(get("ORIGINALDATE", "ORIGINALYEAR"))

	for _, g := range raw["GENRE"] {
		if g = clean(g); g != "" {
			m.Genres = append(m.Genres, g)
		}
	}

	m.Lyrics = rawText(raw, "LYRICS", "UNSYNCEDLYRICS")
	if m.Lyrics == "" {
		var keys []string
		for k := range raw {
			if strings.HasPrefix(k, "LYRICS:") {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		m.Lyrics = rawText(raw, keys...)
	}

	if bpm, err := strconv.ParseFloat(strings.TrimSpace(get("BPM")), 64); err == nil && bpm > 0 && bpm < 10000 {
		m.BPM = int(math.Round(bpm))
	}
	m.Compilation = parseBool(get("COMPILATION", "TCMP", "ITUNESCOMPILATION", "CPIL"))

	m.RGTrackGain = parseGain(get("REPLAYGAIN_TRACK_GAIN"))
	m.RGTrackPeak = parseFloat(get("REPLAYGAIN_TRACK_PEAK"))
	m.RGAlbumGain = parseGain(get("REPLAYGAIN_ALBUM_GAIN"))
	m.RGAlbumPeak = parseFloat(get("REPLAYGAIN_ALBUM_PEAK"))
	if m.RGTrackGain == nil {
		m.RGTrackGain = parseR128(get("R128_TRACK_GAIN"))
	}
	if m.RGAlbumGain == nil {
		m.RGAlbumGain = parseR128(get("R128_ALBUM_GAIN"))
	}
	return m
}

// clean trims a single-line tag value and drops NUL bytes.
func clean(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\x00", ""))
}

// rawText returns the first non-blank value of keys, keeping inner line breaks (lyrics).
func rawText(raw map[string][]string, keys ...string) string {
	for _, k := range keys {
		for _, v := range raw[k] {
			v = strings.ReplaceAll(v, "\x00", "")
			v = strings.ReplaceAll(v, "\r\n", "\n")
			v = strings.ReplaceAll(v, "\r", "\n")
			if strings.TrimSpace(v) != "" {
				return strings.Trim(v, "\n ")
			}
		}
	}
	return ""
}

// parseLeadingInt parses the leading decimal digits of s ("03" → 3, "7 of 12" → 7);
// 0 when there are none or the value is out of range.
func parseLeadingInt(s string) int {
	s = strings.TrimSpace(s)
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end == 0 || end > 6 {
		return 0
	}
	n, _ := strconv.Atoi(s[:end])
	return n
}

// parsePair parses "3/12", "3 of 12", "3" → (3, 12), (3, 12), (3, 0).
func parsePair(s string) (int, int) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0
	}
	num, rest := s, ""
	if i := strings.IndexByte(s, '/'); i >= 0 {
		num, rest = s[:i], s[i+1:]
	} else if i := strings.Index(strings.ToLower(s), " of "); i >= 0 {
		num, rest = s[:i], s[i+4:]
	}
	return parseLeadingInt(num), parseLeadingInt(rest)
}

var yearRe = regexp.MustCompile(`\d{4}`)

// parseYear returns the first 4-digit number in s that is a plausible year, else 0.
func parseYear(s string) int {
	for _, m := range yearRe.FindAllString(s, -1) {
		if y, _ := strconv.Atoi(m); y >= 1000 && y <= 2999 {
			return y
		}
	}
	return 0
}

func parseBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "y", "on":
		return true
	}
	return false
}

// parseFloat parses a plain float; nil when empty or invalid.
func parseFloat(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return nil
	}
	return &f
}

// parseGain parses a ReplayGain value like "-6.50 dB" or "+1.2dB".
func parseGain(s string) *float64 {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && strings.EqualFold(s[len(s)-2:], "db") {
		s = strings.TrimSpace(s[:len(s)-2])
	}
	s = strings.TrimPrefix(s, "+")
	f := parseFloat(s)
	if f == nil || math.Abs(*f) > 100 {
		return nil
	}
	return f
}

// parseR128 converts an Opus R128 gain (Q7.8 fixed point relative to -23 LUFS) to a
// ReplayGain 2.0 value (reference -18 LUFS).
func parseR128(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < -32768 || n > 32767 {
		return nil
	}
	g := math.Round((float64(n)/256+5)*100) / 100
	return &g
}

// fixEncoding repairs mojibake in the user-visible text fields (Raw is left untouched).
func (m *Metadata) fixEncoding() {
	for _, p := range []*string{
		&m.Title, &m.Album, &m.Artist, &m.AlbumArtist, &m.DiscSubtitle, &m.Composer, &m.Comment,
		&m.Lyrics, &m.SortTitle, &m.SortAlbum, &m.SortArtist, &m.SortAlbumArtist,
	} {
		if fixed, _, ok := FixMojibake(*p); ok {
			*p = fixed
		}
	}
	for i, g := range m.Genres {
		if fixed, _, ok := FixMojibake(g); ok {
			m.Genres[i] = fixed
		}
	}
}
