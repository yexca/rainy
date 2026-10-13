/**
 * Tracks added without an artist or album tag (`BatchResult.untagged`, `DownloadJob.untagged`):
 * every way of adding music (upload, links, online) asks for the missing tags the same way.
 */
import { toast } from 'sonner'

import i18n from '@/lib/i18n'
import { useUI } from '@/stores/ui'

/** Asks to fill in the tags of freshly added tracks; the action opens the tag editor on them. */
export function promptUntagged(trackIds: string[]): void {
  if (trackIds.length === 0) return
  const t = i18n.t.bind(i18n)
  const ids = [...trackIds]
  toast.warning(t('manage:untagged.title', { count: ids.length }), {
    description: t('manage:untagged.description'),
    duration: 20_000,
    action: { label: t('manage:untagged.fill'), onClick: () => useUI.getState().openTagEditor(ids) },
  })
}
