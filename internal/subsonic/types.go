package subsonic

import "encoding/xml"

// Protocol constants of the envelope.
const (
	apiVersion   = "1.16.1"
	serverType   = "rainy"
	xmlNamespace = "http://subsonic.org/restapi"
)

// Response is the subsonic-response envelope. Exactly one payload field is set for a
// successful call (none for "empty" responses such as ping); Error is set on failure.
//
// Conventions for the payload types below: every field carries both an `xml` tag (scalar
// values are attributes, lists are repeated elements, text is chardata) and a `json` tag
// (chardata becomes "value"). OpenSubsonic fields that Rainy supports are always present in
// JSON (empty value when unknown), as required by the OpenSubsonic spec.
type Response struct {
	XMLName       xml.Name `xml:"subsonic-response" json:"-"`
	Xmlns         string   `xml:"xmlns,attr" json:"-"`
	Status        string   `xml:"status,attr" json:"status"`
	Version       string   `xml:"version,attr" json:"version"`
	Type          string   `xml:"type,attr" json:"type"`
	ServerVersion string   `xml:"serverVersion,attr" json:"serverVersion"`
	OpenSubsonic  bool     `xml:"openSubsonic,attr" json:"openSubsonic"`

	Error *Error `xml:"error,omitempty" json:"error,omitempty"`

	License                *License               `xml:"license,omitempty" json:"license,omitempty"`
	OpenSubsonicExtensions []Extension            `xml:"openSubsonicExtensions,omitempty" json:"openSubsonicExtensions,omitempty"`
	TokenInfo              *TokenInfo             `xml:"tokenInfo,omitempty" json:"tokenInfo,omitempty"`
	MusicFolders           *MusicFolders          `xml:"musicFolders,omitempty" json:"musicFolders,omitempty"`
	Indexes                *Indexes               `xml:"indexes,omitempty" json:"indexes,omitempty"`
	Directory              *Directory             `xml:"directory,omitempty" json:"directory,omitempty"`
	Genres                 *Genres                `xml:"genres,omitempty" json:"genres,omitempty"`
	Artists                *ArtistsID3            `xml:"artists,omitempty" json:"artists,omitempty"`
	Artist                 *ArtistWithAlbumsID3   `xml:"artist,omitempty" json:"artist,omitempty"`
	Album                  *AlbumWithSongsID3     `xml:"album,omitempty" json:"album,omitempty"`
	Song                   *Child                 `xml:"song,omitempty" json:"song,omitempty"`
	ArtistInfo             *ArtistInfo            `xml:"artistInfo,omitempty" json:"artistInfo,omitempty"`
	ArtistInfo2            *ArtistInfo2           `xml:"artistInfo2,omitempty" json:"artistInfo2,omitempty"`
	AlbumInfo              *AlbumInfo             `xml:"albumInfo,omitempty" json:"albumInfo,omitempty"`
	SimilarSongs           *Songs                 `xml:"similarSongs,omitempty" json:"similarSongs,omitempty"`
	SimilarSongs2          *Songs                 `xml:"similarSongs2,omitempty" json:"similarSongs2,omitempty"`
	TopSongs               *Songs                 `xml:"topSongs,omitempty" json:"topSongs,omitempty"`
	AlbumList              *AlbumList             `xml:"albumList,omitempty" json:"albumList,omitempty"`
	AlbumList2             *AlbumList2            `xml:"albumList2,omitempty" json:"albumList2,omitempty"`
	RandomSongs            *Songs                 `xml:"randomSongs,omitempty" json:"randomSongs,omitempty"`
	SongsByGenre           *Songs                 `xml:"songsByGenre,omitempty" json:"songsByGenre,omitempty"`
	NowPlaying             *NowPlaying            `xml:"nowPlaying,omitempty" json:"nowPlaying,omitempty"`
	Starred                *Starred               `xml:"starred,omitempty" json:"starred,omitempty"`
	Starred2               *Starred2              `xml:"starred2,omitempty" json:"starred2,omitempty"`
	SearchResult           *SearchResult          `xml:"searchResult,omitempty" json:"searchResult,omitempty"`
	SearchResult2          *SearchResult2         `xml:"searchResult2,omitempty" json:"searchResult2,omitempty"`
	SearchResult3          *SearchResult3         `xml:"searchResult3,omitempty" json:"searchResult3,omitempty"`
	Playlists              *Playlists             `xml:"playlists,omitempty" json:"playlists,omitempty"`
	Playlist               *PlaylistWithSongs     `xml:"playlist,omitempty" json:"playlist,omitempty"`
	Lyrics                 *Lyrics                `xml:"lyrics,omitempty" json:"lyrics,omitempty"`
	LyricsList             *LyricsList            `xml:"lyricsList,omitempty" json:"lyricsList,omitempty"`
	Bookmarks              *Bookmarks             `xml:"bookmarks,omitempty" json:"bookmarks,omitempty"`
	PlayQueue              *PlayQueue             `xml:"playQueue,omitempty" json:"playQueue,omitempty"`
	PlayQueueByIndex       *PlayQueueByIndex      `xml:"playQueueByIndex,omitempty" json:"playQueueByIndex,omitempty"`
	User                   *User                  `xml:"user,omitempty" json:"user,omitempty"`
	Users                  *Users                 `xml:"users,omitempty" json:"users,omitempty"`
	ScanStatus             *ScanStatus            `xml:"scanStatus,omitempty" json:"scanStatus,omitempty"`
	InternetRadioStations  *InternetRadioStations `xml:"internetRadioStations,omitempty" json:"internetRadioStations,omitempty"`
	Podcasts               *Podcasts              `xml:"podcasts,omitempty" json:"podcasts,omitempty"`
	NewestPodcasts         *NewestPodcasts        `xml:"newestPodcasts,omitempty" json:"newestPodcasts,omitempty"`
	Shares                 *Shares                `xml:"shares,omitempty" json:"shares,omitempty"`
	ChatMessages           *ChatMessages          `xml:"chatMessages,omitempty" json:"chatMessages,omitempty"`
	Videos                 *Videos                `xml:"videos,omitempty" json:"videos,omitempty"`
}

// Error is the error payload.
type Error struct {
	Code    int    `xml:"code,attr" json:"code"`
	Message string `xml:"message,attr,omitempty" json:"message,omitempty"`
}

// License (Rainy is free software: always valid).
type License struct {
	Valid bool `xml:"valid,attr" json:"valid"`
}

// Extension is one OpenSubsonic extension.
type Extension struct {
	Name     string `xml:"name,attr" json:"name"`
	Versions []int  `xml:"versions" json:"versions"`
}

// TokenInfo is the apiKeyAuthentication tokenInfo payload.
type TokenInfo struct {
	Username string `xml:"username,attr" json:"username"`
}

// MusicFolders lists the libraries.
type MusicFolders struct {
	Folders []MusicFolder `xml:"musicFolder" json:"musicFolder"`
}

// MusicFolder is a library.
type MusicFolder struct {
	ID   int64  `xml:"id,attr" json:"id"`
	Name string `xml:"name,attr" json:"name"`
}

// Indexes is the getIndexes payload (simulated folder browsing: album artists).
type Indexes struct {
	LastModified    int64    `xml:"lastModified,attr" json:"lastModified"`
	IgnoredArticles string   `xml:"ignoredArticles,attr" json:"ignoredArticles"`
	Index           []Index  `xml:"index" json:"index"`
	Child           []Child  `xml:"child" json:"child,omitempty"`
	Shortcut        []Artist `xml:"shortcut" json:"shortcut,omitempty"`
}

// Index groups folder-style artists by initial.
type Index struct {
	Name    string   `xml:"name,attr" json:"name"`
	Artists []Artist `xml:"artist" json:"artist"`
}

// Artist is the folder-style (non-ID3) artist.
type Artist struct {
	ID         string `xml:"id,attr" json:"id"`
	Name       string `xml:"name,attr" json:"name"`
	CoverArt   string `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	Starred    string `xml:"starred,attr,omitempty" json:"starred,omitempty"`
	UserRating int    `xml:"userRating,attr,omitempty" json:"userRating,omitempty"`
}

// Directory is a simulated folder (artist → albums, album → songs).
type Directory struct {
	ID         string  `xml:"id,attr" json:"id"`
	Parent     string  `xml:"parent,attr,omitempty" json:"parent,omitempty"`
	Name       string  `xml:"name,attr" json:"name"`
	Starred    string  `xml:"starred,attr,omitempty" json:"starred,omitempty"`
	UserRating int     `xml:"userRating,attr,omitempty" json:"userRating,omitempty"`
	PlayCount  int     `xml:"playCount,attr,omitempty" json:"playCount,omitempty"`
	Played     string  `xml:"played,attr,omitempty" json:"played,omitempty"`
	Children   []Child `xml:"child" json:"child"`
}

// Genres lists genres.
type Genres struct {
	Genres []Genre `xml:"genre" json:"genre"`
}

// Genre is one genre; its name is the element text.
type Genre struct {
	Name       string `xml:",chardata" json:"value"`
	SongCount  int    `xml:"songCount,attr" json:"songCount"`
	AlbumCount int    `xml:"albumCount,attr" json:"albumCount"`
}

// ItemGenre is an OpenSubsonic genre reference.
type ItemGenre struct {
	Name string `xml:"name,attr" json:"name"`
}

// ArtistRef is an OpenSubsonic artist reference inside songs/albums.
type ArtistRef struct {
	ID   string `xml:"id,attr" json:"id"`
	Name string `xml:"name,attr" json:"name"`
}

// ReplayGain is the OpenSubsonic replay-gain object.
type ReplayGain struct {
	TrackGain *float64 `xml:"trackGain,attr,omitempty" json:"trackGain,omitempty"`
	AlbumGain *float64 `xml:"albumGain,attr,omitempty" json:"albumGain,omitempty"`
	TrackPeak *float64 `xml:"trackPeak,attr,omitempty" json:"trackPeak,omitempty"`
	AlbumPeak *float64 `xml:"albumPeak,attr,omitempty" json:"albumPeak,omitempty"`
}

// Child is a song (isDir=false) or a folder-style album (isDir=true).
type Child struct {
	ID                    string      `xml:"id,attr" json:"id"`
	Parent                string      `xml:"parent,attr,omitempty" json:"parent,omitempty"`
	IsDir                 bool        `xml:"isDir,attr" json:"isDir"`
	Title                 string      `xml:"title,attr" json:"title"`
	Name                  string      `xml:"name,attr,omitempty" json:"name,omitempty"`
	Album                 string      `xml:"album,attr,omitempty" json:"album,omitempty"`
	Artist                string      `xml:"artist,attr,omitempty" json:"artist,omitempty"`
	Track                 int         `xml:"track,attr,omitempty" json:"track,omitempty"`
	Year                  int         `xml:"year,attr,omitempty" json:"year,omitempty"`
	Genre                 string      `xml:"genre,attr,omitempty" json:"genre,omitempty"`
	CoverArt              string      `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	Size                  int64       `xml:"size,attr,omitempty" json:"size,omitempty"`
	ContentType           string      `xml:"contentType,attr,omitempty" json:"contentType,omitempty"`
	Suffix                string      `xml:"suffix,attr,omitempty" json:"suffix,omitempty"`
	TranscodedContentType string      `xml:"transcodedContentType,attr,omitempty" json:"transcodedContentType,omitempty"`
	TranscodedSuffix      string      `xml:"transcodedSuffix,attr,omitempty" json:"transcodedSuffix,omitempty"`
	Duration              int         `xml:"duration,attr,omitempty" json:"duration,omitempty"`
	BitRate               int         `xml:"bitRate,attr,omitempty" json:"bitRate,omitempty"`
	BitDepth              int         `xml:"bitDepth,attr,omitempty" json:"bitDepth"`
	SamplingRate          int         `xml:"samplingRate,attr,omitempty" json:"samplingRate"`
	ChannelCount          int         `xml:"channelCount,attr,omitempty" json:"channelCount"`
	Path                  string      `xml:"path,attr,omitempty" json:"path,omitempty"`
	IsVideo               bool        `xml:"isVideo,attr" json:"isVideo"`
	UserRating            int         `xml:"userRating,attr,omitempty" json:"userRating,omitempty"`
	PlayCount             int         `xml:"playCount,attr" json:"playCount"`
	Played                string      `xml:"played,attr,omitempty" json:"played,omitempty"`
	DiscNumber            int         `xml:"discNumber,attr,omitempty" json:"discNumber,omitempty"`
	Created               string      `xml:"created,attr,omitempty" json:"created,omitempty"`
	Starred               string      `xml:"starred,attr,omitempty" json:"starred,omitempty"`
	AlbumID               string      `xml:"albumId,attr,omitempty" json:"albumId,omitempty"`
	ArtistID              string      `xml:"artistId,attr,omitempty" json:"artistId,omitempty"`
	Type                  string      `xml:"type,attr,omitempty" json:"type,omitempty"`
	MediaType             string      `xml:"mediaType,attr,omitempty" json:"mediaType,omitempty"`
	SongCount             int         `xml:"songCount,attr,omitempty" json:"songCount,omitempty"`
	BookmarkPosition      int64       `xml:"bookmarkPosition,attr,omitempty" json:"bookmarkPosition,omitempty"`
	BPM                   int         `xml:"bpm,attr,omitempty" json:"bpm"`
	Comment               string      `xml:"comment,attr,omitempty" json:"comment"`
	SortName              string      `xml:"sortName,attr,omitempty" json:"sortName"`
	MusicBrainzID         string      `xml:"musicBrainzId,attr,omitempty" json:"musicBrainzId"`
	Genres                []ItemGenre `xml:"genres" json:"genres"`
	Artists               []ArtistRef `xml:"artists" json:"artists"`
	DisplayArtist         string      `xml:"displayArtist,attr,omitempty" json:"displayArtist"`
	AlbumArtists          []ArtistRef `xml:"albumArtists" json:"albumArtists"`
	DisplayAlbumArtist    string      `xml:"displayAlbumArtist,attr,omitempty" json:"displayAlbumArtist"`
	DisplayComposer       string      `xml:"displayComposer,attr,omitempty" json:"displayComposer"`
	Moods                 []string    `xml:"moods" json:"moods"`
	ReplayGain            *ReplayGain `xml:"replayGain,omitempty" json:"replayGain,omitempty"`
}

// ArtistID3 is an ID3-style artist.
type ArtistID3 struct {
	ID            string `xml:"id,attr" json:"id"`
	Name          string `xml:"name,attr" json:"name"`
	CoverArt      string `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	AlbumCount    int    `xml:"albumCount,attr" json:"albumCount"`
	Starred       string `xml:"starred,attr,omitempty" json:"starred,omitempty"`
	UserRating    int    `xml:"userRating,attr,omitempty" json:"userRating,omitempty"`
	MusicBrainzID string `xml:"musicBrainzId,attr,omitempty" json:"musicBrainzId"`
	SortName      string `xml:"sortName,attr,omitempty" json:"sortName"`
}

// ArtistsID3 is the getArtists payload.
type ArtistsID3 struct {
	IgnoredArticles string     `xml:"ignoredArticles,attr" json:"ignoredArticles"`
	Index           []IndexID3 `xml:"index" json:"index"`
}

// IndexID3 groups ID3 artists by initial.
type IndexID3 struct {
	Name    string      `xml:"name,attr" json:"name"`
	Artists []ArtistID3 `xml:"artist" json:"artist"`
}

// ArtistWithAlbumsID3 is the getArtist payload.
type ArtistWithAlbumsID3 struct {
	ArtistID3
	Albums []AlbumID3 `xml:"album" json:"album"`
}

// DiscTitle names one disc of an album.
type DiscTitle struct {
	Disc  int    `xml:"disc,attr" json:"disc"`
	Title string `xml:"title,attr" json:"title"`
}

// AlbumID3 is an ID3-style album.
type AlbumID3 struct {
	ID            string      `xml:"id,attr" json:"id"`
	Name          string      `xml:"name,attr" json:"name"`
	Artist        string      `xml:"artist,attr,omitempty" json:"artist,omitempty"`
	ArtistID      string      `xml:"artistId,attr,omitempty" json:"artistId,omitempty"`
	CoverArt      string      `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	SongCount     int         `xml:"songCount,attr" json:"songCount"`
	Duration      int         `xml:"duration,attr" json:"duration"`
	PlayCount     int         `xml:"playCount,attr" json:"playCount"`
	Created       string      `xml:"created,attr" json:"created"`
	Starred       string      `xml:"starred,attr,omitempty" json:"starred,omitempty"`
	Year          int         `xml:"year,attr,omitempty" json:"year,omitempty"`
	Genre         string      `xml:"genre,attr,omitempty" json:"genre,omitempty"`
	Played        string      `xml:"played,attr,omitempty" json:"played,omitempty"`
	UserRating    int         `xml:"userRating,attr,omitempty" json:"userRating"`
	MusicBrainzID string      `xml:"musicBrainzId,attr,omitempty" json:"musicBrainzId"`
	Genres        []ItemGenre `xml:"genres" json:"genres"`
	Artists       []ArtistRef `xml:"artists" json:"artists"`
	DisplayArtist string      `xml:"displayArtist,attr,omitempty" json:"displayArtist"`
	SortName      string      `xml:"sortName,attr,omitempty" json:"sortName"`
	IsCompilation bool        `xml:"isCompilation,attr" json:"isCompilation"`
	DiscTitles    []DiscTitle `xml:"discTitles" json:"discTitles"`
}

// AlbumWithSongsID3 is the getAlbum payload.
type AlbumWithSongsID3 struct {
	AlbumID3
	Songs []Child `xml:"song" json:"song"`
}

// ArtistInfo is the getArtistInfo payload (text values are child elements in XML).
type ArtistInfo struct {
	Biography      string   `xml:"biography,omitempty" json:"biography,omitempty"`
	MusicBrainzID  string   `xml:"musicBrainzId,omitempty" json:"musicBrainzId,omitempty"`
	LastFmURL      string   `xml:"lastFmUrl,omitempty" json:"lastFmUrl,omitempty"`
	SmallImageURL  string   `xml:"smallImageUrl,omitempty" json:"smallImageUrl,omitempty"`
	MediumImageURL string   `xml:"mediumImageUrl,omitempty" json:"mediumImageUrl,omitempty"`
	LargeImageURL  string   `xml:"largeImageUrl,omitempty" json:"largeImageUrl,omitempty"`
	SimilarArtists []Artist `xml:"similarArtist" json:"similarArtist,omitempty"`
}

// ArtistInfo2 is the getArtistInfo2 payload.
type ArtistInfo2 struct {
	Biography      string      `xml:"biography,omitempty" json:"biography,omitempty"`
	MusicBrainzID  string      `xml:"musicBrainzId,omitempty" json:"musicBrainzId,omitempty"`
	LastFmURL      string      `xml:"lastFmUrl,omitempty" json:"lastFmUrl,omitempty"`
	SmallImageURL  string      `xml:"smallImageUrl,omitempty" json:"smallImageUrl,omitempty"`
	MediumImageURL string      `xml:"mediumImageUrl,omitempty" json:"mediumImageUrl,omitempty"`
	LargeImageURL  string      `xml:"largeImageUrl,omitempty" json:"largeImageUrl,omitempty"`
	SimilarArtists []ArtistID3 `xml:"similarArtist" json:"similarArtist,omitempty"`
}

// AlbumInfo is the getAlbumInfo(2) payload.
type AlbumInfo struct {
	Notes          string `xml:"notes,omitempty" json:"notes,omitempty"`
	MusicBrainzID  string `xml:"musicBrainzId,omitempty" json:"musicBrainzId,omitempty"`
	LastFmURL      string `xml:"lastFmUrl,omitempty" json:"lastFmUrl,omitempty"`
	SmallImageURL  string `xml:"smallImageUrl,omitempty" json:"smallImageUrl,omitempty"`
	MediumImageURL string `xml:"mediumImageUrl,omitempty" json:"mediumImageUrl,omitempty"`
	LargeImageURL  string `xml:"largeImageUrl,omitempty" json:"largeImageUrl,omitempty"`
}

// Songs is a plain song list (randomSongs, songsByGenre, similarSongs, topSongs).
type Songs struct {
	Songs []Child `xml:"song" json:"song"`
}

// AlbumList holds folder-style albums.
type AlbumList struct {
	Albums []Child `xml:"album" json:"album"`
}

// AlbumList2 holds ID3 albums.
type AlbumList2 struct {
	Albums []AlbumID3 `xml:"album" json:"album"`
}

// NowPlaying lists what users are playing.
type NowPlaying struct {
	Entries []NowPlayingEntry `xml:"entry" json:"entry"`
}

// NowPlayingEntry is a song with the player that plays it.
type NowPlayingEntry struct {
	Child
	Username   string `xml:"username,attr" json:"username"`
	MinutesAgo int    `xml:"minutesAgo,attr" json:"minutesAgo"`
	PlayerID   int    `xml:"playerId,attr" json:"playerId"`
	PlayerName string `xml:"playerName,attr,omitempty" json:"playerName,omitempty"`
}

// Starred is the getStarred payload.
type Starred struct {
	Artists []Artist `xml:"artist" json:"artist"`
	Albums  []Child  `xml:"album" json:"album"`
	Songs   []Child  `xml:"song" json:"song"`
}

// Starred2 is the getStarred2 payload.
type Starred2 struct {
	Artists []ArtistID3 `xml:"artist" json:"artist"`
	Albums  []AlbumID3  `xml:"album" json:"album"`
	Songs   []Child     `xml:"song" json:"song"`
}

// SearchResult is the (deprecated) search payload.
type SearchResult struct {
	Offset    int     `xml:"offset,attr" json:"offset"`
	TotalHits int     `xml:"totalHits,attr" json:"totalHits"`
	Matches   []Child `xml:"match" json:"match"`
}

// SearchResult2 is the search2 payload.
type SearchResult2 struct {
	Artists []Artist `xml:"artist" json:"artist"`
	Albums  []Child  `xml:"album" json:"album"`
	Songs   []Child  `xml:"song" json:"song"`
}

// SearchResult3 is the search3 payload.
type SearchResult3 struct {
	Artists []ArtistID3 `xml:"artist" json:"artist"`
	Albums  []AlbumID3  `xml:"album" json:"album"`
	Songs   []Child     `xml:"song" json:"song"`
}

// Playlists lists playlists.
type Playlists struct {
	Playlists []Playlist `xml:"playlist" json:"playlist"`
}

// Playlist is playlist metadata.
type Playlist struct {
	ID        string `xml:"id,attr" json:"id"`
	Name      string `xml:"name,attr" json:"name"`
	Comment   string `xml:"comment,attr,omitempty" json:"comment,omitempty"`
	Owner     string `xml:"owner,attr,omitempty" json:"owner,omitempty"`
	Public    bool   `xml:"public,attr" json:"public"`
	SongCount int    `xml:"songCount,attr" json:"songCount"`
	Duration  int    `xml:"duration,attr" json:"duration"`
	Created   string `xml:"created,attr" json:"created"`
	Changed   string `xml:"changed,attr" json:"changed"`
	CoverArt  string `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	Readonly  bool   `xml:"readonly,attr" json:"readonly"`
}

// PlaylistWithSongs is the getPlaylist payload.
type PlaylistWithSongs struct {
	Playlist
	Entries []Child `xml:"entry" json:"entry"`
}

// Lyrics is the (legacy) getLyrics payload.
type Lyrics struct {
	Artist string `xml:"artist,attr,omitempty" json:"artist,omitempty"`
	Title  string `xml:"title,attr,omitempty" json:"title,omitempty"`
	Value  string `xml:",chardata" json:"value"`
}

// LyricsList is the getLyricsBySongId payload.
type LyricsList struct {
	StructuredLyrics []StructuredLyrics `xml:"structuredLyrics" json:"structuredLyrics"`
}

// StructuredLyrics is one set of (possibly synced) lyrics.
type StructuredLyrics struct {
	DisplayArtist string      `xml:"displayArtist,attr,omitempty" json:"displayArtist,omitempty"`
	DisplayTitle  string      `xml:"displayTitle,attr,omitempty" json:"displayTitle,omitempty"`
	Lang          string      `xml:"lang,attr" json:"lang"`
	Offset        int64       `xml:"offset,attr,omitempty" json:"offset,omitempty"`
	Synced        bool        `xml:"synced,attr" json:"synced"`
	Lines         []LyricLine `xml:"line" json:"line"`
}

// LyricLine is one lyrics line; Start (ms) is omitted for unsynced lyrics.
type LyricLine struct {
	Start *int64 `xml:"start,attr,omitempty" json:"start,omitempty"`
	Value string `xml:",chardata" json:"value"`
}

// Bookmarks lists the user's bookmarks.
type Bookmarks struct {
	Bookmarks []Bookmark `xml:"bookmark" json:"bookmark"`
}

// Bookmark is a saved position in a song.
type Bookmark struct {
	Position int64  `xml:"position,attr" json:"position"`
	Username string `xml:"username,attr" json:"username"`
	Comment  string `xml:"comment,attr,omitempty" json:"comment,omitempty"`
	Created  string `xml:"created,attr" json:"created"`
	Changed  string `xml:"changed,attr" json:"changed"`
	Entry    Child  `xml:"entry" json:"entry"`
}

// PlayQueue is the id-based saved play queue.
type PlayQueue struct {
	Current   string  `xml:"current,attr,omitempty" json:"current,omitempty"`
	Position  int64   `xml:"position,attr" json:"position"`
	Username  string  `xml:"username,attr" json:"username"`
	Changed   string  `xml:"changed,attr" json:"changed"`
	ChangedBy string  `xml:"changedBy,attr" json:"changedBy"`
	Entries   []Child `xml:"entry" json:"entry"`
}

// PlayQueueByIndex is the index-based saved play queue.
type PlayQueueByIndex struct {
	CurrentIndex int     `xml:"currentIndex,attr" json:"currentIndex"`
	Position     int64   `xml:"position,attr" json:"position"`
	Username     string  `xml:"username,attr" json:"username"`
	Changed      string  `xml:"changed,attr" json:"changed"`
	ChangedBy    string  `xml:"changedBy,attr" json:"changedBy"`
	Entries      []Child `xml:"entry" json:"entry"`
}

// User is a Subsonic user with role flags.
type User struct {
	Username            string  `xml:"username,attr" json:"username"`
	Email               string  `xml:"email,attr,omitempty" json:"email,omitempty"`
	ScrobblingEnabled   bool    `xml:"scrobblingEnabled,attr" json:"scrobblingEnabled"`
	AdminRole           bool    `xml:"adminRole,attr" json:"adminRole"`
	SettingsRole        bool    `xml:"settingsRole,attr" json:"settingsRole"`
	DownloadRole        bool    `xml:"downloadRole,attr" json:"downloadRole"`
	UploadRole          bool    `xml:"uploadRole,attr" json:"uploadRole"`
	PlaylistRole        bool    `xml:"playlistRole,attr" json:"playlistRole"`
	CoverArtRole        bool    `xml:"coverArtRole,attr" json:"coverArtRole"`
	CommentRole         bool    `xml:"commentRole,attr" json:"commentRole"`
	PodcastRole         bool    `xml:"podcastRole,attr" json:"podcastRole"`
	StreamRole          bool    `xml:"streamRole,attr" json:"streamRole"`
	JukeboxRole         bool    `xml:"jukeboxRole,attr" json:"jukeboxRole"`
	ShareRole           bool    `xml:"shareRole,attr" json:"shareRole"`
	VideoConversionRole bool    `xml:"videoConversionRole,attr" json:"videoConversionRole"`
	Folders             []int64 `xml:"folder" json:"folder"`
}

// Users lists users.
type Users struct {
	Users []User `xml:"user" json:"user"`
}

// ScanStatus reports scanner progress.
type ScanStatus struct {
	Scanning    bool   `xml:"scanning,attr" json:"scanning"`
	Count       int64  `xml:"count,attr" json:"count"`
	FolderCount int    `xml:"folderCount,attr" json:"folderCount"`
	LastScan    string `xml:"lastScan,attr,omitempty" json:"lastScan,omitempty"`
}

// InternetRadioStations lists radio stations.
type InternetRadioStations struct {
	Stations []InternetRadioStation `xml:"internetRadioStation" json:"internetRadioStation"`
}

// InternetRadioStation is one station.
type InternetRadioStation struct {
	ID          string `xml:"id,attr" json:"id"`
	Name        string `xml:"name,attr" json:"name"`
	StreamURL   string `xml:"streamUrl,attr" json:"streamUrl"`
	HomePageURL string `xml:"homePageUrl,attr,omitempty" json:"homePageUrl,omitempty"`
}

// Podcasts is always empty (podcasts are not supported).
type Podcasts struct {
	Channels []struct{} `xml:"channel" json:"channel"`
}

// NewestPodcasts is always empty.
type NewestPodcasts struct {
	Episodes []struct{} `xml:"episode" json:"episode"`
}

// Shares is always empty (sharing is not supported).
type Shares struct {
	Shares []struct{} `xml:"share" json:"share"`
}

// ChatMessages is always empty (chat is not supported).
type ChatMessages struct {
	Messages []struct{} `xml:"chatMessage" json:"chatMessage"`
}

// Videos is always empty.
type Videos struct {
	Videos []struct{} `xml:"video" json:"video"`
}
