import { Volume, Volume1, Volume2, VolumeX } from 'lucide-react'
import { useState, type KeyboardEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import { useSliderDrag } from '../hooks/use-slider-drag'
import { canControlVolume } from '../lib/audio-support'
import { usePlayer } from '../store'
import type { PlayerTone } from './scrubber'

const STEP = 0.05

function VolumeSlider({ tone, className }: { tone: PlayerTone; className?: string }) {
  const { t } = useTranslation('player')
  const volume = usePlayer((s) => s.volume)
  const muted = usePlayer((s) => s.muted)
  const setVolume = usePlayer((s) => s.setVolume)
  const [dragging, setDragging] = useState(false)
  const shown = muted ? 0 : volume
  const sheet = tone === 'sheet'

  const handlers = useSliderDrag({
    getValue: () => {
      const s = usePlayer.getState()
      return s.muted ? 0 : s.volume
    },
    relativeTouch: true,
    onStart: () => setDragging(true),
    onMove: (ratio) => setVolume(ratio),
    onEnd: (ratio, committed) => {
      setDragging(false)
      if (committed) setVolume(ratio)
    },
  })

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    let next: number | null = null
    if (event.key === 'ArrowRight' || event.key === 'ArrowUp') next = shown + STEP
    else if (event.key === 'ArrowLeft' || event.key === 'ArrowDown') next = shown - STEP
    else if (event.key === 'Home') next = 0
    else if (event.key === 'End') next = 1
    if (next === null) return
    event.preventDefault()
    event.stopPropagation()
    setVolume(Math.min(1, Math.max(0, next)))
  }

  return (
    <div
      role="slider"
      tabIndex={0}
      aria-label={t('volume')}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={Math.round(shown * 100)}
      aria-valuetext={`${Math.round(shown * 100)}%`}
      onKeyDown={onKeyDown}
      {...handlers}
      className={cn(
        'group/vol relative flex min-w-0 flex-1 cursor-pointer touch-none items-center outline-none select-none',
        sheet ? 'h-7' : 'h-4',
        'focus-visible:[&>div]:ring-2 focus-visible:[&>div]:ring-ring/60',
        className,
      )}
    >
      <div
        className={cn(
          'relative w-full overflow-hidden rounded-full transition-transform duration-200 ease-out',
          sheet ? 'h-[9px] bg-white/25' : 'h-[5px] bg-foreground/15',
          dragging ? 'scale-y-100' : sheet ? 'scale-y-[0.5]' : 'scale-y-[0.8] group-hover/vol:scale-y-100',
        )}
      >
        <div
          className={cn(
            'absolute inset-0 origin-left',
            sheet ? (dragging ? 'bg-white' : 'bg-white/80') : dragging ? 'bg-primary' : 'bg-foreground/60 group-hover/vol:bg-primary',
          )}
          style={{ transform: `scaleX(${shown})` }}
        />
      </div>
    </div>
  )
}

function VolumeIcon({ level, className }: { level: number; className?: string }) {
  if (level <= 0) return <VolumeX className={className} strokeWidth={1.75} />
  if (level < 0.34) return <Volume className={className} strokeWidth={1.75} />
  if (level < 0.67) return <Volume1 className={className} strokeWidth={1.75} />
  return <Volume2 className={className} strokeWidth={1.75} />
}

/** Now Playing volume row (hidden where `audio.volume` is read-only, i.e. iOS). */
export function SheetVolume({ className }: { className?: string }) {
  if (!canControlVolume()) return null
  return (
    <div className={cn('flex items-center gap-3 text-white/60', className)}>
      <Volume className="size-4 shrink-0" fill="currentColor" strokeWidth={1.5} aria-hidden />
      <VolumeSlider tone="sheet" />
      <Volume2 className="size-4 shrink-0" fill="currentColor" strokeWidth={1.5} aria-hidden />
    </div>
  )
}

/** Player bar volume: mute toggle + slider (slider from `lg`). */
export function BarVolume({ className }: { className?: string }) {
  const { t } = useTranslation('player')
  const volume = usePlayer((s) => s.volume)
  const muted = usePlayer((s) => s.muted)
  const toggleMute = usePlayer((s) => s.toggleMute)
  if (!canControlVolume()) return null
  return (
    <div className={cn('flex items-center gap-1', className)}>
      <button
        type="button"
        onClick={toggleMute}
        aria-label={muted ? t('unmute') : t('mute')}
        aria-pressed={muted}
        className="grid size-8 place-items-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50 focus-visible:outline-none"
      >
        <VolumeIcon level={muted ? 0 : volume} className="size-[18px]" />
      </button>
      <VolumeSlider tone="bar" className="hidden w-24 flex-none lg:flex" />
    </div>
  )
}
