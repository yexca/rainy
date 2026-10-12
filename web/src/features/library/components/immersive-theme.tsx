import { useEffect, useMemo } from 'react'

import { useCurrentTrack } from '@/features/player/store'
import { useTheme } from '@/hooks/use-theme'

import { coverTheme, withoutAccent } from '../lib/cover-theme'
import { useCoverSeeds, useImmersive, usePageCover } from '../lib/use-cover-theme'

/**
 * Immersive mode (Settings → Appearance): colours the whole app — sidebar, header, player,
 * menus — with the cover of the album / playlist / artist page on screen, or else with the
 * playing track's cover; the previous colours stay while the next cover is sampled. The
 * `original` accent takes the highlight from the cover too; an accent preset keeps its own.
 * Without a cover (or with the mode off) the user's theme applies. Renders nothing; mounted once
 * in the app shell.
 */
export function ImmersiveTheme() {
  const { resolvedTheme, accent, setOverride } = useTheme()
  const immersive = useImmersive()
  const pageCover = usePageCover((s) => s.coverArt)
  const trackCover = useCurrentTrack()?.coverArt
  const coverArt = immersive ? (pageCover ?? trackCover) : undefined
  const seeds = useCoverSeeds(coverArt, immersive, true)

  const theme = useMemo(() => {
    if (!seeds) return null
    const cover = coverTheme(seeds, resolvedTheme === 'dark')
    return accent === 'original' ? cover : withoutAccent(cover)
  }, [seeds, resolvedTheme, accent])

  useEffect(() => {
    setOverride(theme)
  }, [theme, setOverride])

  useEffect(() => () => setOverride(null), [setOverride])

  return null
}
