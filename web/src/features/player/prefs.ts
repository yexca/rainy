/**
 * Client-side playback preferences (persisted in localStorage, per browser): streaming quality,
 * transcode format, ReplayGain and next-track preloading. Edited on the settings page, read by
 * the audio engine.
 */
import { create } from 'zustand'
import { createJSONStorage, persist } from 'zustand/middleware'

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
  }
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
      }),
      merge: (persisted, current) => ({ ...current, ...sanitize(persisted as Partial<PlaybackPrefs>) }),
    },
  ),
)
