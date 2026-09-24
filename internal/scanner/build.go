package scanner

import (
	"path"
	"strings"

	"rainy/internal/model"
	"rainy/internal/tags"
	"rainy/internal/util"
)

// Fallback names for missing tags (docs/architecture/contract.md §5.8).
const (
	UnknownAlbum  = "[Unknown Album]"
	UnknownArtist = "[Unknown Artist]"
	VariousArtist = "Various Artists"
)

// buildTrack converts file info and metadata into a track row (without id), applying the
// fallbacks: title ← file name without extension; album ← "[Unknown Album]"; artist ←
// "[Unknown Artist]"; album artist ← ALBUMARTIST, else "Various Artists" for compilations,
// else the artist. Genres are split on settings.GenreSeparators. created_at (only used on
// first discovery) = min(mtime, now).
func buildTrack(libraryID int64, f fileInfo, m *tags.Metadata, set model.Settings) model.Track {
	now := util.NowMs()
	_, filename, _ := util.PathParts(f.rel)
	title := m.Title
	if title == "" {
		title = strings.TrimSpace(strings.TrimSuffix(filename, path.Ext(filename)))
		if title == "" {
			title = filename
		}
	}
	album := util.FirstNonEmpty(m.Album, UnknownAlbum)
	artist := util.FirstNonEmpty(m.Artist, m.AlbumArtist, UnknownArtist)
	albumArtist := m.AlbumArtist
	if albumArtist == "" {
		if m.Compilation {
			albumArtist = VariousArtist
		} else {
			albumArtist = artist
		}
	}
	genres := util.SplitMulti(m.Genres, set.GenreSeparators)

	bitrate := m.Bitrate
	if bitrate == 0 && m.Duration > 0 {
		bitrate = int(float64(f.size) * 8 / m.Duration / 1000)
	}
	created := f.mtime
	if created <= 0 || created > now {
		created = now
	}

	return model.Track{
		LibraryID:        libraryID,
		Path:             f.rel,
		Size:             f.size,
		Mtime:            f.mtime,
		Title:            title,
		Album:            album,
		Artist:           artist,
		AlbumArtist:      albumArtist,
		AlbumID:          util.AlbumID(albumArtist, album),
		ArtistID:         util.ArtistID(artist),
		AlbumArtistID:    util.ArtistID(albumArtist),
		TrackNumber:      m.TrackNumber,
		TrackTotal:       m.TrackTotal,
		DiscNumber:       m.DiscNumber,
		DiscTotal:        m.DiscTotal,
		DiscSubtitle:     m.DiscSubtitle,
		Year:             m.Year,
		Date:             m.Date,
		OriginalYear:     m.OriginalYear,
		Genres:           genres,
		Genre:            strings.Join(genres, model.GenreJoiner),
		Composer:         m.Composer,
		Comment:          m.Comment,
		Lyrics:           m.Lyrics,
		HasLrc:           f.hasLrc,
		BPM:              m.BPM,
		Compilation:      m.Compilation,
		Duration:         m.Duration,
		Bitrate:          bitrate,
		SampleRate:       m.SampleRate,
		BitDepth:         m.BitDepth,
		Channels:         m.Channels,
		Codec:            m.Codec,
		HasCover:         m.HasPicture,
		RGTrackGain:      m.RGTrackGain,
		RGTrackPeak:      m.RGTrackPeak,
		RGAlbumGain:      m.RGAlbumGain,
		RGAlbumPeak:      m.RGAlbumPeak,
		MbzTrackID:       m.MbzTrackID,
		MbzAlbumID:       m.MbzAlbumID,
		MbzArtistID:      m.MbzArtistID,
		MbzAlbumArtistID: m.MbzAlbumArtistID,
		SortTitle:        m.SortTitle,
		SortAlbum:        m.SortAlbum,
		SortArtist:       m.SortArtist,
		SortAlbumArtist:  m.SortAlbumArtist,
		CreatedAt:        created,
		UpdatedAt:        now,
	}
}

// sameContent reports whether a freshly built track equals the stored row in everything
// the scanner derives from the file (used by full scans to avoid needless updates).
func sameContent(old *model.Track, n *model.Track) bool {
	eqf := func(a, b *float64) bool {
		if a == nil || b == nil {
			return a == nil && b == nil
		}
		return *a == *b
	}
	return !old.Missing && old.LibraryID == n.LibraryID && old.Path == n.Path && old.Size == n.Size &&
		old.Mtime == n.Mtime && old.Title == n.Title && old.Album == n.Album && old.Artist == n.Artist &&
		old.AlbumArtist == n.AlbumArtist && old.AlbumID == n.AlbumID && old.ArtistID == n.ArtistID &&
		old.AlbumArtistID == n.AlbumArtistID && old.TrackNumber == n.TrackNumber && old.TrackTotal == n.TrackTotal &&
		old.DiscNumber == n.DiscNumber && old.DiscTotal == n.DiscTotal && old.DiscSubtitle == n.DiscSubtitle &&
		old.Year == n.Year && old.Date == n.Date && old.OriginalYear == n.OriginalYear && old.Genre == n.Genre &&
		old.Composer == n.Composer && old.Comment == n.Comment && old.Lyrics == n.Lyrics && old.HasLrc == n.HasLrc &&
		old.BPM == n.BPM && old.Compilation == n.Compilation && old.Duration == n.Duration && old.Bitrate == n.Bitrate &&
		old.SampleRate == n.SampleRate && old.BitDepth == n.BitDepth && old.Channels == n.Channels &&
		old.Codec == n.Codec && old.HasCover == n.HasCover && eqf(old.RGTrackGain, n.RGTrackGain) &&
		eqf(old.RGTrackPeak, n.RGTrackPeak) && eqf(old.RGAlbumGain, n.RGAlbumGain) && eqf(old.RGAlbumPeak, n.RGAlbumPeak) &&
		old.MbzTrackID == n.MbzTrackID && old.MbzAlbumID == n.MbzAlbumID && old.MbzArtistID == n.MbzArtistID &&
		old.MbzAlbumArtistID == n.MbzAlbumArtistID && old.SortTitle == n.SortTitle && old.SortAlbum == n.SortAlbum &&
		old.SortArtist == n.SortArtist && old.SortAlbumArtist == n.SortAlbumArtist
}
