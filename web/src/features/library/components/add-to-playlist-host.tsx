import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, ListMusic, Loader2, Plus, Search } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { CoverArt } from '@/components/cover-art'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Skeleton } from '@/components/ui/skeleton'
import { useCurrentUser } from '@/hooks/use-auth'
import { api } from '@/lib/api/endpoints'
import type { Playlist } from '@/lib/api/types'
import { errorMessage } from '@/lib/errors'
import { queryKeys } from '@/lib/query-keys'
import { cn } from '@/lib/utils'
import { useUI } from '@/stores/ui'

import { playlistsQuery } from '../lib/queries'
import { PlaylistFormDialog } from './playlist-form-dialog'
import { ResponsiveDialog } from './responsive-dialog'

/** Show a filter field once the user has this many playlists. */
const FILTER_THRESHOLD = 8

/**
 * Mounted once by the app shell; opened with `useUI.getState().openAddToPlaylist(trackIds)`.
 * Lists the user's own playlists (most recently changed first) and can create a new one with
 * the tracks directly.
 */
export function AddToPlaylistHost() {
  const { t } = useTranslation()
  const { open, trackIds } = useUI((s) => s.addToPlaylist)
  const close = useUI((s) => s.closeAddToPlaylist)
  // Snapshot of the tracks for "New playlist" (the picker may be reopened for others meanwhile).
  const [creating, setCreating] = useState(false)
  const [createIds, setCreateIds] = useState<string[]>([])

  return (
    <>
      <ResponsiveDialog
        open={open}
        onOpenChange={(next) => (next ? undefined : close())}
        title={t('common:addToPlaylist.title')}
        description={t('common:addToPlaylist.description', { count: trackIds.length })}
      >
        <PlaylistPicker
          trackIds={trackIds}
          onDone={close}
          onCreate={() => {
            const ids = [...trackIds]
            close()
            // Let the picker's exit animation finish before the form opens.
            window.setTimeout(() => {
              setCreateIds(ids)
              setCreating(true)
            }, 200)
          }}
        />
      </ResponsiveDialog>
      <PlaylistFormDialog open={creating} onOpenChange={setCreating} trackIds={createIds} />
    </>
  )
}

function PlaylistPicker({
  trackIds,
  onDone,
  onCreate,
}: {
  trackIds: string[]
  onDone: () => void
  onCreate: () => void
}) {
  const { t } = useTranslation()
  const user = useCurrentUser()
  const queryClient = useQueryClient()
  const [filter, setFilter] = useState('')
  const playlists = useQuery(playlistsQuery())

  const own = useMemo(
    () =>
      (playlists.data ?? [])
        .filter((p) => p.ownerId === user?.id)
        .sort((a, b) => b.updatedAt - a.updatedAt),
    [playlists.data, user?.id],
  )
  const needle = filter.trim().toLowerCase()
  const shown = needle ? own.filter((p) => p.name.toLowerCase().includes(needle)) : own

  const add = useMutation({
    mutationFn: (playlist: Playlist) => api.playlists.addTracks(playlist.id, trackIds),
    onSuccess: (_, playlist) => {
      toast.success(t('common:toast.addedToPlaylist', { name: playlist.name }))
      void queryClient.invalidateQueries({ queryKey: queryKeys.playlists })
      onDone()
    },
    onError: (error) => toast.error(errorMessage(error, t)),
  })

  const row =
    'flex w-full items-center gap-3 rounded-xl px-2 py-2 text-left transition-colors outline-none hover:bg-accent focus-visible:bg-accent active:bg-accent disabled:opacity-60'

  return (
    <div className="-mx-2 flex flex-col gap-1">
      {own.length >= FILTER_THRESHOLD ? (
        <label className="relative mx-2 mb-2 block">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" aria-hidden />
          <span className="sr-only">{t('library:playlist.filter')}</span>
          <input
            type="search"
            value={filter}
            onChange={(event) => setFilter(event.target.value)}
            placeholder={t('library:playlist.filter')}
            className="h-10 w-full rounded-xl bg-muted pr-3 pl-9 text-base outline-none placeholder:text-muted-foreground focus-visible:ring-2 focus-visible:ring-ring/50 md:h-9 md:text-sm"
          />
        </label>
      ) : null}

      <button type="button" className={row} onClick={onCreate}>
        <span className="grid size-11 shrink-0 place-items-center rounded-md bg-primary/12 text-primary">
          <Plus className="size-5" strokeWidth={2} aria-hidden />
        </span>
        <span className="text-[15px] font-medium text-primary md:text-sm">{t('common:nav.newPlaylist')}</span>
      </button>

      {playlists.isPending ? (
        Array.from({ length: 3 }, (_, i) => (
          <div key={i} className="flex items-center gap-3 px-2 py-2">
            <Skeleton className="size-11 rounded-md" />
            <div className="flex flex-1 flex-col gap-1.5">
              <Skeleton className="h-3.5 w-1/2" />
              <Skeleton className="h-3 w-1/4" />
            </div>
          </div>
        ))
      ) : playlists.isError ? (
        <ErrorState size="compact" error={playlists.error} onRetry={() => void playlists.refetch()} />
      ) : own.length === 0 ? (
        <EmptyState size="compact" icon={ListMusic} title={t('common:addToPlaylist.empty')} />
      ) : shown.length === 0 ? (
        <p className="px-2 py-6 text-center text-sm text-muted-foreground">{t('library:search.noMatches')}</p>
      ) : (
        <ul className="max-h-[min(50vh,420px)] overflow-y-auto overscroll-contain">
          {shown.map((playlist) => {
            const pending = add.isPending && add.variables?.id === playlist.id
            const done = add.isSuccess && add.variables?.id === playlist.id
            return (
              <li key={playlist.id}>
                <button
                  type="button"
                  disabled={add.isPending}
                  onClick={() => add.mutate(playlist)}
                  className={row}
                >
                  <CoverArt
                    coverArt={playlist.songCount > 0 ? playlist.coverArt : ''}
                    size={44}
                    icon={ListMusic}
                  />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-[15px] font-medium md:text-sm">{playlist.name}</span>
                    <span className="block text-[13px] text-muted-foreground md:text-xs">
                      {t('common:count.songs', { count: playlist.songCount })}
                    </span>
                  </span>
                  <span className={cn('grid size-5 place-items-center', !pending && !done && 'invisible')}>
                    {pending ? (
                      <Loader2 className="size-4 animate-spin text-muted-foreground" aria-hidden />
                    ) : (
                      <Check className="size-4 text-primary" aria-hidden />
                    )}
                  </span>
                </button>
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}
