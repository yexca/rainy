import { useId, type SVGProps } from 'react'

import { cn } from '@/lib/utils'

/** Raindrop outline (64×64 grid): straight flanks tangent to the round bottom. */
const DROP_PATH = 'M32 9.5 45.01 30.07A15.5 15.5 0 1 1 18.99 30.07Z'
const NOTE_FLAG_PATH =
  'M31.6 26c0 4 3.4 5 5.9 6.8 1.9 1.4 2.6 3.3 2.1 5.6-.4-2.4-2.5-4-5.3-4.6-1.1-.2-2 .1-2.7.6Z'

function NoteCutout() {
  return (
    <g fill="#000">
      <ellipse cx="28.4" cy="43.4" rx="4.9" ry="3.7" transform="rotate(-22 28.4 43.4)" />
      <rect x="30.9" y="26" width="2.7" height="17.6" rx="1" />
      <path d={NOTE_FLAG_PATH} />
    </g>
  )
}

export interface LogoProps extends Omit<SVGProps<SVGSVGElement>, 'children'> {
  /** Rendered size in px (square). */
  size?: number
  /**
   * `icon`: the app icon (accent gradient tile with the drop glyph).
   * `mark`: the bare drop glyph in the accent colour, for use on any background.
   */
  variant?: 'icon' | 'mark'
  /** Accessible name; omit for decorative use next to the "Rainy" wordmark. */
  title?: string
}

/**
 * The Rainy logo — a raindrop with a music note. Follows the active accent colour
 * (`public/favicon.svg` is the static rain-blue version).
 */
export function Logo({ size = 32, variant = 'icon', title, className, ...props }: LogoProps) {
  const uid = useId().replace(/[^a-zA-Z0-9_-]/g, '')
  const gradientId = `rainy-g-${uid}`
  const maskId = `rainy-m-${uid}`

  return (
    <svg
      viewBox="0 0 64 64"
      width={size}
      height={size}
      role={title ? 'img' : undefined}
      aria-hidden={title ? undefined : true}
      className={cn('shrink-0', className)}
      {...props}
    >
      {title ? <title>{title}</title> : null}
      <defs>
        <linearGradient id={gradientId} x1="10" y1="2" x2="54" y2="62" gradientUnits="userSpaceOnUse">
          <stop offset="0" style={{ stopColor: 'color-mix(in oklab, var(--primary) 72%, white)' }} />
          <stop offset="1" style={{ stopColor: 'color-mix(in oklab, var(--primary) 82%, black)' }} />
        </linearGradient>
        <mask id={maskId} maskUnits="userSpaceOnUse" x="0" y="0" width="64" height="64">
          <path fill="#fff" d={DROP_PATH} />
          <NoteCutout />
        </mask>
      </defs>
      {variant === 'icon' ? (
        <>
          <rect width="64" height="64" rx="14.4" fill={`url(#${gradientId})`} />
          <rect width="64" height="64" mask={`url(#${maskId})`} style={{ fill: 'var(--primary-foreground)' }} />
        </>
      ) : (
        <rect
          x="0"
          y="0"
          width="64"
          height="64"
          fill={`url(#${gradientId})`}
          mask={`url(#${maskId})`}
        />
      )}
    </svg>
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
