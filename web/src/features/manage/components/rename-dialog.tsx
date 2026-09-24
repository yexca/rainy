import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useVirtualizer } from '@tanstack/react-virtual'
import { ArrowRight, RotateCcw } from 'lucide-react'
import { useDeferredValue, useId, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Spinner } from '@/components/spinner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { useDefaultRenamePattern } from '@/features/admin/queries'
import { api } from '@/lib/api/endpoints'
import type { RenamePlan } from '@/lib/api/types'
import { errorMessage } from '@/lib/errors'
import { cn } from '@/lib/utils'

import { reportBatch, toastError } from '../lib/batch'
import { RENAME_TOKENS } from '../lib/rename-tokens'
import { invalidateLibrary } from '../queries'
import { ResponsiveDialog } from './responsive-dialog'

const PATTERN_KEY = 'rainy.manage.renamePattern'
const STATUSES: readonly RenamePlan['status'][] = ['ok', 'unchanged', 'conflict', 'invalid']
const STATUS_STYLE: Record<RenamePlan['status'], string> = {
  ok: 'bg-emerald-500/12 text-emerald-700 dark:text-emerald-400',
  unchanged: 'bg-muted text-muted-foreground',
  conflict: 'bg-amber-500/15 text-amber-700 dark:text-amber-400',
  invalid: 'bg-destructive/12 text-destructive',
}

function storedPattern(): string {
  try {
    return localStorage.getItem(PATTERN_KEY) ?? ''
  } catch {
    return ''
  }
}

export interface RenameDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  trackIds: readonly string[]
  onDone?: () => void
}

/** Rename / organize files by a tag pattern, with a live server-side preview. */
export function RenameDialog({ open, onOpenChange, trackIds, onDone }: RenameDialogProps) {
  const { t } = useTranslation('manage')
  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('rename.title')}
      description={t('rename.description', { count: trackIds.length })}
      size="xl"
    >
      {open ? <RenameBody trackIds={trackIds} onClose={() => onOpenChange(false)} onDone={onDone} /> : null}
    </ResponsiveDialog>
  )
}

function RenameBody({ trackIds, onClose, onDone }: { trackIds: readonly string[]; onClose: () => void; onDone?: () => void }) {
  const { t } = useTranslation('manage')
  const queryClient = useQueryClient()
  const session = useId()
  const defaultPattern = useDefaultRenamePattern()
  const [pattern, setPattern] = useState(() => storedPattern() || defaultPattern)
  const [changesOnly, setChangesOnly] = useState(true)
  const inputRef = useRef<HTMLInputElement>(null)
  const deferred = useDeferredValue(pattern.trim())

  const preview = useQuery({
    queryKey: ['manage', 'rename-preview', session, deferred],
    enabled: deferred.length > 0 && trackIds.length > 0,
    queryFn: ({ signal }) => api.manage.rename.preview({ trackIds: [...trackIds], pattern: deferred }, { signal }),
    placeholderData: (prev) => prev,
    staleTime: Infinity,
  })

  const plans = preview.data?.items ?? []
  const counts = Object.fromEntries(STATUSES.map((s) => [s, plans.filter((p) => p.status === s).length])) as Record<
    RenamePlan['status'],
    number
  >
  const shown = changesOnly ? plans.filter((p) => p.status !== 'unchanged') : plans
  const okIds = plans.filter((p) => p.status === 'ok').map((p) => p.trackId)
  const stale = deferred !== pattern.trim() || preview.isFetching

  const apply = useMutation({
    mutationFn: () => api.manage.rename.apply({ trackIds: okIds, pattern: deferred }),
    onSuccess: (result) => {
      try {
        localStorage.setItem(PATTERN_KEY, deferred)
      } catch {
        // not remembered
      }
      reportBatch(result, 'manage:rename.done')
      void invalidateLibrary(queryClient)
      onDone?.()
      onClose()
    },
    onError: toastError,
  })

  const insert = (token: string) => {
    const el = inputRef.current
    const start = el?.selectionStart ?? pattern.length
    const end = el?.selectionEnd ?? pattern.length
    const next = pattern.slice(0, start) + token + pattern.slice(end)
    setPattern(next)
    requestAnimationFrame(() => {
      el?.focus()
      el?.setSelectionRange(start + token.length, start + token.length)
    })
  }

  return (
    <div className="grid gap-5">
      <div className="grid gap-2">
        <div className="flex items-center justify-between gap-2">
          <Label htmlFor="rename-pattern" className="text-[13px] font-medium text-foreground/75">
            {t('rename.pattern')}
          </Label>
          {pattern !== defaultPattern ? (
            <Button variant="ghost" size="xs" className="text-muted-foreground" onClick={() => setPattern(defaultPattern)}>
              <RotateCcw />
              {t('rename.useDefault')}
            </Button>
          ) : null}
        </div>
        <Input
          id="rename-pattern"
          ref={inputRef}
          value={pattern}
          onChange={(e) => setPattern(e.target.value)}
          className="font-mono"
          autoComplete="off"
          autoCapitalize="off"
          spellCheck={false}
        />
        <div className="flex flex-wrap gap-1.5">
          {RENAME_TOKENS.map((token) => (
            <button
              key={token}
              type="button"
              onClick={() => insert(token)}
              className="h-7 rounded-full border bg-background px-2.5 font-mono text-xs text-muted-foreground transition-colors hover:border-primary/50 hover:text-foreground"
            >
              {token}
            </button>
          ))}
        </div>
        <p className="text-xs text-muted-foreground">{t('rename.help')}</p>
      </div>

      <div className="grid gap-2">
        <div className="flex flex-wrap items-center gap-1.5">
          {STATUSES.map((s) =>
            counts[s] > 0 ? (
              <span key={s} className={cn('inline-flex h-6 items-center rounded-full px-2.5 text-xs font-medium', STATUS_STYLE[s])}>
                {t(`rename.status.${s}`)} · {counts[s]}
              </span>
            ) : null,
          )}
          {stale ? <Spinner size="sm" className="ml-1" /> : null}
          <Label className="ml-auto flex items-center gap-2 text-xs font-normal text-muted-foreground">
            <Switch checked={changesOnly} onCheckedChange={setChangesOnly} size="sm" />
            {t('rename.changesOnly')}
          </Label>
        </div>
        {preview.isError ? (
          <p className="rounded-lg bg-destructive/10 px-3 py-4 text-sm text-destructive">{errorMessage(preview.error, t)}</p>
        ) : preview.isPending && deferred ? (
          <div className="flex h-40 items-center justify-center rounded-lg border">
            <Spinner />
          </div>
        ) : (
          <PlanList plans={shown} />
        )}
      </div>

      <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end [&_button]:max-sm:h-11">
        <Button variant="outline" onClick={onClose} disabled={apply.isPending}>
          {t('common:actions.cancel')}
        </Button>
        <Button onClick={() => apply.mutate()} disabled={okIds.length === 0 || stale || apply.isPending}>
          {apply.isPending ? <Spinner size="sm" className="text-current" /> : null}
          {t('rename.apply', { count: okIds.length })}
        </Button>
      </div>
    </div>
  )
}

function PlanList({ plans }: { plans: RenamePlan[] }) {
  const { t } = useTranslation('manage')
  const scrollRef = useRef<HTMLDivElement>(null)
  // eslint-disable-next-line react-hooks/incompatible-library -- TanStack Virtual is used as documented
  const virtualizer = useVirtualizer({
    count: plans.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => 58,
    overscan: 8,
  })

  if (plans.length === 0) {
    return <p className="rounded-lg border px-3 py-10 text-center text-sm text-muted-foreground">{t('rename.nothing')}</p>
  }

  return (
    <div ref={scrollRef} className="h-[min(34dvh,22rem)] overflow-y-auto rounded-lg border">
      <div className="relative" style={{ height: virtualizer.getTotalSize() }}>
        {virtualizer.getVirtualItems().map((item) => {
          const plan = plans[item.index]
          return (
            <div
              key={plan.trackId}
              data-index={item.index}
              ref={virtualizer.measureElement}
              className="absolute inset-x-0 top-0 grid gap-1 border-b px-3 py-2 last:border-b-0"
              style={{ transform: `translateY(${item.start}px)` }}
            >
              <div className="flex items-start gap-2">
                <p className="min-w-0 flex-1 font-mono text-xs break-all text-muted-foreground">{plan.from}</p>
                <Badge className={cn('h-5 shrink-0 border-0 text-[10px]', STATUS_STYLE[plan.status])}>
                  {t(`rename.status.${plan.status}`)}
                </Badge>
              </div>
              {plan.status !== 'unchanged' ? (
                <p className="flex min-w-0 items-start gap-1.5 font-mono text-xs break-all">
                  <ArrowRight className="mt-0.5 size-3 shrink-0 text-muted-foreground" aria-hidden />
                  <span className={cn(plan.status === 'ok' ? 'text-foreground' : 'text-muted-foreground')}>{plan.to}</span>
                </p>
              ) : null}
              {plan.message ? <p className="text-xs text-muted-foreground">{plan.message}</p> : null}
            </div>
          )
        })}
      </div>
    </div>
  )
}
