import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { useLocation } from 'react-router'

import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { PageLoader } from '@/components/spinner'
import { useAuth } from '@/hooks/use-auth'

import { AboutSection, InstallSection, ShortcutsSection } from '../components/about-section'
import { PasswordSection, ProfileSection } from '../components/account-sections'
import { AppearanceSection } from '../components/appearance-section'
import { PlaybackSection } from '../components/playback-section'
import { ScrobblingSection } from '../components/scrobbling-section'
import { SubsonicSection } from '../components/subsonic-section'

/** `/settings`: account, Subsonic apps, scrobbling, appearance, playback and app info. */
export default function SettingsPage() {
  const { t } = useTranslation('settings')
  const { user } = useAuth()
  const { hash } = useLocation()

  // Links such as `/settings#scrobbling` open at their section.
  useEffect(() => {
    if (!user || !hash) return
    document.getElementById(decodeURIComponent(hash.slice(1)))?.scrollIntoView({ block: 'start' })
  }, [hash, user])

  return (
    <Page>
      <PageHeader title={t('title')} subtitle={t('subtitle')} />
      {user ? (
        <div className="grid max-w-2xl gap-8 pb-4">
          <ProfileSection user={user} />
          <AppearanceSection />
          <PlaybackSection />
          <SubsonicSection user={user} />
          <ScrobblingSection />
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
