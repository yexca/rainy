import type { ComponentProps } from 'react'

import { cn } from '@/lib/utils'

export interface PageProps extends ComponentProps<'div'> {
  /** Centre the page up to `--page-max-w` while `.bleed-x` bars still span the full pane. */
  centered?: boolean
}

/**
 * Standard page container: horizontal gutter (`.page-x`, incl. landscape safe areas) and bottom
 * padding that clears the player chrome (`.page-pad`). Put `<PageHeader>` first inside it.
 * Full-bleed children can use `.bleed-x`.
 *
 * Pages are full width by default (library browsing, detail pages, listening reports); management,
 * admin and settings pages pass `centered` so every tab shares one centred `--page-max-w` column.
 */
export function Page({ centered = false, className, ...props }: PageProps) {
  const inner = <div className={cn('page-x page-pad w-full', centered && 'page-centered', className)} {...props} />
  return centered ? <div className="page-frame w-full">{inner}</div> : inner
}
