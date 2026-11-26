import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Navigate, useParams, useSearchParams } from 'react-router'

import { ErrorState } from '@/components/error-state'
import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { PageLoader } from '@/components/spinner'
import { useIsMobile } from '@/hooks/use-media-query'
import { ADMIN_SECTION, groupedPages } from '@/layouts/nav'
import { SectionTabs } from '@/layouts/section-tabs'
import { SubTabs } from '@/layouts/sub-tabs'
import { settingsQuery } from '@/lib/admin-queries'

import { ScrobblingSettings } from '../components/scrobbling-settings'
import { SettingsForm } from '../components/settings-form'
import { SourcesSettings } from '../components/sources-settings'
import { SystemInfo } from '../components/system-info'
import { YtdlpSettings } from '../components/ytdlp-settings'

type Tab = 'general' | 'ytdlp' | 'sources' | 'scrobbling' | 'system'
const TABS: readonly Tab[] = ['general', 'ytdlp', 'sources', 'scrobbling', 'system']

/** General, yt-dlp, Sources, Scrobbling, System: the pages grouped under the Admin → Settings tab. */
const PAGES = groupedPages(ADMIN_SECTION.tabs, '/admin/settings')

function isTab(value: string | null | undefined): value is Tab {
  return TABS.includes(value as Tab)
}

/**
 * Admin → Settings. Each part is a page of its own, switched at the top: `/admin/settings`
 * (general), `/admin/settings/ytdlp`, `/sources`, `/scrobbling` and `/system`. Old `?tab=` links redirect.
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
    <Page centered>
      <PageHeader
        title={t('settings.title')}
        back={isMobile ? '/manage' : undefined}
      >
        <SectionTabs section={ADMIN_SECTION} />
      </PageHeader>

      <div className="grid gap-8">
        <SubTabs items={PAGES} label={t('common:nav.serverSettings')} className="justify-self-start" />
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
