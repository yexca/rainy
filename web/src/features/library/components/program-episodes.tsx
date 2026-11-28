import { CircleCheck, Play } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { usePlayer } from '@/features/player/store'
import type { Track } from '@/lib/api/types'
import { formatDurationLong } from '@/lib/format'
import { cn } from '@/lib/utils'

import { continueIndex, episodeProgress, hasProgress } from '../lib/program-progress'

/** Played mark, or a thin progress bar with the time left, under an episode's title. */
export function EpisodeProgressMark({ track, positionMs }: { track: Track; positionMs: number | undefined }) {
  const { t } = useTranslation('library')
  const progress = episodeProgress(track, positionMs)
  if (progress.state === 'played') {
    return (
      <span className="flex items-center gap-1 text-xs leading-4 text-muted-foreground">
        <CircleCheck className="size-3" strokeWidth={2} aria-hidden />
        {t('programs.played')}
      </span>
    )
  }
  if (progress.state === 'unplayed') return null
  const percent = Math.round(progress.fraction * 100)
  return (
    <span className="flex items-center gap-2 text-xs leading-4 text-muted-foreground">
      <span
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={percent}
        aria-label={track.title}
        className="h-[3px] w-14 shrink-0 overflow-hidden rounded-full bg-foreground/12"
      >
        <span className="block h-full rounded-full bg-primary" style={{ width: `${Math.max(4, percent)}%` }} />
      </span>
      {progress.remaining > 0 ? <span className="truncate">{t('programs.left', { time: formatDurationLong(progress.remaining) })}</span> : null}
    </span>
  )
}

/**
 * The hero button of a program show: "Continue" from the first unfinished episode once the
 * listener started the show, else "Play" from the first one. The episode resumes at its bookmark.
 * It waits (disabled) while the positions are first read.
 */
export function ProgramPlayButton({
  tracks,
  positions,
  loading = false,
  layout = 'inline',
}: {
  tracks: Track[]
  positions: ReadonlyMap<string, number>
  loading?: boolean
  layout?: 'inline' | 'stretch'
}) {
  const { t } = useTranslation()
  const started = hasProgress(tracks, positions)
  const start = continueIndex(tracks, positions)
  return (
    <Button
      className={cn(
        'gap-2 font-semibold active:scale-[0.97]',
        layout === 'stretch'
          ? 'h-12 w-full rounded-xl bg-secondary text-[17px] text-primary shadow-none hover:bg-secondary/80 dark:bg-secondary/80'
          : 'h-9 min-w-28 rounded-full px-5 shadow-sm',
      )}
      disabled={loading || start < 0}
      onClick={() => usePlayer.getState().playTracks(tracks, start, { shuffle: false })}
    >
      <Play className="size-[18px]" fill="currentColor" aria-hidden />
      {started ? t('library:programs.continue') : t('common:actions.play')}
    </Button>
  )
}
