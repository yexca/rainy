import { Lock } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'

/** A section of the folder explorer (subfolders, tracks, images, …) with its heading bar. */
export function ExplorerSection({
  title,
  count,
  showTitle = true,
  badge,
  actions,
  children,
}: {
  title: string
  count: number
  /** `false` keeps the heading for assistive tech only (the folder holds one kind of content). */
  showTitle?: boolean
  badge?: ReactNode
  actions?: ReactNode
  children: ReactNode
}) {
  const bar = showTitle || badge || actions
  return (
    <section className="space-y-1.5" aria-label={showTitle ? undefined : title}>
      {bar ? (
        <div className="flex min-h-7 flex-wrap items-center justify-between gap-2">
          <div className="flex items-center gap-1.5">
            {showTitle ? (
              <>
                <h2 className="text-xs font-medium tracking-wide text-muted-foreground uppercase">{title}</h2>
                <span className="tnum text-xs text-muted-foreground/75">({count})</span>
              </>
            ) : null}
            {badge}
          </div>
          {actions ? <div className="ml-auto flex flex-wrap items-center gap-1.5">{actions}</div> : null}
        </div>
      ) : null}
      {children}
    </section>
  )
}

export function ReadonlyBadge() {
  const { t } = useTranslation('manage')
  return (
    <Badge variant="outline" className="shrink-0 gap-1 text-amber-600 dark:text-amber-400">
      <Lock />
      {t('file.readonly')}
    </Badge>
  )
}
