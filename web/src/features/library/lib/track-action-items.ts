import { useQueryClient } from '@tanstack/react-query'
import { Disc3, Download, ListEnd, ListMinus, ListPlus, ListStart, MicVocal, Star, StarOff, Tags } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useLocation, useNavigate } from 'react-router'
import { toast } from 'sonner'

import { usePlayer } from '@/features/player/store'
import { useAuth } from '@/hooks/use-auth'
import { api } from '@/lib/api/endpoints'
import type { PlaylistDetail, Track } from '@/lib/api/types'
import { errorMessage } from '@/lib/errors'
import { queryKeys } from '@/lib/query-keys'
import { useUI } from '@/stores/ui'

import type { ActionGroups } from './actions'
import { withoutPositions } from './playlist-edits'
import { useToggleStar } from './star'

export interface TrackActionsContext {
  /** Set when the tracks are shown inside a playlist (enables "Remove from playlist"). */
  playlistId?: string
  /** 0-based position of the (single) track in that playlist. */
  position?: number
  /** The playlist is editable by the current user (defaults to true when `playlistId` is set). */
  canEditPlaylist?: boolean
}

/** Menu entries for one or more tracks (docs/architecture/contract.md §9.2). */
export function useTrackActionGroups(tracks: readonly Track[], context?: TrackActionsContext): ActionGroups {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { pathname } = useLocation()
  const queryClient = useQueryClient()
  const { user, isManager } = useAuth()
  const toggleStar = useToggleStar()

  if (tracks.length === 0) return []
  const list = [...tracks]
  const ids = list.map((track) => track.id)
  const first = list[0]
  const single = list.length === 1
  const albumId = list.every((track) => track.albumId === first.albumId) ? first.albumId : ''
  const artistId = list.every((track) => track.artistId === first.artistId) ? first.artistId : ''
  const allStarred = list.every((track) => track.starred)
  const canRemove =
    !!context?.playlistId && context.position !== undefined && single && context.canEditPlaylist !== false

  const removeFromPlaylist = async () => {
    if (!context?.playlistId || context.position === undefined) return
    const key = queryKeys.playlist(context.playlistId)
    const position = context.position
    // Positions are indexes into the visible list: drop the row from the cache right away so a
    // second removal before the refetch lands targets the right row.
    await queryClient.cancelQueries({ queryKey: key, exact: true })
    const previous = queryClient.getQueryData<PlaylistDetail>(key)
    if (previous) queryClient.setQueryData(key, withoutPositions(previous, [position]))
    try {
      await api.playlists.removeTracks(context.playlistId, [position])
      toast.success(t('common:toast.removedFromPlaylist'))
    } catch (error) {
      if (previous) queryClient.setQueryData(key, previous)
      toast.error(errorMessage(error, t))
    } finally {
      void queryClient.invalidateQueries({ queryKey: queryKeys.playlists })
    }
  }

  return [
    [
      {
        key: 'playNext',
        label: t('common:actions.playNext'),
        icon: ListStart,
        onSelect: () => {
          usePlayer.getState().playNext(list)
          toast(t('common:toast.playingNext', { count: list.length }))
        },
      },
      {
        key: 'addToQueue',
        label: t('common:actions.addToQueue'),
        icon: ListEnd,
        onSelect: () => {
          usePlayer.getState().addToQueue(list)
          toast(t('common:toast.addedToQueue', { count: list.length }))
        },
      },
    ],
    [
      {
        key: 'addToPlaylist',
        label: t('common:actions.addToPlaylist'),
        icon: ListPlus,
        deferred: true,
        onSelect: () => useUI.getState().openAddToPlaylist(ids),
      },
      ...(canRemove
        ? [
            {
              key: 'removeFromPlaylist',
              label: t('common:actions.removeFromPlaylist'),
              icon: ListMinus,
              destructive: true,
              onSelect: () => void removeFromPlaylist(),
            },
          ]
        : []),
    ],
    [
      ...(albumId && pathname !== `/albums/${albumId}`
        ? [
            {
              key: 'goToAlbum',
              label: t('common:actions.goToAlbum'),
              icon: Disc3,
              onSelect: () => navigate(`/albums/${albumId}`),
            },
          ]
        : []),
      ...(artistId && pathname !== `/artists/${artistId}`
        ? [
            {
              key: 'goToArtist',
              label: t('common:actions.goToArtist'),
              icon: MicVocal,
              onSelect: () => navigate(`/artists/${artistId}`),
            },
          ]
        : []),
    ],
    [
      {
        key: 'star',
        label: allStarred ? t('common:actions.unfavorite') : t('common:actions.favorite'),
        icon: allStarred ? StarOff : Star,
        onSelect: () => void toggleStar('track', list, !allStarred),
      },
      ...(single && user?.canDownload
        ? [{ key: 'download', label: t('common:actions.download'), icon: Download, href: api.downloadUrl(first.id), download: true }]
        : []),
    ],
    isManager
      ? [
          {
            key: 'editTags',
            label: t('common:actions.editTags'),
            icon: Tags,
            deferred: true,
            onSelect: () => useUI.getState().openTagEditor(ids),
          },
        ]
      : [],
  ]
}
