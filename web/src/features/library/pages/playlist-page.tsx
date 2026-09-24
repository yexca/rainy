import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowUpDown, Globe, ListMusic, Lock, Pencil, Trash2 } from 'lucide-react'
import { useMemo, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, useParams } from 'react-router'
import { toast } from 'sonner'

import { EmptyState } from '@/components/empty-state'
import { Page } from '@/components/page'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { usePlayer } from '@/features/player/store'
import { useIsMobile } from '@/hooks/use-media-query'
import { api } from '@/lib/api/endpoints'
import type { PlaylistDetail, Track } from '@/lib/api/types'
import { errorMessage } from '@/lib/errors'
import { formatDurationLong, formatRelative } from '@/lib/format'
import { queryKeys } from '@/lib/query-keys'

import { ActionList, ActionMenu } from '../components/action-menu'
import { ArtworkBackdrop, CollectionHero, CollectionHeroSkeleton } from '../components/collection-hero'
import { DetailNavBar } from '../components/detail-nav-bar'
import { PlayShuffleButtons } from '../components/play-shuffle-buttons'
import { PlaylistEditor } from '../components/playlist-editor'
import { PlaylistFormDialog } from '../components/playlist-form-dialog'
import { QueryError } from '../components/query-error'
import { TrackList, TrackListSkeleton } from '../components/track-list'
import type { ActionGroups } from '../lib/actions'
import { useCollectionQueueGroups } from '../lib/collection-action-items'
import { applyPlaylistEdit, withTracks, type PlaylistEditPlan } from '../lib/playlist-edits'
import { playlistQuery } from '../lib/queries'

type Dialog = 'details' | 'delete' | null

function PlaylistActionItems({
  playlist,
  editable,
  onDialog,
  onReorder,
}: {
  playlist: PlaylistDetail
  editable: boolean
  onDialog: (dialog: Dialog) => void
  onReorder: () => void
}) {
  const { t } = useTranslation('library')
  const queue = useCollectionQueueGroups({ kind: 'playlist', id: playlist.id })
  const groups: ActionGroups = playlist.tracks.length > 0 ? [...queue] : []
  if (editable) {
    groups.push([
      { key: 'details', label: t('playlist.editDetails'), icon: Pencil, deferred: true, onSelect: () => onDialog('details') },
      ...(playlist.tracks.length > 1
        ? [{ key: 'reorder', label: t('playlist.reorder'), icon: ArrowUpDown, onSelect: onReorder }]
        : []),
    ])
    groups.push([
      {
        key: 'delete',
        label: t('playlist.delete'),
        icon: Trash2,
        destructive: true,
        deferred: true,
        onSelect: () => onDialog('delete'),
      },
    ])
  }
  return <ActionList groups={groups} />
}

export default function PlaylistPage() {
  const { t } = useTranslation('library')
  const { id = '' } = useParams()
  const isMobile = useIsMobile()
  const query = useQuery(playlistQuery(id))
  const playlist = query.data
  const [titleEl, setTitleEl] = useState<HTMLHeadingElement | null>(null)
  const [dialog, setDialog] = useState<Dialog>(null)
  const [editing, setEditing] = useState(false)

  const editable = !!playlist && !playlist.readonly
  const menu = playlist && (editable || playlist.tracks.length > 0) ? (
    <ActionMenu label={t('common:actions.more')} className={isMobile ? '-mr-2.5 text-primary' : 'md:size-9'}>
      <PlaylistActionItems
        playlist={playlist}
        editable={editable}
        onDialog={setDialog}
        onReorder={() => setEditing(true)}
      />
    </ActionMenu>
  ) : null

  return (
    <Page>
      <div className="relative isolate">
        {playlist && playlist.songCount > 0 ? (
          <ArtworkBackdrop key={playlist.coverArt} coverArt={playlist.coverArt} className="bleed-x h-[34rem] md:h-[26rem]" />
        ) : null}
        <DetailNavBar
          title={playlist?.name ?? ''}
          watch={titleEl}
          actions={
            isMobile && playlist ? (
              editing ? null : (
                <>
                  {editable && playlist.tracks.length > 1 ? (
                    <Button
                      variant="ghost"
                      onClick={() => setEditing(true)}
                      className="h-11 px-2 text-[17px] text-primary hover:bg-transparent hover:text-primary"
                    >
                      {t('common:actions.edit')}
                    </Button>
                  ) : null}
                  {menu}
                </>
              )
            ) : undefined
          }
        />
        {query.isPending ? (
          <>
            <CollectionHeroSkeleton />
            <TrackListSkeleton rows={8} />
          </>
        ) : query.isError ? (
          <QueryError
            error={query.error}
            onRetry={() => void query.refetch()}
            retrying={query.isFetching}
            notFound={{
              icon: ListMusic,
              title: t('playlist.notFound'),
              backTo: '/playlists',
              backLabel: t('playlist.backToPlaylists'),
            }}
          />
        ) : (
          <PlaylistContent
            playlist={query.data}
            editable={editable}
            editing={editing}
            onEditingChange={setEditing}
            titleRef={setTitleEl}
            trailing={
              <>
                {editable && !editing && query.data.tracks.length > 1 ? (
                  <Button variant="ghost" onClick={() => setEditing(true)} className="h-9 gap-2">
                    <ArrowUpDown className="size-4" strokeWidth={1.75} aria-hidden />
                    {t('playlist.reorder')}
                  </Button>
                ) : null}
                {menu}
              </>
            }
          />
        )}
      </div>

      {playlist && editable ? (
        <>
          <PlaylistFormDialog
            open={dialog === 'details'}
            onOpenChange={(open) => setDialog(open ? 'details' : null)}
            playlist={playlist}
          />
          <DeletePlaylistDialog
            playlist={playlist}
            open={dialog === 'delete'}
            onOpenChange={(open) => setDialog(open ? 'delete' : null)}
          />
        </>
      ) : null}
    </Page>
  )
}

function PlaylistContent({
  playlist,
  editable,
  editing,
  onEditingChange,
  titleRef,
  trailing,
}: {
  playlist: PlaylistDetail
  editable: boolean
  editing: boolean
  onEditingChange: (editing: boolean) => void
  titleRef: (el: HTMLHeadingElement | null) => void
  trailing: ReactNode
}) {
  const { t } = useTranslation('library')
  const queryClient = useQueryClient()
  const tracks = playlist.tracks

  const save = useMutation({
    mutationFn: ({ plan }: { plan: PlaylistEditPlan; tracks: Track[] }) =>
      applyPlaylistEdit(plan, { replace: (trackIds) => api.playlists.setTracks(playlist.id, trackIds) }),
    onSuccess: (_, { tracks: next }) => {
      // Show the result right away (the refetch below confirms it).
      queryClient.setQueryData<PlaylistDetail>(queryKeys.playlist(playlist.id), (old) =>
        old ? withTracks(old, next) : old,
      )
      toast.success(t('toast.playlistSaved'))
      onEditingChange(false)
    },
    onError: (error) => toast.error(errorMessage(error, t)),
    // Also after a failure: show whatever the server has now.
    onSettled: () => queryClient.invalidateQueries({ queryKey: queryKeys.playlists }),
  })
  const listContext = useMemo(() => ({ id: playlist.id, editable }), [playlist.id, editable])

  const meta = (
    <span className="inline-flex flex-wrap items-center justify-center gap-x-1.5 md:justify-start">
      <span>
        {tracks.length > 0
          ? t('songsAndDuration', {
              songs: t('common:count.songs', { count: tracks.length }),
              duration: formatDurationLong(playlist.duration),
            })
          : t('common:count.songs', { count: 0 })}
      </span>
      <span aria-hidden>·</span>
      <span className="inline-flex items-center gap-1">
        {playlist.public ? (
          <Globe className="size-3.5" strokeWidth={1.75} aria-hidden />
        ) : (
          <Lock className="size-3.5" strokeWidth={1.75} aria-hidden />
        )}
        {playlist.public ? t('playlist.publicBadge') : t('playlist.privateBadge')}
      </span>
      <span aria-hidden className="hidden md:inline">
        ·
      </span>
      <span className="hidden md:inline">{t('playlist.updated', { when: formatRelative(playlist.updatedAt) })}</span>
    </span>
  )

  return (
    <>
      <CollectionHero
        coverArt={tracks.length > 0 ? playlist.coverArt : ''}
        icon={ListMusic}
        kind={t('playlist.kind')}
        title={playlist.name}
        titleRef={titleRef}
        byline={playlist.ownerName ? t('playlist.by', { name: playlist.ownerName }) : undefined}
        meta={meta}
        description={playlist.comment || undefined}
        playButtons={(layout) => (
          <PlayShuffleButtons
            layout={layout}
            disabled={tracks.length === 0 || editing}
            onPlay={() => usePlayer.getState().playTracks(tracks, 0, { shuffle: false })}
            onShuffle={() => usePlayer.getState().playTracks(tracks, undefined, { shuffle: true })}
          />
        )}
        trailing={trailing}
      />

      {editing ? (
        <PlaylistEditor
          tracks={tracks}
          saving={save.isPending}
          onCancel={() => onEditingChange(false)}
          onSave={(plan, next) => save.mutate({ plan, tracks: next })}
        />
      ) : (
        <TrackList
          tracks={tracks}
          playlist={listContext}
          empty={
            <EmptyState
              icon={ListMusic}
              size="compact"
              title={t('playlist.emptyTitle')}
              description={t('playlist.emptyDescription')}
            />
          }
        />
      )}
    </>
  )
}

function DeletePlaylistDialog({
  playlist,
  open,
  onOpenChange,
}: {
  playlist: PlaylistDetail
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation('library')
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const remove = useMutation({
    mutationFn: () => api.playlists.delete(playlist.id),
    onSuccess: () => {
      toast.success(t('toast.playlistDeleted', { name: playlist.name }))
      onOpenChange(false)
      navigate('/playlists', { replace: true })
      queryClient.removeQueries({ queryKey: queryKeys.playlist(playlist.id) })
      void queryClient.invalidateQueries({ queryKey: queryKeys.playlists })
    },
    onError: (error) => toast.error(errorMessage(error, t)),
  })

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent className="rounded-2xl">
        <AlertDialogHeader>
          <AlertDialogTitle>{t('playlist.deleteTitle', { name: playlist.name })}</AlertDialogTitle>
          <AlertDialogDescription>{t('playlist.deleteDescription')}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={remove.isPending}>{t('common:actions.cancel')}</AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            disabled={remove.isPending}
            onClick={(event) => {
              event.preventDefault()
              remove.mutate()
            }}
          >
            {t('common:actions.delete')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
