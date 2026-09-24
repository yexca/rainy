import type { ReactElement } from 'react'
import { useTranslation } from 'react-i18next'

import { CoverArt } from '@/components/cover-art'
import type { Album, Artist } from '@/lib/api/types'

import { useAlbumActionGroups, useArtistActionGroups } from '../lib/collection-action-items'
import { ActionList, ActionMenu, ActionSheetHeader } from './action-menu'

export function AlbumActionItems({ album }: { album: Album }) {
  return <ActionList groups={useAlbumActionGroups(album)} />
}

interface MenuProps {
  className?: string
  align?: 'start' | 'center' | 'end'
  trigger?: ReactElement
  onOpenChange?: (open: boolean) => void
  /** Sheet on phones (default) or always a dropdown. */
  responsive?: boolean
}

/** "…" menu for an album (cards, album header). */
export function AlbumActionsMenu({ album, ...props }: MenuProps & { album: Album }) {
  const { t } = useTranslation()
  return (
    <ActionMenu
      label={t('common:actions.more')}
      sheetHeader={
        <ActionSheetHeader
          art={<CoverArt coverArt={album.coverArt} size={48} />}
          title={album.name}
          subtitle={album.artist}
        />
      }
      {...props}
    >
      <AlbumActionItems album={album} />
    </ActionMenu>
  )
}

function ArtistActionItems({ artist }: { artist: Artist }) {
  return <ActionList groups={useArtistActionGroups(artist)} />
}

/** "…" menu for an artist. */
export function ArtistActionsMenu({ artist, ...props }: MenuProps & { artist: Artist }) {
  const { t } = useTranslation()
  return (
    <ActionMenu
      label={t('common:actions.more')}
      sheetHeader={
        <ActionSheetHeader
          art={<CoverArt coverArt={artist.coverArt} size={48} shape="circle" />}
          title={artist.name}
          subtitle={t('common:count.albums', { count: artist.albumCount })}
        />
      }
      {...props}
    >
      <ArtistActionItems artist={artist} />
    </ActionMenu>
  )
}
