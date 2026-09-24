import { useQuery } from '@tanstack/react-query'
import { FastAverageColor } from 'fast-average-color'

import { coverUrlForPixels } from '@/lib/cover'

export interface CoverPalette {
  /** Top of the background gradient. */
  top: string
  /** Bottom of the background gradient. */
  bottom: string
}

/** Neutral fallback (no cover / not loaded yet). */
export const DEFAULT_PALETTE: CoverPalette = { top: 'hsl(222 18% 30%)', bottom: 'hsl(222 20% 12%)' }

let fac: FastAverageColor | null = null

function rgbToHsl(r: number, g: number, b: number): [number, number, number] {
  const rn = r / 255
  const gn = g / 255
  const bn = b / 255
  const max = Math.max(rn, gn, bn)
  const min = Math.min(rn, gn, bn)
  const l = (max + min) / 2
  if (max === min) return [0, 0, l]
  const d = max - min
  const s = l > 0.5 ? d / (2 - max - min) : d / (max + min)
  let h: number
  if (max === rn) h = (gn - bn) / d + (gn < bn ? 6 : 0)
  else if (max === gn) h = (bn - rn) / d + 2
  else h = (rn - gn) / d + 4
  return [h * 60, s, l]
}

function clamp(n: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, n))
}

/**
 * Turn the cover's dominant colour into a dark-enough, saturated gradient so white text stays
 * readable (Apple Music style).
 */
export function paletteFromRgb(r: number, g: number, b: number): CoverPalette {
  const [h, s, l] = rgbToHsl(r, g, b)
  const sat = s < 0.08 ? s : clamp(s * 1.25, 0.25, 0.75)
  const light = clamp(l, 0.22, 0.42)
  const hue = Math.round(h)
  return {
    top: `hsl(${hue} ${Math.round(sat * 100)}% ${Math.round(light * 100)}%)`,
    bottom: `hsl(${hue} ${Math.round(sat * 90)}% ${Math.round(light * 45)}%)`,
  }
}

/** Background palette derived from a cover (cached per cover id). */
export function useCoverColor(coverArt: string | undefined): CoverPalette {
  const { data } = useQuery({
    queryKey: ['player', 'cover-color', coverArt ?? ''],
    queryFn: async () => {
      fac ??= new FastAverageColor()
      const result = await fac.getColorAsync(coverUrlForPixels(coverArt ?? '', 64), {
        algorithm: 'dominant',
        mode: 'speed',
        ignoredColor: [
          [255, 255, 255, 255, 24],
          [0, 0, 0, 255, 24],
        ],
      })
      const [r, g, b] = result.value
      return paletteFromRgb(r, g, b)
    },
    enabled: !!coverArt,
    staleTime: Number.POSITIVE_INFINITY,
    gcTime: 30 * 60_000,
    retry: false,
  })
  return data ?? DEFAULT_PALETTE
}
