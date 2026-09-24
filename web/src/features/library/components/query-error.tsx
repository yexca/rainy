import type { LucideIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Button } from '@/components/ui/button'
import { isApiError } from '@/lib/api/client'

export interface QueryErrorProps {
  error: unknown
  onRetry: () => void
  retrying?: boolean
  /** Shown for 404s. */
  notFound: { icon: LucideIcon; title: string; backTo: string; backLabel: string }
}

/** Error block for detail pages: a friendly "not found" for 404s, otherwise a retryable error. */
export function QueryError({ error, onRetry, retrying, notFound }: QueryErrorProps) {
  const { t } = useTranslation()
  if (isApiError(error) && error.status === 404) {
    return (
      <EmptyState
        icon={notFound.icon}
        title={notFound.title}
        description={t('common:errors.not_found')}
        action={
          <Button variant="outline" asChild>
            <Link to={notFound.backTo}>{notFound.backLabel}</Link>
          </Button>
        }
      />
    )
  }
  return <ErrorState error={error} onRetry={onRetry} retrying={retrying} />
}
