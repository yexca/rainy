import {
  AudioLines,
  AudioWaveform,
  ChartColumn,
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
  Radio,
  Search,
  Settings,
  Shapes,
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
}

export const LIBRARY_NAV: readonly NavItem[] = [
  { to: '/', labelKey: 'nav.home', icon: House, end: true },
  { to: '/search', labelKey: 'nav.search', icon: Search },
  { to: '/albums', labelKey: 'nav.albums', icon: Disc3 },
  { to: '/artists', labelKey: 'nav.artists', icon: MicVocal },
  { to: '/songs', labelKey: 'nav.songs', icon: Music },
  { to: '/genres', labelKey: 'nav.genres', icon: Shapes },
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
    { to: '/manage/upload', labelKey: 'nav.upload', icon: Upload },
    { to: '/manage/links', labelKey: 'nav.links', icon: Link2 },
    { to: '/manage/online', labelKey: 'nav.online', icon: Globe },
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
    { to: '/admin/users', labelKey: 'nav.users', icon: Users },
    { to: '/admin/libraries', labelKey: 'nav.libraries', icon: HardDrive },
    { to: '/admin/settings', labelKey: 'nav.serverSettings', icon: SlidersHorizontal, end: true },
    { to: '/admin/settings/ytdlp', labelKey: 'nav.ytdlp', icon: Download },
    { to: '/admin/settings/sources', labelKey: 'nav.sources', icon: ListMusic },
    { to: '/admin/settings/scrobbling', labelKey: 'nav.scrobbling', icon: AudioWaveform },
    { to: '/admin/settings/system', labelKey: 'nav.system', icon: Info },
  ],
}

/** Whether `pathname` is one of the section's tabs. */
export function isSectionPath(section: NavSection, pathname: string): boolean {
  return section.tabs.some((tab) => pathMatches(pathname, tab.to, tab.end))
}

export interface TabItem extends NavItem {
  /** Path prefixes that mark this tab active. */
  match: readonly string[]
  managersOnly?: boolean
}

/** Mobile bottom tab bar. */
export const TABS: readonly TabItem[] = [
  { to: '/', labelKey: 'nav.home', icon: House, end: true, match: ['/'] },
  {
    to: '/library',
    labelKey: 'nav.library',
    icon: LibraryBig,
    match: ['/library', '/albums', '/artists', '/songs', '/genres', '/favorites', '/listening', '/playlists', '/radio', '/settings', '/about'],
  },
  { to: '/search', labelKey: 'nav.search', icon: Search, match: ['/search'] },
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
