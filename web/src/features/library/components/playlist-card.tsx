import { ListMusic, Loader2, Play } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'

import { CoverArt } from '@/components/cover-art'
import type { Playlist } from '@/lib/api/types'
import { cn } from '@/lib/utils'

import { usePlayCollection } from '../lib/play'

export interface PlaylistCardProps {
  playlist: Playlist
  /** Show the owner (playlists shared by other users). */
  showOwner?: boolean
  sizeHint?: number
  className?: string
}

/** Playlist tile (server-rendered mosaic cover) with a hover play button on desktop. */
export function PlaylistCard({ playlist, showOwner, sizeHint = 200, className }: PlaylistCardProps) {
  const { t } = useTranslation()
  const play = usePlayCollection()
  const [starting, setStarting] = useState(false)
  const count = t('common:count.songs', { count: playlist.songCount })
  const subtitle = showOwner && playlist.ownerName ? `${playlist.ownerName} · ${count}` : count

  return (
    <div className={cn('group/card relative min-w-0', className)}>
      <Link
        to={`/playlists/${playlist.id}`}
        className="block rounded-lg outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
      >
        <div className="transition-transform duration-150 ease-out active:scale-[0.97] md:active:scale-100">
          <CoverArt
            coverArt={playlist.songCount > 0 ? playlist.coverArt : ''}
            size={sizeHint}
            fluid
            icon={ListMusic}
            alt={playlist.name}
          />
        </div>
        <div className="mt-2 h-10 min-w-0">
          <p className="truncate text-[14px] leading-5 font-medium md:text-[13px]">{playlist.name}</p>
          <p className="truncate text-[14px] leading-5 text-muted-foreground md:text-[13px]">{subtitle}</p>
        </div>
      </Link>
      {playlist.songCount > 0 ? (
        <button
          type="button"
          onClick={() => {
            setStarting(true)
            void play({ kind: 'playlist', id: playlist.id }).finally(() => setStarting(false))
          }}
          aria-label={t('library:playlist.playPlaylist', { name: playlist.name })}
          className={cn(
            'absolute bottom-[3.75rem] left-2.5 hidden size-10 place-items-center rounded-full bg-primary text-primary-foreground opacity-0 shadow-lg transition-[opacity,transform] outline-none hover:scale-105 focus-visible:opacity-100 focus-visible:ring-[3px] focus-visible:ring-white/70 active:scale-95 [@media(hover:hover)]:grid',
            'group-hover/card:opacity-100',
            starting && 'opacity-100',
          )}
        >
          {starting ? (
            <Loader2 className="size-5 animate-spin" aria-hidden />
          ) : (
            <Play className="ml-0.5 size-5" fill="currentColor" strokeWidth={0} aria-hidden />
          )}
        </button>
      ) : null}
    </div>
  )
}
