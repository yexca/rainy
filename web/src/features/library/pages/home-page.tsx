import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Disc3, FolderUp, Loader2, RefreshCw, Shuffle } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useAuth } from '@/hooks/use-auth'
import { useIsMobile } from '@/hooks/use-media-query'
import { UserMenu } from '@/layouts/user-menu'
import type { Album } from '@/lib/api/types'
import { formatNumber } from '@/lib/format'

import { AlbumCard, AlbumCardSkeleton } from '../components/album-card'
import { HorizontalShelf } from '../components/horizontal-shelf'
import { usePlayCollection } from '../lib/play'
import { homeQuery } from '../lib/queries'

function greetingKey(hour: number): string {
  if (hour < 5) return 'home.greeting.night'
  if (hour < 12) return 'home.greeting.morning'
  if (hour < 18) return 'home.greeting.afternoon'
  return 'home.greeting.evening'
}

function Shelf({
  title,
  albums,
  to,
  subtitle,
  actions,
}: {
  title: string
  albums: Album[]
  to?: string
  subtitle?: 'artist' | 'year' | 'artistYear'
  actions?: ReactNode
}) {
  if (albums.length === 0) return null
  return (
    <HorizontalShelf title={title} to={to} actions={actions}>
      {albums.map((album, i) => (
        <AlbumCard key={album.id} album={album} subtitle={subtitle} priority={i < 4} />
      ))}
    </HorizontalShelf>
  )
}

function ShelfSkeleton() {
  return (
    <div className="mt-8 md:mt-10" aria-hidden>
      <Skeleton className="mb-4 h-6 w-44" />
      <div className="flex gap-4 overflow-hidden md:gap-5">
        {Array.from({ length: 7 }, (_, i) => (
          <AlbumCardSkeleton key={i} className="w-[calc((100%-1rem)/2.3)] shrink-0 sm:w-[calc((100%-2rem)/3.3)] md:w-44 lg:w-48" />
        ))}
      </div>
    </div>
  )
}

export default function HomePage() {
  const { t } = useTranslation('library')
  const { user, isManager, isAdmin } = useAuth()
  const isMobile = useIsMobile()
  const queryClient = useQueryClient()
  const play = usePlayCollection()
  const home = useQuery(homeQuery())
  const [hour] = useState(() => new Date().getHours())
  const [refreshingRandom, setRefreshingRandom] = useState(false)

  const name = user?.displayName.trim() || user?.username || ''
  const title = t(greetingKey(hour), { name })
  const stats = home.data?.stats
  const empty = !!home.data && home.data.stats.tracks === 0

  const refreshRandom = async () => {
    setRefreshingRandom(true)
    try {
      await queryClient.refetchQueries({ queryKey: homeQuery().queryKey, exact: true })
    } finally {
      setRefreshingRandom(false)
    }
  }

  return (
    <Page>
      <PageHeader
        title={title}
        subtitle={
          stats && stats.tracks > 0
            ? t('home.stats', {
                songs: formatNumber(stats.tracks),
                albums: formatNumber(stats.albums),
                artists: formatNumber(stats.artists),
              })
            : undefined
        }
        navActions={isMobile ? <UserMenu /> : undefined}
        actions={
          empty || !home.data ? undefined : (
            <Button
              onClick={() => void play({ kind: 'library' }, 'shuffle')}
              className="h-11 gap-2 rounded-full px-5 font-semibold shadow-sm active:scale-[0.97] md:h-9"
            >
              <Shuffle className="size-4" strokeWidth={2} aria-hidden />
              {t('home.shuffleAll')}
            </Button>
          )
        }
      />

      {home.isPending ? (
        <>
          <ShelfSkeleton />
          <ShelfSkeleton />
          <ShelfSkeleton />
        </>
      ) : home.isError ? (
        <ErrorState error={home.error} onRetry={() => void home.refetch()} retrying={home.isFetching} />
      ) : empty ? (
        <EmptyState
          icon={Disc3}
          art="empty"
          title={t('home.emptyTitle')}
          description={isManager ? t('home.emptyManager') : t('home.emptyListener')}
          action={
            isManager ? (
              <>
                <Button asChild>
                  <Link to="/manage/upload">
                    <FolderUp aria-hidden />
                    {t('home.upload')}
                  </Link>
                </Button>
                {isAdmin ? (
                  <Button variant="outline" asChild>
                    <Link to="/admin/libraries">{t('home.scanLibrary')}</Link>
                  </Button>
                ) : null}
              </>
            ) : null
          }
        />
      ) : (
        <div className="-mt-2">
          <Shelf title={t('home.recentlyAdded')} albums={home.data.recentlyAdded} to="/albums?sort=recent" />
          <Shelf title={t('home.recentlyPlayed')} albums={home.data.recentlyPlayed} to="/albums?sort=played" />
          <Shelf title={t('home.mostPlayed')} albums={home.data.mostPlayed} to="/albums?sort=frequent" />
          <Shelf
            title={t('home.discover')}
            albums={home.data.random}
            to="/albums?sort=random"
            actions={
              <Button
                variant="ghost"
                size="icon"
                onClick={() => void refreshRandom()}
                disabled={refreshingRandom}
                aria-label={t('home.refreshDiscover')}
                title={t('home.refreshDiscover')}
                className="size-9 rounded-full text-muted-foreground"
              >
                {refreshingRandom ? (
                  <Loader2 className="size-4 animate-spin" aria-hidden />
                ) : (
                  <RefreshCw className="size-4" strokeWidth={1.75} aria-hidden />
                )}
              </Button>
            }
          />
          <Shelf title={t('home.favorites')} albums={home.data.starred} to="/favorites?tab=albums" />
        </div>
      )}
    </Page>
  )
}
