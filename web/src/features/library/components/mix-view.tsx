import type { UseQueryResult } from '@tanstack/react-query'
import { Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { usePlayer } from '@/features/player/store'
import { useIsMobile } from '@/hooks/use-media-query'
import type { Track } from '@/lib/api/types'

import { PlayShuffleButtons } from './play-shuffle-buttons'
import { TrackList, TrackListSkeleton } from './track-list'

export interface MixViewProps {
  query: UseQueryResult<{ tracks: Track[] }>
  title: string
  /** The mix's day or week, shown before the song count. */
  period?: string
  description: string
  emptyTitle: string
  emptyDescription: string
}

/** Page of a stored mix (§9.5a): period · song count, play / shuffle, description, track list. */
export function MixView({ query, title, period, description, emptyTitle, emptyDescription }: MixViewProps) {
  const { t } = useTranslation('library')
  const isMobile = useIsMobile()
  const tracks = query.data?.tracks ?? []

  return (
    <Page>
      <PageHeader
        title={title}
        back={isMobile}
        subtitle={query.data && period ? `${period} · ${t('common:count.songs', { count: tracks.length })}` : undefined}
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
          <p className="mb-4 text-sm text-muted-foreground">{description}</p>
          <TrackList
            tracks={tracks}
            showAlbum
            empty={<EmptyState icon={Sparkles} art="empty" title={emptyTitle} description={emptyDescription} />}
          />
        </>
      )}
    </Page>
  )
}
