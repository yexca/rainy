/** Toast helpers for `BatchResult` responses of the manage API. */
import { toast } from 'sonner'

import { isApiError } from '@/lib/api/client'
import type { BatchResult, ItemError } from '@/lib/api/types'
import { errorMessage } from '@/lib/errors'
import i18n from '@/lib/i18n'

/** Whether an item error / API error is caused by a read-only file system. */
export function isReadonlyMessage(message: string): boolean {
  return /read[- ]?only|permission denied|EROFS|access is denied/i.test(message)
}

export function isReadonlyError(error: unknown): boolean {
  return isApiError(error) && error.code === 'readonly'
}

/**
 * Report a batch result: success toast with `successKey` (plural, `{{count}}`), or a warning
 * listing the first failures. Returns the item errors.
 */
export function reportBatch(result: BatchResult, successKey: string): ItemError[] {
  const t = i18n.t.bind(i18n)
  const errors = result.errors ?? []
  const updated = result.updated?.length ?? 0
  if (errors.length === 0) {
    toast.success(t(successKey, { count: updated }))
    return errors
  }
  const readonly = errors.some((e) => isReadonlyMessage(e.error))
  const lines = errors.slice(0, 3).map((e) => `${e.path || e.trackId}: ${e.error}`)
  if (errors.length > 3) lines.push(t('manage:batch.more', { count: errors.length - 3 }))
  toast.error(t('manage:batch.failed', { count: errors.length, ok: updated }), {
    description: readonly ? t('manage:readonly.short') : lines.join('\n'),
    duration: 8000,
  })
  return errors
}

/** Toast for a thrown error (readonly errors get the volume-mount explanation). */
export function toastError(error: unknown): void {
  const t = i18n.t.bind(i18n)
  if (isReadonlyError(error)) {
    toast.error(t('manage:readonly.title'), { description: t('manage:readonly.description'), duration: 10000 })
    return
  }
  toast.error(errorMessage(error, t))
}
