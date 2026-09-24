import type { ReactElement } from 'react'
import { useTranslation } from 'react-i18next'

import { CoverArt } from '@/components/cover-art'
import type { Track } from '@/lib/api/types'

import { useTrackActionGroups, type TrackActionsContext } from '../lib/track-action-items'
import { ActionList, ActionMenu, ActionSheetHeader } from './action-menu'

export type { TrackActionsContext } from '../lib/track-action-items'

export interface TrackActionsMenuProps {
  tracks: Track[]
  context?: TrackActionsContext
  className?: string
  align?: 'start' | 'center' | 'end'
  /** Custom trigger element (rendered `asChild`). */
  trigger?: ReactElement
}

/** The action groups for `tracks`, rendered for the surrounding menu (dropdown / context / sheet). */
export function TrackActionItems({ tracks, context }: { tracks: readonly Track[]; context?: TrackActionsContext }) {
  const groups = useTrackActionGroups(tracks, context)
  return <ActionList groups={groups} />
}

/**
 * "…" button with the per-track actions (docs/architecture/contract.md §9.2): DropdownMenu on desktop, action
 * sheet on phones. Play next, Add to queue, Add to playlist, Remove from playlist (playlist
 * context), Go to album / artist, Favorite, Download, Edit tags (managers).
 */
export function TrackActionsMenu({ tracks, context, className, align = 'end', trigger }: TrackActionsMenuProps) {
  const { t } = useTranslation()
  if (tracks.length === 0) return null
  const first = tracks[0]
  const single = tracks.length === 1

  return (
    <ActionMenu
      label={t('common:actions.more')}
      className={className}
      align={align}
      trigger={trigger}
      sheetHeader={
        <ActionSheetHeader
          art={<CoverArt coverArt={first.coverArt} size={48} />}
          title={single ? first.title : t('common:count.songs', { count: tracks.length })}
          subtitle={single ? [first.artist, first.album].filter(Boolean).join(' · ') : undefined}
        />
      }
    >
      <TrackActionItems tracks={tracks} context={context} />
    </ActionMenu>
  )
}
