import { Download, ListEnd, ListPlus, ListStart, MicVocal, Shuffle, Star, StarOff, Tags } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useLocation, useNavigate } from 'react-router'
import { toast } from 'sonner'

import { useAuth } from '@/hooks/use-auth'
import { api } from '@/lib/api/endpoints'
import type { Album, Artist } from '@/lib/api/types'
import { errorMessage } from '@/lib/errors'
import { useUI } from '@/stores/ui'

import type { ActionGroups, ActionItem } from './actions'
import { useCollectionTracks, usePlayCollection, type CollectionSource } from './play'
import { useToggleStar } from './star'

/** Shared queue entries for any collection (album, artist, playlist, genre). */
function useQueueItems(source: CollectionSource): ActionItem[] {
  const { t } = useTranslation()
  const play = usePlayCollection()
  return [
    { key: 'shuffle', label: t('common:actions.shuffle'), icon: Shuffle, onSelect: () => void play(source, 'shuffle') },
    { key: 'playNext', label: t('common:actions.playNext'), icon: ListStart, onSelect: () => void play(source, 'next') },
    { key: 'addToQueue', label: t('common:actions.addToQueue'), icon: ListEnd, onSelect: () => void play(source, 'queue') },
  ]
}

/** Load the collection's tracks, then run `fn` with their ids (errors → toast). */
function useWithTrackIds() {
  const { t } = useTranslation()
  const load = useCollectionTracks()
  return (source: CollectionSource, fn: (ids: string[]) => void) => {
    load(source, false)
      .then((tracks) => {
        if (tracks.length === 0) toast(t('library:toast.nothingToPlay'))
        else fn(tracks.map((track) => track.id))
      })
      .catch((error: unknown) => toast.error(errorMessage(error, t)))
  }
}

export function useAlbumActionGroups(album: Album): ActionGroups {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { pathname } = useLocation()
  const { user, isManager } = useAuth()
  const toggleStar = useToggleStar()
  const withIds = useWithTrackIds()
  const source: CollectionSource = { kind: 'album', id: album.id }
  const queue = useQueueItems(source)

  return [
    queue,
    [
      {
        key: 'addToPlaylist',
        label: t('common:actions.addToPlaylist'),
        icon: ListPlus,
        deferred: true,
        onSelect: () => withIds(source, (ids) => useUI.getState().openAddToPlaylist(ids)),
      },
    ],
    [
      ...(album.artistId && pathname !== `/artists/${album.artistId}`
        ? [
            {
              key: 'goToArtist',
              label: t('common:actions.goToArtist'),
              icon: MicVocal,
              onSelect: () => navigate(`/artists/${album.artistId}`),
            },
          ]
        : []),
      {
        key: 'star',
        label: album.starred ? t('common:actions.unfavorite') : t('common:actions.favorite'),
        icon: album.starred ? StarOff : Star,
        onSelect: () => void toggleStar('album', [album], !album.starred),
      },
      ...(user?.canDownload
        ? [
            {
              key: 'download',
              label: t('library:actions.downloadAlbum'),
              icon: Download,
              href: api.albumDownloadUrl(album.id),
              download: true,
            },
          ]
        : []),
    ],
    isManager
      ? [
          {
            key: 'editTags',
            label: t('common:actions.editTags'),
            icon: Tags,
            deferred: true,
            onSelect: () => withIds(source, (ids) => useUI.getState().openTagEditor(ids)),
          },
        ]
      : [],
  ]
}

export function useArtistActionGroups(artist: Artist): ActionGroups {
  const { t } = useTranslation()
  const toggleStar = useToggleStar()
  const withIds = useWithTrackIds()
  const source: CollectionSource = { kind: 'artist', id: artist.id }
  const queue = useQueueItems(source)
  return [
    queue,
    [
      {
        key: 'addToPlaylist',
        label: t('common:actions.addToPlaylist'),
        icon: ListPlus,
        deferred: true,
        onSelect: () => withIds(source, (ids) => useUI.getState().openAddToPlaylist(ids)),
      },
      {
        key: 'star',
        label: artist.starred ? t('common:actions.unfavorite') : t('common:actions.favorite'),
        icon: artist.starred ? StarOff : Star,
        onSelect: () => void toggleStar('artist', [artist], !artist.starred),
      },
    ],
  ]
}

/** Queue entries + add-to-playlist for playlists and genres (page-specific entries are appended by callers). */
export function useCollectionQueueGroups(source: CollectionSource): ActionGroups {
  const { t } = useTranslation()
  const withIds = useWithTrackIds()
  const queue = useQueueItems(source)
  return [
    queue,
    [
      {
        key: 'addToPlaylist',
        label: t('common:actions.addToPlaylist'),
        icon: ListPlus,
        deferred: true,
        onSelect: () => withIds(source, (ids) => useUI.getState().openAddToPlaylist(ids)),
      },
    ],
  ]
}
