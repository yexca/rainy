package metasearch

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"rainy/internal/util"
)

// itunes uses Apple's public iTunes Search API. It is the only provider with track and disc
// totals and genres, and it has no lyrics.
type itunes struct{}

const itunesSearchURL = "https://itunes.apple.com/search"

// itunesRegions are the storefronts offered in the UI; the first is the default.
var itunesRegions = []string{"us", "cn", "hk", "tw", "jp", "kr", "gb", "de", "fr"}

func (itunes) info() ProviderInfo {
	return ProviderInfo{ID: "itunes", Lyrics: false, Regions: append([]string(nil), itunesRegions...)}
}

func (itunes) search(ctx context.Context, c *client, query string, limit int, region string) ([]Result, error) {
	q := url.Values{
		"term": {query}, "media": {"music"}, "entity": {"song"},
		"limit": {strconv.Itoa(limit)}, "country": {region},
	}
	data, err := c.get(ctx, itunesSearchURL+"?"+q.Encode(), nil, maxJSONSize)
	if err != nil {
		return nil, err
	}
	var body struct {
		Results []struct {
			Kind             string  `json:"kind"`
			TrackID          int64   `json:"trackId"`
			TrackName        string  `json:"trackName"`
			ArtistName       string  `json:"artistName"`
			CollectionName   string  `json:"collectionName"`
			CollectionArtist string  `json:"collectionArtistName"`
			TrackNumber      flexInt `json:"trackNumber"`
			TrackCount       flexInt `json:"trackCount"`
			DiscNumber       flexInt `json:"discNumber"`
			DiscCount        flexInt `json:"discCount"`
			ReleaseDate      string  `json:"releaseDate"`
			Genre            string  `json:"primaryGenreName"`
			TrackTimeMillis  int64   `json:"trackTimeMillis"`
			Artwork100       string  `json:"artworkUrl100"`
		} `json:"results"`
	}
	if err := decode(data, "itunes.apple.com", &body); err != nil {
		return nil, err
	}
	out := make([]Result, 0, len(body.Results))
	for _, s := range body.Results {
		if s.Kind != "" && s.Kind != "song" {
			continue
		}
		r := Result{
			ID:          strconv.FormatInt(s.TrackID, 10),
			Title:       s.TrackName,
			Artists:     []string{s.ArtistName},
			Album:       s.CollectionName,
			AlbumArtist: util.FirstNonEmpty(s.CollectionArtist, s.ArtistName),
			TrackNumber: int(s.TrackNumber),
			TrackTotal:  int(s.TrackCount),
			DiscNumber:  int(s.DiscNumber),
			DiscTotal:   int(s.DiscCount),
			Date:        normalizeDate(s.ReleaseDate),
			Genre:       s.Genre,
			Duration:    float64(s.TrackTimeMillis) / 1000,
		}
		if s.TrackID == 0 {
			r.ID = ""
		}
		if strings.Contains(s.Artwork100, "100x100bb") {
			r.CoverURL = strings.Replace(s.Artwork100, "100x100bb", "1200x1200bb", 1)
			r.ThumbURL = strings.Replace(s.Artwork100, "100x100bb", "150x150bb", 1)
		} else if s.Artwork100 != "" {
			r.CoverURL, r.ThumbURL = s.Artwork100, s.Artwork100
		}
		out = append(out, r)
	}
	return out, nil
}

func (itunes) lyrics(context.Context, *client, string) (*Lyrics, error) {
	return nil, ErrNotFound
}
