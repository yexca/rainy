import { useCallback, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Drawer, DrawerContent, DrawerDescription, DrawerTitle } from '@/components/ui/drawer'
import { Sheet, SheetContent, SheetDescription, SheetTitle } from '@/components/ui/sheet'
import { useIsMobile } from '@/hooks/use-media-query'
import { useUI } from '@/stores/ui'

import { ConfirmDialog } from './components/confirm-dialog'
import { TagEditor } from './tag-editor/tag-editor'

/**
 * Mounted once by the app shell; opened from anywhere with
 * `useUI.getState().openTagEditor(trackIds)`. Right-side sheet (560px) on tablets/desktop,
 * full-screen drawer on phones. Closing with unsaved changes asks for confirmation.
 */
export function TagEditorHost() {
  const { t } = useTranslation('manage')
  const { open, trackIds } = useUI((s) => s.tagEditor)
  const close = useUI((s) => s.closeTagEditor)
  const isMobile = useIsMobile()
  const dirtyRef = useRef(false)
  const [confirmDiscard, setConfirmDiscard] = useState(false)

  // A fresh editor (state reset) every time the editor opens, even for the same tracks.
  const [session, setSession] = useState(0)
  const [wasOpen, setWasOpen] = useState(open)
  if (open !== wasOpen) {
    setWasOpen(open)
    if (open) setSession((s) => s + 1)
  }

  const onDirtyChange = useCallback((dirty: boolean) => {
    dirtyRef.current = dirty
  }, [])

  const forceClose = useCallback(() => {
    dirtyRef.current = false
    close()
  }, [close])

  const requestClose = useCallback(() => {
    if (dirtyRef.current) setConfirmDiscard(true)
    else close()
  }, [close])

  const editor =
    trackIds.length > 0 ? (
      <TagEditor
        key={session}
        trackIds={trackIds}
        onClose={forceClose}
        onRequestClose={requestClose}
        onDirtyChange={onDirtyChange}
      />
    ) : null

  const description = t('common:tagEditor.selected', { count: trackIds.length })

  return (
    <>
      {isMobile ? (
        <Drawer open={open} onOpenChange={(next) => (next ? undefined : requestClose())} handleOnly repositionInputs={false}>
          <DrawerContent
            className="h-[calc(100dvh-var(--safe-top)-0.5rem)] data-[vaul-drawer-direction=bottom]:mt-0 data-[vaul-drawer-direction=bottom]:max-h-none data-[vaul-drawer-direction=bottom]:rounded-t-2xl [&>div:first-child]:mt-2 [&>div:first-child]:h-1.5 [&>div:first-child]:w-10"
          >
            <DrawerTitle className="sr-only">{t('common:tagEditor.title')}</DrawerTitle>
            <DrawerDescription className="sr-only">{description}</DrawerDescription>
            {editor}
          </DrawerContent>
        </Drawer>
      ) : (
        <Sheet open={open} onOpenChange={(next) => (next ? undefined : requestClose())}>
          <SheetContent side="right" showCloseButton={false} className="w-full gap-0 p-0 sm:max-w-[560px]">
            <SheetTitle className="sr-only">{t('common:tagEditor.title')}</SheetTitle>
            <SheetDescription className="sr-only">{description}</SheetDescription>
            {editor}
          </SheetContent>
        </Sheet>
      )}
      <ConfirmDialog
        open={confirmDiscard}
        onOpenChange={setConfirmDiscard}
        title={t('editor.discardTitle')}
        description={t('editor.discardDescription')}
        confirmLabel={t('editor.discard')}
        destructive
        onConfirm={forceClose}
      />
    </>
  )
}
