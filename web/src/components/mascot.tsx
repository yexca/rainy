import type { ImgHTMLAttributes } from 'react'

import empty from '@/assets/mascot/empty.webp'
import forbidden from '@/assets/mascot/forbidden.webp'
import idle from '@/assets/mascot/idle.webp'
import listening from '@/assets/mascot/listening.webp'
import lost from '@/assets/mascot/lost.webp'
import offline from '@/assets/mascot/offline.webp'
import settings from '@/assets/mascot/settings.webp'
import sleeping from '@/assets/mascot/sleeping.webp'
import welcome from '@/assets/mascot/welcome.webp'
import { cn } from '@/lib/utils'

/**
 * Poses of Rainy's mascot (docs/development/design.md#mascot). Full-body scenes for status
 * screens; `idle`, `listening` and `sleeping` are upper-body crops with a flat bottom edge for
 * the corner companion.
 */
const MASCOT_POSES = {
  lost,
  offline,
  forbidden,
  empty,
  welcome,
  settings,
  idle,
  listening,
  sleeping,
} as const

export type MascotPose = keyof typeof MASCOT_POSES

export interface MascotArtProps extends Omit<ImgHTMLAttributes<HTMLImageElement>, 'src' | 'alt'> {
  pose: MascotPose
}

/** Decorative mascot illustration (transparent WebP); size it with a height class. */
export function MascotArt({ pose, className, ...props }: MascotArtProps) {
  return (
    <img
      src={MASCOT_POSES[pose]}
      alt=""
      aria-hidden
      draggable={false}
      decoding="async"
      className={cn('pointer-events-none w-auto select-none', className)}
      {...props}
    />
  )
}
