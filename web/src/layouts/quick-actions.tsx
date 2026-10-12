import { useQuery } from '@tanstack/react-query'
import { defaultFilter } from 'cmdk'
import {
  AppWindow,
  AudioWaveform,
  ListMusic,
  Paintbrush,
  Pause,
  Play,
  Plus,
  Search,
  Shuffle,
  SkipBack,
  SkipForward,
  UserRound,
  Zap,
  type LucideIcon,
} from 'lucide-react'
import { useEffect, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { create } from 'zustand'

import { Button } from '@/components/ui/button'
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog'
import { Drawer, DrawerContent, DrawerDescription, DrawerTitle } from '@/components/ui/drawer'
import { usePlayer } from '@/features/player/store'
import { useAuth } from '@/hooks/use-auth'
import { useIsMobile } from '@/hooks/use-media-query'
import { api } from '@/lib/api/endpoints'
import { queryKeys } from '@/lib/query-keys'

import { ADMIN_SECTION, FOOTER_NAV, LIBRARY_NAV, TOOLS_SECTION, TRACKS_SECTION, type NavItem } from './nav'

const APPLE = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform)
const SHORTCUT_LABEL = APPLE ? '⌘K' : 'Ctrl K'
const SHORTCUT_KEYS = APPLE ? 'Meta+K' : 'Control+K'

/**
 * cmdk value of the "search the library" row: shown while there is a query, ranked below every
 * matching action so Enter opens the best match and falls back to a library search.
 */
const SEARCH_VALUE = '__search__'

/** Personal settings tabs (`/settings/:tab`), direct palette destinations. */
const SETTINGS_TABS: readonly NavItem[] = [
  { to: '/settings', labelKey: 'nav.account', icon: UserRound },
  { to: '/settings/appearance', labelKey: 'nav.appearance', icon: Paintbrush },
  { to: '/settings/playback', labelKey: 'nav.playback', icon: Play },
  { to: '/settings/apps', labelKey: 'nav.apps', icon: AppWindow },
  { to: '/settings/scrobbling', labelKey: 'nav.scrobbling', icon: AudioWaveform },
]

function filter(value: string, search: string, keywords?: string[]): number {
  return value === SEARCH_VALUE ? Number.MIN_VALUE : defaultFilter(value, search, keywords)
}

function ActionRow({
  value,
  icon: Icon,
  label,
  hint,
  keywords,
  onSelect,
}: {
  value: string
  icon: LucideIcon
  label: ReactNode
  hint?: string
  keywords?: string[]
  onSelect: () => void
}) {
  return (
    <CommandItem value={value} keywords={keywords} onSelect={onSelect} className="min-h-11 gap-3 rounded-lg px-3 md:min-h-10">
      <Icon strokeWidth={1.75} aria-hidden />
      <span className="min-w-0 flex-1 truncate">{label}</span>
      {hint ? <span className="hidden shrink-0 text-xs text-muted-foreground sm:inline">{hint}</span> : null}
    </CommandItem>
  )
}

/**
 * The palette body: a filter field over grouped actions — playback (while something is queued),
 * library pages and playlists, the manager and admin pages the user may open, and personal
 * settings. Typing also offers a library search for the text.
 */
function QuickActionsPalette({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { isManager, isAdmin } = useAuth()
  const [query, setQuery] = useState('')
  const hasTrack = usePlayer((s) => s.index >= 0)
  const isPlaying = usePlayer((s) => s.isPlaying)
  const shuffle = usePlayer((s) => s.shuffle)
  const { data: playlists } = useQuery({
    queryKey: queryKeys.playlists,
    queryFn: ({ signal }) => api.playlists.list({ signal }),
  })

  const run = (action: () => void) => () => {
    onClose()
    action()
  }
  const go = (to: string) => run(() => void navigate(to))
  const player = () => usePlayer.getState()
  const trimmed = query.trim()

  const pages = (items: readonly NavItem[], group: string) =>
    items.map((item) => (
      <ActionRow
        key={item.to}
        value={`page:${item.to}`}
        icon={item.icon}
        label={t(item.labelKey)}
        hint={item.to}
        keywords={[t(item.labelKey), group, item.to]}
        onSelect={go(item.to)}
      />
    ))

  const manageGroup = t('quickActions.groups.manage')
  const adminGroup = t('nav.admin')
  const settingsGroup = t('nav.settings')

  return (
    <Command filter={filter} loop className="bg-transparent">
      <CommandInput
        value={query}
        onValueChange={setQuery}
        placeholder={t('quickActions.placeholder')}
        className="h-12 text-base md:text-sm"
      />
      <CommandList className="max-h-none flex-1 overscroll-contain px-1.5 py-1.5 md:max-h-[min(26rem,60dvh)]">
        <CommandEmpty>{t('quickActions.empty')}</CommandEmpty>
        {hasTrack ? (
          <CommandGroup heading={t('quickActions.groups.playback')}>
            <ActionRow
              value="player:toggle"
              icon={isPlaying ? Pause : Play}
              label={isPlaying ? t('quickActions.pause') : t('quickActions.play')}
              hint={t('quickActions.keys.space')}
              onSelect={run(() => player().togglePlay())}
            />
            <ActionRow
              value="player:next"
              icon={SkipForward}
              label={t('quickActions.next')}
              hint="Shift →"
              onSelect={run(() => player().next())}
            />
            <ActionRow
              value="player:prev"
              icon={SkipBack}
              label={t('quickActions.previous')}
              hint="Shift ←"
              onSelect={run(() => player().prev())}
            />
            <ActionRow
              value="player:shuffle"
              icon={Shuffle}
              label={shuffle ? t('quickActions.shuffleOff') : t('quickActions.shuffleOn')}
              onSelect={run(() => player().toggleShuffle())}
            />
            <ActionRow
              value="player:now-playing"
              icon={AudioWaveform}
              label={t('quickActions.nowPlaying')}
              onSelect={run(() => player().setNowPlayingOpen(true))}
            />
          </CommandGroup>
        ) : null}
        <CommandGroup heading={t('nav.library')}>
          {pages(LIBRARY_NAV, t('nav.library'))}
          <ActionRow
            value="page:/playlists"
            icon={ListMusic}
            label={t('nav.allPlaylists')}
            hint="/playlists"
            keywords={[t('nav.playlists')]}
            onSelect={go('/playlists')}
          />
          <ActionRow
            value="action:new-playlist"
            icon={Plus}
            label={t('nav.newPlaylist')}
            keywords={[t('nav.playlists')]}
            onSelect={go('/playlists?new=1')}
          />
        </CommandGroup>
        {playlists?.length ? (
          <CommandGroup heading={t('nav.playlists')}>
            {playlists.map((playlist) => (
              <ActionRow
                key={playlist.id}
                value={`playlist:${playlist.id}`}
                icon={ListMusic}
                label={playlist.name}
                keywords={[playlist.name, t('nav.playlists')]}
                onSelect={go(`/playlists/${playlist.id}`)}
              />
            ))}
          </CommandGroup>
        ) : null}
        {isManager ? (
          <CommandGroup heading={manageGroup}>
            {pages([...TRACKS_SECTION.tabs, ...TOOLS_SECTION.tabs], manageGroup)}
          </CommandGroup>
        ) : null}
        {isAdmin ? <CommandGroup heading={adminGroup}>{pages(ADMIN_SECTION.tabs, adminGroup)}</CommandGroup> : null}
        <CommandGroup heading={settingsGroup}>
          {pages([...SETTINGS_TABS, ...FOOTER_NAV.filter((item) => item.to !== '/settings')], settingsGroup)}
        </CommandGroup>
        {trimmed ? (
          <CommandGroup>
            <ActionRow
              value={SEARCH_VALUE}
              icon={Search}
              label={t('quickActions.searchFor', { query: trimmed })}
              onSelect={go(`/search?q=${encodeURIComponent(trimmed)}`)}
            />
          </CommandGroup>
        ) : null}
      </CommandList>
    </Command>
  )
}

/** Whether the palette is open: the app chrome may render several triggers for the one palette. */
const usePalette = create<{ open: boolean }>()(() => ({ open: false }))

function setOpen(open: boolean) {
  usePalette.setState({ open })
}

function close() {
  setOpen(false)
}

/**
 * Quick actions trigger: a lightning icon in the app chrome (an icon, never a search field, so it
 * is not mistaken for library search) that opens the palette of `<QuickActionsHost/>`.
 */
export function QuickActions({ className }: { className?: string }) {
  const { t } = useTranslation()
  const open = usePalette((s) => s.open)
  const label = t('quickActions.label')

  return (
    <Button
      variant="ghost"
      size="icon"
      aria-label={label}
      aria-keyshortcuts={SHORTCUT_KEYS}
      aria-haspopup="dialog"
      title={`${label} (${SHORTCUT_LABEL})`}
      data-state={open ? 'open' : 'closed'}
      className={className}
      onClick={() => setOpen(true)}
    >
      <Zap strokeWidth={1.75} />
    </Button>
  )
}

/**
 * The quick actions palette, mounted once in the app shell: `<QuickActions/>` and Ctrl/⌘K
 * anywhere open a searchable command palette — a centered dialog on tablets and desktops, a
 * bottom sheet on phones.
 */
export function QuickActionsHost() {
  const { t } = useTranslation()
  const isMobile = useIsMobile()
  const open = usePalette((s) => s.open)
  const label = t('quickActions.label')

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key.toLowerCase() !== 'k' || !(event.metaKey || event.ctrlKey) || event.altKey || event.shiftKey) return
      event.preventDefault()
      setOpen(!usePalette.getState().open)
    }
    window.addEventListener('keydown', onKeyDown)
    return () => {
      window.removeEventListener('keydown', onKeyDown)
      close()
    }
  }, [])

  if (isMobile) {
    return (
      <Drawer open={open} onOpenChange={setOpen} repositionInputs={false}>
        <DrawerContent className="h-[85dvh] max-h-[85dvh] rounded-t-2xl border-none pb-safe data-[vaul-drawer-direction=bottom]:max-h-[85dvh]">
          <DrawerTitle className="sr-only">{label}</DrawerTitle>
          <DrawerDescription className="sr-only">{t('quickActions.description')}</DrawerDescription>
          <div className="flex min-h-0 flex-1 flex-col pt-2">{open ? <QuickActionsPalette onClose={close} /> : null}</div>
        </DrawerContent>
      </Drawer>
    )
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent
        showCloseButton={false}
        className="top-[12vh] translate-y-0 gap-0 overflow-hidden rounded-2xl p-0 sm:max-w-xl"
      >
        <DialogTitle className="sr-only">{label}</DialogTitle>
        <DialogDescription className="sr-only">{t('quickActions.description')}</DialogDescription>
        <QuickActionsPalette onClose={close} />
      </DialogContent>
    </Dialog>
  )
}
