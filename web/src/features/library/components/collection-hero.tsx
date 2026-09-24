import type { LucideIcon } from 'lucide-react'
import { useState, type ReactNode, type Ref } from 'react'

import { CoverArt } from '@/components/cover-art'
import { Skeleton } from '@/components/ui/skeleton'
import { coverUrlForPixels } from '@/lib/cover'
import { cn } from '@/lib/utils'

export interface CollectionHeroProps {
  coverArt: string
  shape?: 'square' | 'circle'
  icon?: LucideIcon
  /** Small label above the title on desktop ("Album", "Playlist", …). */
  kind?: ReactNode
  title: string
  /** Receives the `<h1>` (the nav bar watches it to collapse). */
  titleRef?: Ref<HTMLHeadingElement>
  /** Accent line under the title (artist link, owner…). */
  byline?: ReactNode
  /** Muted meta line (genre · year · format). */
  meta?: ReactNode
  description?: ReactNode
  /** Play / Shuffle buttons: rendered stretched on phones, inline on desktop. */
  playButtons?: (layout: 'inline' | 'stretch') => ReactNode
  /** Desktop-only trailing controls next to the buttons (star, "…"). */
  trailing?: ReactNode
  className?: string
}

/** Blurred, saturated artwork glow behind a hero (fades into the page background). */
export function ArtworkBackdrop({ coverArt, className }: { coverArt: string; className?: string }) {
  const [loaded, setLoaded] = useState(false)
  if (!coverArt) return null
  return (
    <div aria-hidden className={cn('pointer-events-none absolute inset-x-0 top-0 -z-10 overflow-hidden', className)}>
      <img
        src={coverUrlForPixels(coverArt, 64)}
        alt=""
        decoding="async"
        onLoad={() => setLoaded(true)}
        className={cn(
          'absolute inset-0 size-full scale-125 object-cover blur-3xl saturate-150 transition-opacity duration-700',
          loaded ? 'opacity-45 dark:opacity-40' : 'opacity-0',
        )}
      />
      <div className="absolute inset-0 bg-linear-to-b from-background/10 via-background/55 to-background" />
    </div>
  )
}

/**
 * Header of album / playlist / artist pages: large artwork with title, byline, meta and
 * Play / Shuffle. Phones: centred stack (Apple Music iOS); desktop: artwork left, text right.
 */
export function CollectionHero({
  coverArt,
  shape = 'square',
  icon,
  kind,
  title,
  titleRef,
  byline,
  meta,
  description,
  playButtons,
  trailing,
  className,
}: CollectionHeroProps) {
  const circle = shape === 'circle'
  return (
    <div className={cn('flex flex-col items-center gap-5 pt-2 pb-6 md:flex-row md:items-end md:gap-8 md:pt-4 md:pb-8', className)}>
      <div className={cn('w-[min(68vw,300px)] shrink-0 md:w-56 lg:w-60', circle && 'w-[min(52vw,220px)] md:w-52 lg:w-56')}>
        <CoverArt
          coverArt={coverArt}
          size={300}
          fluid
          priority
          shape={shape}
          icon={icon}
          alt={title}
          className="shadow-xl shadow-black/10 dark:shadow-black/40"
        />
      </div>
      <div className="flex w-full min-w-0 flex-col items-center text-center md:items-start md:text-left">
        {kind ? (
          <div className="mb-1 hidden text-xs font-semibold tracking-wider text-muted-foreground uppercase md:block">
            {kind}
          </div>
        ) : null}
        <h1
          ref={titleRef}
          className="max-w-full text-[22px] leading-7 font-bold tracking-tight text-balance break-words md:text-3xl md:leading-tight lg:text-4xl"
        >
          {title}
        </h1>
        {byline ? (
          <div className="mt-0.5 max-w-full truncate text-[20px] leading-7 text-primary md:mt-1 md:text-xl">{byline}</div>
        ) : null}
        {meta ? (
          <div className="mt-1 max-w-full text-[13px] font-medium text-muted-foreground md:text-sm md:font-normal">{meta}</div>
        ) : null}
        {description ? (
          <p className="mt-2 line-clamp-3 max-w-prose text-sm text-pretty text-muted-foreground">{description}</p>
        ) : null}
        {playButtons ? (
          <>
            <div className="mt-5 w-full md:hidden">{playButtons('stretch')}</div>
            <div className="mt-6 hidden items-center gap-2 md:flex">
              {playButtons('inline')}
              {trailing ? <div className="ml-2 flex items-center gap-1">{trailing}</div> : null}
            </div>
          </>
        ) : trailing ? (
          <div className="mt-6 hidden items-center gap-1 md:flex">{trailing}</div>
        ) : null}
      </div>
    </div>
  )
}

export function CollectionHeroSkeleton({ shape = 'square' }: { shape?: 'square' | 'circle' }) {
  return (
    <div className="flex flex-col items-center gap-5 pt-2 pb-6 md:flex-row md:items-end md:gap-8 md:pt-4 md:pb-8" aria-hidden>
      <Skeleton
        className={cn(
          'aspect-square w-[min(68vw,300px)] shrink-0 md:w-56 lg:w-60',
          shape === 'circle' ? 'w-[min(52vw,220px)] rounded-full md:w-52' : 'rounded-lg',
        )}
      />
      <div className="flex w-full flex-col items-center gap-2.5 md:items-start">
        <Skeleton className="h-7 w-2/3 max-w-sm md:h-9" />
        <Skeleton className="h-5 w-1/3 max-w-48" />
        <Skeleton className="h-4 w-1/4 max-w-40" />
        <div className="mt-4 flex w-full gap-3 md:w-auto">
          <Skeleton className="h-12 flex-1 rounded-xl md:h-9 md:w-28 md:flex-none md:rounded-full" />
          <Skeleton className="h-12 flex-1 rounded-xl md:h-9 md:w-28 md:flex-none md:rounded-full" />
        </div>
      </div>
    </div>
  )
}
