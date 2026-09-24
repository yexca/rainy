/**
 * Play whole collections (album, artist, playlist, genre, library shuffle) from anywhere — cards,
 * shelves and menus — without first opening their pages.
 */
import { useQueryClient } from '@tanstack/react-query'
import { useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { usePlayer } from '@/features/player/store'
import { api } from '@/lib/api/endpoints'
import type { Track } from '@/lib/api/types'
import { errorMessage } from '@/lib/errors'

import { albumQuery, artistQuery, playlistQuery } from './queries'

/** Max tracks queued by "play all" style actions. */
export const MAX_QUEUE = 1000
/** Size of server-side random picks ("Shuffle all", genre shuffle). */
export const SHUFFLE_SIZE = 300

export type CollectionSource =
  | { kind: 'album'; id: string }
  | { kind: 'artist'; id: string }
  | { kind: 'playlist'; id: string }
  | { kind: 'genre'; name: string }
  | { kind: 'starred' }
  | { kind: 'library' }

export type QueueMode = 'play' | 'shuffle' | 'next' | 'queue'

/** Hook returning a loader for the tracks of a collection (cached through React Query). */
export function useCollectionTracks() {
  const queryClient = useQueryClient()
  return useCallback(
    async (source: CollectionSource, shuffle: boolean): Promise<Track[]> => {
      switch (source.kind) {
        case 'album':
          return (await queryClient.fetchQuery(albumQuery(source.id))).tracks
        case 'playlist':
          return (await queryClient.fetchQuery(playlistQuery(source.id))).tracks
        case 'artist': {
          const page = await api.tracks.list({ artistId: source.id, sort: 'album', limit: MAX_QUEUE })
          if (page.items.length > 0) return page.items
          // Artists whose tracks only credit them as album artist are covered by the query too;
          // fall back to the top tracks of the detail endpoint just in case.
          return (await queryClient.fetchQuery(artistQuery(source.id))).topTracks
        }
        case 'genre':
          if (shuffle) return api.random({ size: SHUFFLE_SIZE, genre: source.name })
          return (await api.tracks.list({ genre: source.name, sort: 'album', limit: MAX_QUEUE })).items
        case 'starred':
          return (await api.tracks.list({ starred: true, sort: 'starred', order: 'desc', limit: MAX_QUEUE })).items
        case 'library':
          return api.random({ size: SHUFFLE_SIZE })
      }
    },
    [queryClient],
  )
}

/**
 * `play(source, mode)` loads the collection's tracks and plays / shuffles / queues them, with
 * toasts for queue actions and errors.
 */
export function usePlayCollection() {
  const { t } = useTranslation()
  const load = useCollectionTracks()

  return useCallback(
    async (source: CollectionSource, mode: QueueMode = 'play') => {
      try {
        const tracks = await load(source, mode === 'shuffle')
        if (tracks.length === 0) {
          toast(t('library:toast.nothingToPlay'))
          return
        }
        const player = usePlayer.getState()
        switch (mode) {
          case 'play':
            player.playTracks(tracks, 0, { shuffle: false })
            break
          case 'shuffle':
            player.playTracks(tracks, undefined, { shuffle: true })
            break
          case 'next':
            player.playNext(tracks)
            toast(t('common:toast.playingNext', { count: tracks.length }))
            break
          case 'queue':
            player.addToQueue(tracks)
            toast(t('common:toast.addedToQueue', { count: tracks.length }))
            break
        }
      } catch (error) {
        toast.error(errorMessage(error, t))
      }
    },
    [load, t],
  )
}
