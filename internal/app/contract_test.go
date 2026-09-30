package app

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"rainy/internal/events"
	"rainy/internal/lyrics"
	"rainy/internal/manage"
	"rainy/internal/metasearch"
	"rainy/internal/nowplaying"
	"rainy/internal/scanner"
	"rainy/internal/store"
	"rainy/internal/ytdlp"
)

// TestServiceJSONMatchesTSContract locks the JSON names of the non-model types that the
// native API serialises directly to the TypeScript contract (docs/architecture/contract.md §8). The
// model types are checked in internal/model.
func TestServiceJSONMatchesTSContract(t *testing.T) {
	cases := []struct {
		name string
		v    any
		want string
	}{
		{"LibraryStats", store.LibraryStats{}, `tracks albums artists genres playlists users missingTracks
			totalDuration totalSize formats playsLast30Days`},
		{"FormatStat", store.FormatStat{}, `suffix count size`},
		{"SearchResult", store.SearchResult{}, `artists albums tracks`},
		{"ScanStatus", scanner.Status{}, `scanning full libraryId phase filesSeen added updated removed moved
			errors startedAt finishedAt lastError`},
		{"Lyrics", lyrics.Lyrics{}, `synced lines source raw offset lang`},
		{"LyricsLine", lyrics.Line{}, `start text`},
		{"NowPlayingEntry", nowplaying.Entry{}, `userId username trackId player since`},
		{"ServerEvent", events.Event{}, `type data`},
		{"LibraryEventData", events.LibraryData{}, `reason`},
		{"MetadataResult", metasearch.Result{}, `provider id title artists album albumArtist trackNumber trackTotal
			discNumber discTotal date genre duration coverUrl thumbUrl`},
		{"MetadataLyrics", metasearch.Lyrics{}, `text translation`},
		{"MetadataProvider", metasearch.ProviderInfo{}, `id lyrics regions`},
		{"DownloadJob", manage.DownloadJob{}, `id url site title status phase progress item items speed eta error
			libraryId dir organize format playlist trackIds errors createdBy createdAt startedAt finishedAt`},
		{"CookieInfo", ytdlp.CookieInfo{}, `site configured count signedIn expiresAt updatedAt`},
		{"CookieSaveResult", ytdlp.CookieSaveResult{}, `site configured count signedIn expiresAt updatedAt dropped`},
		{"YtdlpInstallState", ytdlp.InstallState{}, `running error finishedAt`},
		{"YtdlpInfo (status fields)", ytdlp.Status{}, `managed installed version error latest checkedAt asset jsRuntime ffmpeg install`},
	}
	for _, c := range cases {
		b, err := json.Marshal(c.v)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatalf("%s: not an object: %s", c.name, b)
		}
		got := make([]string, 0, len(m))
		for k := range m {
			got = append(got, k)
		}
		sort.Strings(got)
		want := strings.Fields(c.want)
		sort.Strings(want)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s JSON keys\n got: %v\nwant: %v", c.name, got, want)
		}
	}
}
