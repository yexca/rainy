import { ListMusic, Music, Radio, type LucideIcon } from 'lucide-react'
import { useCallback, useState, type CSSProperties } from 'react'
import { useTranslation } from 'react-i18next'

import { useMascotArt } from '@/hooks/use-mascot-art'
import { coverSrcSet, coverUrlForPixels } from '@/lib/cover'
import { placeholderArt, type PlaceholderKind } from '@/lib/placeholder-art'
import { cn } from '@/lib/utils'

const ROUNDED = {
  none: 'rounded-none',
  sm: 'rounded-sm',
  md: 'rounded-md',
  lg: 'rounded-lg',
  xl: 'rounded-xl',
} as const

export interface CoverArtProps {
  /** Cover art id (`track.coverArt`, `album.coverArt`, …). Empty → placeholder. */
  coverArt?: string | null
  /**
   * Rendered width in CSS px. Picks the image resolution (1x/2x `srcset`) and, unless `fluid`,
   * the element size.
   */
  size: number
  /** Fill the parent's width instead of rendering at `size` (grids, hero art). */
  fluid?: boolean
  alt?: string
  /** `circle` for artists. */
  shape?: 'square' | 'circle'
  /** Corner radius (default: `lg` from 160px up, `md` below). Ignored for circles. */
  rounded?: keyof typeof ROUNDED
  /** Load eagerly (above-the-fold hero art). */
  priority?: boolean
  /** Placeholder glyph (default: music note), shown when mascot illustrations are off. */
  icon?: LucideIcon
  /**
   * Which mascot artwork stands in for a missing cover. Default: `artist` for circles,
   * `playlist` / `radio` for the `ListMusic` / `Radio` icons, else `cover` (one of three
   * variants picked by `alt`).
   */
  placeholder?: PlaceholderKind
  /** Drop the shadow + hairline ring (e.g. inside an already framed surface). */
  flat?: boolean
  /**
   * Keep showing the previously loaded cover until the new one has loaded, then fade the new
   * one in on top (no placeholder flash between tracks — use for the player's artwork).
   */
  keepPrevious?: boolean
  className?: string
  style?: CSSProperties
}

/**
 * Square artwork with lazy loading, a fade-in, a 1x/2x `srcset`, and a placeholder when the
 * cover is missing or fails to load: the mascot (docs/development/design.md#mascot), or a soft
 * gradient with a glyph when illustrations are off (docs/architecture/contract.md §9.3). A cover
 * that is still loading sits on the plain gradient, so the mascot never flashes before it.
 */
export function CoverArt({
  coverArt,
  size,
  fluid,
  alt,
  shape = 'square',
  rounded,
  priority,
  icon: Icon = Music,
  placeholder,
  flat,
  keepPrevious,
  className,
  style,
}: CoverArtProps) {
  const { t } = useTranslation()
  const [shown, setShown] = useState<{ src: string; srcSet: string } | null>(null)
  const radius = shape === 'circle' ? 'rounded-full' : ROUNDED[rounded ?? (size >= 160 ? 'lg' : 'md')]
  const src = coverArt ? coverUrlForPixels(coverArt, size) : ''
  const iconSize = Math.round(Math.min(64, Math.max(14, size * 0.34)))
  const mascot = useMascotArt()
  const [failed, setFailed] = useState('')
  const missing = !src || failed === src
  const kind: PlaceholderKind =
    placeholder ?? (shape === 'circle' ? 'artist' : Icon === ListMusic ? 'playlist' : Icon === Radio ? 'radio' : 'cover')

  return (
    <div
      className={cn(
        'relative isolate aspect-square shrink-0 overflow-hidden bg-muted select-none',
        !flat && 'shadow-sm ring-1 ring-black/5 dark:ring-white/10',
        radius,
        fluid && 'w-full',
        className,
      )}
      style={fluid ? style : { width: size, height: size, ...style }}
    >
      <div
        aria-hidden
        className="absolute inset-0 grid place-items-center bg-linear-to-br from-muted via-muted to-muted-foreground/20"
      >
        {mascot ? null : (
          <Icon className="text-muted-foreground/55" style={{ width: iconSize, height: iconSize }} strokeWidth={1.5} />
        )}
      </div>
      {mascot && missing ? (
        <img
          aria-hidden
          alt=""
          src={placeholderArt(kind, alt)}
          loading="lazy"
          decoding="async"
          draggable={false}
          className="absolute inset-0 size-full object-cover"
        />
      ) : null}
      {keepPrevious && shown && src && shown.src !== src ? (
        <img
          aria-hidden
          alt=""
          src={shown.src}
          srcSet={shown.srcSet}
          draggable={false}
          className="absolute inset-0 size-full object-cover"
        />
      ) : null}
      {src ? (
        <CoverImage
          key={src}
          src={src}
          srcSet={coverSrcSet(coverArt ?? '', size)}
          alt={alt ?? t('a11y.coverArt')}
          priority={priority}
          onSettled={keepPrevious ? setShown : undefined}
          onError={setFailed}
        />
      ) : null}
    </div>
  )
}

interface CoverImageProps {
  src: string
  srcSet: string
  alt: string
  priority?: boolean
  /** Called with the image once it loaded, or `null` if it failed. */
  onSettled?: (image: { src: string; srcSet: string } | null) => void
  /** Called with `src` when it fails to load. */
  onError?: (src: string) => void
}

/** Keyed by `src` so load/error state resets whenever the cover changes. */
function CoverImage({ src, srcSet, alt, priority, onSettled, onError }: CoverImageProps) {
  const [state, setState] = useState<'loading' | 'loaded' | 'error'>('loading')
  const loaded = useCallback(() => {
    setState('loaded')
    onSettled?.({ src, srcSet })
  }, [onSettled, src, srcSet])

  // Cached images can finish before React attaches `onLoad`.
  const ref = useCallback(
    (img: HTMLImageElement | null) => {
      if (img?.complete && img.naturalWidth > 0) loaded()
    },
    [loaded],
  )

  if (state === 'error') return null
  return (
    <img
      ref={ref}
      src={src}
      srcSet={srcSet}
      alt={alt}
      loading={priority ? 'eager' : 'lazy'}
      fetchPriority={priority ? 'high' : undefined}
      decoding="async"
      draggable={false}
      onLoad={loaded}
      onError={() => {
        setState('error')
        onSettled?.(null)
        onError?.(src)
      }}
      className={cn(
        'absolute inset-0 size-full object-cover transition-opacity duration-300 ease-out',
        state === 'loaded' ? 'opacity-100' : 'opacity-0',
      )}
    />
  )
}
