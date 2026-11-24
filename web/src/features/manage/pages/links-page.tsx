import { useTranslation } from 'react-i18next'

import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'

import { AddBar } from '../components/add-bar'
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
    <Page centered>
      <PageHeader title={t('title')}>
        <TracksTabs />
      </PageHeader>

      <div className="grid gap-6">
        <AddBar review />
        <LinkDownload libraryId={target.libraryId} dir={target.dir} invalidDir={target.invalidDir} review={target.review} />
      </div>
    </Page>
  )
}
