import { AnimatePresence, motion } from 'motion/react'

import { useMediaQuery } from '@/hooks/use-media-query'
import { coverUrlForPixels } from '@/lib/cover'
import { cn } from '@/lib/utils'

import { useCoverColor } from '../hooks/use-cover-color'

/**
 * Cover-colour gradients on phones; larger screens add one container-sized blurred cover.
 * Only the desktop artwork drifts. Palette updates repaint the current layer instead of
 * creating another cross-fade. A scrim keeps white text readable.
 */
export function NowPlayingBackground({ coverArt, playing, className }: { coverArt: string | undefined; playing: boolean; className?: string }) {
  const palette = useCoverColor(coverArt)
  const detailed = useMediaQuery('(width >= 48rem) and (height > 540px)')
  const src = coverArt ? coverUrlForPixels(coverArt, 128) : ''

  return (
    <div aria-hidden className={cn('pointer-events-none absolute inset-0 overflow-hidden bg-neutral-900', className)}>
      <AnimatePresence initial={false}>
        <motion.div
          key={coverArt ?? ''}
          className="absolute inset-0"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.45, ease: 'easeOut' }}
          style={{
            background: `radial-gradient(ellipse at 15% 20%, ${palette.top}, transparent 65%), radial-gradient(ellipse at 90% 65%, color-mix(in srgb, ${palette.top} 80%, white), transparent 70%), linear-gradient(180deg, ${palette.top}, ${palette.bottom})`,
          }}
        >
          {detailed && src ? (
            <div className="absolute -inset-12 motion-safe:animate-blob" style={{ animationPlayState: playing ? 'running' : 'paused' }}>
              <img
                src={src}
                alt=""
                draggable={false}
                decoding="async"
                className="size-full max-w-none object-cover opacity-50 blur-[32px] saturate-150"
              />
            </div>
          ) : null}
        </motion.div>
      </AnimatePresence>
      <div className="absolute inset-0 bg-linear-to-b from-black/10 via-black/20 to-black/45" />
    </div>
  )
}
