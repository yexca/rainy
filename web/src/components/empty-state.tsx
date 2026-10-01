import type { LucideIcon } from 'lucide-react'
import type { ReactNode } from 'react'

import { MascotArt, type MascotPose } from '@/components/mascot'
import { useMascotArt } from '@/hooks/use-mascot-art'
import { cn } from '@/lib/utils'

export interface EmptyStateProps {
  icon?: LucideIcon
  /**
   * Mascot illustration shown instead of the icon at the default size, unless the listener
   * turned illustrations off (Settings › Mascot).
   */
  art?: MascotPose
  title: ReactNode
  description?: ReactNode
  /** Button(s) below the text. */
  action?: ReactNode
  /** `compact` for panels, sheets and list sections. */
  size?: 'default' | 'compact'
  className?: string
}

/** Friendly placeholder for empty lists, missing pages and unfinished features. */
export function EmptyState({ icon: Icon, art, title, description, action, size = 'default', className }: EmptyStateProps) {
  const compact = size === 'compact'
  const artEnabled = useMascotArt()
  const showArt = art !== undefined && artEnabled && !compact
  return (
    <div
      className={cn(
        'mx-auto flex w-full max-w-md flex-col items-center text-center',
        compact ? 'gap-2 py-8' : 'gap-3 py-16 md:py-24',
        className,
      )}
    >
      {showArt ? (
        <MascotArt pose={art} className="mb-1 h-40 md:h-48" />
      ) : Icon ? (
        <div
          className={cn(
            'mb-1 grid place-items-center rounded-2xl bg-muted text-muted-foreground',
            compact ? 'size-11' : 'size-14',
          )}
        >
          <Icon className={compact ? 'size-5' : 'size-6'} strokeWidth={1.75} aria-hidden />
        </div>
      ) : null}
      <h2 className={cn('font-semibold tracking-tight text-balance', compact ? 'text-sm' : 'text-lg')}>{title}</h2>
      {description ? (
        <p className={cn('text-muted-foreground text-pretty', compact ? 'text-xs' : 'text-sm')}>{description}</p>
      ) : null}
      {action ? <div className="mt-2 flex flex-wrap items-center justify-center gap-2">{action}</div> : null}
    </div>
  )
}
