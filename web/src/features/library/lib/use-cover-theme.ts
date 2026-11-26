import { queryOptions, useQuery } from '@tanstack/react-query'
import { useEffect, useMemo } from 'react'
import { create } from 'zustand'

import { useTheme } from '@/hooks/use-theme'
import { coverUrlForPixels } from '@/lib/cover'

import { extractCoverSeeds, type CoverSeeds } from './cover-theme'

/** The page paints its own background and text colour, down to the bottom. */
const PAGE_CLASS = 'grow bg-background text-foreground'
/** In immersive mode the page stays see-through over the app's backdrop. */
const IMMERSIVE_PAGE_CLASS = 'grow text-foreground'

/** Side of the square the cover is sampled at. */
const SAMPLE_SIZE = 48

async function sampleCover(coverArt: string): Promise<CoverSeeds | null> {
  // `load` rather than `decode()`: decode() can stay pending while the page is not rendering
  // (a background tab), which would leave the page uncoloured.
  const image = await new Promise<HTMLImageElement>((resolve, reject) => {
    const img = new Image()
    img.onload = () => resolve(img)
    img.onerror = () => reject(new Error('cover failed to load'))
    img.src = coverUrlForPixels(coverArt, 64)
  })
  const canvas = document.createElement('canvas')
  canvas.width = SAMPLE_SIZE
  canvas.height = SAMPLE_SIZE
  const context = canvas.getContext('2d', { willReadFrequently: true })
  if (!context) return null
  context.drawImage(image, 0, 0, SAMPLE_SIZE, SAMPLE_SIZE)
  return extractCoverSeeds(context.getImageData(0, 0, SAMPLE_SIZE, SAMPLE_SIZE).data, SAMPLE_SIZE)
}

/**
 * The cover's background / accent seeds, sampled once per cover and cached. Album cards prefetch
 * them in immersive mode, so the page they open takes on its colours as soon as it shows.
 */
export function coverSeedsQuery(coverArt: string) {
  return queryOptions({
    queryKey: ['library', 'cover-theme', coverArt],
    queryFn: () => sampleCover(coverArt),
    staleTime: Number.POSITIVE_INFINITY,
    gcTime: 30 * 60_000,
    retry: false,
  })
}

/** The cover's seeds (`coverSeedsQuery`). `keepPrevious` keeps the previous cover's while the next one is sampled. */
export function useCoverSeeds(coverArt: string | undefined, enabled = true, keepPrevious = false): CoverSeeds | null {
  const { data } = useQuery({
    ...coverSeedsQuery(coverArt ?? ''),
    enabled: enabled && !!coverArt,
    placeholderData: keepPrevious ? (previous) => previous : undefined,
  })
  return (enabled && coverArt && data) || null
}

/** The cover of the album / playlist / artist page on screen, for immersive mode. */
export const usePageCover = create<{ coverArt: string | null }>()(() => ({ coverArt: null }))

/** A page let go of its cover: it is cleared unless the next page takes over in the same commit. */
let release: ReturnType<typeof setTimeout> | undefined

/** Keep the page cover on screen, or replace it (`undefined` keeps the current one). */
function keepPageCover(coverArt?: string | null): void {
  clearTimeout(release)
  if (coverArt !== undefined) usePageCover.setState({ coverArt })
}

function releasePageCover(): void {
  clearTimeout(release)
  release = setTimeout(() => usePageCover.setState({ coverArt: null }), 0)
}

/** Immersive mode is on: the whole app follows the cover on screen or playing. */
export function useImmersive(): boolean {
  return useTheme().immersive
}

export interface CoverThemeScope {
  /** Spread on the page element (`<Page {...scope}>`). */
  className: string
}

/**
 * For a page that leads with a cover (album, playlist and artist pages). In immersive mode the
 * page publishes its cover (`usePageCover`), `<ImmersiveTheme/>` colours the whole app with it,
 * and the page paints no background of its own. While the page is `loading`, the colours on
 * screen stay (the previous cover page's, or the playing song's), so going from one cover page
 * to the next changes them once. With the mode off nothing is taken from the cover: the page
 * keeps the user's theme like every other page.
 *
 *   <Page {...useCoverTheme(album?.coverArt, query.isPending)}>
 */
export function useCoverTheme(coverArt: string | undefined, loading = false): CoverThemeScope {
  const immersive = useImmersive()

  useEffect(() => {
    if (!immersive) return
    if (!coverArt && !loading) {
      keepPageCover(null)
      return
    }
    keepPageCover(coverArt)
    return releasePageCover
  }, [immersive, coverArt, loading])

  return useMemo(() => ({ className: immersive ? IMMERSIVE_PAGE_CLASS : PAGE_CLASS }), [immersive])
}
