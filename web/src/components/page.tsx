import type { ComponentProps } from 'react'

import { cn } from '@/lib/utils'

/**
 * Standard page container: horizontal gutter (`.page-x`, incl. landscape safe areas) and bottom
 * padding that clears the player chrome (`.page-pad`). Put `<PageHeader>` first inside it.
 * Full-bleed children can use `.bleed-x`.
 */
export function Page({ className, ...props }: ComponentProps<'div'>) {
  return <div className={cn('page-x page-pad w-full', className)} {...props} />
}
