import { PictureInPicture } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import { pipLyricsSupport, togglePipLyrics, usePipLyrics } from '../pip/pip-lyrics'
import { useCurrentTrack } from '../store'
import { ModeToggle } from './transport'

/**
 * Opens or closes the picture-in-picture lyrics window (§9.4). Renders nothing where the browser
 * has no picture-in-picture (Firefox). `sheet`: phone Now Playing and floating window action row,
 * `stage`: desktop full-screen header, `panel`: side panel header. The bottom bar has it
 * in its options menu (`bar-options.tsx`).
 */
export function PipLyricsButton({ tone }: { tone: 'sheet' | 'stage' | 'panel' }) {
  const { t } = useTranslation('player')
  const [supported] = useState(() => pipLyricsSupport() !== null)
  const open = usePipLyrics((s) => s.mode !== null)
  const track = useCurrentTrack()
  if (!supported) return null
  const label = open ? t('pip.close') : t('pip.open')
  const disabled = !track && !open
  const icon = <PictureInPicture className={tone === 'sheet' ? 'size-[22px]' : tone === 'panel' ? 'size-4' : 'size-[18px]'} strokeWidth={1.9} />

  if (tone === 'sheet') {
    return (
      <ModeToggle tone="sheet" active={open} disabled={disabled} onClick={togglePipLyrics} aria-label={label} title={label} className="size-11">
        {icon}
      </ModeToggle>
    )
  }
  return (
    <button
      type="button"
      onClick={togglePipLyrics}
      disabled={disabled}
      aria-pressed={open}
      aria-label={label}
      title={label}
      className={cn(
        'grid place-items-center outline-none disabled:pointer-events-none disabled:opacity-40',
        tone === 'stage'
          ? cn(
              'size-10 rounded-full transition-colors focus-visible:ring-2 focus-visible:ring-white/60',
              open ? 'bg-white/25 text-white' : 'text-white/70 hover:bg-white/10 hover:text-white',
            )
          : cn(
              'size-8 rounded-md focus-visible:ring-2 focus-visible:ring-ring/50',
              open ? 'bg-accent text-foreground' : 'text-muted-foreground hover:bg-accent hover:text-foreground',
            ),
      )}
    >
      {icon}
    </button>
  )
}
