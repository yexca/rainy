import { useQueryClient } from '@tanstack/react-query'
import { useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { api } from '@/lib/api/endpoints'
import { errorMessage } from '@/lib/errors'
import { LIBRARY_QUERY_ROOTS } from '@/lib/query-keys'

import { usePlayer } from '../store'
import type { PlayableTrack } from '../types'

/** Star / unstar a track from the player (optimistic in the queue, then refresh library views). */
export function useToggleStar(): (track: PlayableTrack) => Promise<void> {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  return useCallback(
    async (track: PlayableTrack) => {
      if (track.isRadio) return
      const starred = !track.starred
      const previous = { starred: track.starred, starredAt: track.starredAt }
      usePlayer.getState().patchTrack(track.id, { starred, starredAt: starred ? Date.now() : null })
      try {
        await api.star({ type: 'track', ids: [track.id], starred })
        void queryClient.invalidateQueries({ predicate: (q) => LIBRARY_QUERY_ROOTS.has(String(q.queryKey[0])) })
      } catch (error) {
        usePlayer.getState().patchTrack(track.id, previous)
        toast.error(errorMessage(error, t))
      }
    },
    [queryClient, t],
  )
}
