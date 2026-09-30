import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate, useSearchParams } from 'react-router'
import { toast } from 'sonner'

import { ErrorState } from '@/components/error-state'
import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { PageLoader } from '@/components/spinner'
import { Button } from '@/components/ui/button'
import { api } from '@/lib/api/endpoints'
import { queryKeys } from '@/lib/query-keys'

/**
 * `/settings/lastfm`: Last.fm sends the browser here with `token` (and the `state` Rainy
 * issued) after the user allowed access; this finishes linking the account.
 */
export default function LastfmCallbackPage() {
  const { t } = useTranslation('settings')
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const started = useRef(false)
  const token = params.get('token') ?? ''
  const state = params.get('state') ?? ''

  const link = useMutation({
    mutationFn: () => api.me.scrobbling.linkLastfm({ token, state }),
    onSuccess: (account) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.scrobbling })
      toast.success(t('scrobbling.linked', { service: 'Last.fm', user: account.username }))
      navigate('/settings#scrobbling', { replace: true })
    },
  })

  useEffect(() => {
    // Once: a token can be exchanged only one time (StrictMode runs effects twice).
    if (started.current || !token) return
    started.current = true
    link.mutate()
  }, [link, token])

  return (
    <Page>
      <PageHeader title={t('scrobbling.callbackTitle')} back="/settings" />
      {!token || link.isError ? (
        <div className="grid justify-items-center gap-4">
          {!token ? (
            <ErrorState title={t('scrobbling.callbackDenied')} description={t('scrobbling.callbackDeniedHint')} />
          ) : (
            <ErrorState error={link.error} title={t('scrobbling.callbackFailed')} />
          )}
          <Button variant="outline" asChild>
            <Link to="/settings#scrobbling" replace>
              {t('scrobbling.backToSettings')}
            </Link>
          </Button>
        </div>
      ) : (
        <PageLoader />
      )}
    </Page>
  )
}
