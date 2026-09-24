import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { api } from '@/lib/api/endpoints'

import { reportBatch, toastError } from '../lib/batch'
import { invalidateLibrary } from '../queries'
import { ConfirmDialog } from './confirm-dialog'
import { CoverDialog } from './cover-dialog'
import { EncodingDialog } from './encoding-dialog'
import { RenameDialog } from './rename-dialog'

export type BatchDialogKind = 'rename' | 'cover' | 'encoding' | 'delete'

export interface BatchDialogState {
  kind: BatchDialogKind
  trackIds: readonly string[]
}

export interface BatchDialogsProps {
  dialog: BatchDialogState | null
  onClose: () => void
  /** Called after an action succeeded (e.g. to clear the selection). */
  onDone?: (kind: BatchDialogKind) => void
}

/** The batch action dialogs shared by the manager, folder browser and doctor. */
export function BatchDialogs({ dialog, onClose, onDone }: BatchDialogsProps) {
  const { t } = useTranslation('manage')
  const queryClient = useQueryClient()
  // Keep showing the last selection while a dialog animates out.
  const [last, setLast] = useState(dialog)
  if (dialog && dialog !== last) setLast(dialog)
  const ids = (dialog ?? last)?.trackIds ?? []
  const openChange = (open: boolean) => (open ? undefined : onClose())

  const remove = async () => {
    try {
      const result = await api.manage.deleteTracks([...ids])
      reportBatch(result, 'manage:delete.done')
      void invalidateLibrary(queryClient)
      onDone?.('delete')
    } catch (error) {
      toastError(error)
      throw error
    }
  }

  return (
    <>
      <RenameDialog open={dialog?.kind === 'rename'} onOpenChange={openChange} trackIds={ids} onDone={() => onDone?.('rename')} />
      <CoverDialog open={dialog?.kind === 'cover'} onOpenChange={openChange} trackIds={ids} onDone={() => onDone?.('cover')} />
      <EncodingDialog open={dialog?.kind === 'encoding'} onOpenChange={openChange} trackIds={ids} onDone={() => onDone?.('encoding')} />
      <ConfirmDialog
        open={dialog?.kind === 'delete'}
        onOpenChange={openChange}
        title={t('delete.title', { count: ids.length })}
        description={t('delete.description', { count: ids.length })}
        confirmLabel={t('delete.confirm')}
        destructive
        onConfirm={remove}
      />
    </>
  )
}
