import { useTranslation } from 'react-i18next'

import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'

import { DestinationSection } from '../components/destination-section'
import { LinkDownload } from '../components/link-download'
import { TracksTabs } from '../components/tracks-tabs'
import { useDestinationTarget } from '../lib/destination'

/**
 * Tracks → Links: download the audio of YouTube and bilibili videos with yt-dlp into the
 * destination shared with Upload and Online. Jobs run on the server.
 */
export default function LinksPage() {
  const { t } = useTranslation('manage')
  const target = useDestinationTarget()

  return (
    <Page>
      <PageHeader title={t('title')} subtitle={t('download.pageSubtitle')}>
        <TracksTabs />
      </PageHeader>

      <div className="mx-auto grid max-w-3xl gap-6">
        <DestinationSection />
        <LinkDownload libraryId={target.libraryId} dir={target.dir} organize={target.organize} invalidDir={target.invalidDir} />
      </div>
    </Page>
  )
}
