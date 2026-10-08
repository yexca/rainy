import { useMutation, useQueryClient } from '@tanstack/react-query'
import { LogOut } from 'lucide-react'
import { useState, type FormEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/spinner'
import { PasswordInput } from '@/features/auth/components/password-input'
import { useLogout } from '@/hooks/use-auth'
import { isApiError } from '@/lib/api/client'
import { api, type UpdateMeInput } from '@/lib/api/endpoints'
import type { AuthStatus, User } from '@/lib/api/types'
import { errorMessage } from '@/lib/errors'
import { authQueryKey } from '@/lib/query-client'
import { initials } from '@/lib/utils'

import { SettingsSection } from './settings-ui'

const MIN_PASSWORD = 4
const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

function roleKey(user: User): string {
  if (user.isAdmin) return 'common:user.admin'
  if (user.canManage) return 'common:user.manager'
  return 'common:user.listener'
}

function Field({ id, label, error, hint, children }: { id: string; label: string; error?: string; hint?: string; children: ReactNode }) {
  return (
    <div className="grid gap-1.5">
      <label htmlFor={id} className="text-[13px] font-medium text-foreground/80">
        {label}
      </label>
      {children}
      {error ? (
        <p id={`${id}-error`} role="alert" className="text-[13px] text-destructive">
          {error}
        </p>
      ) : hint ? (
        <p id={`${id}-hint`} className="text-[13px] text-muted-foreground">
          {hint}
        </p>
      ) : null}
    </div>
  )
}

// ---------------------------------------------------------------------------------------------
// Profile
// ---------------------------------------------------------------------------------------------

export function ProfileSection({ user }: { user: User }) {
  const { t } = useTranslation('settings')
  const queryClient = useQueryClient()
  const [displayName, setDisplayName] = useState(user.displayName)
  const [email, setEmail] = useState(user.email)
  const [emailError, setEmailError] = useState<string>()

  // Follow server-side changes (e.g. edited by an admin) while the form is untouched.
  const [synced, setSynced] = useState(user)
  if (synced !== user) {
    setSynced(user)
    if (displayName === synced.displayName) setDisplayName(user.displayName)
    if (email === synced.email) setEmail(user.email)
  }

  const save = useMutation({
    mutationFn: (body: UpdateMeInput) => api.me.update(body),
    onSuccess: (updated) => {
      queryClient.setQueryData<AuthStatus>(authQueryKey, (old) => (old ? { ...old, user: updated } : old))
      toast.success(t('profile.saved'))
    },
    onError: (error) => toast.error(errorMessage(error, t)),
  })

  const name = displayName.trim()
  const mail = email.trim()
  const dirty = name !== user.displayName || mail !== user.email

  const onSubmit = (event: FormEvent) => {
    event.preventDefault()
    if (mail && !EMAIL_RE.test(mail)) {
      setEmailError(t('profile.invalidEmail'))
      return
    }
    setEmailError(undefined)
    save.mutate({ displayName: name, email: mail })
  }

  const shownName = name || user.username

  return (
    <SettingsSection id="profile" title={t('profile.title')}>
      <div className="flex items-center gap-4 px-4 py-4">
        <div
          aria-hidden
          className="grid size-14 shrink-0 place-items-center rounded-full bg-linear-to-br from-primary/80 to-primary text-lg font-semibold text-primary-foreground shadow-sm"
        >
          {initials(shownName)}
        </div>
        <div className="min-w-0">
          <p className="truncate text-lg leading-tight font-semibold">{shownName}</p>
          <p className="truncate text-[13px] text-muted-foreground">
            @{user.username} · {t(roleKey(user))}
          </p>
        </div>
      </div>
      <form onSubmit={onSubmit} className="grid gap-4 px-4 py-4" noValidate>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field id="settings-display-name" label={t('profile.displayName')}>
            <Input
              id="settings-display-name"
              value={displayName}
              maxLength={100}
              autoComplete="name"
              placeholder={user.username}
              onChange={(e) => setDisplayName(e.target.value)}
            />
          </Field>
          <Field id="settings-email" label={t('profile.email')} error={emailError}>
            <Input
              id="settings-email"
              type="email"
              inputMode="email"
              autoComplete="email"
              autoCapitalize="none"
              spellCheck={false}
              value={email}
              maxLength={254}
              aria-invalid={emailError ? true : undefined}
              aria-describedby={emailError ? 'settings-email-error' : undefined}
              onChange={(e) => {
                setEmail(e.target.value)
                setEmailError(undefined)
              }}
            />
          </Field>
        </div>
        <div className="flex justify-end">
          <Button type="submit" disabled={!dirty || save.isPending} className="min-w-24">
            {save.isPending ? <Spinner size="sm" className="text-current" /> : null}
            {t('common:actions.save')}
          </Button>
        </div>
      </form>
    </SettingsSection>
  )
}

// ---------------------------------------------------------------------------------------------
// Password
// ---------------------------------------------------------------------------------------------

interface PasswordErrors {
  current?: string
  next?: string
  confirm?: string
}

export function PasswordSection({ username }: { username: string }) {
  const { t } = useTranslation('settings')
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [confirm, setConfirm] = useState('')
  const [errors, setErrors] = useState<PasswordErrors>({})

  const change = useMutation({
    mutationFn: () => api.me.changePassword({ currentPassword: current, newPassword: next }),
    onSuccess: () => {
      setCurrent('')
      setNext('')
      setConfirm('')
      setErrors({})
      toast.success(t('password.changed'))
    },
    onError: (error) => {
      if (isApiError(error) && (error.status === 403 || error.status === 401)) {
        setErrors({ current: t('password.wrongCurrent') })
        return
      }
      toast.error(errorMessage(error, t))
    },
  })

  const onSubmit = (event: FormEvent) => {
    event.preventDefault()
    const found: PasswordErrors = {}
    if (!current) found.current = t('password.required')
    if ([...next].length < MIN_PASSWORD) found.next = t('password.tooShort', { count: MIN_PASSWORD })
    else if (next === current) found.next = t('password.same')
    if (confirm !== next) found.confirm = t('password.mismatch')
    setErrors(found)
    if (Object.keys(found).length === 0) change.mutate()
  }

  const describedBy = (id: string, error?: string) => (error ? `${id}-error` : undefined)

  return (
    <SettingsSection id="password" title={t('password.title')} footer={t('password.footer')}>
      <form onSubmit={onSubmit} className="grid gap-4 px-4 py-4" noValidate>
        {/* Helps password managers pair the new password with the account. */}
        <input type="text" name="username" autoComplete="username" value={username} className="hidden" readOnly tabIndex={-1} aria-hidden />
        <Field id="settings-current-password" label={t('password.current')} error={errors.current}>
          <PasswordInput
            id="settings-current-password"
            autoComplete="current-password"
            value={current}
            aria-invalid={errors.current ? true : undefined}
            aria-describedby={describedBy('settings-current-password', errors.current)}
            onChange={(e) => setCurrent(e.target.value)}
          />
        </Field>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field id="settings-new-password" label={t('password.new')} error={errors.next} hint={t('password.hint', { count: MIN_PASSWORD })}>
            <PasswordInput
              id="settings-new-password"
              autoComplete="new-password"
              value={next}
              aria-invalid={errors.next ? true : undefined}
              aria-describedby={errors.next ? 'settings-new-password-error' : 'settings-new-password-hint'}
              onChange={(e) => setNext(e.target.value)}
            />
          </Field>
          <Field id="settings-confirm-password" label={t('password.confirm')} error={errors.confirm}>
            <PasswordInput
              id="settings-confirm-password"
              autoComplete="new-password"
              value={confirm}
              aria-invalid={errors.confirm ? true : undefined}
              aria-describedby={describedBy('settings-confirm-password', errors.confirm)}
              onChange={(e) => setConfirm(e.target.value)}
            />
          </Field>
        </div>
        <div className="flex justify-end">
          <Button type="submit" disabled={change.isPending || !current || !next} className="min-w-24">
            {change.isPending ? <Spinner size="sm" className="text-current" /> : null}
            {t('password.submit')}
          </Button>
        </div>
      </form>
    </SettingsSection>
  )
}

/** Full-width "Log out" button at the end of the Account tab. */
export function LogoutButton() {
  const { t } = useTranslation('settings')
  const logout = useLogout()
  const navigate = useNavigate()

  const onLogout = () => {
    logout.mutate(undefined, {
      onSettled: () => navigate('/login', { replace: true }),
    })
  }

  return (
    <Button
      variant="outline"
      onClick={onLogout}
      disabled={logout.isPending}
      className="h-11 w-full rounded-xl text-destructive hover:bg-destructive/10 hover:text-destructive md:h-10"
    >
      {logout.isPending ? <Spinner size="sm" className="text-current" /> : <LogOut />}
      {t('common:actions.logout')}
    </Button>
  )
}
