/**
 * Client-side playback preferences (persisted in localStorage, per browser): streaming quality,
 * transcode format, ReplayGain, next-track preloading, lyric translations and the long-song filter.
 * Edited on the settings page (translations also from the lyrics view), read by the audio engine,
 * lyrics and the player store.
 */
import { create } from 'zustand'
import { createJSONStorage, persist } from 'zustand/middleware'

import { clampLongTrackMinutes, DEFAULT_LONG_TRACK_MINUTES } from './lib/long-tracks'
import {
  REPLAY_GAIN_MODES,
  STREAM_QUALITIES,
  TRANSCODE_FORMATS,
  type ReplayGainMode,
  type StreamQuality,
  type TranscodeFormat,
} from './types'

export interface PlaybackPrefs {
  quality: StreamQuality
  /** Format used whenever the stream is transcoded. */
  format: TranscodeFormat
  replayGain: ReplayGainMode
  /** Buffer the next track ~20 s before the current one ends (near-gapless playback). */
  preloadNext: boolean
  /** Show the translation under bilingual lyric lines. */
  lyricsTranslation: boolean
  /** Leave songs of `longTrackMinutes` or more out when songs are queued (`lib/long-tracks.ts`). */
  skipLongTracks: boolean
  /** Whole minutes, 1–600. */
  longTrackMinutes: number
}

export interface PlaybackPrefsState extends PlaybackPrefs {
  setPrefs(patch: Partial<PlaybackPrefs>): void
  resetPrefs(): void
}

export const DEFAULT_PLAYBACK_PREFS: PlaybackPrefs = {
  quality: 'original',
  format: 'mp3',
  replayGain: 'off',
  preloadNext: true,
  lyricsTranslation: true,
  skipLongTracks: true,
  longTrackMinutes: DEFAULT_LONG_TRACK_MINUTES,
}

function sanitize(input: Partial<PlaybackPrefs> | undefined): PlaybackPrefs {
  const p = input ?? {}
  return {
    quality: (STREAM_QUALITIES as readonly string[]).includes(p.quality ?? '')
      ? (p.quality as StreamQuality)
      : DEFAULT_PLAYBACK_PREFS.quality,
    format: (TRANSCODE_FORMATS as readonly string[]).includes(p.format ?? '')
      ? (p.format as TranscodeFormat)
      : DEFAULT_PLAYBACK_PREFS.format,
    replayGain: (REPLAY_GAIN_MODES as readonly string[]).includes(p.replayGain ?? '')
      ? (p.replayGain as ReplayGainMode)
      : DEFAULT_PLAYBACK_PREFS.replayGain,
    preloadNext: typeof p.preloadNext === 'boolean' ? p.preloadNext : DEFAULT_PLAYBACK_PREFS.preloadNext,
    lyricsTranslation:
      typeof p.lyricsTranslation === 'boolean' ? p.lyricsTranslation : DEFAULT_PLAYBACK_PREFS.lyricsTranslation,
    skipLongTracks: typeof p.skipLongTracks === 'boolean' ? p.skipLongTracks : DEFAULT_PLAYBACK_PREFS.skipLongTracks,
    longTrackMinutes: clampLongTrackMinutes(p.longTrackMinutes),
  }
}

/** The long-song limit in seconds, or 0 while the filter is off. */
export function longTrackLimit(prefs: Pick<PlaybackPrefs, 'skipLongTracks' | 'longTrackMinutes'>): number {
  return prefs.skipLongTracks ? prefs.longTrackMinutes * 60 : 0
}

export const usePlaybackPrefs = create<PlaybackPrefsState>()(
  persist(
    (set) => ({
      ...DEFAULT_PLAYBACK_PREFS,
      setPrefs: (patch) => set((s) => sanitize({ ...s, ...patch })),
      resetPrefs: () => set(DEFAULT_PLAYBACK_PREFS),
    }),
    {
      name: 'rainy.playback',
      version: 1,
      storage: createJSONStorage(() => localStorage),
      partialize: (s): PlaybackPrefs => ({
        quality: s.quality,
        format: s.format,
        replayGain: s.replayGain,
        preloadNext: s.preloadNext,
        lyricsTranslation: s.lyricsTranslation,
        skipLongTracks: s.skipLongTracks,
        longTrackMinutes: s.longTrackMinutes,
      }),
      merge: (persisted, current) => ({ ...current, ...sanitize(persisted as Partial<PlaybackPrefs>) }),
    },
  ),
)
