import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { useSearchParams } from 'react-router'

import { ErrorState } from '@/components/error-state'
import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { PageLoader } from '@/components/spinner'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useIsMobile } from '@/hooks/use-media-query'

import { SettingsForm } from '../components/settings-form'
import { SourcesSettings } from '../components/sources-settings'
import { SystemInfo } from '../components/system-info'
import { YtdlpSettings } from '../components/ytdlp-settings'
import { settingsQuery } from '../queries'

type Tab = 'general' | 'ytdlp' | 'sources' | 'system'
const TABS: readonly Tab[] = ['general', 'ytdlp', 'sources', 'system']

export default function ServerSettingsPage() {
  const { t } = useTranslation('admin')
  const isMobile = useIsMobile()
  const [searchParams, setSearchParams] = useSearchParams()
  const requested = searchParams.get('tab') as Tab | null
  const tab: Tab = requested && TABS.includes(requested) ? requested : 'general'
  // Background refetches (library events invalidate `admin` queries) must not reset edits:
  // the form copies the first loaded value and only resets itself after saving.
  const settings = useQuery(settingsQuery)

  return (
    <Page>
      <PageHeader title={t('settings.title')} subtitle={t('settings.subtitle')} back={isMobile ? '/manage' : undefined}>
        <Tabs value={tab} onValueChange={(v) => setSearchParams(v === 'general' ? {} : { tab: v }, { replace: true })} className="pb-6 sm:items-start">
          <TabsList className="w-full sm:w-auto">
            {TABS.map((id) => (
              <TabsTrigger key={id} value={id} className="sm:px-6">
                {t(`settings.tabs.${id}`)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
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
        ) : (
          <SystemInfo />
        )}
      </div>
    </Page>
  )
}
