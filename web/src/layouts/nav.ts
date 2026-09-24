import {
  Disc3,
  FolderTree,
  HardDrive,
  History,
  House,
  LibraryBig,
  MicVocal,
  Music,
  Radio,
  Search,
  Shapes,
  SlidersHorizontal,
  Star,
  Stethoscope,
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
  { to: '/radio', labelKey: 'nav.radio', icon: Radio },
]

export const MANAGE_NAV: readonly NavItem[] = [
  { to: '/manage', labelKey: 'nav.manageLibrary', icon: LibraryBig, end: true },
  { to: '/manage/folders', labelKey: 'nav.folders', icon: FolderTree },
  { to: '/manage/upload', labelKey: 'nav.upload', icon: Upload },
  { to: '/manage/doctor', labelKey: 'nav.doctor', icon: Stethoscope },
  { to: '/manage/trash', labelKey: 'nav.trash', icon: Trash2 },
  { to: '/manage/history', labelKey: 'nav.history', icon: History },
]

export const ADMIN_NAV: readonly NavItem[] = [
  { to: '/admin/users', labelKey: 'nav.users', icon: Users },
  { to: '/admin/libraries', labelKey: 'nav.libraries', icon: HardDrive },
  { to: '/admin/settings', labelKey: 'nav.serverSettings', icon: SlidersHorizontal },
]

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
    match: ['/library', '/albums', '/artists', '/songs', '/genres', '/favorites', '/playlists', '/radio', '/settings'],
  },
  { to: '/search', labelKey: 'nav.search', icon: Search, match: ['/search'] },
  { to: '/manage', labelKey: 'nav.manage', icon: Wrench, match: ['/manage', '/admin'], managersOnly: true },
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
