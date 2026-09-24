import { AnimatePresence, motion } from 'motion/react'

import { coverUrlForPixels } from '@/lib/cover'
import { cn } from '@/lib/utils'

import { useCoverColor } from '../hooks/use-cover-color'

/**
 * Dynamic Now Playing backdrop: a gradient from the cover's dominant colour plus the artwork
 * itself, hugely blurred and saturated, drifting slowly (transform-only animation, paused for
 * reduced motion). Cross-fades between tracks. A scrim keeps white text readable.
 */
export function NowPlayingBackground({ coverArt, playing, className }: { coverArt: string | undefined; playing: boolean; className?: string }) {
  const palette = useCoverColor(coverArt)
  const src = coverArt ? coverUrlForPixels(coverArt, 128) : ''

  return (
    <div aria-hidden className={cn('pointer-events-none absolute inset-0 overflow-hidden bg-neutral-900', className)}>
      <AnimatePresence initial={false}>
        <motion.div
          key={`${coverArt ?? ''}|${palette.top}`}
          className="absolute inset-0"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.9, ease: 'easeOut' }}
          style={{ background: `linear-gradient(180deg, ${palette.top} 0%, ${palette.bottom} 100%)` }}
        >
          {src ? (
            <img
              src={src}
              alt=""
              draggable={false}
              decoding="async"
              className={cn(
                'absolute top-1/2 left-1/2 size-[160vmax] max-w-none -translate-x-1/2 -translate-y-1/2 object-cover opacity-55 blur-[72px] saturate-[1.7] will-change-transform',
              )}
            />
          ) : null}
          {src ? (
            <div className="absolute inset-0 animate-blob will-change-transform" style={{ animationPlayState: playing ? 'running' : 'paused' }}>
              <img
                src={src}
                alt=""
                draggable={false}
                decoding="async"
                className="absolute -top-1/4 -right-1/3 size-[110vmax] max-w-none rotate-180 object-cover opacity-45 blur-[90px] saturate-[1.8]"
              />
            </div>
          ) : null}
        </motion.div>
      </AnimatePresence>
      <div className="absolute inset-0 bg-linear-to-b from-black/10 via-black/20 to-black/45" />
    </div>
  )
}
