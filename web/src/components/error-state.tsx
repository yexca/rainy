import { RotateCw, TriangleAlert, WifiOff } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'
import { Button } from '@/components/ui/button'
import { errorMessage, isNetworkError } from '@/lib/errors'

export interface ErrorStateProps {
  /** The thrown value (ApiError, Error, …) — used for the icon and message. */
  error?: unknown
  title?: ReactNode
  /** Overrides the message derived from `error`. */
  description?: ReactNode
  /** Shows a "Try again" button. */
  onRetry?: () => void
  /** Spinner on the retry button while refetching. */
  retrying?: boolean
  size?: 'default' | 'compact'
  className?: string
}

/** Error placeholder with a translated message and an optional retry button. */
export function ErrorState({ error, title, description, onRetry, retrying, size, className }: ErrorStateProps) {
  const { t } = useTranslation()
  const offline = isNetworkError(error)
  return (
    <EmptyState
      icon={offline ? WifiOff : TriangleAlert}
      title={title ?? (offline ? t('errors.serverUnreachableTitle') : t('errors.title'))}
      description={description ?? errorMessage(error, t)}
      size={size}
      className={className}
      action={
        onRetry ? (
          <Button variant="outline" onClick={onRetry} disabled={retrying}>
            <RotateCw className={retrying ? 'animate-spin' : undefined} aria-hidden />
            {t('actions.retry')}
          </Button>
        ) : null
      }
    />
  )
}
