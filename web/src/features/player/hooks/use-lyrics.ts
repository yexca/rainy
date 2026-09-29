import { useQuery } from '@tanstack/react-query'
import { useMemo } from 'react'

import { isApiError } from '@/lib/api/client'
import { api } from '@/lib/api/endpoints'
import type { Lyrics } from '@/lib/api/types'
import { groupBilingual } from '@/lib/lyrics/bilingual'
import { queryKeys } from '@/lib/query-keys'

import type { PlayableTrack } from '../types'

const NO_LYRICS: Lyrics = { synced: false, lines: [], source: 'none', raw: '', offset: 0, lang: '' }

/** Lyrics of a track (`/api/lyrics/{id}`); missing / unsupported lyrics resolve to "none". */
export function useLyrics(track: PlayableTrack | undefined) {
  const id = track && !track.isRadio ? track.id : ''
  return useQuery({
    queryKey: queryKeys.lyrics(id),
    queryFn: async ({ signal }) => {
      try {
        const lyrics = await api.lyrics(id, { signal })
        return { ...NO_LYRICS, ...lyrics, lines: lyrics.lines ?? [] }
      } catch (error) {
        if (isApiError(error) && (error.status === 404 || error.status === 501)) return NO_LYRICS
        throw error
      }
    },
    enabled: id !== '',
    staleTime: 10 * 60_000,
  })
}

/** Whether the track's lyrics have translations to show (bilingual lyrics, lib/lyrics/bilingual). */
export function useHasLyricTranslations(track: PlayableTrack | undefined): boolean {
  const { data } = useLyrics(track)
  return useMemo(
    () => !!data && groupBilingual(data.lines, data.synced).some((line) => line.translations.length > 0),
    [data],
  )
}

/** Index of the line active at `ms` (last line whose start ≤ ms), -1 before the first. */
export function activeLineIndex(lines: readonly { start: number }[], ms: number): number {
  let lo = 0
  let hi = lines.length - 1
  let found = -1
  while (lo <= hi) {
    const mid = (lo + hi) >> 1
    if (lines[mid].start <= ms) {
      found = mid
      lo = mid + 1
    } else {
      hi = mid - 1
    }
  }
  return found
}
