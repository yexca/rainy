/** Program show helpers shared by the album page and the Programs tab of the Radio page. */
import { useQuery } from '@tanstack/react-query'
import type { TFunction } from 'i18next'
import { useMemo } from 'react'

import type { ProgramShow, Track } from '@/lib/api/types'

import { bookmarksQuery } from './queries'

/** The listener's saved positions (ms) in these episodes, by track id. */
export function useEpisodePositions(tracks: readonly Track[]): ReadonlyMap<string, number> {
  const ids = useMemo(() => tracks.map((t) => t.id), [tracks])
  const query = useQuery(bookmarksQuery(ids))
  return useMemo(() => new Map((query.data ?? []).map((b) => [b.trackId, b.positionMs])), [query.data])
}

/** Second line of a show card: its episodes and how far the listener got. */
export function programCaption(show: ProgramShow, t: TFunction<'library'>): string {
  const parts = [t('programs.episodes', { count: show.episodes })]
  if (show.inProgress > 0) parts.push(t('programs.inProgress', { count: show.inProgress }))
  else if (show.episodes > 0 && show.played >= show.episodes) parts.push(t('programs.allPlayed'))
  return parts.join(' · ')
}
