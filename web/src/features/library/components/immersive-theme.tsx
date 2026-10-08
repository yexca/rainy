import { useEffect, useMemo } from 'react'

import { useCurrentTrack } from '@/features/player/store'
import { useTheme } from '@/hooks/use-theme'

import { coverTheme } from '../lib/cover-theme'
import { useCoverSeeds, useImmersive, usePageCover } from '../lib/use-cover-theme'

/**
 * Immersive mode (Settings → Appearance): colours the whole app — sidebar, header, player,
 * menus — with the cover of the album / playlist / artist page on screen, or else with the
 * playing track's cover; the previous colours stay while the next cover is sampled. Without a
 * cover (or with the mode off) the user's theme applies. Renders nothing; mounted once in the
 * app shell.
 */
export function ImmersiveTheme() {
  const { resolvedTheme, setOverride } = useTheme()
  const immersive = useImmersive()
  const pageCover = usePageCover((s) => s.coverArt)
  const trackCover = useCurrentTrack()?.coverArt
  const coverArt = immersive ? (pageCover ?? trackCover) : undefined
  const seeds = useCoverSeeds(coverArt, immersive, true)

  const theme = useMemo(() => (seeds ? coverTheme(seeds, resolvedTheme === 'dark') : null), [seeds, resolvedTheme])

  useEffect(() => {
    setOverride(theme)
  }, [theme, setOverride])

  useEffect(() => () => setOverride(null), [setOverride])

  return null
}
