import { useQuery } from '@tanstack/react-query'
import { ListMusic, Plus } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, useSearchParams } from 'react-router'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { Button } from '@/components/ui/button'
import { useCurrentUser } from '@/hooks/use-auth'
import { useIsMobile } from '@/hooks/use-media-query'

import { AlbumCardSkeleton } from '../components/album-card'
import { CardGrid } from '../components/card-grid'
import { PlaylistCard } from '../components/playlist-card'
import { PlaylistFormDialog } from '../components/playlist-form-dialog'
import { SectionHeader } from '../components/section-header'
import { playlistsQuery } from '../lib/queries'

export default function PlaylistsPage() {
  const { t } = useTranslation('library')
  const user = useCurrentUser()
  const isMobile = useIsMobile()
  const navigate = useNavigate()
  const [params, setParams] = useSearchParams()
  const creating = params.get('new') === '1'
  const query = useQuery(playlistsQuery())

  const [own, shared] = useMemo(() => {
    const list = [...(query.data ?? [])].sort((a, b) => b.updatedAt - a.updatedAt)
    return [list.filter((p) => p.ownerId === user?.id), list.filter((p) => p.ownerId !== user?.id)]
  }, [query.data, user?.id])

  const setCreating = (open: boolean) =>
    setParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        if (open) next.set('new', '1')
        else next.delete('new')
        return next
      },
      { replace: true },
    )

  const newButton = (
    <Button
      onClick={() => setCreating(true)}
      variant={isMobile ? 'ghost' : 'default'}
      size={isMobile ? 'icon' : 'default'}
      aria-label={t('common:nav.newPlaylist')}
      className={
        isMobile ? 'size-11 rounded-full text-primary hover:text-primary' : 'h-9 gap-2 rounded-full px-4 font-semibold'
      }
    >
      <Plus className={isMobile ? 'size-6' : 'size-4'} strokeWidth={2} aria-hidden />
      {isMobile ? null : t('common:nav.newPlaylist')}
    </Button>
  )

  return (
    <Page>
      <PageHeader
        title={t('common:nav.playlists')}
        back={isMobile}
        subtitle={query.data ? t('common:count.playlists', { count: query.data.length }) : undefined}
        navActions={isMobile ? newButton : undefined}
        actions={isMobile ? undefined : newButton}
      />

      {query.isPending ? (
        <CardGrid>
          {Array.from({ length: 8 }, (_, i) => (
            <AlbumCardSkeleton key={i} />
          ))}
        </CardGrid>
      ) : query.isError ? (
        <ErrorState error={query.error} onRetry={() => void query.refetch()} retrying={query.isFetching} />
      ) : own.length + shared.length === 0 ? (
        <EmptyState
          icon={ListMusic}
          art="empty"
          title={t('playlists.emptyTitle')}
          description={t('playlists.emptyDescription')}
          action={
            <Button onClick={() => setCreating(true)}>
              <Plus aria-hidden />
              {t('common:nav.newPlaylist')}
            </Button>
          }
        />
      ) : (
        <>
          {own.length > 0 ? (
            <section>
              {shared.length > 0 ? <SectionHeader title={t('playlists.mine')} /> : null}
              <CardGrid>
                {own.map((playlist) => (
                  <PlaylistCard key={playlist.id} playlist={playlist} />
                ))}
              </CardGrid>
            </section>
          ) : null}
          {shared.length > 0 ? (
            <section className={own.length > 0 ? 'mt-10' : undefined}>
              <SectionHeader title={t('playlists.shared')} />
              <CardGrid>
                {shared.map((playlist) => (
                  <PlaylistCard key={playlist.id} playlist={playlist} showOwner />
                ))}
              </CardGrid>
            </section>
          ) : null}
        </>
      )}

      <PlaylistFormDialog
        open={creating}
        onOpenChange={setCreating}
        onSaved={(playlist) => navigate(`/playlists/${playlist.id}`)}
      />
    </Page>
  )
}
