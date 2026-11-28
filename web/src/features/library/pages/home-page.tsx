import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Disc3, FolderUp, RefreshCw, Shuffle } from 'lucide-react'
import { useMemo, useState, type ReactNode } from 'react'
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
import { useTheme } from '@/hooks/use-theme'
import { QuickActionsField } from '@/layouts/quick-actions'
import type { Album, SongPicks } from '@/lib/api/types'
import { STYLE_QUICK_ACTIONS } from '@/lib/theme-styles'
import { cn } from '@/lib/utils'

import { AlbumCard, AlbumCardSkeleton, type AlbumCardSubtitle } from '../components/album-card'
import { ForYou } from '../components/for-you'
import { HorizontalShelf } from '../components/horizontal-shelf'
import { SongShelf, SongShelfSkeleton } from '../components/song-shelf'
import { availablePickModes, recentAlbums, resolvePickMode, type PickMode } from '../lib/home'
import { usePlayCollection } from '../lib/play'
import { homeQuery, songPicksQuery } from '../lib/queries'

const PICK_LABEL_KEYS: Record<PickMode, string> = {
  quick: 'home.quickPicks',
  onRepeat: 'home.onRepeat',
  rediscover: 'home.rediscover',
}

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
  subtitle?: AlbumCardSubtitle
  actions?: ReactNode
}) {
  if (albums.length === 0) return null
  return (
    <HorizontalShelf title={title} to={to} actions={actions}>
      {albums.map((album) => (
        <AlbumCard key={album.id} album={album} subtitle={subtitle} />
      ))}
    </HorizontalShelf>
  )
}

/** "Show others" button of a shelf whose content is picked at random. */
function RefreshButton({ label, busy, onClick }: { label: string; busy: boolean; onClick: () => void }) {
  return (
    <Button
      variant="ghost"
      size="icon"
      disabled={busy}
      onClick={onClick}
      aria-label={label}
      title={label}
      className="size-11 rounded-full text-muted-foreground hover:text-foreground md:size-9"
    >
      <RefreshCw className={cn('size-4', busy && 'animate-spin')} strokeWidth={1.75} aria-hidden />
    </Button>
  )
}

function ShelfSkeleton() {
  return (
    <div className="mt-8 md:mt-10" aria-hidden>
      <Skeleton className="mb-4 h-6 w-44" />
      <div className="flex gap-4 overflow-hidden md:gap-5">
        {Array.from({ length: 7 }, (_, i) => (
          <AlbumCardSkeleton
            key={i}
            className="w-[calc((100%-1rem)/2.3)] shrink-0 sm:w-[calc((100%-2rem)/3.3)] md:w-44 lg:w-48"
          />
        ))}
      </div>
    </div>
  )
}

/**
 * Consolidated song-picks shelf: one 4-row horizontal song grid with lightweight inline pills
 * beside the heading (`Quick picks · On repeat · Rediscover`) when multiple modes have songs.
 */
function PickedSongs({
  data,
  isPending,
  isRefetching,
  onRefresh,
}: {
  data: SongPicks | undefined
  isPending: boolean
  isRefetching: boolean
  onRefresh: () => void
}) {
  const { t } = useTranslation('library')
  const [selected, setSelected] = useState<PickMode>('quick')
  const modes = useMemo(() => availablePickModes(data), [data])
  const active = resolvePickMode(selected, modes)

  if (isPending) return <SongShelfSkeleton />
  if (!data || !active) return null

  const tracks = data[active]
  const activeLabel = t(PICK_LABEL_KEYS[active])
  const showTabs = modes.length > 1

  return (
    <SongShelf
      title={showTabs ? t('home.pickedSongs') : activeLabel}
      playLabel={activeLabel}
      tracks={tracks}
      tabs={
        showTabs ? (
          <div
            role="tablist"
            aria-label={t('home.pickedSongs')}
            className="inline-flex items-center gap-0.5 rounded-full bg-muted/70 p-0.5"
          >
            {modes.map((mode) => {
              const isSelected = mode === active
              return (
                <button
                  key={mode}
                  type="button"
                  role="tab"
                  aria-selected={isSelected}
                  onClick={() => setSelected(mode)}
                  className={cn(
                    'relative rounded-full px-3 py-1 text-xs font-medium transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring/50',
                    // Expand touch hit area to >= 44px height on mobile while keeping a slim capsule visually.
                    'after:absolute after:-inset-y-2 after:inset-x-0 md:after:hidden',
                    isSelected
                      ? 'bg-background text-foreground shadow-2xs immersive:bg-foreground/15'
                      : 'text-muted-foreground hover:text-foreground',
                  )}
                >
                  {t(PICK_LABEL_KEYS[mode])}
                </button>
              )
            })}
          </div>
        ) : undefined
      }
      actions={
        active !== 'onRepeat' ? (
          <RefreshButton label={t('home.refreshPicks')} busy={isRefetching} onClick={onRefresh} />
        ) : undefined
      }
    />
  )
}

export default function HomePage() {
  const { t } = useTranslation('library')
  const { user, isManager, isAdmin } = useAuth()
  const isMobile = useIsMobile()
  const { style } = useTheme()
  const queryClient = useQueryClient()
  const play = usePlayCollection()
  const home = useQuery(homeQuery())
  const picks = useQuery(songPicksQuery())
  const [hour] = useState(() => new Date().getHours())
  const [refreshingRandom, setRefreshingRandom] = useState(false)

  const name = user?.displayName.trim() || user?.username || ''
  const title = t(greetingKey(hour), { name })
  const empty = !!home.data && home.data.stats.tracks === 0
  const chromeHasField = STYLE_QUICK_ACTIONS[style] === 'field'
  const recentAdded = useMemo(() => recentAlbums(home.data?.recentlyAdded ?? []), [home.data?.recentlyAdded])

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
        bar={false}
        titleAction={
          empty || !home.data ? undefined : (
            <Button
              variant="secondary"
              size="icon"
              onClick={() => void play({ kind: 'library' }, 'shuffle')}
              aria-label={t('home.shuffleAll')}
              title={t('home.shuffleAll')}
              className="size-11 rounded-full active:scale-[0.95]"
            >
              <Shuffle className="size-4" strokeWidth={2} aria-hidden />
            </Button>
          )
        }
        actions={
          isMobile || empty || !home.data ? undefined : (
            <div className="flex items-center gap-2">
              {chromeHasField ? null : <QuickActionsField appearance="page" className="w-56 lg:w-64" />}
              <Button
                variant="secondary"
                onClick={() => void play({ kind: 'library' }, 'shuffle')}
                className="h-9 gap-2 rounded-full px-4 font-medium active:scale-[0.97]"
              >
                <Shuffle className="size-4" strokeWidth={1.9} aria-hidden />
                {t('home.shuffleAll')}
              </Button>
            </div>
          )
        }
      />

      {home.isPending ? (
        <>
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
          <ForYou />
          <PickedSongs
            data={picks.data}
            isPending={picks.isPending}
            isRefetching={picks.isRefetching}
            onRefresh={() => void picks.refetch()}
          />
          <Shelf title={t('home.recentlyPlayed')} albums={home.data.recentlyPlayed} to="/albums?sort=played" />
          <Shelf title={t('home.recentlyAdded')} albums={recentAdded} to="/albums?sort=recent" />
          <Shelf
            title={t('home.discover')}
            albums={home.data.random}
            to="/albums?sort=random"
            actions={
              <RefreshButton
                label={t('home.refreshDiscover')}
                busy={refreshingRandom}
                onClick={() => void refreshRandom()}
              />
            }
          />
        </div>
      )}
    </Page>
  )
}
