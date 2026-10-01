import { useTranslation } from 'react-i18next'

import { MascotArt } from '@/components/mascot'
import { Switch } from '@/components/ui/switch'
import { useUI } from '@/stores/ui'

import { SettingsRow, SettingsSection } from './settings-ui'

/** Settings › Mascot: illustrations on status screens and the corner companion. */
export function MascotSection() {
  const { t } = useTranslation('settings')
  const mascotArt = useUI((s) => s.mascotArt)
  const setMascotArt = useUI((s) => s.setMascotArt)
  const mascotCompanion = useUI((s) => s.mascotCompanion)
  const setMascotCompanion = useUI((s) => s.setMascotCompanion)

  return (
    <SettingsSection id="mascot" title={t('mascot.title')}>
      <div className="flex items-center gap-4 px-4 py-3">
        <MascotArt pose="settings" className="h-24 shrink-0 sm:h-28" />
        <p className="text-sm text-pretty text-muted-foreground">{t('mascot.intro')}</p>
      </div>
      <SettingsRow label={t('mascot.art')} description={t('mascot.artHint')} htmlFor="settings-mascot-art">
        <Switch id="settings-mascot-art" checked={mascotArt} onCheckedChange={setMascotArt} />
      </SettingsRow>
      <SettingsRow label={t('mascot.companion')} description={t('mascot.companionHint')} htmlFor="settings-mascot-companion">
        <Switch id="settings-mascot-companion" checked={mascotCompanion} onCheckedChange={setMascotCompanion} />
      </SettingsRow>
    </SettingsSection>
  )
}
