/**
 * Favorites (stars) with optimistic updates across every cached library query.
 *
 *   const toggleStar = useToggleStar()
 *   toggleStar('album', [album], !album.starred)
 */
import { useQueryClient, type QueryClient, type QueryKey } from '@tanstack/react-query'
import { useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { usePlayer } from '@/features/player/store'
import { api } from '@/lib/api/endpoints'
import type { StarType } from '@/lib/api/types'
import { errorMessage } from '@/lib/errors'
import { queryKeys } from '@/lib/query-keys'

/** Query roots whose data may contain tracks / albums / artists with a `starred` flag. */
const STAR_ROOTS: ReadonlySet<string> = new Set([
  'home',
  'albums',
  'artists',
  'tracks',
  'search',
  'starred',
  'recent-tracks',
  'playlists',
  'random',
])

type Json = Record<string, unknown>

function isKind(obj: Json, type: StarType): boolean {
  switch (type) {
    case 'track':
      return 'albumId' in obj && 'title' in obj
    case 'album':
      return 'songCount' in obj && 'discCount' in obj
    case 'artist':
      return 'albumCount' in obj && 'indexKey' in obj
  }
}

/**
 * Return `data` with `starred`/`starredAt` set on every `type` object whose id is in `ids`.
 * Unchanged branches keep their identity (structural sharing), so untouched components don't
 * re-render.
 */
export function patchStarred(data: unknown, type: StarType, ids: ReadonlySet<string>, starred: boolean): unknown {
  const starredAt = starred ? Date.now() : null
  const visit = (value: unknown): unknown => {
    if (Array.isArray(value)) {
      let changed = false
      const out = value.map((item) => {
        const next = visit(item)
        if (next !== item) changed = true
        return next
      })
      return changed ? out : value
    }
    if (value === null || typeof value !== 'object') return value
    const obj = value as Json
    let result: Json = obj
    if (typeof obj.id === 'string' && ids.has(obj.id) && 'starred' in obj && obj.starred !== starred && isKind(obj, type)) {
      result = { ...obj, starred, starredAt }
    }
    for (const [key, child] of Object.entries(obj)) {
      if (child === null || typeof child !== 'object') continue
      const next = visit(child)
      if (next === child) continue
      if (result === obj) result = { ...obj }
      result[key] = next
    }
    return result
  }
  return visit(data)
}

interface StarredItem {
  id: string
  starred?: boolean
}

/** Apply the change to every cached query; returns the previous data for rollback. */
function applyOptimistic(
  queryClient: QueryClient,
  type: StarType,
  ids: ReadonlySet<string>,
  starred: boolean,
): [QueryKey, unknown][] {
  const snapshot: [QueryKey, unknown][] = []
  const queries = queryClient
    .getQueryCache()
    .findAll({ predicate: (q) => STAR_ROOTS.has(String(q.queryKey[0])) && q.state.data !== undefined })
  for (const query of queries) {
    const before = query.state.data
    const after = patchStarred(before, type, ids, starred)
    if (after === before) continue
    snapshot.push([query.queryKey, before])
    queryClient.setQueryData(query.queryKey, after)
  }
  return snapshot
}

/**
 * Star / unstar tracks, albums or artists. Updates every cached list immediately, rolls back
 * and shows an error toast if the request fails, and refreshes the favorites afterwards.
 */
export function useToggleStar() {
  const queryClient = useQueryClient()
  const { t } = useTranslation()

  return useCallback(
    async (type: StarType, items: readonly StarredItem[], starred: boolean, opts: { silent?: boolean } = {}) => {
      if (items.length === 0) return
      const ids = new Set(items.map((i) => i.id))
      await queryClient.cancelQueries({ queryKey: queryKeys.starred })
      const snapshot = applyOptimistic(queryClient, type, ids, starred)
      // Queue entries are copies: keep the player's star buttons in sync too.
      const patchQueue = (value: boolean) => {
        if (type !== 'track') return
        const { patchTrack } = usePlayer.getState()
        for (const id of ids) patchTrack(id, { starred: value, starredAt: value ? Date.now() : null })
      }
      patchQueue(starred)
      try {
        await api.star({ type, ids: [...ids], starred })
        if (!opts.silent) toast.success(starred ? t('common:toast.favorited') : t('common:toast.unfavorited'))
        void queryClient.invalidateQueries({ queryKey: queryKeys.starred })
        // The home "Favorites" shelf: refresh on next visit only (home also has a random shelf).
        void queryClient.invalidateQueries({ queryKey: queryKeys.home, refetchType: 'none' })
      } catch (error) {
        for (const [key, data] of snapshot) queryClient.setQueryData(key, data)
        patchQueue(!starred)
        toast.error(errorMessage(error, t))
      }
    },
    [queryClient, t],
  )
}
