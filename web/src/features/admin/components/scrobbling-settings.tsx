import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CircleAlert, CircleCheck, ExternalLink } from 'lucide-react'
import { useState, type FormEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { toast } from 'sonner'

import { ErrorState } from '@/components/error-state'
import { PageLoader, Spinner } from '@/components/spinner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { adminKeys, scrobblingQuery } from '@/lib/admin-queries'
import { api } from '@/lib/api/endpoints'
import type { ScrobbleAccount, ScrobbleService, ScrobblingAdmin, Settings } from '@/lib/api/types'
import { errorMessage } from '@/lib/errors'
import { formatNumber } from '@/lib/format'
import { queryKeys } from '@/lib/query-keys'
import { cn } from '@/lib/utils'

const LASTFM_CREATE_URL = 'https://www.last.fm/api/account/create'
const HEX32 = /^[0-9a-fA-F]{32}$/

/** Admin → Settings → Scrobbling: allow Last.fm / ListenBrainz and set the Last.fm API account. */
export function ScrobblingSettings() {
  const { t } = useTranslation('admin')
  const queryClient = useQueryClient()
  const info = useQuery(scrobblingQuery)
  // The administrator's own accounts: the settings here only allow a service, every user
  // (administrators included) connects their own account in Settings → Scrobbling.
  const mine = useQuery({
    queryKey: queryKeys.scrobbling,
    queryFn: ({ signal }) => api.me.scrobbling.get({ signal }),
  })
  const own = (service: ScrobbleService) => mine.data?.find((a) => a.service === service)

  const setInfo = (next: ScrobblingAdmin) => queryClient.setQueryData(adminKeys.scrobbling, next)
  const toggle = useMutation({
    mutationFn: (patch: Partial<Pick<Settings, 'lastfmEnabled' | 'listenBrainzEnabled'>>) => api.admin.settings.update(patch),
    onSuccess: (saved) => {
      queryClient.setQueryData<Settings>(adminKeys.settings, (prev) =>
        prev ? { ...prev, lastfmEnabled: saved.lastfmEnabled, listenBrainzEnabled: saved.listenBrainzEnabled } : prev,
      )
      void queryClient.invalidateQueries({ queryKey: adminKeys.scrobbling })
      void queryClient.invalidateQueries({ queryKey: queryKeys.scrobbling })
    },
    onError: (error) => toast.error(errorMessage(error, t)),
  })

  if (!info.data) {
    return info.isError ? <ErrorState error={info.error} onRetry={() => void info.refetch()} retrying={info.isFetching} /> : <PageLoader />
  }
  const d = info.data
  const pending = toggle.isPending ? toggle.variables : {}
  const lastfmOn = pending.lastfmEnabled ?? d.lastfm.enabled
  const lbOn = pending.listenBrainzEnabled ?? d.listenBrainz.enabled
  const connected = (users: number) => t('scrobbling.users', { count: users, formatted: formatNumber(users) })

  return (
    <div className="grid gap-8">
      <p className="px-1 text-xs text-muted-foreground">{t('scrobbling.intro')}</p>
      <OwnAccounts
        accounts={[
          { service: 'lastfm', account: own('lastfm'), enabled: lastfmOn },
          { service: 'listenbrainz', account: own('listenbrainz'), enabled: lbOn },
        ]}
      />
      <Section title={t('scrobbling.lastfm.title')}>
        <Row label={t('scrobbling.lastfm.enable')} description={connected(d.lastfm.users)} inline>
          <Switch
            checked={lastfmOn}
            onCheckedChange={(v) => toggle.mutate({ lastfmEnabled: v })}
            disabled={toggle.isPending}
            aria-label={t('scrobbling.lastfm.enable')}
          />
        </Row>
        <LastfmCredentials info={d} onSaved={setInfo} />
      </Section>

      <Section title={t('scrobbling.listenbrainz.title')}>
        <Row label={t('scrobbling.listenbrainz.enable')} description={connected(d.listenBrainz.users)} inline>
          <Switch
            checked={lbOn}
            onCheckedChange={(v) => toggle.mutate({ listenBrainzEnabled: v })}
            disabled={toggle.isPending}
            aria-label={t('scrobbling.listenbrainz.enable')}
          />
        </Row>
      </Section>
    </div>
  )
}

function LastfmCredentials({ info, onSaved }: { info: ScrobblingAdmin; onSaved: (next: ScrobblingAdmin) => void }) {
  const { t } = useTranslation('admin')
  const [apiKey, setApiKey] = useState(info.lastfm.apiKey)
  const [secret, setSecret] = useState('')
  const keyInvalid = apiKey.trim() !== '' && !HEX32.test(apiKey.trim())
  const secretInvalid = secret.trim() !== '' && !HEX32.test(secret.trim())
  const keyChanged = apiKey.trim() !== info.lastfm.apiKey

  const save = useMutation({
    mutationFn: (body: { apiKey?: string; secret?: string }) => api.admin.scrobbling.setLastfm(body),
    onSuccess: (next) => {
      onSaved(next)
      setApiKey(next.lastfm.apiKey)
      setSecret('')
      toast.success(t('scrobbling.lastfm.saved'))
    },
    onError: (error) => toast.error(errorMessage(error, t)),
  })

  const submit = (event: FormEvent) => {
    event.preventDefault()
    const body: { apiKey?: string; secret?: string } = {}
    if (keyChanged) body.apiKey = apiKey.trim()
    if (secret.trim()) body.secret = secret.trim()
    save.mutate(body)
  }

  return (
    <form onSubmit={submit} className="grid gap-4 p-4" noValidate>
      <div className="grid gap-1">
        <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1">
          <p className="text-sm font-medium">{t('scrobbling.lastfm.account')}</p>
          <Status
            ok={info.lastfm.configured}
            label={info.lastfm.configured ? t('scrobbling.lastfm.configured') : t('scrobbling.lastfm.notConfigured')}
          />
        </div>
        <p className="text-xs text-muted-foreground">
          {t('scrobbling.lastfm.accountHelp')}{' '}
          <a href={LASTFM_CREATE_URL} target="_blank" rel="noreferrer noopener" className="inline-flex items-center gap-0.5 font-medium text-primary hover:underline">
            {t('scrobbling.lastfm.createAccount')}
            <ExternalLink className="size-3" aria-hidden />
          </a>
        </p>
      </div>
      <div className="grid gap-4 sm:grid-cols-2">
        <div className="grid gap-1.5">
          <Label htmlFor="lastfm-key">{t('scrobbling.lastfm.apiKey')}</Label>
          <Input
            id="lastfm-key"
            value={apiKey}
            onChange={(e) => setApiKey(e.target.value)}
            autoComplete="off"
            spellCheck={false}
            className="font-mono"
            aria-invalid={keyInvalid || undefined}
          />
          {keyInvalid ? <p className="text-xs text-destructive">{t('scrobbling.lastfm.hexHint')}</p> : null}
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor="lastfm-secret">{t('scrobbling.lastfm.secret')}</Label>
          <Input
            id="lastfm-secret"
            type="password"
            value={secret}
            onChange={(e) => setSecret(e.target.value)}
            placeholder={info.lastfm.hasSecret ? t('scrobbling.lastfm.secretStored') : ''}
            autoComplete="off"
            spellCheck={false}
            className="font-mono"
            aria-invalid={secretInvalid || undefined}
          />
          {secretInvalid ? <p className="text-xs text-destructive">{t('scrobbling.lastfm.hexHint')}</p> : null}
        </div>
      </div>
      <p className="text-xs text-muted-foreground">{t('scrobbling.lastfm.secretHelp')}</p>
      <div className="flex flex-wrap justify-end gap-2">
        {info.lastfm.hasSecret ? (
          <Button
            type="button"
            variant="ghost"
            disabled={save.isPending}
            onClick={() => save.mutate({ secret: '' })}
            className="text-destructive hover:bg-destructive/10 hover:text-destructive max-md:h-11"
          >
            {t('scrobbling.lastfm.removeSecret')}
          </Button>
        ) : null}
        <Button type="submit" disabled={save.isPending || keyInvalid || secretInvalid || (!keyChanged && !secret.trim())} className="max-md:h-11">
          {save.isPending ? <Spinner size="sm" className="text-current" /> : null}
          {t('scrobbling.lastfm.save')}
        </Button>
      </div>
    </form>
  )
}

/**
 * "Your account": whether the signed-in administrator connected their own account to each
 * service that is allowed (or still linked), with one link to Settings → Scrobbling.
 */
function OwnAccounts({ accounts }: { accounts: { service: ScrobbleService; account: ScrobbleAccount | undefined; enabled: boolean }[] }) {
  const { t } = useTranslation('admin')
  const shown = accounts.filter((a) => a.account && (a.enabled || a.account.linked))
  if (shown.length === 0) return null
  const anyToConnect = shown.some((a) => a.enabled && !(a.account!.linked && !a.account!.needsRelink))
  return (
    <div className="grid gap-3 rounded-xl border p-4 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
      <div className="grid gap-1.5">
        <p className="text-sm font-medium">{t('scrobbling.own.label')}</p>
        <div className="flex flex-wrap gap-x-4 gap-y-1">
          {shown.map(({ service, account }) => {
            const a = account!
            const linked = a.linked && !a.needsRelink
            const state = a.needsRelink
              ? t('scrobbling.own.needsRelink')
              : !a.linked
                ? t('scrobbling.own.notLinked')
                : a.enabled
                  ? t('scrobbling.own.linked', { user: a.username })
                  : t('scrobbling.own.paused', { user: a.username })
            return <Status key={service} ok={linked && a.enabled} label={`${t(`scrobbling.${service}.title`)} · ${state}`} />
          })}
        </div>
      </div>
      <Button variant="outline" size="sm" asChild className="justify-self-start max-md:h-11">
        <Link to="/settings/scrobbling">{anyToConnect ? t('scrobbling.own.connect') : t('scrobbling.own.manage')}</Link>
      </Button>
    </div>
  )
}

function Status({ ok, label }: { ok: boolean; label: string }) {
  const Icon = ok ? CircleCheck : CircleAlert
  return (
    <p className={cn('flex items-center gap-1.5 text-sm sm:justify-end', ok ? 'text-muted-foreground' : 'text-amber-600 dark:text-amber-400')}>
      <Icon className={cn('size-4 shrink-0', ok && 'text-emerald-500')} strokeWidth={1.75} aria-hidden />
      {label}
    </p>
  )
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="grid gap-2">
      <h2 className="px-1 text-xs font-medium tracking-wide text-muted-foreground uppercase">{title}</h2>
      <div className="divide-y rounded-xl border">{children}</div>
    </section>
  )
}

function Row({ label, description, inline, children }: { label: string; description?: string; inline?: boolean; children: ReactNode }) {
  return (
    <div className={cn('grid gap-3 p-4', inline ? 'grid-cols-[minmax(0,1fr)_auto] items-center' : 'sm:grid-cols-[minmax(0,1fr)_minmax(0,1.1fr)] sm:items-center')}>
      <div className="grid gap-0.5">
        <Label className="text-sm font-medium">{label}</Label>
        {description ? <p className="text-xs text-muted-foreground">{description}</p> : null}
      </div>
      <div className="grid gap-1.5">{children}</div>
    </div>
  )
}
