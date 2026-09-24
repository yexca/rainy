import { useQuery } from '@tanstack/react-query'
import { ChevronRight, Disc3, ListMusic, MicVocal, Music, Radio, Settings, Shapes, Star, type LucideIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'

import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { useIsMobile } from '@/hooks/use-media-query'
import { UserMenu } from '@/layouts/user-menu'
import { cn } from '@/lib/utils'

import { AlbumCard, AlbumCardSkeleton } from '../components/album-card'
import { CardGrid } from '../components/card-grid'
import { SectionHeader } from '../components/section-header'
import { albumListQuery } from '../lib/queries'

interface Section {
  to: string
  labelKey: string
  icon: LucideIcon
}

const SECTIONS: readonly Section[] = [
  { to: '/playlists', labelKey: 'common:nav.playlists', icon: ListMusic },
  { to: '/artists', labelKey: 'common:nav.artists', icon: MicVocal },
  { to: '/albums', labelKey: 'common:nav.albums', icon: Disc3 },
  { to: '/songs', labelKey: 'common:nav.songs', icon: Music },
  { to: '/genres', labelKey: 'common:nav.genres', icon: Shapes },
  { to: '/favorites', labelKey: 'common:nav.favorites', icon: Star },
  { to: '/radio', labelKey: 'common:nav.radio', icon: Radio },
  { to: '/settings', labelKey: 'common:nav.settings', icon: Settings },
]

const RECENT_COUNT = 12

/** Library hub (the "Library" tab on phones): section list + recently added albums. */
export default function LibraryPage() {
  const { t } = useTranslation('library')
  const isMobile = useIsMobile()
  const recent = useQuery(albumListQuery({ sort: 'recent', order: 'desc', limit: RECENT_COUNT }))

  return (
    <Page>
      <PageHeader title={t('common:nav.library')} navActions={isMobile ? <UserMenu /> : undefined} />
      <nav aria-label={t('common:nav.library')}>
        <ul className="bleed-x md:mx-0 md:grid md:grid-cols-2 md:gap-2 lg:grid-cols-4">
          {SECTIONS.map((section) => (
            <li
              key={section.to}
              className="hairline-inset page-x [--hairline-inset:calc(var(--page-px)+var(--safe-left)+2.5rem)] md:px-0 md:after:hidden"
            >
              <Link
                to={section.to}
                className={cn(
                  'flex h-12 items-center gap-4 text-[17px] transition-colors outline-none active:bg-accent/60',
                  'md:h-14 md:rounded-xl md:bg-muted/60 md:px-4 md:text-[15px] md:font-medium md:hover:bg-accent md:focus-visible:ring-[3px] md:focus-visible:ring-ring/50',
                )}
              >
                <section.icon className="size-6 shrink-0 text-primary md:size-5" strokeWidth={1.75} aria-hidden />
                <span className="flex-1">{t(section.labelKey)}</span>
                <ChevronRight className="size-4 text-muted-foreground/60" aria-hidden />
              </Link>
            </li>
          ))}
        </ul>
      </nav>

      {recent.isPending || (recent.data && recent.data.items.length > 0) ? (
        <section className="mt-8 md:mt-10">
          <SectionHeader title={t('library.recentlyAdded')} to="/albums?sort=recent" />
          <CardGrid>
            {recent.data
              ? recent.data.items.map((album) => <AlbumCard key={album.id} album={album} />)
              : Array.from({ length: 6 }, (_, i) => <AlbumCardSkeleton key={i} />)}
          </CardGrid>
        </section>
      ) : null}
    </Page>
  )
}
