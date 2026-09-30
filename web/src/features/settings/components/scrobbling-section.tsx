import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { TriangleAlert } from 'lucide-react'
import { useEffect, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useLocation } from 'react-router'
import { toast } from 'sonner'

import { Spinner } from '@/components/spinner'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { api } from '@/lib/api/endpoints'
import type { ScrobbleAccount, ScrobbleService } from '@/lib/api/types'
import { errorMessage } from '@/lib/errors'
import { formatNumber, formatRelative } from '@/lib/format'
import { queryKeys } from '@/lib/query-keys'

import { SettingsRow, SettingsSection } from './settings-ui'

/** Where Last.fm sends the browser back after the user allowed access (route of `lastfm-callback-page`). */
const LASTFM_CALLBACK_PATH = '/settings/lastfm'

const NAMES: Record<ScrobbleService, string> = { lastfm: 'Last.fm', listenbrainz: 'ListenBrainz' }


/** Settings → Scrobbling: link Last.fm / ListenBrainz accounts, pause or unlink them. */
export function ScrobblingSection() {
  const { t } = useTranslation('settings')
  const accounts = useQuery({
    queryKey: queryKeys.scrobbling,
    queryFn: ({ signal }) => api.me.scrobbling.get({ signal }),
  })
  const list = accounts.data ?? []
  const { hash } = useLocation()
  const loaded = accounts.data !== undefined

  // `/settings#scrobbling` (e.g. from the listening report or after linking Last.fm): the
  // sections above settle first, so scroll once this section has its content.
  useEffect(() => {
    if (loaded && hash === '#scrobbling') document.getElementById('scrobbling')?.scrollIntoView({ block: 'start' })
  }, [loaded, hash])

  return (
    <SettingsSection id="scrobbling" title={t('scrobbling.title')} description={t('scrobbling.description')} footer={t('scrobbling.footer')}>
      {accounts.isPending ? (
        <div className="grid min-h-24 place-items-center">
          <Spinner />
        </div>
      ) : accounts.isError ? (
        <p className="px-4 py-3 text-[13px] text-destructive">{errorMessage(accounts.error, t)}</p>
      ) : (
        list.map((a) => <AccountRow key={a.service} account={a} />)
      )}
    </SettingsSection>
  )
}

function AccountRow({ account }: { account: ScrobbleAccount }) {
  const { t } = useTranslation('settings')
  const queryClient = useQueryClient()
  const [confirm, setConfirm] = useState(false)
  const name = NAMES[account.service]

  const setOne = (next: ScrobbleAccount) =>
    queryClient.setQueryData<ScrobbleAccount[]>(queryKeys.scrobbling, (old) => old?.map((a) => (a.service === next.service ? next : a)))

  const toggle = useMutation({
    mutationFn: (enabled: boolean) => api.me.scrobbling.setEnabled(account.service, enabled),
    onSuccess: setOne,
    onError: (error) => toast.error(errorMessage(error, t)),
  })
  const unlink = useMutation({
    mutationFn: () => api.me.scrobbling.unlink(account.service),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.scrobbling })
      toast.success(t('scrobbling.unlinked', { service: name }))
    },
    onError: (error) => toast.error(errorMessage(error, t)),
  })

  if (!account.available && !account.linked) {
    return <SettingsRow label={name} description={t('scrobbling.unavailable', { service: name })} />
  }

  const status = account.linked ? (
    <span className="grid gap-0.5">
      <span>
        {account.needsRelink
          ? t('scrobbling.needsRelink', { service: name })
          : !account.enabled
            ? t('scrobbling.paused', { user: account.username })
            : t('scrobbling.linkedAs', { user: account.username })}
      </span>
      {account.enabled && !account.needsRelink ? (
        <span>
          {[
            account.lastSentAt ? t('scrobbling.lastSent', { when: formatRelative(account.lastSentAt) }) : t('scrobbling.nothingSent'),
            account.queued > 0 ? t('scrobbling.queued', { count: account.queued, formatted: formatNumber(account.queued) }) : '',
          ]
            .filter(Boolean)
            .join(' · ')}
        </span>
      ) : null}
      {account.lastError ? (
        <span className="flex items-start gap-1 text-destructive">
          <TriangleAlert className="mt-0.5 size-3.5 shrink-0" aria-hidden />
          <span className="break-words">{account.lastError}</span>
        </span>
      ) : null}
      {!account.available ? <span>{t('scrobbling.turnedOff', { service: name })}</span> : null}
    </span>
  ) : (
    t(`scrobbling.${account.service}.help`)
  )

  return (
    <>
      <SettingsRow label={name} description={status} stacked>
        {account.linked ? (
          <>
            {account.needsRelink && account.available ? <Connect account={account} relink /> : null}
            {!account.needsRelink ? (
              <Switch
                checked={toggle.isPending ? toggle.variables : account.enabled}
                disabled={toggle.isPending}
                onCheckedChange={(v) => toggle.mutate(v)}
                aria-label={t('scrobbling.enable', { service: name })}
              />
            ) : null}
            <Button
              variant="ghost"
              size="sm"
              disabled={unlink.isPending}
              onClick={() => setConfirm(true)}
              className="text-destructive hover:bg-destructive/10 hover:text-destructive max-md:h-11"
            >
              {unlink.isPending ? <Spinner size="sm" /> : null}
              {t('scrobbling.unlink')}
            </Button>
          </>
        ) : (
          <Connect account={account} />
        )}
      </SettingsRow>
      <AlertDialog open={confirm} onOpenChange={setConfirm}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('scrobbling.unlinkTitle', { service: name })}</AlertDialogTitle>
            <AlertDialogDescription>{t(`scrobbling.${account.service}.unlinkDescription`)}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common:actions.cancel')}</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={() => unlink.mutate()}>
              {t('scrobbling.unlink')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}

/** Connect button (Last.fm: opens its sign-in page) or the token form (ListenBrainz). */
function Connect({ account, relink }: { account: ScrobbleAccount; relink?: boolean }) {
  const { t } = useTranslation('settings')
  const queryClient = useQueryClient()
  const [token, setToken] = useState('')
  const lastfm = useMutation({
    mutationFn: () => api.me.scrobbling.lastfmAuth(`${window.location.origin}${LASTFM_CALLBACK_PATH}`),
    onSuccess: ({ url }) => window.location.assign(url),
    onError: (error) => toast.error(errorMessage(error, t)),
  })
  const listenBrainz = useMutation({
    mutationFn: (value: string) => api.me.scrobbling.linkListenBrainz(value),
    onSuccess: (next) => {
      setToken('')
      queryClient.setQueryData<ScrobbleAccount[]>(queryKeys.scrobbling, (old) => old?.map((a) => (a.service === next.service ? next : a)))
      toast.success(t('scrobbling.linked', { service: NAMES[next.service], user: next.username }))
    },
    onError: (error) => toast.error(errorMessage(error, t)),
  })

  if (account.service === 'lastfm') {
    return (
      <Button size="sm" disabled={lastfm.isPending} onClick={() => lastfm.mutate()} className="max-md:h-11">
        {lastfm.isPending ? <Spinner size="sm" className="text-current" /> : null}
        {relink ? t('scrobbling.relink') : t('scrobbling.connect')}
      </Button>
    )
  }
  const submit = (event: FormEvent) => {
    event.preventDefault()
    if (token.trim()) listenBrainz.mutate(token.trim())
  }
  return (
    <form onSubmit={submit} className="flex w-full gap-2 sm:w-auto">
      <Input
        type="password"
        value={token}
        onChange={(e) => setToken(e.target.value)}
        placeholder={t('scrobbling.listenbrainz.token')}
        aria-label={t('scrobbling.listenbrainz.token')}
        autoComplete="off"
        spellCheck={false}
        className="min-w-0 flex-1 sm:w-56"
      />
      <Button type="submit" size="sm" disabled={!token.trim() || listenBrainz.isPending} className="max-md:h-11">
        {listenBrainz.isPending ? <Spinner size="sm" className="text-current" /> : null}
        {relink ? t('scrobbling.relink') : t('scrobbling.connect')}
      </Button>
    </form>
  )
}
