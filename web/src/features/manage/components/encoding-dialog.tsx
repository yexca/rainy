import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowRight, Languages } from 'lucide-react'
import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Spinner } from '@/components/spinner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { api } from '@/lib/api/endpoints'
import type { EncodingFix } from '@/lib/api/types'

import { reportBatch, toastError } from '../lib/batch'
import { invalidateLibrary } from '../queries'
import { ConfirmDialog } from './confirm-dialog'
import { ResponsiveDialog } from './responsive-dialog'

export interface EncodingDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Tracks to check; empty = every track the library doctor flagged (server semantics). */
  trackIds: readonly string[]
  onDone?: () => void
}

/** Preview mojibake repairs (GBK / Big5 / Shift-JIS) for a selection, then write the chosen ones. */
export function EncodingDialog({ open, onOpenChange, trackIds, onDone }: EncodingDialogProps) {
  const { t } = useTranslation('manage')
  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('encoding.title')}
      description={trackIds.length > 0 ? t('encoding.description', { count: trackIds.length }) : t('encoding.descriptionAll')}
      size="lg"
    >
      {open ? <EncodingBody trackIds={trackIds} onClose={() => onOpenChange(false)} onDone={onDone} /> : null}
    </ResponsiveDialog>
  )
}

function EncodingBody({ trackIds, onClose, onDone }: { trackIds: readonly string[]; onClose: () => void; onDone?: () => void }) {
  const { t } = useTranslation('manage')
  const queryClient = useQueryClient()
  const [confirmOpen, setConfirmOpen] = useState(false)
  const session = useId()
  const [excluded, setExcluded] = useState<ReadonlySet<string>>(new Set())

  const preview = useQuery({
    queryKey: ['manage', 'encoding-preview', session],
    queryFn: () => api.manage.encoding({ trackIds: [...trackIds], apply: false }),
    staleTime: Infinity,
  })
  const fixes = preview.data?.items ?? []
  const chosen = fixes.filter((f) => !excluded.has(f.trackId)).map((f) => f.trackId)

  const apply = useMutation({
    mutationFn: () => api.manage.encoding({ trackIds: chosen, apply: true }),
    onSuccess: (res) => {
      if (res.result) reportBatch(res.result, 'manage:encoding.done')
      void invalidateLibrary(queryClient)
      onDone?.()
      onClose()
    },
    onError: toastError,
  })

  const toggle = (id: string, include: boolean) =>
    setExcluded((prev) => {
      const next = new Set(prev)
      if (include) next.delete(id)
      else next.add(id)
      return next
    })

  let body
  if (preview.isPending) {
    body = (
      <div className="flex h-40 items-center justify-center">
        <Spinner />
      </div>
    )
  } else if (preview.isError) {
    body = <ErrorState error={preview.error} onRetry={() => void preview.refetch()} size="compact" />
  } else if (fixes.length === 0) {
    body = <EmptyState icon={Languages} size="compact" title={t('encoding.none')} description={t('encoding.noneDescription')} />
  } else {
    body = (
      <ul className="max-h-[50dvh] divide-y overflow-y-auto rounded-lg border sm:max-h-96">
        {fixes.map((fix) => (
          <FixRow key={fix.trackId} fix={fix} included={!excluded.has(fix.trackId)} onToggle={(v) => toggle(fix.trackId, v)} />
        ))}
      </ul>
    )
  }

  return (
    <div className="grid gap-4">
      <p className="text-xs text-muted-foreground">{t('encoding.hint')}</p>
      {body}
      <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end [&_button]:max-sm:h-11">
        <Button variant="outline" onClick={onClose} disabled={apply.isPending}>
          {fixes.length === 0 ? t('common:actions.close') : t('common:actions.cancel')}
        </Button>
        {fixes.length > 0 ? (
          <Button onClick={() => setConfirmOpen(true)} disabled={chosen.length === 0 || apply.isPending}>
            {apply.isPending ? <Spinner size="sm" className="text-current" /> : null}
            {t('encoding.apply', { count: chosen.length })}
          </Button>
        ) : null}
      </div>
      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={t('confirmation.editTitle', { count: chosen.length })}
        description={t('encoding.hint')}
        confirmLabel={t('encoding.apply', { count: chosen.length })}
        twoStep
        onConfirm={() => {
          if (chosen.length === 0 || apply.isPending) return
          return apply.mutateAsync()
        }}
      />
    </div>
  )
}

function FixRow({ fix, included, onToggle }: { fix: EncodingFix; included: boolean; onToggle: (v: boolean) => void }) {
  const { t } = useTranslation('manage')
  return (
    <li className="grid grid-cols-[auto_minmax(0,1fr)] gap-3 px-3 py-2.5">
      <Checkbox className="mt-0.5" checked={included} onCheckedChange={(v) => onToggle(v === true)} aria-label={t('encoding.include')} />
      <div className="grid min-w-0 gap-1.5">
        <div className="flex items-start gap-2">
          <p className="min-w-0 flex-1 font-mono text-xs break-all text-muted-foreground">{fix.path}</p>
          <Badge variant="secondary" className="shrink-0 uppercase">
            {fix.encoding}
          </Badge>
        </div>
        {Object.entries(fix.changes).map(([key, change]) => (
          <div key={key} className="grid gap-0.5 text-[13px] sm:grid-cols-[7rem_minmax(0,1fr)] sm:gap-2">
            <span className="font-mono text-[11px] text-muted-foreground sm:pt-0.5">{key}</span>
            <span className="flex min-w-0 flex-wrap items-baseline gap-x-1.5">
              <span className="break-all text-muted-foreground line-through decoration-destructive/40">{change.old.join('; ')}</span>
              <ArrowRight className="size-3 shrink-0 self-center text-muted-foreground" aria-hidden />
              <span className="font-medium break-all">{change.new.join('; ')}</span>
            </span>
          </div>
        ))}
      </div>
    </li>
  )
}
