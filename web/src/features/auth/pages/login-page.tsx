import { zodResolver } from '@hookform/resolvers/zod'
import { CircleAlert, WifiOff } from 'lucide-react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

import { Spinner } from '@/components/spinner'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useAuth, useLogin } from '@/hooks/use-auth'
import { isApiError } from '@/lib/api/client'
import { isTouch } from '@/lib/platform'

import { authErrorMessage } from '../auth-errors'
import { AuthCard } from '../components/auth-card'
import { FormField } from '../components/form-field'
import { PasswordInput } from '../components/password-input'

// Messages are i18n keys in the `auth` namespace.
const loginSchema = z.object({
  username: z.string().trim().min(1, 'validation.usernameRequired'),
  password: z.string().min(1, 'validation.passwordRequired'),
})
type LoginValues = z.infer<typeof loginSchema>

export default function LoginPage() {
  const { t } = useTranslation('auth')
  const auth = useAuth()
  const login = useLogin()
  const {
    register,
    handleSubmit,
    resetField,
    setFocus,
    formState: { errors },
  } = useForm<LoginValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: { username: '', password: '' },
  })

  const clearServerError = () => {
    if (login.isError) login.reset()
  }

  const onSubmit = handleSubmit((values) => {
    login.mutate(values, {
      onError: (error) => {
        if (isApiError(error) && error.status === 401) {
          resetField('password')
          setFocus('password')
        }
      },
    })
    // On success the auth guard redirects to the page that asked for a login.
  })

  return (
    <AuthCard title={t('login.title')} subtitle={t('login.subtitle')}>
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
        <FormField
          id="username"
          label={t('fields.username')}
          error={errors.username?.message ? t(errors.username.message) : undefined}
        >
          <Input
            id="username"
            autoComplete="username"
            autoCapitalize="none"
            autoCorrect="off"
            spellCheck={false}
            enterKeyHint="next"
            autoFocus={!isTouch}
            placeholder={t('placeholders.username')}
            aria-invalid={errors.username ? true : undefined}
            aria-describedby={errors.username ? 'username-error' : undefined}
            className="h-11 rounded-xl bg-background/60"
            {...register('username', { onChange: clearServerError })}
          />
        </FormField>

        <FormField
          id="password"
          label={t('fields.password')}
          error={errors.password?.message ? t(errors.password.message) : undefined}
        >
          <PasswordInput
            id="password"
            autoComplete="current-password"
            enterKeyHint="go"
            placeholder={t('placeholders.password')}
            aria-invalid={errors.password ? true : undefined}
            aria-describedby={errors.password ? 'password-error' : undefined}
            className="h-11 rounded-xl bg-background/60"
            {...register('password', { onChange: clearServerError })}
          />
        </FormField>

        {login.isError ? (
          <Alert variant="destructive" className="rounded-xl" role="alert">
            <CircleAlert aria-hidden />
            <AlertDescription>{authErrorMessage(login.error, t, 'login')}</AlertDescription>
          </Alert>
        ) : null}

        <Button type="submit" size="lg" className="mt-2 h-11 w-full rounded-xl text-[15px]" disabled={login.isPending}>
          {login.isPending ? (
            <>
              <Spinner size="sm" className="text-primary-foreground" />
              {t('login.submitting')}
            </>
          ) : (
            t('login.submit')
          )}
        </Button>
      </form>
    </AuthCard>
  )
}
