import { Repeat, Repeat1, Shuffle } from 'lucide-react'
import type { ComponentProps } from 'react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import { usePlayback, usePlayer } from '../store'
import { BackwardGlyph, ForwardGlyph, PauseGlyph, PlayGlyph } from './icons'
import type { PlayerTone } from './scrubber'

type ButtonProps = Omit<ComponentProps<'button'>, 'children'>

/**
 * Round icon button with iOS press feedback: a soft circular highlight fades in behind the glyph
 * and the glyph shrinks while pressed (transform/opacity only).
 */
export function PressButton({
  tone,
  className,
  children,
  ...props
}: ComponentProps<'button'> & { tone: PlayerTone }) {
  return (
    <button
      type="button"
      {...props}
      className={cn(
        'group/press relative grid shrink-0 place-items-center rounded-full outline-none select-none disabled:pointer-events-none disabled:opacity-40',
        'focus-visible:ring-2',
        tone === 'sheet' ? 'text-white focus-visible:ring-white/60' : 'text-foreground focus-visible:ring-ring/60',
        className,
      )}
    >
      <span
        aria-hidden
        className={cn(
          'absolute inset-0 scale-75 rounded-full opacity-0 transition-[opacity,scale] duration-200 ease-out group-active/press:scale-100 group-active/press:opacity-100',
          tone === 'sheet' ? 'bg-white/15' : 'bg-foreground/10',
        )}
      />
      <span className="relative grid place-items-center transition-transform duration-150 ease-out group-active/press:scale-[0.86]">
        {children}
      </span>
    </button>
  )
}

/** Play/pause with a cross-fading glyph. */
export function PlayPauseButton({
  tone,
  size = 'lg',
  className,
  ...props
}: ButtonProps & { tone: PlayerTone; size?: 'sm' | 'md' | 'lg' }) {
  const { t } = useTranslation()
  const isPlaying = usePlayer((s) => s.isPlaying)
  const togglePlay = usePlayer((s) => s.togglePlay)
  const hasTrack = usePlayer((s) => s.index >= 0)
  const buffering = usePlayback((s) => s.buffering)
  const glyph = { sm: 'size-6', md: 'size-7', lg: 'size-12' }[size]
  const box = { sm: 'size-11', md: 'size-10', lg: 'size-18' }[size]

  return (
    <PressButton
      tone={tone}
      onClick={togglePlay}
      disabled={!hasTrack}
      aria-label={isPlaying ? t('actions.pause') : t('actions.play')}
      className={cn(box, className)}
      {...props}
    >
      <PauseGlyph
        className={cn(
          glyph,
          'col-start-1 row-start-1 transition-[opacity,scale] duration-200 ease-out',
          isPlaying ? 'scale-100 opacity-100' : 'scale-50 opacity-0',
        )}
      />
      <PlayGlyph
        className={cn(
          glyph,
          'col-start-1 row-start-1 transition-[opacity,scale] duration-200 ease-out',
          isPlaying ? 'scale-50 opacity-0' : 'scale-100 opacity-100',
        )}
      />
      {buffering && isPlaying ? (
        <span
          aria-hidden
          className={cn(
            'pointer-events-none absolute -inset-1 animate-spin rounded-full border-2 border-transparent [animation-duration:1.1s]',
            tone === 'sheet' ? 'border-t-white/70' : 'border-t-primary',
          )}
        />
      ) : null}
    </PressButton>
  )
}

export function PrevButton({ tone, size = 'lg', className, ...props }: ButtonProps & { tone: PlayerTone; size?: 'sm' | 'lg' }) {
  const { t } = useTranslation()
  const prev = usePlayer((s) => s.prev)
  const hasTrack = usePlayer((s) => s.index >= 0)
  return (
    <PressButton
      tone={tone}
      onClick={prev}
      disabled={!hasTrack}
      aria-label={t('actions.previous')}
      className={cn(size === 'lg' ? 'size-16' : 'size-9', className)}
      {...props}
    >
      <BackwardGlyph className={size === 'lg' ? 'size-10' : 'size-[22px]'} />
    </PressButton>
  )
}

export function NextButton({ tone, size = 'lg', className, ...props }: ButtonProps & { tone: PlayerTone; size?: 'sm' | 'lg' }) {
  const { t } = useTranslation()
  const next = usePlayer((s) => s.next)
  const hasTrack = usePlayer((s) => s.index >= 0)
  return (
    <PressButton
      tone={tone}
      onClick={next}
      disabled={!hasTrack}
      aria-label={t('actions.next')}
      className={cn(size === 'lg' ? 'size-16' : 'size-9', className)}
      {...props}
    >
      <ForwardGlyph className={size === 'lg' ? 'size-10' : 'size-[22px]'} />
    </PressButton>
  )
}

/** Toggle with an "on" state (accent tint in the bar, filled pill on artwork). */
export function ModeToggle({
  tone,
  active,
  className,
  children,
  ...props
}: ComponentProps<'button'> & { tone: PlayerTone; active: boolean }) {
  return (
    <button
      type="button"
      aria-pressed={active}
      {...props}
      className={cn(
        'relative grid shrink-0 place-items-center rounded-lg transition-[background-color,color,scale] duration-150 outline-none select-none active:scale-90 disabled:pointer-events-none disabled:opacity-40',
        tone === 'sheet'
          ? cn('focus-visible:ring-2 focus-visible:ring-white/60', active ? 'bg-white/25 text-white' : 'text-white/60 hover:text-white')
          : cn(
              'focus-visible:ring-2 focus-visible:ring-ring/50',
              active ? 'text-primary hover:bg-primary/10' : 'text-muted-foreground hover:bg-accent hover:text-foreground',
            ),
        className,
      )}
    >
      {children}
    </button>
  )
}

export function ShuffleButton({ tone, className }: { tone: PlayerTone; className?: string }) {
  const { t } = useTranslation('player')
  const shuffle = usePlayer((s) => s.shuffle)
  const toggleShuffle = usePlayer((s) => s.toggleShuffle)
  return (
    <ModeToggle tone={tone} active={shuffle} onClick={toggleShuffle} aria-label={t('shuffle')} className={className}>
      <Shuffle className="size-[18px]" strokeWidth={2} />
    </ModeToggle>
  )
}

export function RepeatButton({ tone, className }: { tone: PlayerTone; className?: string }) {
  const { t } = useTranslation('player')
  const repeat = usePlayer((s) => s.repeat)
  const cycleRepeat = usePlayer((s) => s.cycleRepeat)
  const label = repeat === 'one' ? t('repeat.one') : repeat === 'all' ? t('repeat.all') : t('repeat.off')
  const Icon = repeat === 'one' ? Repeat1 : Repeat
  return (
    <ModeToggle tone={tone} active={repeat !== 'off'} onClick={cycleRepeat} aria-label={label} title={label} className={className}>
      <Icon className="size-[18px]" strokeWidth={2} />
    </ModeToggle>
  )
}
