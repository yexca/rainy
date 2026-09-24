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
import { SystemInfo } from '../components/system-info'
import { settingsQuery } from '../queries'

type Tab = 'general' | 'system'

export default function ServerSettingsPage() {
  const { t } = useTranslation('admin')
  const isMobile = useIsMobile()
  const [searchParams, setSearchParams] = useSearchParams()
  const tab: Tab = searchParams.get('tab') === 'system' ? 'system' : 'general'
  // Background refetches (library events invalidate `admin` queries) must not reset edits:
  // the form copies the first loaded value and only resets itself after saving.
  const settings = useQuery(settingsQuery)

  return (
    <Page>
      <PageHeader title={t('settings.title')} subtitle={t('settings.subtitle')} back={isMobile ? '/manage' : undefined}>
        <Tabs value={tab} onValueChange={(v) => setSearchParams(v === 'system' ? { tab: 'system' } : {}, { replace: true })} className="pb-6 sm:items-start">
          <TabsList className="w-full sm:w-auto">
            <TabsTrigger value="general" className="sm:px-6">
              {t('settings.tabs.general')}
            </TabsTrigger>
            <TabsTrigger value="system" className="sm:px-6">
              {t('settings.tabs.system')}
            </TabsTrigger>
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
        ) : (
          <SystemInfo />
        )}
      </div>
    </Page>
  )
}
