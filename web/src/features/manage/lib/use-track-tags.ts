/**
 * Loads `TrackTags` for a selection (one request per track, limited concurrency, progress).
 *
 * Deliberately not a React Query: the editor works on a snapshot, so a `library` event from a
 * concurrent scan must not swap the originals under pending edits. `reload()` refreshes the
 * snapshot explicitly (after saving).
 */
import { useCallback, useEffect, useRef, useState } from 'react'

import { isAbortError } from '@/lib/api/client'
import { api } from '@/lib/api/endpoints'
import type { TrackTags } from '@/lib/api/types'

const CONCURRENCY = 6

export interface TrackTagsState {
  status: 'loading' | 'ready' | 'error'
  /** Loaded items in selection order (failed ids skipped). */
  items: TrackTags[]
  /** Loaded so far / total, while loading. */
  loaded: number
  total: number
  /** Ids that failed to load, with the error. */
  failures: { id: string; error: unknown }[]
}

export async function loadTrackTags(
  ids: readonly string[],
  signal: AbortSignal,
  onProgress?: (loaded: number) => void,
): Promise<{ items: TrackTags[]; failures: { id: string; error: unknown }[] }> {
  const results: (TrackTags | undefined)[] = new Array(ids.length)
  const failures: { id: string; error: unknown }[] = []
  let next = 0
  let done = 0
  const worker = async () => {
    while (next < ids.length) {
      const i = next++
      try {
        results[i] = await api.manage.tags.get(ids[i], { signal })
      } catch (error) {
        if (isAbortError(error) || signal.aborted) throw error
        failures.push({ id: ids[i], error })
      }
      done++
      onProgress?.(done)
    }
  }
  await Promise.all(Array.from({ length: Math.min(CONCURRENCY, ids.length) }, worker))
  return { items: results.filter((r): r is TrackTags => r !== undefined), failures }
}

export function useTrackTags(ids: readonly string[]): TrackTagsState & { reload: (subset?: readonly string[]) => Promise<TrackTags[]> } {
  const [state, setState] = useState<TrackTagsState>({
    status: 'loading',
    items: [],
    loaded: 0,
    total: ids.length,
    failures: [],
  })
  const controllerRef = useRef<AbortController | null>(null)
  const idsKey = ids.join(',')

  const run = useCallback(
    async (target: readonly string[], merge: boolean): Promise<TrackTags[]> => {
      controllerRef.current?.abort()
      const controller = new AbortController()
      controllerRef.current = controller
      if (!merge) setState({ status: 'loading', items: [], loaded: 0, total: target.length, failures: [] })
      try {
        const { items, failures } = await loadTrackTags(target, controller.signal, (loaded) => {
          if (!merge) setState((s) => ({ ...s, loaded }))
        })
        if (controller.signal.aborted) return []
        setState((s) => {
          if (!merge) {
            return { status: items.length > 0 || target.length === 0 ? 'ready' : 'error', items, loaded: target.length, total: target.length, failures }
          }
          const fresh = new Map(items.map((i) => [i.track.id, i]))
          return { ...s, items: s.items.map((i) => fresh.get(i.track.id) ?? i) }
        })
        return items
      } catch (error) {
        if (!isAbortError(error)) setState((s) => ({ ...s, status: 'error', failures: [{ id: '', error }] }))
        return []
      }
    },
    [],
  )

  useEffect(() => {
    const target = idsKey ? idsKey.split(',') : []
    void run(target, false)
    return () => controllerRef.current?.abort()
  }, [idsKey, run])

  const reload = useCallback(
    (subset?: readonly string[]) => run(subset ?? (idsKey ? idsKey.split(',') : []), subset !== undefined),
    [idsKey, run],
  )

  return { ...state, reload }
}
