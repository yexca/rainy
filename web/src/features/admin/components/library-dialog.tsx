import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { Spinner } from '@/components/spinner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { FormField } from '@/features/auth/components/form-field'
import { ResponsiveDialog } from '@/features/manage/components/responsive-dialog'
import { isApiError } from '@/lib/api/client'
import { api } from '@/lib/api/endpoints'
import type { LibraryInfo } from '@/lib/api/types'
import { errorMessage } from '@/lib/errors'

import { adminKeys } from '../queries'

const schema = z.object({
  name: z.string().trim().min(1, 'libraries.validation.nameRequired').max(100),
  path: z.string().trim().min(1, 'libraries.validation.pathRequired'),
})
type Values = z.infer<typeof schema>

export interface LibraryDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** `null` = add a new library. */
  library: LibraryInfo | null
}

export function LibraryDialog({ open, onOpenChange, library }: LibraryDialogProps) {
  const { t } = useTranslation('admin')
  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={onOpenChange}
      title={library ? t('libraries.editTitle') : t('libraries.addTitle')}
      description={t('libraries.dialogDescription')}
    >
      {open ? <LibraryForm key={library?.id ?? 'new'} library={library} onClose={() => onOpenChange(false)} /> : null}
    </ResponsiveDialog>
  )
}

function LibraryForm({ library, onClose }: { library: LibraryInfo | null; onClose: () => void }) {
  const { t } = useTranslation('admin')
  const queryClient = useQueryClient()
  const {
    register,
    handleSubmit,
    setError,
    formState: { errors, isDirty },
  } = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { name: library?.name ?? '', path: library?.path ?? '' },
  })

  const save = useMutation({
    mutationFn: (v: Values) => (library ? api.admin.libraries.update(library.id, v) : api.admin.libraries.create(v)),
    onSuccess: (saved) => {
      toast.success(library ? t('libraries.saved', { name: saved.name }) : t('libraries.added', { name: saved.name }))
      void queryClient.invalidateQueries({ queryKey: adminKeys.all })
      onClose()
    },
    onError: (error) => {
      if (isApiError(error) && (error.code === 'bad_request' || error.code === 'conflict')) setError('path', { message: error.message })
      else toast.error(errorMessage(error, t))
    },
  })

  const message = (key: string | undefined) => (key ? t(key, { defaultValue: key }) : undefined)

  return (
    <form onSubmit={handleSubmit((v) => save.mutate(v))} className="grid gap-4" noValidate>
      <FormField id="library-name" label={t('libraries.fields.name')} error={message(errors.name?.message)}>
        <Input id="library-name" autoComplete="off" aria-invalid={!!errors.name} {...register('name')} />
      </FormField>
      <FormField id="library-path" label={t('libraries.fields.path')} error={message(errors.path?.message)}>
        <Input
          id="library-path"
          autoComplete="off"
          autoCapitalize="none"
          spellCheck={false}
          placeholder="/music"
          className="font-mono"
          aria-invalid={!!errors.path}
          {...register('path')}
        />
      </FormField>
      <p className="text-xs text-muted-foreground">{t('libraries.pathHint')}</p>
      <div className="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end [&_button]:max-sm:h-11">
        <Button type="button" variant="outline" onClick={onClose} disabled={save.isPending}>
          {t('common:actions.cancel')}
        </Button>
        <Button type="submit" disabled={save.isPending || (!!library && !isDirty)}>
          {save.isPending ? <Spinner size="sm" className="text-current" /> : null}
          {library ? t('common:actions.save') : t('common:actions.add')}
        </Button>
      </div>
    </form>
  )
}
