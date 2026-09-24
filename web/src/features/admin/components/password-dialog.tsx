import { useMutation } from '@tanstack/react-query'
import { Copy, Dices } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Spinner } from '@/components/spinner'
import { Button } from '@/components/ui/button'
import { FormField } from '@/features/auth/components/form-field'
import { PasswordInput } from '@/features/auth/components/password-input'
import { ResponsiveDialog } from '@/features/manage/components/responsive-dialog'
import { api } from '@/lib/api/endpoints'
import type { User } from '@/lib/api/types'
import { errorMessage } from '@/lib/errors'

import { PASSWORD_MIN } from './user-dialog'

const ALPHABET = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789'

function randomPassword(length = 14): string {
  const bytes = crypto.getRandomValues(new Uint8Array(length))
  return Array.from(bytes, (b) => ALPHABET[b % ALPHABET.length]).join('')
}

export interface PasswordDialogProps {
  user: User | null
  onOpenChange: (open: boolean) => void
}

/** Admin reset of another user's password (`PUT /api/admin/users/{id}` with `password`). */
export function PasswordDialog({ user, onOpenChange }: PasswordDialogProps) {
  const { t } = useTranslation('admin')
  return (
    <ResponsiveDialog
      open={user !== null}
      onOpenChange={onOpenChange}
      title={t('users.resetTitle')}
      description={user ? t('users.resetDescription', { name: user.username }) : undefined}
      size="sm"
    >
      {user ? <PasswordForm key={user.id} user={user} onClose={() => onOpenChange(false)} /> : null}
    </ResponsiveDialog>
  )
}

function PasswordForm({ user, onClose }: { user: User; onClose: () => void }) {
  const { t } = useTranslation('admin')
  const [password, setPassword] = useState('')
  const [touched, setTouched] = useState(false)
  const tooShort = password.length < PASSWORD_MIN

  const save = useMutation({
    mutationFn: () => api.admin.users.update(user.id, { password }),
    onSuccess: () => {
      toast.success(t('users.passwordReset', { name: user.username }))
      onClose()
    },
    onError: (error) => toast.error(errorMessage(error, t)),
  })

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(password)
      toast.success(t('common:actions.copied'))
    } catch {
      toast.error(t('common:errors.unknown'))
    }
  }

  return (
    <form
      className="grid gap-4"
      noValidate
      onSubmit={(e) => {
        e.preventDefault()
        setTouched(true)
        if (!tooShort) save.mutate()
      }}
    >
      <FormField id="reset-password" label={t('users.fields.newPassword')} error={touched && tooShort ? t('users.validation.passwordMin', { min: PASSWORD_MIN }) : undefined}>
        <PasswordInput id="reset-password" autoComplete="new-password" value={password} onChange={(e) => setPassword(e.target.value)} />
      </FormField>
      <div className="flex flex-wrap gap-2">
        <Button type="button" variant="outline" size="sm" onClick={() => setPassword(randomPassword())}>
          <Dices />
          {t('users.generate')}
        </Button>
        <Button type="button" variant="ghost" size="sm" onClick={() => void copy()} disabled={!password}>
          <Copy />
          {t('common:actions.copy')}
        </Button>
      </div>
      <p className="text-xs text-muted-foreground">{t('users.resetHint')}</p>
      <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end [&_button]:max-sm:h-11">
        <Button type="button" variant="outline" onClick={onClose} disabled={save.isPending}>
          {t('common:actions.cancel')}
        </Button>
        <Button type="submit" disabled={save.isPending}>
          {save.isPending ? <Spinner size="sm" className="text-current" /> : null}
          {t('users.resetAction')}
        </Button>
      </div>
    </form>
  )
}
