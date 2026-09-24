import { Star } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import { useToggleStar } from '../hooks/use-toggle-star'
import type { PlayableTrack } from '../types'
import type { PlayerTone } from './scrubber'

export function StarButton({ track, tone, className }: { track: PlayableTrack; tone: PlayerTone; className?: string }) {
  const { t } = useTranslation()
  const toggleStar = useToggleStar()
  if (track.isRadio) return null
  const starred = track.starred
  return (
    <button
      type="button"
      onClick={() => void toggleStar(track)}
      aria-pressed={starred}
      aria-label={starred ? t('actions.unfavorite') : t('actions.favorite')}
      className={cn(
        'grid shrink-0 place-items-center rounded-full transition-[color,background-color,scale] duration-150 outline-none select-none active:scale-85 focus-visible:ring-2',
        tone === 'sheet'
          ? cn('size-9 bg-white/10 focus-visible:ring-white/60 hover:bg-white/20', starred ? 'text-white' : 'text-white/80')
          : cn(
              'size-8 focus-visible:ring-ring/50 hover:bg-accent',
              starred ? 'text-primary' : 'text-muted-foreground hover:text-foreground',
            ),
        className,
      )}
    >
      <Star className={tone === 'sheet' ? 'size-[18px]' : 'size-4'} fill={starred ? 'currentColor' : 'none'} strokeWidth={2} />
    </button>
  )
}
