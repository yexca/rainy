import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { KeyRound, Trash2 } from 'lucide-react'
import { Controller, useForm, useWatch, type Control } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { FormField } from '@/components/form-field'
import { PasswordInput } from '@/components/password-input'
import { ResponsiveDialog } from '@/components/responsive-dialog'
import { Spinner } from '@/components/spinner'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { adminKeys, librariesQuery } from '@/lib/admin-queries'
import { isApiError } from '@/lib/api/client'
import { api, type CreateUserInput } from '@/lib/api/endpoints'
import type { User } from '@/lib/api/types'
import { errorMessage } from '@/lib/errors'

export const PASSWORD_MIN = 4
const USERNAME_MAX = 64

// Messages are i18n keys of the `admin` namespace.
const schema = z.object({
  username: z
    .string()
    .trim()
    .min(1, 'users.validation.usernameRequired')
    .max(USERNAME_MAX, 'users.validation.usernameTooLong')
    .regex(/^[\p{L}\p{N}._\-@+]+$/u, 'users.validation.usernameInvalid'),
  displayName: z.string().trim().max(128),
  email: z.union([z.literal(''), z.string().trim().email('users.validation.emailInvalid')]),
  password: z.string(),
  isAdmin: z.boolean(),
  canManage: z.boolean(),
  canDownload: z.boolean(),
  restrictLibraries: z.boolean(),
  libraryIds: z.array(z.number()),
})
type Values = z.infer<typeof schema>

export interface UserDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** `null` = create a new user. */
  user: User | null
  /** The signed-in admin (to prevent locking yourself out). */
  currentUserId: string
  onResetPassword?: (user: User) => void
  onDelete?: (user: User) => void
}

export function UserDialog({ open, onOpenChange, user, currentUserId, onResetPassword, onDelete }: UserDialogProps) {
  const { t } = useTranslation('admin')
  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={onOpenChange}
      title={user ? t('users.editTitle') : t('users.createTitle')}
      description={user ? user.username : t('users.createDescription')}
    >
      {open ? (
        <UserForm
          key={user?.id ?? 'new'}
          user={user}
          currentUserId={currentUserId}
          onClose={() => onOpenChange(false)}
          onResetPassword={onResetPassword}
          onDelete={onDelete}
        />
      ) : null}
    </ResponsiveDialog>
  )
}

interface UserFormProps {
  user: User | null
  currentUserId: string
  onClose: () => void
  onResetPassword?: (user: User) => void
  onDelete?: (user: User) => void
}

function UserForm({ user, currentUserId, onClose, onResetPassword, onDelete }: UserFormProps) {
  const { t } = useTranslation('admin')
  const queryClient = useQueryClient()
  const isSelf = !!user && user.id === currentUserId
  const {
    register,
    control,
    handleSubmit,
    setError,
    formState: { errors, isDirty },
  } = useForm<Values>({
    resolver: zodResolver(
      user
        ? schema
        : schema.extend({ password: z.string().min(PASSWORD_MIN, 'users.validation.passwordMin') }),
    ),
    defaultValues: {
      username: user?.username ?? '',
      displayName: user?.displayName ?? '',
      email: user?.email ?? '',
      password: '',
      isAdmin: user?.isAdmin ?? false,
      canManage: user?.canManage ?? false,
      canDownload: user?.canDownload ?? true,
      restrictLibraries: user?.restrictLibraries ?? false,
      libraryIds: user?.libraryIds ?? [],
    },
  })
  const isAdmin = useWatch({ control, name: 'isAdmin' })
  const canManage = useWatch({ control, name: 'canManage' })
  // Administrators and managers always see every library.
  const seesAll = isAdmin || canManage

  const save = useMutation({
    mutationFn: (values: Values) => {
      const body: CreateUserInput = {
        username: values.username,
        password: values.password,
        displayName: values.displayName,
        email: values.email,
        isAdmin: values.isAdmin,
        canManage: values.canManage || values.isAdmin,
        canDownload: values.canDownload || values.isAdmin,
        restrictLibraries: values.restrictLibraries && !values.isAdmin && !values.canManage,
        libraryIds: values.restrictLibraries && !values.isAdmin && !values.canManage ? values.libraryIds : [],
      }
      if (user) {
        // Editing never changes the password; that is a separate action.
        const rest: Omit<typeof body, 'password'> & { password?: string } = { ...body }
        delete rest.password
        return api.admin.users.update(user.id, isSelf ? { ...rest, isAdmin: true } : rest)
      }
      return api.admin.users.create(body)
    },
    onSuccess: (saved) => {
      toast.success(user ? t('users.saved', { name: saved.username }) : t('users.created', { name: saved.username }))
      void queryClient.invalidateQueries({ queryKey: adminKeys.users })
      if (isSelf) void queryClient.invalidateQueries({ queryKey: ['auth'] })
      onClose()
    },
    onError: (error) => {
      if (isApiError(error) && error.code === 'conflict') setError('username', { message: 'users.validation.usernameTaken' })
      else toast.error(errorMessage(error, t))
    },
  })

  const message = (key: string | undefined) => (key ? t(key, { min: PASSWORD_MIN, max: USERNAME_MAX }) : undefined)

  const submit = (values: Values) => {
    if (values.restrictLibraries && !values.isAdmin && !values.canManage && values.libraryIds.length === 0) {
      setError('libraryIds', { message: 'users.validation.libraryRequired' })
      return
    }
    save.mutate(values)
  }

  return (
    <form onSubmit={handleSubmit(submit)} className="grid gap-4" noValidate>
      <FormField id="user-username" label={t('users.fields.username')} error={message(errors.username?.message)}>
        <Input
          id="user-username"
          autoComplete="off"
          autoCapitalize="none"
          spellCheck={false}
          aria-invalid={!!errors.username}
          {...register('username')}
        />
      </FormField>
      <div className="grid gap-4 sm:grid-cols-2">
        <FormField id="user-display" label={t('users.fields.displayName')} error={message(errors.displayName?.message)}>
          <Input id="user-display" autoComplete="off" {...register('displayName')} />
        </FormField>
        <FormField id="user-email" label={t('users.fields.email')} error={message(errors.email?.message)}>
          <Input id="user-email" type="email" autoComplete="off" aria-invalid={!!errors.email} {...register('email')} />
        </FormField>
      </div>
      {user ? null : (
        <FormField id="user-password" label={t('users.fields.password')} error={message(errors.password?.message)}>
          <PasswordInput id="user-password" autoComplete="new-password" aria-invalid={!!errors.password} {...register('password')} />
        </FormField>
      )}

      <fieldset className="grid gap-1 rounded-xl border p-1">
        <legend className="sr-only">{t('users.fields.roles')}</legend>
        <RoleSwitch
          name="isAdmin"
          control={control}
          label={t('roles.admin')}
          description={isSelf ? t('users.selfAdmin') : t('roles.adminDescription')}
          disabled={isSelf}
        />
        <RoleSwitch
          name="canManage"
          control={control}
          label={t('roles.manager')}
          description={t('roles.managerDescription')}
          disabled={isAdmin}
          forced={isAdmin}
        />
        <RoleSwitch
          name="canDownload"
          control={control}
          label={t('roles.download')}
          description={t('roles.downloadDescription')}
          disabled={isAdmin}
          forced={isAdmin}
        />
      </fieldset>

      <LibraryAccess control={control} seesAll={seesAll} error={message(errors.libraryIds?.message)} />

      {user && (onResetPassword || onDelete) ? (
        <div className="flex flex-wrap gap-2">
          {onResetPassword ? (
            <Button type="button" variant="outline" size="sm" className="max-sm:h-10" onClick={() => onResetPassword(user)}>
              <KeyRound />
              {t('users.resetTitle')}
            </Button>
          ) : null}
          {onDelete ? (
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="text-destructive hover:text-destructive max-sm:h-10"
              disabled={isSelf}
              title={isSelf ? t('users.selfDelete') : undefined}
              onClick={() => onDelete(user)}
            >
              <Trash2 />
              {t('common:actions.delete')}
            </Button>
          ) : null}
        </div>
      ) : null}

      <div className="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end [&_button]:max-sm:h-11">
        <Button type="button" variant="outline" onClick={onClose} disabled={save.isPending}>
          {t('common:actions.cancel')}
        </Button>
        <Button type="submit" disabled={save.isPending || (!!user && !isDirty)}>
          {save.isPending ? <Spinner size="sm" className="text-current" /> : null}
          {user ? t('common:actions.save') : t('common:actions.create')}
        </Button>
      </div>
    </form>
  )
}

function RoleSwitch({
  name,
  control,
  label,
  description,
  disabled,
  forced,
}: {
  name: 'isAdmin' | 'canManage' | 'canDownload'
  control: Control<Values>
  label: string
  description: string
  disabled?: boolean
  /** Shown as on (implied by admin) regardless of the stored value. */
  forced?: boolean
}) {
  const id = `role-${name}`
  return (
    <Controller
      name={name}
      control={control}
      render={({ field }) => (
        <Label htmlFor={id} className="flex min-h-14 items-center gap-3 rounded-lg px-3 py-2 font-normal hover:bg-accent/40">
          <span className="grid flex-1 gap-0.5">
            <span className="text-sm font-medium">{label}</span>
            <span className="text-xs text-muted-foreground">{description}</span>
          </span>
          <Switch id={id} checked={forced || field.value} onCheckedChange={field.onChange} disabled={disabled} />
        </Label>
      )}
    />
  )
}

/** "All libraries", or the libraries a listener may see (administrators and managers see all). */
function LibraryAccess({ control, seesAll, error }: { control: Control<Values>; seesAll: boolean; error?: string }) {
  const { t } = useTranslation('admin')
  const restrict = useWatch({ control, name: 'restrictLibraries' })
  const libraries = useQuery({ ...librariesQuery, enabled: !seesAll && restrict })
  return (
    <fieldset className="grid gap-1 rounded-xl border p-1">
      <legend className="sr-only">{t('users.access.legend')}</legend>
      <Controller
        name="restrictLibraries"
        control={control}
        render={({ field }) => (
          <Label
            htmlFor="user-all-libraries"
            className="flex min-h-14 items-center gap-3 rounded-lg px-3 py-2 font-normal hover:bg-accent/40"
          >
            <span className="grid flex-1 gap-0.5">
              <span className="text-sm font-medium">{t('users.access.all')}</span>
              <span className="text-xs text-muted-foreground">
                {seesAll ? t('users.access.allForced') : t('users.access.allDescription')}
              </span>
            </span>
            <Switch
              id="user-all-libraries"
              checked={seesAll || !field.value}
              onCheckedChange={(all) => field.onChange(!all)}
              disabled={seesAll}
            />
          </Label>
        )}
      />
      {!seesAll && restrict ? (
        <Controller
          name="libraryIds"
          control={control}
          render={({ field }) => (
            <div role="group" aria-label={t('users.access.legend')} className="grid gap-0.5 border-t pt-1">
              {libraries.isPending ? (
                <div className="flex min-h-11 items-center px-3">
                  <Spinner size="sm" />
                </div>
              ) : libraries.data?.length ? (
                libraries.data.map((lib) => {
                  const id = `user-library-${lib.id}`
                  return (
                    <Label
                      key={lib.id}
                      htmlFor={id}
                      className="flex min-h-11 items-center gap-3 rounded-lg px-3 py-2 font-normal hover:bg-accent/40"
                    >
                      <Checkbox
                        id={id}
                        checked={field.value.includes(lib.id)}
                        aria-invalid={!!error}
                        onCheckedChange={(on) =>
                          field.onChange(on === true ? [...field.value, lib.id] : field.value.filter((v) => v !== lib.id))
                        }
                      />
                      <span className="min-w-0 flex-1 truncate text-sm">{lib.name}</span>
                    </Label>
                  )
                })
              ) : (
                <p className="px-3 py-2 text-sm text-muted-foreground">{t('users.access.none')}</p>
              )}
              {error ? (
                <p role="alert" className="px-3 pb-1 text-xs text-destructive">
                  {error}
                </p>
              ) : null}
            </div>
          )}
        />
      ) : null}
    </fieldset>
  )
}
