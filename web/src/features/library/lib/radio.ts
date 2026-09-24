/**
 * Internet radio → player bridge.
 *
 * Contract shared with the player: a station is queued as a `PlayableTrack` with
 * `isRadio: true` and `streamUrlOverride` = its stream URL; the engine plays that URL, disables
 * seeking / scrobbling / queue sync and shows "Live".
 */
import type { PlayableTrack } from '@/features/player/types'
import type { RadioStation } from '@/lib/api/types'

export type { PlayableTrack }

/** Prefix of the synthetic track ids used for radio stations. */
export const RADIO_ID_PREFIX = 'radio:'

/** `https://radio.example.com/stream` → `radio.example.com` (`''` for invalid URLs). */
export function urlHost(url: string): string {
  try {
    return new URL(url).host
  } catch {
    return ''
  }
}

/** Only http(s) stream / homepage URLs are accepted. */
export function isHttpUrl(value: string): boolean {
  try {
    const url = new URL(value)
    return url.protocol === 'http:' || url.protocol === 'https:'
  } catch {
    return false
  }
}

/** Build a queue entry for a station. `liveLabel` is shown where the artist would be. */
export function radioToTrack(station: RadioStation, liveLabel: string): PlayableTrack {
  const now = station.updatedAt || station.createdAt
  return {
    id: `${RADIO_ID_PREFIX}${station.id}`,
    libraryId: 0,
    path: '',
    dir: '',
    filename: '',
    suffix: '',
    size: 0,
    mtime: 0,
    title: station.name,
    album: urlHost(station.homepageUrl || station.streamUrl),
    artist: liveLabel,
    albumArtist: '',
    albumId: '',
    artistId: '',
    albumArtistId: '',
    trackNumber: 0,
    trackTotal: 0,
    discNumber: 0,
    discTotal: 0,
    discSubtitle: '',
    year: 0,
    date: '',
    originalYear: 0,
    genre: '',
    genres: [],
    composer: '',
    comment: '',
    hasLrc: false,
    hasLyrics: false,
    bpm: 0,
    compilation: false,
    duration: 0,
    bitrate: 0,
    sampleRate: 0,
    bitDepth: 0,
    channels: 0,
    codec: '',
    hasCover: false,
    rgTrackGain: null,
    rgTrackPeak: null,
    rgAlbumGain: null,
    rgAlbumPeak: null,
    mbzTrackId: '',
    mbzAlbumId: '',
    mbzArtistId: '',
    mbzAlbumArtistId: '',
    sortTitle: '',
    sortAlbum: '',
    sortArtist: '',
    sortAlbumArtist: '',
    missing: false,
    createdAt: station.createdAt,
    updatedAt: now,
    starred: false,
    starredAt: null,
    rating: 0,
    playCount: 0,
    playedAt: 0,
    coverArt: '',
    contentType: '',
    streamUrlOverride: station.streamUrl,
    isRadio: true,
  }
}
