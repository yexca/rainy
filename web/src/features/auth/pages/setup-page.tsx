import { zodResolver } from '@hookform/resolvers/zod'
import { CircleAlert, ShieldCheck, WifiOff } from 'lucide-react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

import { Spinner } from '@/components/spinner'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useAuth, useSetup } from '@/hooks/use-auth'
import { isApiError } from '@/lib/api/client'
import { isTouch } from '@/lib/platform'

import { authErrorMessage } from '../auth-errors'
import { AuthCard } from '../components/auth-card'
import { FormField } from '../components/form-field'
import { PasswordInput } from '../components/password-input'

const PASSWORD_MIN = 4
const USERNAME_MAX = 64

// Messages are i18n keys in the `auth` namespace (interpolated with {min, max}).
const setupSchema = z
  .object({
    username: z
      .string()
      .trim()
      .min(1, 'validation.usernameRequired')
      .max(USERNAME_MAX, 'validation.usernameTooLong')
      .regex(/^\S+$/, 'validation.usernameInvalid'),
    password: z.string().min(PASSWORD_MIN, 'validation.passwordMin'),
    confirm: z.string(),
  })
  .refine((values) => values.password === values.confirm, {
    path: ['confirm'],
    error: 'validation.passwordMismatch',
  })
type SetupValues = z.infer<typeof setupSchema>

/** First run: create the administrator account. */
export default function SetupPage() {
  const { t } = useTranslation('auth')
  const auth = useAuth()
  const setup = useSetup()
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<SetupValues>({
    resolver: zodResolver(setupSchema),
    defaultValues: { username: '', password: '', confirm: '' },
  })

  const message = (key: string | undefined) => (key ? t(key, { min: PASSWORD_MIN, max: USERNAME_MAX }) : undefined)
  const clearServerError = () => {
    if (setup.isError) setup.reset()
  }

  const onSubmit = handleSubmit(({ username, password }) => {
    setup.mutate(
      { username, password },
      {
        onError: (error) => {
          // Someone else finished the setup: the refreshed status sends us to /login.
          if (isApiError(error) && (error.status === 403 || error.status === 409)) auth.refetch()
        },
      },
    )
  })

  const inputClass = 'h-11 rounded-xl bg-background/60'

  return (
    <AuthCard title={t('setup.title')} subtitle={t('setup.subtitle')}>
      {auth.isError ? (
        <Alert className="mb-5 rounded-xl">
          <WifiOff aria-hidden />
          <AlertDescription className="flex flex-wrap items-center gap-x-2">
            <span>{t('server.unreachable')}</span>
            <Button variant="link" size="sm" className="h-auto p-0" onClick={auth.refetch}>
              {t('server.retry')}
            </Button>
          </AlertDescription>
        </Alert>
      ) : null}

      <form onSubmit={onSubmit} noValidate className="grid gap-4">
        <FormField id="username" label={t('fields.username')} error={message(errors.username?.message)}>
          <Input
            id="username"
            autoComplete="username"
            autoCapitalize="none"
            autoCorrect="off"
            spellCheck={false}
            enterKeyHint="next"
            autoFocus={!isTouch}
            placeholder={t('placeholders.newUsername')}
            aria-invalid={errors.username ? true : undefined}
            aria-describedby={errors.username ? 'username-error' : undefined}
            className={inputClass}
            {...register('username', { onChange: clearServerError })}
          />
        </FormField>

        <FormField id="password" label={t('fields.password')} error={message(errors.password?.message)}>
          <PasswordInput
            id="password"
            autoComplete="new-password"
            enterKeyHint="next"
            placeholder={t('placeholders.newPassword', { min: PASSWORD_MIN })}
            aria-invalid={errors.password ? true : undefined}
            aria-describedby={errors.password ? 'password-error' : undefined}
            className={inputClass}
            {...register('password', { onChange: clearServerError })}
          />
        </FormField>

        <FormField id="confirm" label={t('fields.confirmPassword')} error={message(errors.confirm?.message)}>
          <PasswordInput
            id="confirm"
            autoComplete="new-password"
            enterKeyHint="go"
            placeholder={t('placeholders.confirmPassword')}
            aria-invalid={errors.confirm ? true : undefined}
            aria-describedby={errors.confirm ? 'confirm-error' : undefined}
            className={inputClass}
            {...register('confirm', { onChange: clearServerError })}
          />
        </FormField>

        {setup.isError ? (
          <Alert variant="destructive" className="rounded-xl" role="alert">
            <CircleAlert aria-hidden />
            <AlertDescription>{authErrorMessage(setup.error, t, 'setup')}</AlertDescription>
          </Alert>
        ) : (
          <p className="flex items-start gap-2 text-xs text-muted-foreground">
            <ShieldCheck className="mt-px size-4 shrink-0 text-primary" aria-hidden />
            <span>{t('setup.note')}</span>
          </p>
        )}

        <Button type="submit" size="lg" className="mt-1 h-11 w-full rounded-xl text-[15px]" disabled={setup.isPending}>
          {setup.isPending ? (
            <>
              <Spinner size="sm" className="text-primary-foreground" />
              {t('setup.submitting')}
            </>
          ) : (
            t('setup.submit')
          )}
        </Button>
      </form>
    </AuthCard>
  )
}
