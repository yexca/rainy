import { motion, useReducedMotion } from 'motion/react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

const BARS = [
  { keyframes: [0.35, 1, 0.5, 0.85, 0.35], duration: 1.05 },
  { keyframes: [0.9, 0.4, 1, 0.3, 0.9], duration: 0.9 },
  { keyframes: [0.5, 0.8, 0.3, 1, 0.5], duration: 1.2 },
] as const

/** Three equaliser bars: animated while playing, resting when paused (current track). */
export function PlayingIndicator({ playing, className }: { playing: boolean; className?: string }) {
  const { t } = useTranslation()
  const reduced = useReducedMotion()
  const animate = playing && !reduced
  return (
    <span
      role="img"
      aria-label={playing ? t('library:track.nowPlaying') : t('library:track.paused')}
      className={cn('inline-flex h-3.5 w-3.5 items-end justify-between text-primary', className)}
    >
      {BARS.map((bar, i) => (
        <motion.span
          key={i}
          className="block h-full w-[3px] origin-bottom rounded-full bg-current"
          initial={false}
          animate={animate ? { scaleY: [...bar.keyframes] } : { scaleY: [0.35, 0.6, 0.45][i] }}
          transition={
            animate
              ? { duration: bar.duration, repeat: Number.POSITIVE_INFINITY, ease: 'easeInOut' }
              : { duration: 0.25 }
          }
        />
      ))}
    </span>
  )
}
