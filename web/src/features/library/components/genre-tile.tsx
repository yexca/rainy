import { Music2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'

import type { Genre } from '@/lib/api/types'
import { cn } from '@/lib/utils'

import { seedGradient } from '../lib/colors'
import { genrePath } from '../lib/paths'

/** Colourful gradient tile for a genre (colour derived from its name). */
export function GenreTile({ genre, className }: { genre: Genre; className?: string }) {
  const { t } = useTranslation()
  return (
    <Link
      to={genrePath(genre.name)}
      className={cn(
        'group/genre relative isolate block aspect-[16/10] overflow-hidden rounded-xl p-3 text-white shadow-sm ring-1 ring-black/5 outline-none transition-transform duration-150 ease-out focus-visible:ring-[3px] focus-visible:ring-ring/60 active:scale-[0.97] md:p-4 md:hover:-translate-y-0.5 md:hover:shadow-md dark:ring-white/10',
        className,
      )}
      style={{ backgroundImage: seedGradient(genre.name) }}
    >
      <Music2
        aria-hidden
        strokeWidth={1.5}
        className="absolute -right-4 -bottom-5 -z-10 size-24 rotate-[18deg] text-white/20 transition-transform duration-300 group-hover/genre:rotate-[8deg] md:size-28"
      />
      <span className="absolute inset-0 -z-10 bg-linear-to-t from-black/25 to-transparent" aria-hidden />
      <span className="flex h-full flex-col justify-between">
        <span className="line-clamp-2 text-[17px] leading-tight font-bold tracking-tight text-balance drop-shadow-sm md:text-lg">
          {genre.name}
        </span>
        <span className="text-xs font-medium text-white/85">
          {t('common:count.songs', { count: genre.songCount })}
        </span>
      </span>
    </Link>
  )
}
