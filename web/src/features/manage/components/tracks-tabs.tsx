import { useIsMobile } from '@/hooks/use-media-query'
import { TRACKS_SECTION } from '@/layouts/nav'
import { SectionTabs } from '@/layouts/section-tabs'

import { ManageSections } from './manage-sections'

/**
 * Tabs of the Tracks entry (Metadata / Upload / Online). Phones also get links to the library
 * tools and admin sections here, since they have no sidebar.
 */
export function TracksTabs({ className }: { className?: string }) {
  const isMobile = useIsMobile()
  return (
    <SectionTabs section={TRACKS_SECTION} className={className}>
      {isMobile ? <ManageSections /> : null}
    </SectionTabs>
  )
}
