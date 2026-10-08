import type { ComponentProps } from 'react'

import { cn } from '@/lib/utils'

/**
 * Standard page container: horizontal gutter (`.page-x`, incl. landscape safe areas) and bottom
 * padding that clears the player chrome (`.page-pad`). Put `<PageHeader>` first inside it.
 * Full-bleed children can use `.bleed-x`.
 *
 * Content is full width by default (collections, tables, dashboards). Forms, settings and
 * single-task tools wrap their body in `<PageColumn>`.
 */
export function Page({ className, ...props }: ComponentProps<'div'>) {
  return <div className={cn('page-x page-pad w-full', className)} {...props} />
}

/**
 * Readable column for forms, settings and single-task tools. It stays left-aligned under the
 * page title and section tabs (never centred), so switching between a full-width tab and a
 * column tab doesn't shift the content's left edge.
 */
export function PageColumn({ className, ...props }: ComponentProps<'div'>) {
  return <div className={cn('grid w-full max-w-3xl gap-8', className)} {...props} />
}
