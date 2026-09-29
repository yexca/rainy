import { Languages } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import { useHasLyricTranslations } from '../hooks/use-lyrics'
import { usePlaybackPrefs } from '../prefs'
import type { PlayableTrack } from '../types'
import { ModeToggle } from './transport'

/**
 * Shows or hides the translations of bilingual lyrics (a persisted playback preference). Renders
 * nothing when the track's lyrics have no translations. `sheet`: phone Now Playing action row,
 * `stage`: desktop full-screen header, `panel`: side panel header.
 */
export function TranslationToggle({ track, tone }: { track: PlayableTrack | undefined; tone: 'sheet' | 'stage' | 'panel' }) {
  const { t } = useTranslation('player')
  const hasTranslations = useHasLyricTranslations(track)
  const show = usePlaybackPrefs((s) => s.lyricsTranslation)
  const setPrefs = usePlaybackPrefs((s) => s.setPrefs)
  if (!hasTranslations) return null
  const label = show ? t('lyrics.hideTranslation') : t('lyrics.showTranslation')
  const toggle = () => setPrefs({ lyricsTranslation: !show })

  if (tone === 'sheet') {
    return (
      <ModeToggle tone="sheet" active={show} onClick={toggle} aria-label={label} title={label} className="size-11">
        <Languages className="size-[22px]" strokeWidth={1.9} />
      </ModeToggle>
    )
  }
  return (
    <button
      type="button"
      onClick={toggle}
      aria-pressed={show}
      aria-label={label}
      title={label}
      className={cn(
        'grid place-items-center outline-none',
        tone === 'stage'
          ? cn(
              'size-10 rounded-full transition-colors focus-visible:ring-2 focus-visible:ring-white/60',
              show ? 'bg-white/25 text-white' : 'text-white/70 hover:bg-white/10 hover:text-white',
            )
          : cn(
              'size-8 rounded-md focus-visible:ring-2 focus-visible:ring-ring/50',
              show ? 'bg-accent text-foreground' : 'text-muted-foreground hover:bg-accent hover:text-foreground',
            ),
      )}
    >
      <Languages className={tone === 'stage' ? 'size-[18px]' : 'size-4'} strokeWidth={1.9} />
    </button>
  )
}
