import type { ImgHTMLAttributes } from 'react'

import iconUrl from '@/assets/brand/rainy-icon.webp'
import { cn } from '@/lib/utils'

export interface LogoProps extends Omit<ImgHTMLAttributes<HTMLImageElement>, 'src' | 'alt' | 'width' | 'height' | 'title'> {
  /** Rendered size in px (square). */
  size?: number
  /** Accessible name; omit for decorative use next to the "Rainy" wordmark. */
  title?: string
}

/**
 * The Rainy app icon: the mascot listening to music on a rain-blue rounded tile. The same art
 * as the PWA icons in `public/` (generated from `public/icon-source*.png`).
 */
export function Logo({ size = 32, title, className, ...props }: LogoProps) {
  return (
    <img
      src={iconUrl}
      width={size}
      height={size}
      alt={title ?? ''}
      aria-hidden={title ? undefined : true}
      title={title}
      draggable={false}
      decoding="async"
      className={cn('shrink-0 select-none', className)}
      {...props}
    />
  )
}

/** Logo tile + "Rainy" wordmark. */
export function BrandLogo({ size = 28, className }: { size?: number; className?: string }) {
  return (
    <span className={cn('inline-flex items-center gap-2', className)}>
      <Logo size={size} />
      <span className="text-[17px] font-semibold tracking-tight">Rainy</span>
    </span>
  )
}
