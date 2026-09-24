import { ListMusic, Maximize2, MessageSquareQuote, Music, Radio } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'

import { CoverArt } from '@/components/cover-art'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

import { useCurrentTrack, usePlayer } from '../store'
import type { PlayableTrack, PlayerPanel } from '../types'
import { LiveProgress } from './live-badge'
import { Scrubber } from './scrubber'
import { StarButton } from './star-button'
import { ModeToggle, NextButton, PlayPauseButton, PrevButton, RepeatButton, ShuffleButton } from './transport'
import { BarVolume } from './volume-control'

/**
 * Desktop / tablet bottom bar. Rendered by the shell inside a `fixed inset-x-0 bottom-0` slot;
 * owns its height (`--playerbar-h` + safe area) and always renders (idle state when empty).
 */
export function PlayerBar() {
  const { t } = useTranslation('player')
  const track = useCurrentTrack()
  const panel = usePlayer((s) => s.panel)
  const setPanel = usePlayer((s) => s.setPanel)
  const setNowPlayingOpen = usePlayer((s) => s.setNowPlayingOpen)
  const togglePanel = (p: PlayerPanel) => setPanel(panel === p ? 'none' : p)

  return (
    <div
      role="region"
      aria-label={t('region')}
      className="ui-chrome glass hairline-t grid h-[calc(var(--playerbar-h)+var(--safe-bottom))] grid-cols-[minmax(0,1fr)_minmax(0,1.6fr)_minmax(0,1fr)] items-center gap-4 pr-[calc(1rem+var(--safe-right))] pb-safe pl-[calc(1rem+var(--safe-left))] lg:gap-6"
    >
      <NowPlayingInfo track={track} onOpen={() => setNowPlayingOpen(true)} />

      <div className="mx-auto flex w-full max-w-[640px] flex-col items-center gap-0.5">
        <div className="flex items-center gap-1 lg:gap-2">
          <ShuffleButton tone="bar" className="size-8" />
          <PrevButton tone="bar" size="sm" />
          <PlayPauseButton tone="bar" size="md" />
          <NextButton tone="bar" size="sm" />
          <RepeatButton tone="bar" className="size-8" />
        </div>
        {track?.isRadio ? <LiveProgress tone="bar" className="px-12" /> : <Scrubber tone="bar" />}
      </div>

      <div className="flex items-center justify-end gap-0.5">
        <BarToggle
          label={t('lyrics.title')}
          active={panel === 'lyrics'}
          disabled={!track}
          onClick={() => togglePanel('lyrics')}
        >
          <MessageSquareQuote className="size-[18px]" strokeWidth={1.9} />
        </BarToggle>
        <BarToggle label={t('queue.title')} active={panel === 'queue'} onClick={() => togglePanel('queue')}>
          <ListMusic className="size-[18px]" strokeWidth={1.9} />
        </BarToggle>
        <BarVolume className="ml-1" />
        <BarToggle label={t('expand')} active={false} disabled={!track} onClick={() => setNowPlayingOpen(true)}>
          <Maximize2 className="size-4" strokeWidth={1.9} />
        </BarToggle>
      </div>
    </div>
  )
}

function BarToggle({
  label,
  active,
  disabled,
  onClick,
  children,
}: {
  label: string
  active: boolean
  disabled?: boolean
  onClick: () => void
  children: ReactNode
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <ModeToggle tone="bar" active={active} disabled={disabled} onClick={onClick} aria-label={label} className="size-8 rounded-md">
          {children}
        </ModeToggle>
      </TooltipTrigger>
      <TooltipContent side="top">{label}</TooltipContent>
    </Tooltip>
  )
}

function NowPlayingInfo({ track, onOpen }: { track: PlayableTrack | undefined; onOpen: () => void }) {
  const { t } = useTranslation('player')

  if (!track) {
    return (
      <div className="flex min-w-0 items-center gap-3">
        <div className="grid size-14 shrink-0 place-items-center rounded-md bg-muted text-muted-foreground/60">
          <Music className="size-5" strokeWidth={1.5} />
        </div>
        <div className="min-w-0 leading-tight">
          <p className="truncate text-sm font-medium text-muted-foreground">{t('idle.title')}</p>
          <p className="truncate text-xs text-muted-foreground/70">{t('idle.hint')}</p>
        </div>
      </div>
    )
  }

  return (
    <div className="flex min-w-0 items-center gap-3">
      <button
        type="button"
        onClick={onOpen}
        aria-label={t('openNowPlaying')}
        className="group/art relative shrink-0 rounded-md outline-none focus-visible:ring-2 focus-visible:ring-ring/60"
      >
        <CoverArt coverArt={track.coverArt} size={56} keepPrevious icon={track.isRadio ? Radio : undefined} />
        <span className="absolute inset-0 grid place-items-center rounded-md bg-black/40 text-white opacity-0 transition-opacity group-hover/art:opacity-100 group-focus-visible/art:opacity-100">
          <Maximize2 className="size-4" />
        </span>
      </button>
      <div className="min-w-0 leading-tight">
        {track.isRadio || !track.albumId ? (
          <p className="truncate text-sm font-medium">{track.title}</p>
        ) : (
          <Link
            to={`/albums/${encodeURIComponent(track.albumId)}`}
            className="block truncate text-sm font-medium hover:underline focus-visible:underline focus-visible:outline-none"
          >
            {track.title}
          </Link>
        )}
        {track.isRadio ? (
          <p className="truncate text-xs text-muted-foreground">{t('live.radio')}</p>
        ) : track.artistId ? (
          <Link
            to={`/artists/${encodeURIComponent(track.artistId)}`}
            className="block truncate text-xs text-muted-foreground hover:text-foreground hover:underline focus-visible:underline focus-visible:outline-none"
          >
            {track.artist}
          </Link>
        ) : (
          <p className="truncate text-xs text-muted-foreground">{track.artist}</p>
        )}
      </div>
      <StarButton track={track} tone="bar" className={cn('ml-1 hidden lg:grid')} />
    </div>
  )
}
