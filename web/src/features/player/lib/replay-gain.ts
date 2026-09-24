import type { PlayableTrack, ReplayGainMode } from '../types'

type GainFields = Pick<PlayableTrack, 'rgTrackGain' | 'rgTrackPeak' | 'rgAlbumGain' | 'rgAlbumPeak'>

function finite(n: number | null | undefined): n is number {
  return typeof n === 'number' && Number.isFinite(n)
}

/**
 * Linear volume factor (0..1] for ReplayGain. Album mode falls back to the track gain (and vice
 * versa); peaks prevent clipping. HTMLMediaElement volume can't exceed 1, so positive gains are
 * capped at unity. Tracks without ReplayGain tags play at unity.
 */
export function replayGainFactor(track: GainFields | undefined, mode: ReplayGainMode): number {
  if (!track || mode === 'off') return 1
  const album = mode === 'album'
  const gain = album ? (track.rgAlbumGain ?? track.rgTrackGain) : (track.rgTrackGain ?? track.rgAlbumGain)
  if (!finite(gain)) return 1
  const peak = album ? (track.rgAlbumPeak ?? track.rgTrackPeak) : (track.rgTrackPeak ?? track.rgAlbumPeak)
  let factor = 10 ** (gain / 20)
  if (finite(peak) && peak > 0) factor = Math.min(factor, 1 / peak)
  return Math.min(1, Math.max(0, factor))
}

/** Perceptual volume curve: slider position (0..1) → element gain. */
export function sliderToGain(v: number): number {
  const x = Math.min(1, Math.max(0, v))
  return x * x
}
