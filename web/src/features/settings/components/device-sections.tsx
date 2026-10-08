import { Download, Share, SquarePlus } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { promptInstall, useInstallPrompt } from '@/features/player/lib/install-prompt'
import { isIOS, isStandalone } from '@/lib/platform'

import { SettingsRow, SettingsSection } from './settings-ui'

/** "Install app" row: Chromium prompt, iOS instructions, or nothing when already installed. */
export function InstallSection() {
  const { t } = useTranslation('settings')
  const event = useInstallPrompt((s) => s.event)
  const installed = useInstallPrompt((s) => s.installed)

  if (installed || isStandalone) return null
  if (!event && !isIOS) return null

  return (
    <SettingsSection id="install" title={t('install.title')}>
      {event ? (
        <SettingsRow label={t('install.label')} description={t('install.hint')}>
          <Button size="sm" onClick={() => void promptInstall()}>
            <Download />
            {t('install.button')}
          </Button>
        </SettingsRow>
      ) : (
        <div className="grid gap-2 px-4 py-4 text-[15px] md:text-sm">
          <p className="font-medium">{t('install.label')}</p>
          <ol className="grid gap-1.5 text-[13px] text-muted-foreground">
            <li className="flex items-center gap-2">
              <span className="grid size-5 place-items-center rounded-full bg-muted text-[11px] font-semibold text-foreground">1</span>
              {t('install.iosStep1')}
              <Share className="size-4 text-primary" aria-hidden />
            </li>
            <li className="flex items-center gap-2">
              <span className="grid size-5 place-items-center rounded-full bg-muted text-[11px] font-semibold text-foreground">2</span>
              {t('install.iosStep2')}
              <SquarePlus className="size-4 text-foreground" aria-hidden />
            </li>
          </ol>
        </div>
      )}
    </SettingsSection>
  )
}

const SHORTCUTS = [
  { keys: ['Space'], label: 'shortcuts.playPause' },
  { keys: ['←', '→'], label: 'shortcuts.seek' },
  { keys: ['Shift', '←'], label: 'shortcuts.previous' },
  { keys: ['Shift', '→'], label: 'shortcuts.next' },
  { keys: ['M'], label: 'shortcuts.mute' },
] as const

/** Player keyboard shortcuts (hidden on touch-only devices). */
export function ShortcutsSection() {
  const { t } = useTranslation('settings')
  if (typeof window.matchMedia === 'function' && !window.matchMedia('(any-pointer: fine)').matches) return null
  return (
    <SettingsSection id="shortcuts" title={t('shortcuts.title')}>
      {SHORTCUTS.map((shortcut) => (
        <SettingsRow key={shortcut.label} label={t(shortcut.label)}>
          <span className="flex items-center gap-1">
            {shortcut.keys.map((key) => (
              <kbd
                key={key}
                className="inline-flex h-6 min-w-6 items-center justify-center rounded-md border border-b-2 bg-muted px-1.5 font-sans text-xs font-medium text-muted-foreground"
              >
                {key === 'Space' ? t('shortcuts.space') : key}
              </kbd>
            ))}
          </span>
        </SettingsRow>
      ))}
    </SettingsSection>
  )
}
