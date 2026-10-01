import { useQuery } from '@tanstack/react-query'
import { Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { usePlayer } from '@/features/player/store'
import { useIsMobile } from '@/hooks/use-media-query'

import { PlayShuffleButtons } from '../components/play-shuffle-buttons'
import { TrackList, TrackListSkeleton } from '../components/track-list'
import { dailyDateLabel } from '../lib/daily'
import { dailyMixQuery } from '../lib/queries'

/** `/daily`: today's daily mix (§9.5a), the same songs all day. */
export default function DailyPage() {
  const { t } = useTranslation('library')
  const isMobile = useIsMobile()
  const query = useQuery(dailyMixQuery())
  const tracks = query.data?.tracks ?? []

  return (
    <Page>
      <PageHeader
        title={t('daily.title')}
        back={isMobile}
        subtitle={query.data ? `${dailyDateLabel(query.data.date)} · ${t('common:count.songs', { count: tracks.length })}` : undefined}
        actions={
          tracks.length > 0 ? (
            <PlayShuffleButtons
              onPlay={() => usePlayer.getState().playTracks(tracks, 0, { shuffle: false })}
              onShuffle={() => usePlayer.getState().playTracks(tracks, undefined, { shuffle: true })}
            />
          ) : undefined
        }
      />
      {query.isPending ? (
        <TrackListSkeleton rows={12} />
      ) : query.isError ? (
        <ErrorState error={query.error} onRetry={() => void query.refetch()} retrying={query.isFetching} />
      ) : (
        <>
          <p className="mb-4 text-sm text-muted-foreground">{t('daily.description')}</p>
          <TrackList
            tracks={tracks}
            showAlbum
            empty={<EmptyState icon={Sparkles} art="empty" title={t('daily.emptyTitle')} description={t('daily.emptyDescription')} />}
          />
        </>
      )}
    </Page>
  )
}
