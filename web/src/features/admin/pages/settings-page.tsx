import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Navigate, useParams, useSearchParams } from 'react-router'

import { ErrorState } from '@/components/error-state'
import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { PageLoader } from '@/components/spinner'
import { useIsMobile } from '@/hooks/use-media-query'
import { ADMIN_SECTION } from '@/layouts/nav'
import { SectionTabs } from '@/layouts/section-tabs'

import { ScrobblingSettings } from '../components/scrobbling-settings'
import { SettingsForm } from '../components/settings-form'
import { SourcesSettings } from '../components/sources-settings'
import { SystemInfo } from '../components/system-info'
import { YtdlpSettings } from '../components/ytdlp-settings'
import { settingsQuery } from '../queries'

type Tab = 'general' | 'ytdlp' | 'sources' | 'scrobbling' | 'system'
const TABS: readonly Tab[] = ['general', 'ytdlp', 'sources', 'scrobbling', 'system']

function isTab(value: string | null | undefined): value is Tab {
  return TABS.includes(value as Tab)
}

/**
 * Admin → server settings. Each part is an Admin tab of its own: `/admin/settings` (general),
 * `/admin/settings/ytdlp`, `/sources`, `/scrobbling` and `/system`. Old `?tab=` links redirect.
 */
export default function ServerSettingsPage() {
  const { t } = useTranslation('admin')
  const isMobile = useIsMobile()
  const { tab: param } = useParams()
  const [searchParams] = useSearchParams()
  const legacy = searchParams.get('tab')
  // Background refetches (library events invalidate `admin` queries) must not reset edits:
  // the form copies the first loaded value and only resets itself after saving.
  const settings = useQuery(settingsQuery)

  if (param === undefined && isTab(legacy) && legacy !== 'general') return <Navigate to={`/admin/settings/${legacy}`} replace />
  if (param !== undefined && (!isTab(param) || param === 'general')) return <Navigate to="/admin/settings" replace />
  const tab: Tab = param ?? 'general'

  return (
    <Page>
      <PageHeader
        title={tab === 'general' ? t('settings.title') : t(`common:nav.${tab}`)}
        subtitle={t('settings.subtitle')}
        back={isMobile ? '/manage' : undefined}
      >
        <SectionTabs section={ADMIN_SECTION} />
      </PageHeader>

      <div className="mx-auto max-w-3xl">
        {tab === 'general' ? (
          settings.data ? (
            <SettingsForm initial={settings.data} />
          ) : settings.isError ? (
            <ErrorState error={settings.error} onRetry={() => void settings.refetch()} retrying={settings.isFetching} />
          ) : (
            <PageLoader />
          )
        ) : tab === 'ytdlp' ? (
          <YtdlpSettings />
        ) : tab === 'sources' ? (
          <SourcesSettings />
        ) : tab === 'scrobbling' ? (
          <ScrobblingSettings />
        ) : (
          <SystemInfo />
        )}
      </div>
    </Page>
  )
}
