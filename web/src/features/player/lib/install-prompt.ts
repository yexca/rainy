/**
 * "Install app" support: captures Chromium's `beforeinstallprompt` so the settings page can
 * offer an install button (iOS has no prompt: the page shows "Share → Add to Home Screen").
 * Imported eagerly by the player host so the event is never missed.
 */
import { create } from 'zustand'

import { isStandalone } from '@/lib/platform'

interface BeforeInstallPromptEvent extends Event {
  prompt(): Promise<void>
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>
}

interface InstallState {
  /** Deferred prompt (Chromium), or null. */
  event: BeforeInstallPromptEvent | null
  installed: boolean
}

export const useInstallPrompt = create<InstallState>()(() => ({ event: null, installed: isStandalone }))

if (typeof window !== 'undefined') {
  window.addEventListener('beforeinstallprompt', (event) => {
    event.preventDefault()
    useInstallPrompt.setState({ event: event as BeforeInstallPromptEvent })
  })
  window.addEventListener('appinstalled', () => {
    useInstallPrompt.setState({ event: null, installed: true })
  })
}

/** Show the browser's install dialog; resolves to whether the user accepted. */
export async function promptInstall(): Promise<boolean> {
  const { event } = useInstallPrompt.getState()
  if (!event) return false
  await event.prompt()
  const { outcome } = await event.userChoice
  useInstallPrompt.setState({ event: null })
  return outcome === 'accepted'
}
