import { useTranslation } from 'react-i18next'

import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { PageLoader } from '@/components/spinner'
import { useAuth } from '@/hooks/use-auth'

import { AboutSection, InstallSection, ShortcutsSection } from '../components/about-section'
import { PasswordSection, ProfileSection } from '../components/account-sections'
import { AppearanceSection } from '../components/appearance-section'
import { PlaybackSection } from '../components/playback-section'
import { SubsonicSection } from '../components/subsonic-section'

/** `/settings`: account, Subsonic apps, appearance, playback and app info. */
export default function SettingsPage() {
  const { t } = useTranslation('settings')
  const { user } = useAuth()

  return (
    <Page>
      <PageHeader title={t('title')} subtitle={t('subtitle')} />
      {user ? (
        <div className="grid max-w-2xl gap-8 pb-4">
          <ProfileSection user={user} />
          <AppearanceSection />
          <PlaybackSection />
          <SubsonicSection user={user} />
          <PasswordSection username={user.username} />
          <InstallSection />
          <ShortcutsSection />
          <AboutSection />
        </div>
      ) : (
        <PageLoader />
      )}
    </Page>
  )
}
