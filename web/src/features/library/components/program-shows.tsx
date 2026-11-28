import type { UseQueryResult } from '@tanstack/react-query'
import { Podcast } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { useAuth } from '@/hooks/use-auth'
import type { ProgramShow } from '@/lib/api/types'

import { AlbumCard, AlbumCardSkeleton } from './album-card'
import { CardGrid } from './card-grid'
import { programCaption } from '../lib/programs'

/**
 * The Programs tab of the Radio page: program shows as cards, most recent activity first. A
 * show opens its album page, which lists the episodes with the listener's progress.
 */
export function ProgramShows({ query }: { query: UseQueryResult<ProgramShow[]> }) {
  const { t } = useTranslation('library')
  const { isManager } = useAuth()
  if (query.isPending) {
    return (
      <CardGrid>
        {Array.from({ length: 8 }, (_, i) => (
          <AlbumCardSkeleton key={i} />
        ))}
      </CardGrid>
    )
  }
  if (query.isError) {
    return <ErrorState error={query.error} onRetry={() => void query.refetch()} retrying={query.isFetching} />
  }
  const shows = query.data
  if (shows.length === 0) {
    return (
      <EmptyState
        icon={Podcast}
        title={t('programs.emptyTitle')}
        description={isManager ? t('programs.emptyManager') : t('programs.emptyListener')}
      />
    )
  }
  return (
    <CardGrid>
      {shows.map((show) => (
        <AlbumCard key={show.id} album={show} caption={programCaption(show, t)} />
      ))}
    </CardGrid>
  )
}
