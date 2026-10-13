import { useQuery } from '@tanstack/react-query'
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
 * The cover's background / accent seeds (sampled once per cover and cached). `keepPrevious`
 * keeps the previous cover's seeds while the next one is sampled.
 */
export function useCoverSeeds(coverArt: string | undefined, enabled = true, keepPrevious = false): CoverSeeds | null {
  const { data } = useQuery({
    queryKey: ['library', 'cover-theme', coverArt ?? ''],
    queryFn: () => sampleCover(coverArt ?? ''),
    enabled: enabled && !!coverArt,
    staleTime: Number.POSITIVE_INFINITY,
    gcTime: 30 * 60_000,
    retry: false,
    placeholderData: keepPrevious ? (previous) => previous : undefined,
  })
  return (enabled && coverArt && data) || null
}

/** The cover of the album / playlist / artist page on screen, for immersive mode. */
export const usePageCover = create<{ coverArt: string | null }>()(() => ({ coverArt: null }))

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
 * and the page paints no background of its own. With the mode off nothing is taken from the
 * cover: the page keeps the user's theme like every other page.
 *
 *   <Page {...useCoverTheme(album?.coverArt)}>
 */
export function useCoverTheme(coverArt: string | undefined): CoverThemeScope {
  const immersive = useImmersive()

  useEffect(() => {
    if (!immersive || !coverArt) return
    usePageCover.setState({ coverArt })
    return () => {
      if (usePageCover.getState().coverArt === coverArt) usePageCover.setState({ coverArt: null })
    }
  }, [immersive, coverArt])

  return useMemo(() => ({ className: immersive ? IMMERSIVE_PAGE_CLASS : PAGE_CLASS }), [immersive])
}
