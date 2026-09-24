import { FileQuestion, RotateCw, ShieldAlert, TriangleAlert } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { isRouteErrorResponse, Link, useRouteError } from 'react-router'

import { EmptyState } from '@/components/empty-state'
import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { Button } from '@/components/ui/button'

/** 404 inside the app shell. */
export function NotFoundPage() {
  const { t } = useTranslation()
  return (
    <Page>
      <PageHeader title={t('notFound.title')} back />
      <EmptyState
        icon={FileQuestion}
        title={t('notFound.title')}
        description={t('notFound.description')}
        action={
          <Button asChild>
            <Link to="/">{t('actions.goHome')}</Link>
          </Button>
        }
      />
    </Page>
  )
}

/** Shown by the manager / admin route guards. */
export function ForbiddenPage() {
  const { t } = useTranslation()
  return (
    <Page>
      <PageHeader title={t('forbidden.title')} back />
      <EmptyState
        icon={ShieldAlert}
        title={t('forbidden.title')}
        description={t('forbidden.description')}
        action={
          <Button asChild variant="outline">
            <Link to="/">{t('actions.goHome')}</Link>
          </Button>
        }
      />
    </Page>
  )
}

/**
 * Router `errorElement`: render crashes and failed lazy imports (e.g. a stale chunk after a
 * deploy — reloading fetches the new build).
 */
export function RouteErrorPage() {
  const { t } = useTranslation()
  const error = useRouteError()

  if (isRouteErrorResponse(error) && error.status === 404) {
    return (
      <div className="grid min-h-dvh place-items-center p-6">
        <EmptyState icon={FileQuestion} title={t('notFound.title')} description={t('notFound.description')} />
      </div>
    )
  }

  if (import.meta.env.DEV) console.error(error)
  return (
    <div className="grid min-h-dvh place-items-center p-6">
      <EmptyState
        icon={TriangleAlert}
        title={t('errors.renderTitle')}
        description={t('errors.renderDescription')}
        action={
          <>
            <Button onClick={() => window.location.reload()}>
              <RotateCw aria-hidden />
              {t('actions.reload')}
            </Button>
            <Button variant="outline" asChild>
              <a href="/">{t('actions.goHome')}</a>
            </Button>
          </>
        }
      />
    </div>
  )
}
