import type { ReactNode } from 'react'

export interface TrackSection {
  key: string
  label: ReactNode
}

type LoadedRow =
  | { type: 'header'; key: string; label: ReactNode }
  | { type: 'track'; key: string; index: number }

export type TrackRow = LoadedRow | { type: 'skeleton'; index: number }

/** Keep only loaded rows in memory; unloaded tracks have implicit positions and keys. */
export function createTrackRows<T extends { id: string }>(
  tracks: readonly T[],
  total: number,
  sectionOf?: (track: T, index: number) => TrackSection | null,
) {
  const loaded: LoadedRow[] = []
  let lastSection: string | null = null
  tracks.forEach((track, index) => {
    const section = sectionOf?.(track, index)
    if (section && section.key !== lastSection) {
      loaded.push({ type: 'header', key: `h:${section.key}:${index}`, label: section.label })
      lastSection = section.key
    }
    loaded.push({ type: 'track', key: `t:${index}:${track.id}`, index })
  })
  const count = loaded.length + Math.max(0, total - tracks.length)
  const trackIndex = (index: number) => tracks.length + index - loaded.length

  return {
    loaded,
    count,
    getRow(index: number): TrackRow | undefined {
      if (index < 0 || index >= count) return undefined
      return loaded[index] ?? { type: 'skeleton', index: trackIndex(index) }
    },
    // Pass this function directly to the virtualizer: recreating it during scroll invalidates
    // measurements for every track, even when only a few rows are visible.
    getItemKey(index: number): string | number {
      if (index < 0 || index >= count) return index
      return loaded[index]?.key ?? `s:${trackIndex(index)}`
    },
  }
}
