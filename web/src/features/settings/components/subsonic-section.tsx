import { useMutation, useQueryClient } from '@tanstack/react-query'
import { KeyRound, Laptop, Smartphone, TabletSmartphone, TriangleAlert } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

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
import { Spinner } from '@/components/spinner'
import { api } from '@/lib/api/endpoints'
import type { AuthStatus, User } from '@/lib/api/types'
import { errorMessage } from '@/lib/errors'
import { authQueryKey } from '@/lib/query-client'

import { CopyField, SettingsRow, SettingsSection } from './settings-ui'

const CLIENTS = [
  { platform: 'ios', icon: Smartphone, apps: ['Amperfy', 'play:Sub', 'Substreamer'] },
  { platform: 'android', icon: TabletSmartphone, apps: ['Symfonium', 'Tempo', 'Ultrasonic'] },
  { platform: 'desktop', icon: Laptop, apps: ['Feishin', 'Supersonic', 'Sonixd'] },
] as const

/** Connection details for Subsonic / OpenSubsonic apps + API key management. */
export function SubsonicSection({ user }: { user: User }) {
  const { t } = useTranslation('settings')
  const queryClient = useQueryClient()
  const [newKey, setNewKey] = useState<string | null>(null)
  const [confirm, setConfirm] = useState<'regenerate' | 'revoke' | null>(null)
  const serverUrl = typeof window === 'undefined' ? '' : window.location.origin

  const setHasKey = (hasApiKey: boolean) =>
    queryClient.setQueryData<AuthStatus>(authQueryKey, (old) =>
      old?.user ? { ...old, user: { ...old.user, hasApiKey } } : old,
    )

  const generate = useMutation({
    mutationFn: () => api.me.createApiKey(),
    onSuccess: ({ apiKey }) => {
      setNewKey(apiKey)
      setHasKey(true)
    },
    onError: (error) => toast.error(errorMessage(error, t)),
  })

  const revoke = useMutation({
    mutationFn: () => api.me.deleteApiKey(),
    onSuccess: () => {
      setNewKey(null)
      setHasKey(false)
      toast.success(t('subsonic.revoked'))
    },
    onError: (error) => toast.error(errorMessage(error, t)),
  })

  const busy = generate.isPending || revoke.isPending

  return (
    <SettingsSection
      id="subsonic"
      title={t('subsonic.title')}
      description={t('subsonic.description')}
      footer={t('subsonic.passwordNote')}
    >
      <SettingsRow label={t('subsonic.server')} stacked>
        <CopyField value={serverUrl} label={t('subsonic.server')} className="w-full sm:w-72" />
      </SettingsRow>
      <SettingsRow label={t('subsonic.username')} stacked>
        <CopyField value={user.username} label={t('subsonic.username')} className="w-full sm:w-72" />
      </SettingsRow>

      <SettingsRow
        label={
          <span className="inline-flex items-center gap-2">
            <KeyRound className="size-4 text-muted-foreground" aria-hidden />
            {t('subsonic.apiKey')}
          </span>
        }
        description={user.hasApiKey ? t('subsonic.keyActive') : t('subsonic.keyNone')}
        stacked
      >
        {user.hasApiKey ? (
          <>
            <Button variant="outline" size="sm" disabled={busy} onClick={() => setConfirm('regenerate')}>
              {generate.isPending ? <Spinner size="sm" /> : null}
              {t('subsonic.regenerate')}
            </Button>
            <Button
              variant="ghost"
              size="sm"
              disabled={busy}
              onClick={() => setConfirm('revoke')}
              className="text-destructive hover:bg-destructive/10 hover:text-destructive"
            >
              {revoke.isPending ? <Spinner size="sm" /> : null}
              {t('subsonic.revoke')}
            </Button>
          </>
        ) : (
          <Button size="sm" disabled={busy} onClick={() => generate.mutate()}>
            {generate.isPending ? <Spinner size="sm" className="text-current" /> : null}
            {t('subsonic.generate')}
          </Button>
        )}
      </SettingsRow>

      {newKey ? (
        <div className="grid gap-2 bg-primary/5 px-4 py-4" role="status">
          <p className="flex items-start gap-2 text-[13px] font-medium">
            <TriangleAlert className="mt-0.5 size-4 shrink-0 text-amber-500" aria-hidden />
            {t('subsonic.copyNow')}
          </p>
          <CopyField value={newKey} label={t('subsonic.apiKey')} className="bg-background" />
          <div className="flex justify-end">
            <Button variant="ghost" size="sm" onClick={() => setNewKey(null)}>
              {t('common:actions.done')}
            </Button>
          </div>
        </div>
      ) : null}

      <div className="px-4 py-4">
        <p className="text-[15px] font-medium md:text-sm">{t('subsonic.clients')}</p>
        <p className="mt-0.5 text-[13px] text-muted-foreground">{t('subsonic.clientsHint')}</p>
        <div className="mt-3 grid gap-2 sm:grid-cols-3">
          {CLIENTS.map(({ platform, icon: Icon, apps }) => (
            <div key={platform} className="rounded-lg border bg-muted/30 px-3 py-2.5">
              <p className="flex items-center gap-1.5 text-xs font-semibold text-muted-foreground">
                <Icon className="size-3.5" aria-hidden />
                {t(`subsonic.platform.${platform}`)}
              </p>
              <p className="mt-1 text-sm">{apps.join(' · ')}</p>
            </div>
          ))}
        </div>
      </div>

      <AlertDialog open={confirm !== null} onOpenChange={(open) => !open && setConfirm(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {confirm === 'revoke' ? t('subsonic.revokeTitle') : t('subsonic.regenerateTitle')}
            </AlertDialogTitle>
            <AlertDialogDescription>{t('subsonic.invalidateWarning')}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common:actions.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              variant={confirm === 'revoke' ? 'destructive' : 'default'}
              onClick={() => {
                if (confirm === 'revoke') revoke.mutate()
                else generate.mutate()
                setConfirm(null)
              }}
            >
              {confirm === 'revoke' ? t('subsonic.revoke') : t('subsonic.regenerate')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SettingsSection>
  )
}
