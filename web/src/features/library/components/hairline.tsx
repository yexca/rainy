import type { CSSProperties } from 'react'

import { cn } from '@/lib/utils'

/**
 * iOS row separator for absolutely positioned (virtualized) rows, where the `hairline-inset`
 * utility's `:last-child` rule can't tell which row is last. Parent must be `relative`.
 * `inset` = left offset (CSS length), e.g. past the artwork.
 */
export function Hairline({ inset = '0px', className }: { inset?: string; className?: string }) {
  return (
    <span
      aria-hidden
      style={{ left: inset } as CSSProperties}
      className={cn(
        'pointer-events-none absolute right-0 bottom-0 h-px origin-bottom bg-border [@media(min-resolution:2dppx)]:scale-y-50',
        className,
      )}
    />
  )
}
