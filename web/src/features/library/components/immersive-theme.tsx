import { useEffect, useMemo } from 'react'

import { useCurrentTrack, usePlayer } from '@/features/player/store'
import { useTheme } from '@/hooks/use-theme'
import { cn } from '@/lib/utils'

import { coverTheme, withoutAccent } from '../lib/cover-theme'
import { useCoverSeeds, useImmersive, usePageCover } from '../lib/use-cover-theme'

/** Where each colour field sits: the cover's own colour behind the sidebar and header, the others across the page. */
const FIELD_CLASSES = [
  '-top-[38vmax] -left-[26vmax] size-[92vmax]',
  '-right-[22vmax] -bottom-[42vmax] size-[96vmax] [animation-delay:-8s]',
  '-top-[12vmax] right-[4vmax] size-[62vmax] [animation-delay:-15s]',
]

/**
 * The colour fields behind the whole app (radial gradients, no blur filters). The sidebar, the
 * header and the page are see-through over them (`:root[data-immersive]` in index.css). They
 * drift from tablets up while music plays.
 */
function Backdrop({ colors }: { colors: readonly string[] }) {
  const playing = usePlayer((s) => s.isPlaying)
  return (
    <div aria-hidden className="pointer-events-none fixed inset-0 -z-10 overflow-hidden">
      {colors.map((color, i) => (
        <div
          key={i}
          className={cn('absolute max-w-none rounded-full motion-safe:md:animate-blob', FIELD_CLASSES[i % FIELD_CLASSES.length])}
          style={{
            background: `radial-gradient(closest-side, ${color}, transparent)`,
            animationPlayState: playing ? 'running' : 'paused',
          }}
        />
      ))}
    </div>
  )
}

/**
 * Immersive mode (Settings → Appearance): colours the whole app — sidebar, header, player,
 * menus — with the cover of the album / playlist / artist page on screen, or else with the
 * playing track's cover; the previous colours stay while the next cover is sampled. The cover's
 * colours also fill the window as soft colour fields that show through the sidebar, the header
 * and the page. The `original` accent takes the highlight from the cover too; an accent preset
 * keeps its own. Without a cover (or with the mode off) the user's theme applies and nothing is
 * rendered. Mounted once in the app shell.
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

  return theme ? <Backdrop colors={theme.backdrop} /> : null
}
