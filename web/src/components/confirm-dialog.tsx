import { useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Spinner } from '@/components/spinner'
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'

export interface ConfirmDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: ReactNode
  description?: ReactNode
  /** Extra content between the description and the buttons. */
  children?: ReactNode
  confirmLabel: ReactNode
  destructive?: boolean
  /** File mutations require a review and a separate final confirmation. */
  twoStep?: boolean
  /** May be async: the dialog shows a spinner and closes when it resolves (stays open on throw). */
  onConfirm: () => unknown
}

/** Confirmation dialog for destructive or irreversible actions. */
export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  children,
  confirmLabel,
  destructive,
  twoStep = false,
  onConfirm,
}: ConfirmDialogProps) {
  const { t } = useTranslation()
  const [pending, setPending] = useState(false)
  const [finalStep, setFinalStep] = useState(false)
  const [acknowledged, setAcknowledged] = useState(false)
  const cancelRef = useRef<HTMLButtonElement>(null)
  const pendingRef = useRef(false)

  // Reset a completed/cancelled session before the next opening, even while mounted.
  const [wasOpen, setWasOpen] = useState(open)
  if (wasOpen !== open) {
    setWasOpen(open)
    setFinalStep(false)
    setAcknowledged(false)
  }

  const confirm = async () => {
    if (pendingRef.current) return
    if (twoStep && !finalStep) {
      setFinalStep(true)
      setAcknowledged(false)
      cancelRef.current?.focus()
      return
    }
    if (twoStep && !acknowledged) return
    pendingRef.current = true
    setPending(true)
    try {
      await onConfirm()
      onOpenChange(false)
    } catch {
      // the caller reports the error; keep the dialog open so the user can retry or cancel
      setFinalStep(false)
      setAcknowledged(false)
    } finally {
      pendingRef.current = false
      setPending(false)
    }
  }

  return (
    <AlertDialog open={open} onOpenChange={(next) => (pending ? undefined : onOpenChange(next))}>
      <AlertDialogContent onOpenAutoFocus={twoStep ? (event) => {
        event.preventDefault()
        cancelRef.current?.focus()
      } : undefined}>
        <AlertDialogHeader>
          {twoStep ? <p className="text-xs text-muted-foreground">{t('manage:confirmation.step', { step: finalStep ? 2 : 1 })}</p> : null}
          <AlertDialogTitle>{finalStep ? t('manage:confirmation.finalTitle') : title}</AlertDialogTitle>
          {finalStep ? <p className="text-sm font-medium">{title}</p> : null}
          {description ? <AlertDialogDescription>{description}</AlertDialogDescription> : null}
        </AlertDialogHeader>
        {children}
        {twoStep && finalStep ? (
          <Label className="flex min-h-11 items-start gap-3 rounded-lg border p-3 text-sm leading-relaxed">
            <Checkbox className="mt-1" checked={acknowledged} onCheckedChange={(value) => setAcknowledged(value === true)} disabled={pending} />
            {t('manage:confirmation.acknowledge')}
          </Label>
        ) : null}
        <AlertDialogFooter>
          <AlertDialogCancel ref={cancelRef} className="max-sm:min-h-11" disabled={pending}>{t('common:actions.cancel')}</AlertDialogCancel>
          <Button className="max-sm:min-h-11" variant={destructive && (!twoStep || finalStep) ? 'destructive' : 'default'} onClick={() => void confirm()} disabled={pending || (twoStep && finalStep && !acknowledged)}>
            {pending ? <Spinner size="sm" className="text-current" /> : null}
            {twoStep && !finalStep ? t('manage:confirmation.continue') : confirmLabel}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
