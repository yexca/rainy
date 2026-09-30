import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, type CSSProperties } from 'react'

import { useTheme } from '@/hooks/use-theme'
import { coverUrlForPixels } from '@/lib/cover'
import { THEME_COLORS } from '@/lib/theme'
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

export interface CoverThemeScope {
  /** Spread on the page element (`<Page {...scope}>`): `className` (`dark` for deep pages) and the token `style`. */
  className: string
  style?: CSSProperties
}

/**
 * Scoped theme for a page coloured by its cover (album, playlist and artist pages): background, text, fills and the
 * accent follow the artwork while the rest of the app keeps the user's theme. Also tints the
 * browser chrome (`<meta name="theme-color">`) while mounted. Returns an empty scope while the
 * cover is loading, for covers without art, and when the user turned album colours off.
 *
 *   <Page {...useCoverTheme(album?.coverArt)}>
 */
export function useCoverTheme(coverArt: string | undefined): CoverThemeScope {
  const { resolvedTheme, albumColors } = useTheme()
  const enabled = albumColors && !!coverArt
  const { data: seeds } = useQuery({
    queryKey: ['library', 'cover-theme', coverArt ?? ''],
    queryFn: () => sampleCover(coverArt ?? ''),
    enabled,
    staleTime: Number.POSITIVE_INFINITY,
    gcTime: 30 * 60_000,
    retry: false,
  })

  const theme = useMemo(
    () => (enabled && seeds ? coverTheme(seeds, resolvedTheme === 'dark') : null),
    [enabled, seeds, resolvedTheme],
  )

  useEffect(() => {
    if (!theme) return
    const metas = document.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]')
    for (const meta of metas) meta.content = theme.themeColor
    return () => {
      // The app theme may have changed meanwhile: restore what applyTheme() would set now.
      const current = THEME_COLORS[document.documentElement.classList.contains('dark') ? 'dark' : 'light']
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
