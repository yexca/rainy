import { apiUrl, seg } from '@/lib/api/client'

/** Sizes the server resizes covers to (keeps the disk cache small and URLs cacheable). */
export const COVER_SIZES = [64, 128, 256, 512, 1024] as const
export type CoverSize = (typeof COVER_SIZES)[number]

/** Smallest standard size ≥ `px` (capped at 1024). */
export function snapCoverSize(px: number): CoverSize {
  for (const size of COVER_SIZES) {
    if (px <= size) return size
  }
  return COVER_SIZES[COVER_SIZES.length - 1]
}

function devicePixelRatio(): number {
  return typeof window === 'undefined' ? 1 : Math.min(window.devicePixelRatio || 1, 3)
}

/** Cover URL for an exact (snapped) pixel size, without device-pixel scaling. */
export function coverUrlForPixels(coverArt: string, px: number): string {
  if (!coverArt) return ''
  return apiUrl(`/cover/${seg(coverArt)}`, { size: snapCoverSize(px) })
}

/**
 * Cover image URL for an element that is `size` CSS pixels wide; the size is scaled by the
 * device pixel ratio and snapped to 64/128/256/512/1024. Returns `''` for empty ids.
 */
export function coverUrl(coverArt: string, size: number): string {
  return coverUrlForPixels(coverArt, Math.ceil(size * devicePixelRatio()))
}

/** `srcset` with 1x/2x candidates for an element `size` CSS pixels wide (`''` for empty ids). */
export function coverSrcSet(coverArt: string, size: number): string {
  if (!coverArt) return ''
  const x1 = coverUrlForPixels(coverArt, size)
  const x2 = coverUrlForPixels(coverArt, size * 2)
  return x1 === x2 ? x1 : `${x1} 1x, ${x2} 2x`
}
