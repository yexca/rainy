import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, type CSSProperties } from 'react'
import { create } from 'zustand'

import { useTheme } from '@/hooks/use-theme'
import { coverUrlForPixels } from '@/lib/cover'
import { currentThemeColor } from '@/lib/theme'
import { cn } from '@/lib/utils'

import { coverTheme, extractCoverSeeds, type CoverSeeds } from './cover-theme'

/** The page paints its own (possibly scoped) background and text colour, down to the bottom. */
const PAGE_CLASS = 'grow bg-background text-foreground'

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

/** Immersive mode is on: the whole app follows the cover, so pages need no scoped tokens. */
export function useImmersive(): boolean {
  return useTheme().immersive
}

export interface CoverThemeScope {
  /** Spread on the page element (`<Page {...scope}>`): `className` (`dark` for deep pages) and the token `style`. */
  className: string
  style?: CSSProperties
}

/**
 * Scoped theme for a page coloured by its cover (album, playlist and artist pages): background, text, fills and the
 * accent follow the artwork while the rest of the app keeps the user's theme. Also tints the
 * browser chrome (`<meta name="theme-color">`) while mounted. Returns an empty scope while the
 * cover is loading, for covers without art, and when the user turned album colours off. In
 * immersive mode the page instead publishes its cover (`usePageCover`) and `<ImmersiveTheme/>`
 * colours the whole app with it, whatever the album colours setting.
 *
 *   <Page {...useCoverTheme(album?.coverArt)}>
 */
export function useCoverTheme(coverArt: string | undefined): CoverThemeScope {
  const { resolvedTheme, albumColors } = useTheme()
  const immersive = useImmersive()
  const seeds = useCoverSeeds(coverArt, albumColors && !immersive)

  useEffect(() => {
    if (!immersive || !coverArt) return
    usePageCover.setState({ coverArt })
    return () => {
      if (usePageCover.getState().coverArt === coverArt) usePageCover.setState({ coverArt: null })
    }
  }, [immersive, coverArt])

  const theme = useMemo(
    () => (!immersive && seeds ? coverTheme(seeds, resolvedTheme === 'dark') : null),
    [immersive, seeds, resolvedTheme],
  )

  useEffect(() => {
    if (!theme) return
    const metas = document.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]')
    for (const meta of metas) meta.content = theme.themeColor
    return () => {
      // The app theme may have changed meanwhile: restore what applyTheme() would set now.
      const current = currentThemeColor()
      for (const meta of metas) meta.content = current
    }
  }, [theme])

  return useMemo(
    () =>
      theme
        ? {
            className: cn(PAGE_CLASS, theme.dark && 'dark'),
            style: { ...theme.vars, colorScheme: theme.dark ? 'dark' : 'light' } as CSSProperties,
          }
        : { className: PAGE_CLASS },
    [theme],
  )
}
