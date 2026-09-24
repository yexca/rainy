import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import type { PlayerTone } from './scrubber'

/** "LIVE" pill shown instead of the scrubber for internet radio. */
export function LiveBadge({ tone, className }: { tone: PlayerTone; className?: string }) {
  const { t } = useTranslation('player')
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-[10px] leading-4 font-bold tracking-[0.08em] uppercase',
        tone === 'sheet' ? 'bg-white/20 text-white' : 'bg-destructive/10 text-destructive',
        className,
      )}
    >
      <span aria-hidden className="relative flex size-1.5">
        <span className="absolute inline-flex size-full animate-ping rounded-full bg-current opacity-60" />
        <span className="relative inline-flex size-1.5 rounded-full bg-current" />
      </span>
      {t('live.badge')}
    </span>
  )
}

/** Scrubber replacement for live streams: a full "on air" line with the badge. */
export function LiveProgress({ tone, className }: { tone: PlayerTone; className?: string }) {
  return (
    <div className={cn('flex w-full items-center gap-3', tone === 'sheet' ? 'h-11' : 'h-4', className)}>
      <span aria-hidden className={cn('h-1 flex-1 rounded-full', tone === 'sheet' ? 'bg-white/25' : 'bg-foreground/10')} />
      <LiveBadge tone={tone} />
      <span aria-hidden className={cn('h-1 flex-1 rounded-full', tone === 'sheet' ? 'bg-white/25' : 'bg-foreground/10')} />
    </div>
  )
}
