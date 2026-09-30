import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { RotateCcw } from 'lucide-react'
import type { ReactNode } from 'react'
import { Controller, useForm, useWatch } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { Spinner } from '@/components/spinner'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { RENAME_TOKENS } from '@/features/manage/lib/rename-tokens'
import { api } from '@/lib/api/endpoints'
import type { Settings } from '@/lib/api/types'
import { errorMessage } from '@/lib/errors'
import { cn } from '@/lib/utils'

import { DEFAULT_RENAME_PATTERN, adminKeys } from '../queries'

const DURATION_RE = /^(0|(\d+(\.\d+)?(ns|us|µs|ms|s|m|h))+)$/
const INTERVAL_PRESETS = ['0', '15m', '30m', '1h', '6h', '12h', '24h'] as const
const BITRATES = [96, 128, 160, 192, 256, 320] as const

const schema = z.object({
  scanInterval: z.string().trim().regex(DURATION_RE, 'settings.validation.duration'),
  genreSeparators: z.string().max(16),
  ignoredArticles: z.string().max(500),
  coverArtFiles: z.string().trim().min(1, 'settings.validation.required'),
  transcodeFormat: z.enum(['mp3', 'opus', 'aac']),
  transcodeBitrate: z.number().int().min(32).max(512),
  renamePattern: z.string().trim().min(1, 'settings.validation.required').regex(/\{[a-z]+(:\d+)?\}/, 'settings.validation.pattern'),
  fixEncodingOnScan: z.boolean(),
  enableDownloads: z.boolean(),
  onlineMetadata: z.boolean(),
  onlineMetadataChinaIp: z.boolean(),
  ytdlpEnabled: z.boolean(), // edited on the yt-dlp tab
})

/** Server settings. Initialised once from `initial`; background refetches never overwrite edits. */
export function SettingsForm({ initial }: { initial: Settings }) {
  const { t } = useTranslation('admin')
  const queryClient = useQueryClient()
  const {
    register,
    control,
    handleSubmit,
    reset,
    setValue,
    formState: { errors, isDirty, dirtyFields },
  } = useForm<Settings>({ resolver: zodResolver(schema), defaultValues: initial })
  const onlineMetadata = useWatch({ control, name: 'onlineMetadata' })

  const save = useMutation({
    mutationFn: (values: Settings) => {
      // Send only what changed (partial update).
      const patch: Partial<Settings> = {}
      for (const key of Object.keys(dirtyFields) as (keyof Settings)[]) {
        ;(patch as Record<string, unknown>)[key] = values[key]
      }
      return api.admin.settings.update(patch)
    },
    onSuccess: (saved) => {
      queryClient.setQueryData(adminKeys.settings, saved)
      reset(saved)
      toast.success(t('settings.saved'))
    },
    onError: (error) => toast.error(errorMessage(error, t)),
  })

  const err = (key: string | undefined) => (key ? t(key, { defaultValue: key }) : undefined)

  return (
    <form onSubmit={handleSubmit((v) => save.mutate(v))} className="grid gap-8" noValidate>
      <Section title={t('settings.sections.scanning')}>
        <Row label={t('settings.fields.scanInterval')} description={t('settings.help.scanInterval')} error={err(errors.scanInterval?.message)}>
          <Controller
            control={control}
            name="scanInterval"
            render={({ field }) => (
              <Select value={field.value} onValueChange={field.onChange}>
                <SelectTrigger className="w-full sm:w-48">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {!INTERVAL_PRESETS.includes(field.value as (typeof INTERVAL_PRESETS)[number]) ? (
                    <SelectItem value={field.value}>{field.value}</SelectItem>
                  ) : null}
                  {INTERVAL_PRESETS.map((p) => (
                    <SelectItem key={p} value={p}>
                      {t(`settings.intervals.${p}`)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          />
        </Row>
        <Row label={t('settings.fields.fixEncodingOnScan')} description={t('settings.help.fixEncodingOnScan')} inline>
          <Controller control={control} name="fixEncodingOnScan" render={({ field }) => <Switch checked={field.value} onCheckedChange={field.onChange} />} />
        </Row>
      </Section>

      <Section title={t('settings.sections.metadata')}>
        <Row label={t('settings.fields.genreSeparators')} description={t('settings.help.genreSeparators')} error={err(errors.genreSeparators?.message)}>
          <Input className="font-mono sm:w-48" autoComplete="off" spellCheck={false} {...register('genreSeparators')} />
        </Row>
        <Row label={t('settings.fields.ignoredArticles')} description={t('settings.help.ignoredArticles')} error={err(errors.ignoredArticles?.message)}>
          <Input autoComplete="off" spellCheck={false} {...register('ignoredArticles')} />
        </Row>
        <Row label={t('settings.fields.coverArtFiles')} description={t('settings.help.coverArtFiles')} error={err(errors.coverArtFiles?.message)}>
          <Input className="font-mono" autoComplete="off" spellCheck={false} {...register('coverArtFiles')} />
        </Row>
        <Row label={t('settings.fields.onlineMetadata')} description={t('settings.help.onlineMetadata')} inline>
          <Controller control={control} name="onlineMetadata" render={({ field }) => <Switch checked={field.value} onCheckedChange={field.onChange} />} />
        </Row>
        <Row label={t('settings.fields.onlineMetadataChinaIp')} description={t('settings.help.onlineMetadataChinaIp')} inline>
          <Controller
            control={control}
            name="onlineMetadataChinaIp"
            render={({ field }) => <Switch checked={field.value} onCheckedChange={field.onChange} disabled={!onlineMetadata} />}
          />
        </Row>
      </Section>

      <Section title={t('settings.sections.streaming')}>
        <Row label={t('settings.fields.transcodeFormat')} description={t('settings.help.transcodeFormat')}>
          <Controller
            control={control}
            name="transcodeFormat"
            render={({ field }) => (
              <Select value={field.value} onValueChange={field.onChange}>
                <SelectTrigger className="w-full sm:w-48">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="mp3">MP3</SelectItem>
                  <SelectItem value="opus">Opus</SelectItem>
                  <SelectItem value="aac">AAC</SelectItem>
                </SelectContent>
              </Select>
            )}
          />
        </Row>
        <Row label={t('settings.fields.transcodeBitrate')} description={t('settings.help.transcodeBitrate')}>
          <Controller
            control={control}
            name="transcodeBitrate"
            render={({ field }) => (
              <Select value={String(field.value)} onValueChange={(v) => field.onChange(Number(v))}>
                <SelectTrigger className="w-full sm:w-48">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {!BITRATES.includes(field.value as (typeof BITRATES)[number]) ? (
                    <SelectItem value={String(field.value)}>{field.value} kbps</SelectItem>
                  ) : null}
                  {BITRATES.map((b) => (
                    <SelectItem key={b} value={String(b)}>
                      {b} kbps
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          />
        </Row>
        <Row label={t('settings.fields.enableDownloads')} description={t('settings.help.enableDownloads')} inline>
          <Controller control={control} name="enableDownloads" render={({ field }) => <Switch checked={field.value} onCheckedChange={field.onChange} />} />
        </Row>
      </Section>

      <Section title={t('settings.sections.organize')}>
        <Row label={t('settings.fields.renamePattern')} description={t('settings.help.renamePattern')} error={err(errors.renamePattern?.message)} stacked>
          <div className="grid gap-2">
            <div className="flex gap-2">
              <Input className="font-mono" autoComplete="off" autoCapitalize="none" spellCheck={false} {...register('renamePattern')} />
              <Button
                type="button"
                variant="ghost"
                size="icon"
                aria-label={t('settings.resetPattern')}
                title={t('settings.resetPattern')}
                onClick={() => setValue('renamePattern', DEFAULT_RENAME_PATTERN, { shouldDirty: true, shouldValidate: true })}
              >
                <RotateCcw />
              </Button>
            </div>
            <p className="flex flex-wrap gap-1">
              {RENAME_TOKENS.map((token) => (
                <code key={token} className="rounded bg-muted px-1.5 py-0.5 text-[11px] text-muted-foreground">
                  {token}
                </code>
              ))}
            </p>
          </div>
        </Row>
      </Section>

      <div
        className={cn(
          'sticky bottom-[calc(var(--tabbar-h)+var(--miniplayer-h)+var(--safe-bottom)+1rem)] z-20 flex items-center justify-end gap-2 transition-opacity md:bottom-[calc(var(--player-clearance)+1rem)]',
          isDirty ? 'opacity-100' : 'pointer-events-none opacity-0',
        )}
        aria-hidden={!isDirty}
      >
        <div className="glass flex items-center gap-2 rounded-2xl border p-2 shadow-lg">
          <span className="px-2 text-sm text-muted-foreground">{t('settings.unsaved')}</span>
          <Button type="button" variant="ghost" onClick={() => reset()} disabled={save.isPending}>
            {t('settings.discard')}
          </Button>
          <Button type="submit" disabled={save.isPending}>
            {save.isPending ? <Spinner size="sm" className="text-current" /> : null}
            {t('common:actions.save')}
          </Button>
        </div>
      </div>
    </form>
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

function Row({
  label,
  description,
  error,
  inline,
  stacked,
  children,
}: {
  label: string
  description?: string
  error?: string
  /** Control stays on the right even on phones (switches). */
  inline?: boolean
  /** Control always below the label. */
  stacked?: boolean
  children: ReactNode
}) {
  return (
    <div
      className={cn(
        'grid gap-3 p-4',
        inline ? 'grid-cols-[minmax(0,1fr)_auto] items-center' : stacked ? '' : 'sm:grid-cols-[minmax(0,1fr)_minmax(0,1.1fr)] sm:items-center',
      )}
    >
      <div className="grid gap-0.5">
        <Label className="text-sm font-medium">{label}</Label>
        {description ? <p className="text-xs text-muted-foreground">{description}</p> : null}
      </div>
      <div className="grid gap-1.5">
        {children}
        {error ? <p className="text-xs text-destructive">{error}</p> : null}
      </div>
    </div>
  )
}
