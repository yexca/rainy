import { MicVocal } from 'lucide-react'
import { memo } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'

import { CoverArt } from '@/components/cover-art'
import { Skeleton } from '@/components/ui/skeleton'
import type { Artist } from '@/lib/api/types'
import { cn } from '@/lib/utils'

import { ARTIST_CAPTION_HEIGHT } from '../lib/layout'

export interface ArtistCardProps {
  artist: Artist
  sizeHint?: number
  /** Show "N albums" under the name. */
  showCount?: boolean
  className?: string
}

/** Circular artist tile with the name centred below. */
export const ArtistCard = memo(function ArtistCard({
  artist,
  sizeHint = 180,
  showCount = true,
  className,
}: ArtistCardProps) {
  const { t } = useTranslation()
  const count =
    artist.albumCount > 0
      ? t('common:count.albums', { count: artist.albumCount })
      : t('common:count.songs', { count: artist.songCount })
  return (
    <Link
      to={`/artists/${artist.id}`}
      className={cn(
        'group/artist block min-w-0 rounded-xl text-center outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50',
        className,
      )}
    >
      <div className="transition-transform duration-150 ease-out group-hover/artist:scale-[1.02] active:scale-[0.97]">
        <CoverArt coverArt={artist.coverArt} size={sizeHint} fluid shape="circle" icon={MicVocal} alt={artist.name} />
      </div>
      <div className="mt-2" style={{ height: ARTIST_CAPTION_HEIGHT - 8 }}>
        <p className="truncate text-[14px] leading-5 font-medium md:text-[13px]">{artist.name}</p>
        {showCount ? <p className="truncate text-[13px] leading-5 text-muted-foreground md:text-xs">{count}</p> : null}
      </div>
    </Link>
  )
})

export function ArtistCardSkeleton({ className }: { className?: string }) {
  return (
    <div className={cn('flex min-w-0 flex-col items-center', className)} aria-hidden>
      <Skeleton className="aspect-square w-full rounded-full" />
      <div className="mt-2 flex w-full flex-col items-center gap-1.5" style={{ height: ARTIST_CAPTION_HEIGHT - 8 }}>
        <Skeleton className="h-3.5 w-3/5" />
        <Skeleton className="h-3 w-2/5" />
      </div>
    </div>
  )
}
