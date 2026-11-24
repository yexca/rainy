import {
  AudioLines,
  AudioWaveform,
  ChartColumn,
  ClipboardCheck,
  Disc3,
  Download,
  FolderTree,
  Globe,
  HardDrive,
  History,
  House,
  Info,
  LibraryBig,
  Link2,
  ListMusic,
  MicVocal,
  Music,
  Plus,
  Radio,
  Settings,
  ShieldCheck,
  SlidersHorizontal,
  Star,
  Stethoscope,
  Tags,
  Trash2,
  Upload,
  Users,
  Wrench,
  type LucideIcon,
} from 'lucide-react'

export interface NavItem {
  to: string
  /** i18n key in the `common` namespace. */
  labelKey: string
  icon: LucideIcon
  /** Active only on an exact match. */
  end?: boolean
  /**
   * Pages grouped under this tab: the page shows them as a segmented control and the tab stays
   * active on each of them. `to` is the first one.
   */
  items?: readonly NavItem[]
}

export const LIBRARY_NAV: readonly NavItem[] = [
  { to: '/', labelKey: 'nav.home', icon: House, end: true },
  { to: '/albums', labelKey: 'nav.albums', icon: Disc3 },
  { to: '/artists', labelKey: 'nav.artists', icon: MicVocal },
  { to: '/songs', labelKey: 'nav.songs', icon: Music },
  { to: '/favorites', labelKey: 'nav.favorites', icon: Star },
  { to: '/listening', labelKey: 'nav.listening', icon: ChartColumn },
  { to: '/radio', labelKey: 'nav.radio', icon: Radio },
]

/** Pinned below the scrolling sidebar groups. */
export const FOOTER_NAV: readonly NavItem[] = [
  { to: '/settings', labelKey: 'nav.settings', icon: Settings },
  { to: '/about', labelKey: 'nav.about', icon: Info },
]

/**
 * A manager entry: one sidebar item whose pages are route tabs in each page header
 * (`SectionTabs`). The item links to the first tab and stays active on every tab.
 */
export interface NavSection {
  /** i18n key in the `common` namespace. */
  labelKey: string
  icon: LucideIcon
  tabs: readonly NavItem[]
}

/**
 * Tracks: the track table + tag editor, and the ways of adding music (upload from this device,
 * YouTube / bilibili links, online search). Used often, so it comes first.
 */
export const TRACKS_SECTION: NavSection = {
  labelKey: 'nav.tracks',
  icon: AudioLines,
  tabs: [
    { to: '/manage', labelKey: 'nav.metadata', icon: Tags, end: true },
    {
      to: '/manage/upload',
      labelKey: 'nav.add',
      icon: Plus,
      items: [
        { to: '/manage/upload', labelKey: 'nav.upload', icon: Upload },
        { to: '/manage/links', labelKey: 'nav.links', icon: Link2 },
        { to: '/manage/online', labelKey: 'nav.online', icon: Globe },
      ],
    },
    { to: '/manage/review', labelKey: 'nav.review', icon: ClipboardCheck },
  ],
}

/** The rarer library tools. */
export const TOOLS_SECTION: NavSection = {
  labelKey: 'nav.libraryTools',
  icon: Wrench,
  tabs: [
    { to: '/manage/folders', labelKey: 'nav.folders', icon: FolderTree },
    { to: '/manage/doctor', labelKey: 'nav.doctor', icon: Stethoscope },
    { to: '/manage/trash', labelKey: 'nav.trash', icon: Trash2 },
    { to: '/manage/history', labelKey: 'nav.history', icon: History },
  ],
}

/** Administration (admins only). Each server settings page is a tab of its own. */
export const ADMIN_SECTION: NavSection = {
  labelKey: 'nav.admin',
  icon: ShieldCheck,
  tabs: [
    { to: '/admin/libraries', labelKey: 'nav.libraries', icon: HardDrive },
    {
      to: '/admin/settings',
      labelKey: 'nav.serverSettings',
      icon: SlidersHorizontal,
      end: true,
      items: [
        { to: '/admin/settings', labelKey: 'nav.general', icon: SlidersHorizontal, end: true },
        { to: '/admin/settings/ytdlp', labelKey: 'nav.ytdlp', icon: Download },
        { to: '/admin/settings/sources', labelKey: 'nav.sources', icon: ListMusic },
        { to: '/admin/settings/scrobbling', labelKey: 'nav.scrobbling', icon: AudioWaveform },
        { to: '/admin/settings/system', labelKey: 'nav.system', icon: Info },
      ],
    },
    { to: '/admin/users', labelKey: 'nav.users', icon: Users },
  ],
}

/** Whether `pathname` is `item` or one of its grouped pages. */
export function navItemActive(item: NavItem, pathname: string): boolean {
  return pathMatches(pathname, item.to, item.end) || (item.items ?? []).some((sub) => navItemActive(sub, pathname))
}

/** Whether `pathname` is one of the section's tabs. */
export function isSectionPath(section: NavSection, pathname: string): boolean {
  return section.tabs.some((tab) => navItemActive(tab, pathname))
}

/** The pages grouped with `to` under one of `tabs` (`NavItem.items`). */
export function groupedPages(tabs: readonly NavItem[], to: string): readonly NavItem[] {
  return tabs.find((tab) => tab.items?.some((item) => item.to === to))?.items ?? []
}

/** The section's pages, with grouped pages listed one by one (quick actions). */
export function sectionPages(section: NavSection): NavItem[] {
  return section.tabs.flatMap((tab) => tab.items ?? [tab])
}

export interface TabItem extends NavItem {
  /** Path prefixes that mark this tab active. */
  match: readonly string[]
  managersOnly?: boolean
}

/** Mobile bottom tab bar. */
export const TABS: readonly TabItem[] = [
  { to: '/', labelKey: 'nav.home', icon: House, end: true, match: ['/'] },
  { to: '/playlists', labelKey: 'nav.playlists', icon: ListMusic, match: ['/playlists'] },
  {
    to: '/library',
    labelKey: 'nav.library',
    icon: LibraryBig,
    match: ['/library', '/albums', '/artists', '/songs', '/favorites', '/listening', '/radio', '/settings', '/about'],
  },
  // Phones have no sidebar: this tab holds Tracks and links to the library tools and admin pages.
  { to: '/manage', labelKey: 'nav.manage', icon: Wrench, end: true, match: ['/manage', '/admin'], managersOnly: true },
]

/** Whether `pathname` is `prefix` or below it (`/albums` matches `/albums/1`, not `/albumsx`). */
export function pathMatches(pathname: string, prefix: string, end = false): boolean {
  if (prefix === '/') return pathname === '/'
  if (end) return pathname === prefix || pathname === `${prefix}/`
  return pathname === prefix || pathname.startsWith(`${prefix}/`)
}

export function isTabActive(tab: TabItem, pathname: string): boolean {
  return tab.match.some((prefix) => pathMatches(pathname, prefix, prefix === '/'))
}
