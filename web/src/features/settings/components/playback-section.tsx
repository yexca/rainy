import { useTranslation } from 'react-i18next'

import { Switch } from '@/components/ui/switch'
import { canControlVolume, canPlayFormat } from '@/features/player/lib/audio-support'
import { usePlaybackPrefs } from '@/features/player/prefs'
import { usePlayer } from '@/features/player/store'
import type { ReplayGainMode, StreamQuality, TranscodeFormat } from '@/features/player/types'
import { formatNumber } from '@/lib/format'
import { isIOS } from '@/lib/platform'

import { Segmented, SettingsRow, SettingsSection } from './settings-ui'

/** Approximate data use in MB per hour for a bit rate (kbps). */
function mbPerHour(kbps: number): number {
  return Math.round((kbps * 3600) / 8 / 1000)
}

export function PlaybackSection() {
  const { t } = useTranslation('settings')
  const quality = usePlaybackPrefs((s) => s.quality)
  const format = usePlaybackPrefs((s) => s.format)
  const replayGain = usePlaybackPrefs((s) => s.replayGain)
  const preloadNext = usePlaybackPrefs((s) => s.preloadNext)
  const lyricsTranslation = usePlaybackPrefs((s) => s.lyricsTranslation)
  const setPrefs = usePlaybackPrefs((s) => s.setPrefs)
  const infinite = usePlayer((s) => s.infinite)
  const toggleInfinite = usePlayer((s) => s.toggleInfinite)
  const volumeControl = canControlVolume()
  const opusSupported = canPlayFormat('opus')

  const qualityHint =
    quality === 'original'
      ? t('playback.qualityOriginalHint')
      : t('playback.qualityHint', { mb: formatNumber(mbPerHour(Number(quality))) })

  const qualities: { value: StreamQuality; label: string }[] = [
    { value: 'original', label: t('playback.original') },
    { value: '320', label: '320' },
    { value: '192', label: '192' },
    { value: '128', label: '128' },
  ]
  const formats: { value: TranscodeFormat; label: string; disabled?: boolean }[] = [
    { value: 'mp3', label: 'MP3' },
    { value: 'aac', label: 'AAC' },
    { value: 'opus', label: 'Opus' },
  ]
  const gains: { value: ReplayGainMode; label: string }[] = [
    { value: 'off', label: t('playback.rgOff') },
    { value: 'track', label: t('playback.rgTrack') },
    { value: 'album', label: t('playback.rgAlbum') },
  ]

  return (
    <SettingsSection id="playback" title={t('playback.title')} footer={t('playback.footer')}>
      <SettingsRow label={t('playback.quality')} description={qualityHint} stacked>
        <Segmented label={t('playback.quality')} value={quality} onChange={(v) => setPrefs({ quality: v })} options={qualities} />
      </SettingsRow>
      <SettingsRow
        label={t('playback.format')}
        description={format === 'opus' && !opusSupported ? t('playback.opusUnsupported') : t('playback.formatHint')}
        stacked
      >
        <Segmented label={t('playback.format')} value={format} onChange={(v) => setPrefs({ format: v })} options={formats} />
      </SettingsRow>
      <SettingsRow
        label={t('playback.replayGain')}
        description={volumeControl ? t('playback.replayGainHint') : t('playback.replayGainUnsupported')}
        stacked
      >
        <Segmented
          label={t('playback.replayGain')}
          value={volumeControl ? replayGain : 'off'}
          onChange={(v) => setPrefs({ replayGain: v })}
          options={gains}
          disabled={!volumeControl}
        />
      </SettingsRow>
      <SettingsRow
        label={t('playback.preload')}
        description={isIOS ? t('playback.preloadUnsupported') : t('playback.preloadHint')}
        htmlFor="settings-preload"
      >
        <Switch
          id="settings-preload"
          checked={preloadNext && !isIOS}
          disabled={isIOS}
          onCheckedChange={(checked) => setPrefs({ preloadNext: checked })}
        />
      </SettingsRow>
      <SettingsRow label={t('playback.infinite')} description={t('playback.infiniteHint')} htmlFor="settings-infinite">
        <Switch id="settings-infinite" checked={infinite} onCheckedChange={(checked) => checked !== infinite && toggleInfinite()} />
      </SettingsRow>
      <SettingsRow
        label={t('playback.lyricsTranslation')}
        description={t('playback.lyricsTranslationHint')}
        htmlFor="settings-lyrics-translation"
      >
        <Switch
          id="settings-lyrics-translation"
          checked={lyricsTranslation}
          onCheckedChange={(checked) => setPrefs({ lyricsTranslation: checked })}
        />
      </SettingsRow>
    </SettingsSection>
  )
}
