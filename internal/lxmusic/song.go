package lxmusic

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Songs come back from the web app when a manager starts a download, so they are validated
// before their ids reach a source script.

var albumIDPattern = regexp.MustCompile(`^[0-9A-Za-z_-]{0,64}$`)

// extraKeys lists the Extra keys each platform may carry and how to validate them;
// required keys must be present.
var extraKeys = map[string]map[string]struct {
	valid    func(string) bool
	required bool
}{
	Kuwo:    {},
	NetEase: {},
	Kugou: {
		"hash":         {valid: hexHash.MatchString, required: true},
		"albumAudioId": {valid: optional(numericID.MatchString)},
	},
	QQ: {
		"strMediaMid": {valid: qqMid.MatchString, required: true},
		"albumMid":    {valid: optional(qqMid.MatchString)},
		"songId":      {valid: optional(numericID.MatchString)},
	},
	Migu: {
		"copyrightId": {valid: miguID.MatchString, required: true},
		"lrcUrl":      {valid: optional(miguLink)},
		"mrcUrl":      {valid: optional(miguLink)},
		"trcUrl":      {valid: optional(miguLink)},
	},
}

func optional(fn func(string) bool) func(string) bool {
	return func(s string) bool { return s == "" || fn(s) }
}

func miguLink(s string) bool {
	u, err := url.Parse(s)
	return err == nil && len(s) <= 512 && u.Scheme == "https" && u.User == nil &&
		(u.Hostname() == "migu.cn" || strings.HasSuffix(u.Hostname(), ".migu.cn"))
}

func validSongID(platform, id string) bool {
	switch platform {
	case Kuwo, Kugou, NetEase:
		return numericID.MatchString(id)
	case QQ:
		return qqMid.MatchString(id)
	case Migu:
		return miguID.MatchString(id)
	}
	return false
}

func textOK(s string, max int) bool {
	return utf8.ValidString(s) && utf8.RuneCountInString(s) <= max && !strings.ContainsAny(s, "\x00")
}

// Validate checks a song sent by a client.
func (s Song) Validate() error {
	bad := func(what string) error { return fmt.Errorf("%w: invalid song %s", ErrInvalid, what) }
	if !ValidPlatform(s.Platform) {
		return bad("platform")
	}
	if !validSongID(s.Platform, s.ID) {
		return bad("id")
	}
	if strings.TrimSpace(s.Title) == "" || !textOK(s.Title, 500) || !textOK(s.Album, 500) {
		return bad("title or album")
	}
	if len(s.Artists) > 50 {
		return bad("artists")
	}
	for _, a := range s.Artists {
		if !textOK(a, 200) {
			return bad("artists")
		}
	}
	if !albumIDPattern.MatchString(s.AlbumID) {
		return bad("album id")
	}
	if s.Duration < 0 || s.Duration > 24*3600 {
		return bad("duration")
	}
	if s.CoverURL != "" && (len(s.CoverURL) > 1024 || !strings.HasPrefix(s.CoverURL, "https://") || !CoverAllowed(s.CoverURL)) {
		return bad("cover")
	}
	if len(s.Qualities) > len(Qualities) {
		return bad("qualities")
	}
	seen := map[string]bool{}
	for _, q := range s.Qualities {
		if !ValidQuality(q.Type) || seen[q.Type] || !textOK(q.Size, 32) || (q.Hash != "" && !hexHash.MatchString(q.Hash)) {
			return bad("qualities")
		}
		seen[q.Type] = true
	}
	allowed := extraKeys[s.Platform]
	for k, v := range s.Extra {
		rule, ok := allowed[k]
		if !ok || !rule.valid(v) {
			return bad("extra." + k)
		}
	}
	for k, rule := range allowed {
		if rule.required && s.Extra[k] == "" {
			return bad("extra." + k)
		}
	}
	return nil
}

const kugouSongPage = "https://www.kugou.com/song/"

// pageURL returns the song's page on its catalogue's website (lx-music's
// getMusicDetailPageUrl, upgraded to HTTPS).
func (s Song) pageURL() string {
	switch s.Platform {
	case Kuwo:
		return "https://www.kuwo.cn/play_detail/" + url.PathEscape(s.ID)
	case Kugou:
		// Kugou's song page takes its parameters in the fragment.
		return kugouSongPage + "#hash=" + url.QueryEscape(s.Extra["hash"]) + "&album_id=" + url.QueryEscape(s.AlbumID)
	case QQ:
		return "https://y.qq.com/n/yqq/song/" + url.PathEscape(s.ID) + ".html"
	case NetEase:
		return "https://music.163.com/song?id=" + url.QueryEscape(s.ID)
	case Migu:
		return "https://music.migu.cn/v3/music/song/" + url.PathEscape(s.Extra["copyrightId"])
	}
	return ""
}

// Normalize validates a song sent by a client and recomputes the fields the server owns.
func (s *Song) Normalize() error {
	if err := s.Validate(); err != nil {
		return err
	}
	if s.Artists == nil {
		s.Artists = []string{}
	}
	if s.Qualities == nil {
		s.Qualities = []Quality{}
	}
	if s.Extra == nil {
		s.Extra = map[string]string{}
	}
	s.PageURL = s.pageURL()
	return nil
}

// MusicInfo returns the song in the format lx-music passes to source scripts
// (toOldMusicInfo in lx-music's src/common/utils/tools.ts).
func (s Song) MusicInfo() map[string]any {
	types := make([]map[string]any, 0, len(s.Qualities))
	byType := map[string]any{}
	for _, q := range s.Qualities {
		var size any
		if q.Size != "" {
			size = q.Size
		}
		t := map[string]any{"type": q.Type, "size": size}
		b := map[string]any{"size": size}
		if q.Hash != "" {
			t["hash"], b["hash"] = q.Hash, q.Hash
		}
		types = append(types, t)
		byType[q.Type] = b
	}
	info := map[string]any{
		"name":      s.Title,
		"singer":    strings.Join(s.Artists, "、"),
		"source":    s.Platform,
		"songmid":   s.ID,
		"interval":  formatInterval(s.Duration),
		"albumName": s.Album,
		"img":       s.CoverURL,
		"typeUrl":   map[string]any{},
		"albumId":   s.AlbumID,
		"types":     types,
		"_types":    byType,
	}
	number := func(v string) any {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
		return v
	}
	switch s.Platform {
	case Kugou:
		info["songmid"] = number(s.ID)
		info["hash"] = s.Extra["hash"]
		info["albumAudioId"] = s.Extra["albumAudioId"]
	case QQ:
		info["strMediaMid"] = s.Extra["strMediaMid"]
		info["albumMid"] = s.Extra["albumMid"]
		info["songId"] = number(s.Extra["songId"])
	case NetEase:
		info["songmid"] = number(s.ID)
		info["albumId"] = number(s.AlbumID)
	case Migu:
		info["copyrightId"] = s.Extra["copyrightId"]
		info["lrcUrl"] = s.Extra["lrcUrl"]
		info["mrcUrl"] = s.Extra["mrcUrl"]
		info["trcUrl"] = s.Extra["trcUrl"]
	}
	return info
}
