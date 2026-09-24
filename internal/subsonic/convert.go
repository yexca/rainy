package subsonic

import (
	"math"
	"strings"

	"rainy/internal/model"
	"rainy/internal/transcode"
	"rainy/internal/util"
)

// seconds rounds a float duration to whole seconds.
func seconds(d float64) int {
	if d <= 0 || math.IsNaN(d) || math.IsInf(d, 0) {
		return 0
	}
	return int(math.Round(d))
}

// date formats unix ms as RFC3339 ("" for 0).
func date(ms int64) string { return util.RFC3339Ms(ms) }

// datePtr formats an optional unix ms timestamp.
func datePtr(ms *int64) string {
	if ms == nil {
		return ""
	}
	return util.RFC3339Ms(*ms)
}

func itemGenres(genres []string) []ItemGenre {
	out := make([]ItemGenre, 0, len(genres))
	for _, g := range genres {
		out = append(out, ItemGenre{Name: g})
	}
	return out
}

func artistRefs(id, name string) []ArtistRef {
	if id == "" && name == "" {
		return []ArtistRef{}
	}
	return []ArtistRef{{ID: id, Name: name}}
}

func firstGenre(genres []string) string {
	if len(genres) > 0 {
		return genres[0]
	}
	return ""
}

// defaultTranscodeSuffixes are formats most Subsonic clients cannot decode; when ffmpeg is
// available they are transcoded to the default format unless the client asks for "raw".
var defaultTranscodeSuffixes = map[string]bool{
	"ape": true, "wma": true, "dsf": true, "dff": true, "wv": true, "mpc": true, "tta": true,
}

// defaultTranscode reports the format a track is transcoded to by a plain stream request
// (no format/maxBitRate), or "" when it is streamed as-is.
func (q *request) defaultTranscode(t *model.Track) string {
	if !defaultTranscodeSuffixes[strings.ToLower(t.Suffix)] || !q.api.app.Transcoder.Available() {
		return ""
	}
	f := strings.ToLower(q.appSettings().TranscodeFormat)
	if !transcode.IsFormat(f) {
		f = transcode.FormatMP3
	}
	return f
}

// child converts a track to a Subsonic song.
func (q *request) child(t *model.Track) Child {
	genres := t.Genres
	if genres == nil {
		genres = model.SplitGenre(t.Genre)
	}
	c := Child{
		ID:                 t.ID,
		Parent:             t.AlbumID,
		IsDir:              false,
		Title:              t.Title,
		Album:              t.Album,
		Artist:             t.Artist,
		Track:              t.TrackNumber,
		Year:               t.Year,
		Genre:              firstGenre(genres),
		CoverArt:           t.CoverArt,
		Size:               t.Size,
		ContentType:        util.MimeType(t.Suffix),
		Suffix:             t.Suffix,
		Duration:           seconds(t.Duration),
		BitRate:            t.Bitrate,
		BitDepth:           t.BitDepth,
		SamplingRate:       t.SampleRate,
		ChannelCount:       t.Channels,
		Path:               t.Path,
		UserRating:         t.Rating,
		PlayCount:          t.PlayCount,
		Played:             date(t.PlayedAt),
		DiscNumber:         t.DiscNumber,
		Created:            date(t.CreatedAt),
		Starred:            datePtr(t.StarredAt),
		AlbumID:            t.AlbumID,
		ArtistID:           t.ArtistID,
		Type:               "music",
		MediaType:          "song",
		BPM:                t.BPM,
		Comment:            t.Comment,
		SortName:           t.SortTitle,
		MusicBrainzID:      t.MbzTrackID,
		Genres:             itemGenres(genres),
		Artists:            artistRefs(t.ArtistID, t.Artist),
		DisplayArtist:      t.Artist,
		AlbumArtists:       artistRefs(t.AlbumArtistID, t.AlbumArtist),
		DisplayAlbumArtist: t.AlbumArtist,
		DisplayComposer:    t.Composer,
		Moods:              []string{},
		ReplayGain: &ReplayGain{
			TrackGain: t.RGTrackGain, AlbumGain: t.RGAlbumGain,
			TrackPeak: t.RGTrackPeak, AlbumPeak: t.RGAlbumPeak,
		},
	}
	if f := q.defaultTranscode(t); f != "" {
		c.TranscodedContentType = transcode.ContentType(f)
		c.TranscodedSuffix = transcode.Suffix(f)
	}
	return c
}

// children converts tracks to songs (never nil).
func (q *request) children(ts []model.Track) []Child {
	out := make([]Child, 0, len(ts))
	for i := range ts {
		out = append(out, q.child(&ts[i]))
	}
	return out
}

// albumChild converts an album to a folder-style directory entry.
func albumChild(al *model.Album) Child {
	genres := model.SplitGenre(al.Genre)
	return Child{
		ID:            al.ID,
		Parent:        al.AlbumArtistID,
		IsDir:         true,
		Title:         al.Name,
		Name:          al.Name,
		Album:         al.Name,
		Artist:        al.AlbumArtist,
		Year:          al.Year,
		Genre:         firstGenre(genres),
		CoverArt:      al.CoverArt,
		Duration:      seconds(al.Duration),
		UserRating:    al.Rating,
		PlayCount:     al.PlayCount,
		Played:        date(al.PlayedAt),
		Created:       date(al.CreatedAt),
		Starred:       datePtr(al.StarredAt),
		AlbumID:       al.ID,
		ArtistID:      al.AlbumArtistID,
		MediaType:     "album",
		SongCount:     al.SongCount,
		SortName:      al.SortName,
		MusicBrainzID: al.MbzAlbumID,
		Genres:        itemGenres(genres),
		Artists:       artistRefs(al.AlbumArtistID, al.AlbumArtist),
		DisplayArtist: al.AlbumArtist,
		AlbumArtists:  artistRefs(al.AlbumArtistID, al.AlbumArtist),
		Moods:         []string{},
	}
}

func albumChildren(as []model.Album) []Child {
	out := make([]Child, 0, len(as))
	for i := range as {
		out = append(out, albumChild(&as[i]))
	}
	return out
}

// albumID3 converts an album.
func albumID3(al *model.Album) AlbumID3 {
	genres := model.SplitGenre(al.Genre)
	return AlbumID3{
		ID:            al.ID,
		Name:          al.Name,
		Artist:        al.AlbumArtist,
		ArtistID:      al.AlbumArtistID,
		CoverArt:      al.CoverArt,
		SongCount:     al.SongCount,
		Duration:      seconds(al.Duration),
		PlayCount:     al.PlayCount,
		Created:       date(al.CreatedAt),
		Starred:       datePtr(al.StarredAt),
		Year:          al.Year,
		Genre:         firstGenre(genres),
		Played:        date(al.PlayedAt),
		UserRating:    al.Rating,
		MusicBrainzID: al.MbzAlbumID,
		Genres:        itemGenres(genres),
		Artists:       artistRefs(al.AlbumArtistID, al.AlbumArtist),
		DisplayArtist: al.AlbumArtist,
		SortName:      al.SortName,
		IsCompilation: al.Compilation,
		DiscTitles:    []DiscTitle{},
	}
}

func albumsID3(as []model.Album) []AlbumID3 {
	out := make([]AlbumID3, 0, len(as))
	for i := range as {
		out = append(out, albumID3(&as[i]))
	}
	return out
}

// discTitles collects the disc subtitles of an album's tracks.
func discTitles(ts []model.Track) []DiscTitle {
	out := []DiscTitle{}
	seen := map[int]bool{}
	for _, t := range ts {
		if t.DiscSubtitle == "" || seen[t.DiscNumber] {
			continue
		}
		seen[t.DiscNumber] = true
		out = append(out, DiscTitle{Disc: t.DiscNumber, Title: t.DiscSubtitle})
	}
	return out
}

// artistID3 converts an artist.
func artistID3(ar *model.Artist) ArtistID3 {
	return ArtistID3{
		ID:            ar.ID,
		Name:          ar.Name,
		CoverArt:      ar.CoverArt,
		AlbumCount:    ar.AlbumCount,
		Starred:       datePtr(ar.StarredAt),
		UserRating:    ar.Rating,
		MusicBrainzID: ar.MbzArtistID,
		SortName:      ar.SortName,
	}
}

func artistsID3(as []model.Artist) []ArtistID3 {
	out := make([]ArtistID3, 0, len(as))
	for i := range as {
		out = append(out, artistID3(&as[i]))
	}
	return out
}

// folderArtist converts an artist to the folder-style form.
func folderArtist(ar *model.Artist) Artist {
	return Artist{
		ID:         ar.ID,
		Name:       ar.Name,
		CoverArt:   ar.CoverArt,
		Starred:    datePtr(ar.StarredAt),
		UserRating: ar.Rating,
	}
}

func folderArtists(as []model.Artist) []Artist {
	out := make([]Artist, 0, len(as))
	for i := range as {
		out = append(out, folderArtist(&as[i]))
	}
	return out
}

// playlist converts playlist metadata for the requesting user.
func playlistOf(p *model.Playlist, u *model.User) Playlist {
	return Playlist{
		ID:        p.ID,
		Name:      p.Name,
		Comment:   p.Comment,
		Owner:     p.OwnerName,
		Public:    p.Public,
		SongCount: p.SongCount,
		Duration:  seconds(p.Duration),
		Created:   date(p.CreatedAt),
		Changed:   date(p.UpdatedAt),
		CoverArt:  p.CoverArt,
		Readonly:  !canModifyPlaylist(p, u),
	}
}

// canModifyPlaylist: owners and admins may change a playlist.
func canModifyPlaylist(p *model.Playlist, u *model.User) bool {
	return u != nil && (p.OwnerID == u.ID || u.IsAdmin)
}

// canReadPlaylist: owners, admins and everyone for public playlists.
func canReadPlaylist(p *model.Playlist, u *model.User) bool {
	return p.Public || canModifyPlaylist(p, u)
}
