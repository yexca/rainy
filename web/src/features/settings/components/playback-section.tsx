import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { canControlVolume, canPlayFormat } from '@/features/player/lib/audio-support'
import {
  clampLongTrackMinutes,
  MAX_LONG_TRACK_MINUTES,
  MIN_LONG_TRACK_MINUTES,
} from '@/features/player/lib/long-tracks'
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
  const skipLongTracks = usePlaybackPrefs((s) => s.skipLongTracks)
  const longTrackMinutes = usePlaybackPrefs((s) => s.longTrackMinutes)
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
        label={t('playback.skipLong')}
        description={t('playback.skipLongHint', { minutes: longTrackMinutes })}
        htmlFor="settings-skip-long"
      >
        <Switch
          id="settings-skip-long"
          checked={skipLongTracks}
          onCheckedChange={(checked) => setPrefs({ skipLongTracks: checked })}
        />
      </SettingsRow>
      {skipLongTracks ? (
        <SettingsRow
          label={t('playback.longMinutes')}
          description={t('playback.longMinutesHint', { min: MIN_LONG_TRACK_MINUTES, max: MAX_LONG_TRACK_MINUTES })}
          htmlFor="settings-long-minutes"
        >
          {/* Keyed by the saved value so the field resets to it after an edit is committed. */}
          <LongMinutesInput
            key={longTrackMinutes}
            value={longTrackMinutes}
            unit={t('playback.minutesUnit')}
            onCommit={(minutes) => setPrefs({ longTrackMinutes: minutes })}
          />
        </SettingsRow>
      ) : null}
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

/** Whole-minute field for the long-song limit; saved (clamped) on Enter or when it loses focus. */
function LongMinutesInput({ value, unit, onCommit }: { value: number; unit: string; onCommit: (minutes: number) => void }) {
  const [draft, setDraft] = useState(String(value))

  const commit = () => {
    const minutes = clampLongTrackMinutes(draft === '' ? Number.NaN : Number(draft), value)
    setDraft(String(minutes))
    if (minutes !== value) onCommit(minutes)
  }
  const onSubmit = (event: FormEvent) => {
    event.preventDefault()
    commit()
  }

  return (
    <form onSubmit={onSubmit} className="flex items-center gap-2">
      <Input
        id="settings-long-minutes"
        value={draft}
        onChange={(event) => setDraft(event.target.value.replace(/\D/g, '').slice(0, 3))}
        onBlur={commit}
        inputMode="numeric"
        autoComplete="off"
        enterKeyHint="done"
        className="tnum h-11 w-20 text-right md:h-9"
      />
      <span className="text-sm text-muted-foreground">{unit}</span>
    </form>
  )
}
