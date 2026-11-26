import { useQuery } from '@tanstack/react-query'

import { useIsMobile } from '@/hooks/use-media-query'
import { TRACKS_SECTION } from '@/layouts/nav'
import { SectionTabs } from '@/layouts/section-tabs'

import { stagingCountQuery } from '../queries'
import { ManageSections } from './manage-sections'

const REVIEW = '/manage/review'

/**
 * Tabs of the Tracks entry (Metadata / Add / Review). Review carries the number of downloads
 * waiting and only shows while there are some (or while it is open). Phones also get links to
 * the library tools and admin sections here, since they have no sidebar.
 */
export function TracksTabs({ className }: { className?: string }) {
  const isMobile = useIsMobile()
  const waiting = useQuery(stagingCountQuery).data ?? 0
  return (
    <SectionTabs
      section={TRACKS_SECTION}
      badges={{ [REVIEW]: waiting }}
      hidden={waiting === 0 ? [REVIEW] : undefined}
      className={className}
    >
      {isMobile ? <ManageSections /> : null}
    </SectionTabs>
  )
}
