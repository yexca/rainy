import { AudioWaveform, Headphones, Palette, Settings, Smartphone, UserRound } from 'lucide-react'
import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Navigate, useLocation, useParams } from 'react-router'

import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { PageLoader } from '@/components/spinner'
import { useAuth } from '@/hooks/use-auth'
import type { NavSection } from '@/layouts/nav'
import { SectionTabs } from '@/layouts/section-tabs'

import { LogoutButton, PasswordSection, ProfileSection } from '../components/account-sections'
import { AppearanceSection } from '../components/appearance-section'
import { InstallSection, ShortcutsSection } from '../components/device-sections'
import { MascotSection } from '../components/mascot-section'
import { PlaybackSection } from '../components/playback-section'
import { ScrobblingSection } from '../components/scrobbling-section'
import { SubsonicSection } from '../components/subsonic-section'

type Tab = 'account' | 'appearance' | 'playback' | 'apps' | 'scrobbling'
const TABS: readonly Tab[] = ['account', 'appearance', 'playback', 'apps', 'scrobbling']

function isTab(value: string | undefined): value is Tab {
  return TABS.includes(value as Tab)
}

/** The section named by the URL hash; a malformed escape names none. */
function hashSection(hash: string): string {
  try {
    return decodeURIComponent(hash.slice(1))
  } catch {
    return ''
  }
}

function tabPath(tab: Tab): string {
  return tab === 'account' ? '/settings' : `/settings/${tab}`
}

const SETTINGS_SECTION: NavSection = {
  labelKey: 'nav.settings',
  icon: Settings,
  tabs: [
    { to: '/settings', labelKey: 'nav.account', icon: UserRound, end: true },
    { to: '/settings/appearance', labelKey: 'nav.appearance', icon: Palette },
    { to: '/settings/playback', labelKey: 'nav.playback', icon: Headphones },
    { to: '/settings/apps', labelKey: 'nav.apps', icon: Smartphone },
    { to: '/settings/scrobbling', labelKey: 'nav.scrobbling', icon: AudioWaveform },
  ],
}

/** The tab of each section, for links from before the tabs (`/settings#scrobbling`). */
const SECTION_TABS = new Map<string, Tab>([
  ['profile', 'account'],
  ['password', 'account'],
  ['appearance', 'appearance'],
  ['mascot', 'appearance'],
  ['playback', 'playback'],
  ['shortcuts', 'playback'],
  ['subsonic', 'apps'],
  ['install', 'apps'],
  ['scrobbling', 'scrobbling'],
])

/**
 * `/settings`: the user's own settings, one route tab per part — `/settings` (account),
 * `/settings/appearance`, `/playback`, `/apps` and `/scrobbling`. App information lives on `/about`.
 */
export default function SettingsPage() {
  const { t } = useTranslation('settings')
  const { user } = useAuth()
  const { tab: param } = useParams()
  const { hash } = useLocation()
  const section = hashSection(hash)

  // A section link such as `/settings/appearance#mascot` opens at that section.
  useEffect(() => {
    if (!user || !section) return
    document.getElementById(section)?.scrollIntoView({ block: 'start' })
  }, [section, param, user])

  if (param === undefined && section === 'about') return <Navigate to="/about" replace />
  const legacy = param === undefined ? SECTION_TABS.get(section) : undefined
  if (legacy && legacy !== 'account') {
    // Dropping the hash when it names the tab itself keeps the page header in view.
    return <Navigate to={{ pathname: tabPath(legacy), hash: section === legacy ? '' : hash }} replace />
  }
  if (param !== undefined && (!isTab(param) || param === 'account')) return <Navigate to="/settings" replace />
  const tab: Tab = param ?? 'account'

  return (
    <Page>
      <PageHeader title={t('title')} subtitle={t('subtitle')}>
        <SectionTabs section={SETTINGS_SECTION} />
      </PageHeader>
      {user ? (
        <div className="grid max-w-2xl gap-8 pb-4">
          {tab === 'account' ? (
            <>
              <ProfileSection user={user} />
              <PasswordSection username={user.username} />
              <LogoutButton />
            </>
          ) : tab === 'appearance' ? (
            <>
              <AppearanceSection />
              <MascotSection />
            </>
          ) : tab === 'playback' ? (
            <>
              <PlaybackSection />
              <ShortcutsSection />
            </>
          ) : tab === 'apps' ? (
            <>
              <SubsonicSection user={user} />
              <InstallSection />
            </>
          ) : (
            <ScrobblingSection />
          )}
        </div>
      ) : (
        <PageLoader />
      )}
    </Page>
  )
}
