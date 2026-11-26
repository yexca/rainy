/**
 * Player contract for the settings feature (docs/architecture/contract.md §9.2): playback
 * preferences, the long-song limit, the install prompt, and browser audio support. Settings
 * imports these from here instead of reaching into the player's internal modules.
 */
export { canControlVolume, canPlayFormat } from './lib/audio-support'
export { promptInstall, useInstallPrompt } from './lib/install-prompt'
export { clampLongTrackMinutes, MAX_LONG_TRACK_MINUTES, MIN_LONG_TRACK_MINUTES } from './lib/long-tracks'
export { usePlaybackPrefs } from './prefs'
