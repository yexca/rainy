import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useRegisterSW } from 'virtual:pwa-register/react'

// Side effect: start listening for `beforeinstallprompt` as early as possible.
import '../lib/install-prompt'

/** Check for a new app version this often while the app stays open (ms). */
const UPDATE_CHECK_INTERVAL = 60 * 60 * 1000

/** One periodic update check per page, even when the host remounts (logout → login). */
let updateTimer: ReturnType<typeof setInterval> | undefined

/**
 * Service worker registration (`registerType: 'prompt'`): shows an "update available" toast
 * with a reload action, and a one-time "ready to work offline" toast after the first install.
 */
export function PwaPrompt() {
  const { t } = useTranslation('player')
  const {
    needRefresh: [needRefresh, setNeedRefresh],
    offlineReady: [offlineReady, setOfflineReady],
    updateServiceWorker,
  } = useRegisterSW({
    onRegisteredSW(_url, registration) {
      if (!registration || updateTimer !== undefined) return
      updateTimer = setInterval(() => {
        if (registration.installing || !navigator.onLine) return
        void registration.update().catch(() => undefined)
      }, UPDATE_CHECK_INTERVAL)
    },
  })

  useEffect(() => {
    if (!needRefresh) return
    toast(t('pwa.updateTitle'), {
      id: 'pwa-update',
      description: t('pwa.updateDescription'),
      duration: Number.POSITIVE_INFINITY,
      action: { label: t('pwa.reload'), onClick: () => void updateServiceWorker(true) },
      cancel: { label: t('pwa.later'), onClick: () => setNeedRefresh(false) },
      onDismiss: () => setNeedRefresh(false),
    })
  }, [needRefresh, setNeedRefresh, t, updateServiceWorker])

  useEffect(() => {
    if (!offlineReady) return
    toast.success(t('pwa.offlineReady'), {
      id: 'pwa-offline',
      description: t('pwa.offlineReadyDescription'),
      onDismiss: () => setOfflineReady(false),
      onAutoClose: () => setOfflineReady(false),
    })
  }, [offlineReady, setOfflineReady, t])

  return null
}
