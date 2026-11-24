import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { Command as CommandPrimitive, defaultFilter } from 'cmdk'
import {
  AppWindow,
  ArrowRight,
  AudioWaveform,
  Clock,
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
  X,
  type LucideIcon,
} from 'lucide-react'
import { useEffect, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from 'react-router'
import { create } from 'zustand'

import { CoverArt } from '@/components/cover-art'
import { Spinner } from '@/components/spinner'
import { Button } from '@/components/ui/button'
import { Command, CommandGroup, CommandItem, CommandList } from '@/components/ui/command'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog'
import { Drawer, DrawerContent, DrawerDescription, DrawerTitle } from '@/components/ui/drawer'
import { usePlayer } from '@/features/player/store'
import { useAuth } from '@/hooks/use-auth'
import { useImeValue } from '@/hooks/use-ime-value'
import { useIsMobile } from '@/hooks/use-media-query'
import { api } from '@/lib/api/endpoints'
import { formatDuration } from '@/lib/format'
import { isImeKeyEvent } from '@/lib/ime'
import { queryKeys } from '@/lib/query-keys'
import { addRecentSearch, clearRecentSearches, removeRecentSearch, useRecentSearches } from '@/lib/recent-searches'
import { cn } from '@/lib/utils'

import { ADMIN_SECTION, FOOTER_NAV, LIBRARY_NAV, sectionPages, TOOLS_SECTION, TRACKS_SECTION, type NavItem } from './nav'

const APPLE = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform)
const SHORTCUT_LABEL = APPLE ? '⌘K' : 'Ctrl K'
const SHORTCUT_KEYS = APPLE ? 'Meta+K' : 'Control+K'
const DEBOUNCE_MS = 180
const MAX_RECENT_IDLE = 5
const MAX_MATCHED_ACTIONS = 6
const MAX_MATCHED_PLAYLISTS = 4

/** Personal settings tabs (`/settings/:tab`), direct palette destinations. */
const SETTINGS_TABS: readonly NavItem[] = [
  { to: '/settings', labelKey: 'nav.account', icon: UserRound },
  { to: '/settings/appearance', labelKey: 'nav.appearance', icon: Paintbrush },
  { to: '/settings/playback', labelKey: 'nav.playback', icon: Play },
  { to: '/settings/apps', labelKey: 'nav.apps', icon: AppWindow },
  { to: '/settings/scrobbling', labelKey: 'nav.scrobbling', icon: AudioWaveform },
]

interface ActionEntry {
  value: string
  icon: LucideIcon
  label: string
  hint?: string
  group: string
  keywords: string[]
  onSelect: () => void
}

/** Substring match first (reliable for CJK), then cmdk's fuzzy score for Latin abbreviations. */
function matchesText(query: string, primary: string, keywords: readonly string[] = []): boolean {
  const q = query.toLowerCase()
  if (primary.toLowerCase().includes(q)) return true
  if (keywords.some((k) => k.toLowerCase().includes(q))) return true
  return defaultFilter(primary, query, [...keywords]) > 0
}

function ActionRow({
  value,
  icon: Icon,
  label,
  hint,
  onSelect,
}: {
  value: string
  icon: LucideIcon
  label: ReactNode
  hint?: string
  onSelect: () => void
}) {
  return (
    <CommandItem value={value} onSelect={onSelect} className="min-h-11 gap-3 rounded-lg px-3 md:min-h-10">
      <Icon strokeWidth={1.75} aria-hidden />
      <span className="min-w-0 flex-1 truncate">{label}</span>
      {hint ? <span className="hidden shrink-0 text-xs text-muted-foreground sm:inline">{hint}</span> : null}
    </CommandItem>
  )
}

/**
 * Spotlight body: unified live library search (songs, albums, artists, playlists) + quick
 * navigation and playback commands. `useImeValue` keeps uncommitted CJK readings out of queries.
 */
function SpotlightPalette({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { isManager, isAdmin } = useAuth()
  const [query, setQuery] = useState('')
  const [debounced, setDebounced] = useState('')
  const ime = useImeValue(query, setQuery)
  const recent = useRecentSearches()

  const hasTrack = usePlayer((s) => s.index >= 0)
  const isPlaying = usePlayer((s) => s.isPlaying)
  const shuffle = usePlayer((s) => s.shuffle)
  const { data: playlists } = useQuery({
    queryKey: queryKeys.playlists,
    queryFn: ({ signal }) => api.playlists.list({ signal }),
  })

  const trimmed = query.trim()

  useEffect(() => {
    if (!trimmed) return
    const timer = window.setTimeout(() => setDebounced(trimmed), DEBOUNCE_MS)
    return () => window.clearTimeout(timer)
  }, [trimmed])

  const activeQuery = trimmed ? debounced : ''
  const search = useQuery({
    queryKey: [...queryKeys.search(activeQuery), 4, 5, 6],
    queryFn: ({ signal }) => api.search({ q: activeQuery, artists: 4, albums: 5, tracks: 6 }, { signal }),
    enabled: activeQuery.length > 0,
    placeholderData: keepPreviousData,
    staleTime: 60_000,
  })

  const run = (action: () => void) => () => {
    onClose()
    action()
  }
  const go = (to: string) => run(() => void navigate(to))
  const player = () => usePlayer.getState()

  const navEntries = (items: readonly NavItem[], group: string): ActionEntry[] =>
    items.map((item) => {
      const label = t(item.labelKey)
      return {
        value: `page:${item.to}`,
        icon: item.icon,
        label,
        hint: item.to,
        group,
        keywords: [group, item.to],
        onSelect: go(item.to),
      }
    })

  const playbackGroup = t('quickActions.groups.playback')
  const libraryGroup = t('nav.library')
  const manageGroup = t('quickActions.groups.manage')
  const adminGroup = t('nav.admin')
  const settingsGroup = t('nav.settings')

  const playbackActions: ActionEntry[] = hasTrack
    ? [
        {
          value: 'player:toggle',
          icon: isPlaying ? Pause : Play,
          label: isPlaying ? t('quickActions.pause') : t('quickActions.play'),
          hint: t('quickActions.keys.space'),
          group: playbackGroup,
          keywords: [playbackGroup],
          onSelect: run(() => player().togglePlay()),
        },
        {
          value: 'player:next',
          icon: SkipForward,
          label: t('quickActions.next'),
          hint: 'Shift →',
          group: playbackGroup,
          keywords: [playbackGroup],
          onSelect: run(() => player().next()),
        },
        {
          value: 'player:prev',
          icon: SkipBack,
          label: t('quickActions.previous'),
          hint: 'Shift ←',
          group: playbackGroup,
          keywords: [playbackGroup],
          onSelect: run(() => player().prev()),
        },
        {
          value: 'player:shuffle',
          icon: Shuffle,
          label: shuffle ? t('quickActions.shuffleOff') : t('quickActions.shuffleOn'),
          group: playbackGroup,
          keywords: [playbackGroup],
          onSelect: run(() => player().toggleShuffle()),
        },
        {
          value: 'player:now-playing',
          icon: AudioWaveform,
          label: t('quickActions.nowPlaying'),
          group: playbackGroup,
          keywords: [playbackGroup],
          onSelect: run(() => player().setNowPlayingOpen(true)),
        },
      ]
    : []

  const libraryActions: ActionEntry[] = [
    ...navEntries(LIBRARY_NAV, libraryGroup),
    {
      value: 'page:/playlists',
      icon: ListMusic,
      label: t('nav.allPlaylists'),
      hint: '/playlists',
      group: libraryGroup,
      keywords: [t('nav.playlists'), '/playlists'],
      onSelect: go('/playlists'),
    },
    {
      value: 'action:new-playlist',
      icon: Plus,
      label: t('nav.newPlaylist'),
      group: libraryGroup,
      keywords: [t('nav.playlists')],
      onSelect: go('/playlists?new=1'),
    },
  ]

  const manageActions = isManager ? navEntries([...sectionPages(TRACKS_SECTION), ...sectionPages(TOOLS_SECTION)], manageGroup) : []
  const adminActions = isAdmin ? navEntries(sectionPages(ADMIN_SECTION), adminGroup) : []
  const settingsActions = navEntries(
    [...SETTINGS_TABS, ...FOOTER_NAV.filter((item) => item.to !== '/settings')],
    settingsGroup,
  )

  const matchedActions = trimmed
    ? [...playbackActions, ...libraryActions, ...manageActions, ...adminActions, ...settingsActions]
        .filter((entry) => matchesText(trimmed, entry.label, entry.keywords))
        .slice(0, MAX_MATCHED_ACTIONS)
    : []

  const matchedPlaylists =
    trimmed && playlists
      ? playlists.filter((pl) => matchesText(trimmed, pl.name, [t('nav.playlists')])).slice(0, MAX_MATCHED_PLAYLISTS)
      : []

  const searching = Boolean(trimmed) && (activeQuery !== trimmed || search.isFetching)
  const media = activeQuery ? search.data : undefined
  const tracks = media?.tracks ?? []
  const albums = media?.albums ?? []
  const artists = media?.artists ?? []

  return (
    <Command shouldFilter={false} loop className="bg-transparent">
      <div data-slot="command-input-wrapper" className="flex h-12 items-center gap-2.5 border-b px-3.5">
        <Search className="size-4 shrink-0 text-muted-foreground" strokeWidth={2} aria-hidden />
        <CommandPrimitive.Input
          data-slot="command-input"
          autoComplete="off"
          autoCorrect="off"
          autoCapitalize="none"
          spellCheck={false}
          value={ime.value}
          placeholder={t('quickActions.placeholder')}
          onInput={(event) => ime.change(event.currentTarget.value, event.nativeEvent)}
          {...ime.handlers}
          onKeyDown={(event) => {
            // Confirming or cancelling an IME reading must not select a row or close the dialog.
            if (isImeKeyEvent(event.nativeEvent)) {
              event.stopPropagation()
              return
            }
            if (event.key === 'Escape' && query) {
              event.preventDefault()
              event.stopPropagation()
              setQuery('')
            }
          }}
          className="flex h-full w-full min-w-0 bg-transparent text-base outline-hidden placeholder:text-muted-foreground md:text-sm"
        />
        {searching ? <Spinner className="size-4 shrink-0 text-muted-foreground" /> : null}
        {ime.value ? (
          <button
            type="button"
            aria-label={t('actions.clear')}
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => setQuery('')}
            className="-mr-1 grid size-8 shrink-0 place-items-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground"
          >
            <X className="size-4" strokeWidth={2} aria-hidden />
          </button>
        ) : null}
      </div>

      <CommandList className="max-h-none flex-1 overscroll-contain px-1.5 py-1.5 md:max-h-[min(30rem,66dvh)]">
        {!trimmed ? (
          <>
            {recent.length > 0 ? (
              <CommandGroup
                heading={
                  <span className="flex items-center justify-between">
                    <span>{t('quickActions.recent')}</span>
                    <button
                      type="button"
                      onMouseDown={(event) => event.preventDefault()}
                      onClick={clearRecentSearches}
                      className="rounded px-1.5 py-0.5 text-[11px] font-medium text-primary hover:bg-primary/10"
                    >
                      {t('actions.clear')}
                    </button>
                  </span>
                }
              >
                {recent.slice(0, MAX_RECENT_IDLE).map((term) => (
                  <CommandItem
                    key={term}
                    value={`recent:${term}`}
                    onSelect={() => setQuery(term)}
                    className="min-h-11 gap-3 rounded-lg px-3 md:min-h-10"
                  >
                    <Clock className="size-4 text-muted-foreground" strokeWidth={1.75} aria-hidden />
                    <span className="min-w-0 flex-1 truncate">{term}</span>
                    <button
                      type="button"
                      aria-label={t('quickActions.removeRecent', { term })}
                      onMouseDown={(event) => event.preventDefault()}
                      onClick={(event) => {
                        event.stopPropagation()
                        removeRecentSearch(term)
                      }}
                      className="-mr-1.5 grid size-8 shrink-0 place-items-center rounded-full text-muted-foreground hover:bg-background/80 hover:text-foreground"
                    >
                      <X className="size-3.5" aria-hidden />
                    </button>
                  </CommandItem>
                ))}
              </CommandGroup>
            ) : null}

            {playbackActions.length > 0 ? (
              <CommandGroup heading={playbackGroup}>
                {playbackActions.map((entry) => (
                  <ActionRow key={entry.value} {...entry} />
                ))}
              </CommandGroup>
            ) : null}

            <CommandGroup heading={libraryGroup}>
              {libraryActions.map((entry) => (
                <ActionRow key={entry.value} {...entry} />
              ))}
            </CommandGroup>

            {playlists?.length ? (
              <CommandGroup heading={t('nav.playlists')}>
                {playlists.map((playlist) => (
                  <ActionRow
                    key={playlist.id}
                    value={`playlist:${playlist.id}`}
                    icon={ListMusic}
                    label={playlist.name}
                    onSelect={go(`/playlists/${encodeURIComponent(playlist.id)}`)}
                  />
                ))}
              </CommandGroup>
            ) : null}

            {manageActions.length > 0 ? (
              <CommandGroup heading={manageGroup}>
                {manageActions.map((entry) => (
                  <ActionRow key={entry.value} {...entry} />
                ))}
              </CommandGroup>
            ) : null}

            {adminActions.length > 0 ? (
              <CommandGroup heading={adminGroup}>
                {adminActions.map((entry) => (
                  <ActionRow key={entry.value} {...entry} />
                ))}
              </CommandGroup>
            ) : null}

            <CommandGroup heading={settingsGroup}>
              {settingsActions.map((entry) => (
                <ActionRow key={entry.value} {...entry} />
              ))}
            </CommandGroup>
          </>
        ) : (
          <>
            {tracks.length > 0 ? (
              <CommandGroup heading={t('quickActions.groups.songs')}>
                {tracks.map((track, idx) => (
                  <CommandItem
                    key={track.id}
                    value={`song:${track.id}`}
                    onSelect={run(() => {
                      addRecentSearch(trimmed)
                      player().playTracks(tracks, idx)
                    })}
                    className="min-h-12 gap-3 rounded-lg px-2.5 py-1.5 md:min-h-11"
                  >
                    <CoverArt coverArt={track.coverArt} size={36} className="shrink-0" />
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-sm font-medium">{track.title}</span>
                      <span className="block truncate text-xs text-muted-foreground">
                        {[track.artist, track.album].filter(Boolean).join(' · ')}
                      </span>
                    </span>
                    <span className="shrink-0 text-xs text-muted-foreground tabular-nums">
                      {formatDuration(track.duration)}
                    </span>
                  </CommandItem>
                ))}
              </CommandGroup>
            ) : null}

            {albums.length > 0 ? (
              <CommandGroup heading={t('quickActions.groups.albums')}>
                {albums.map((album) => (
                  <CommandItem
                    key={album.id}
                    value={`album:${album.id}`}
                    onSelect={run(() => {
                      addRecentSearch(trimmed)
                      void navigate(`/albums/${encodeURIComponent(album.id)}`)
                    })}
                    className="min-h-12 gap-3 rounded-lg px-2.5 py-1.5 md:min-h-11"
                  >
                    <CoverArt coverArt={album.coverArt} size={36} className="shrink-0" />
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-sm font-medium">{album.name}</span>
                      <span className="block truncate text-xs text-muted-foreground">
                        {[album.artist, album.year > 0 ? String(album.year) : ''].filter(Boolean).join(' · ')}
                      </span>
                    </span>
                  </CommandItem>
                ))}
              </CommandGroup>
            ) : null}

            {artists.length > 0 ? (
              <CommandGroup heading={t('quickActions.groups.artists')}>
                {artists.map((artist) => (
                  <CommandItem
                    key={artist.id}
                    value={`artist:${artist.id}`}
                    onSelect={run(() => {
                      addRecentSearch(trimmed)
                      void navigate(`/artists/${encodeURIComponent(artist.id)}`)
                    })}
                    className="min-h-12 gap-3 rounded-lg px-2.5 py-1.5 md:min-h-11"
                  >
                    <CoverArt coverArt={artist.coverArt} size={36} shape="circle" className="shrink-0" />
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-sm font-medium">{artist.name}</span>
                      <span className="block truncate text-xs text-muted-foreground">
                        {t('count.songs', { count: artist.songCount })}
                      </span>
                    </span>
                  </CommandItem>
                ))}
              </CommandGroup>
            ) : null}

            {matchedPlaylists.length > 0 ? (
              <CommandGroup heading={t('nav.playlists')}>
                {matchedPlaylists.map((playlist) => (
                  <CommandItem
                    key={playlist.id}
                    value={`playlist:${playlist.id}`}
                    onSelect={go(`/playlists/${encodeURIComponent(playlist.id)}`)}
                    className="min-h-12 gap-3 rounded-lg px-2.5 py-1.5 md:min-h-11"
                  >
                    <CoverArt coverArt={playlist.coverArt} size={36} icon={ListMusic} className="shrink-0" />
                    <span className="min-w-0 flex-1">
                      <span className="block truncate text-sm font-medium">{playlist.name}</span>
                      <span className="block truncate text-xs text-muted-foreground">
                        {t('count.songs', { count: playlist.songCount })}
                      </span>
                    </span>
                  </CommandItem>
                ))}
              </CommandGroup>
            ) : null}

            {matchedActions.length > 0 ? (
              <CommandGroup heading={t('quickActions.groups.pages')}>
                {matchedActions.map((entry) => (
                  <ActionRow key={entry.value} {...entry} />
                ))}
              </CommandGroup>
            ) : null}

            {!searching &&
            tracks.length === 0 &&
            albums.length === 0 &&
            artists.length === 0 &&
            matchedPlaylists.length === 0 &&
            matchedActions.length === 0 ? (
              <p className="py-6 text-center text-sm text-muted-foreground">{t('quickActions.empty')}</p>
            ) : null}

            <CommandGroup>
              <CommandItem
                value={`search:all:${trimmed}`}
                onSelect={run(() => {
                  addRecentSearch(trimmed)
                  void navigate(`/search?q=${encodeURIComponent(trimmed)}`)
                })}
                className="min-h-11 gap-3 rounded-lg px-3 text-primary data-[selected=true]:text-primary md:min-h-10"
              >
                <Search className="size-4 text-primary!" strokeWidth={1.75} aria-hidden />
                <span className="min-w-0 flex-1 truncate font-medium">
                  {t('quickActions.seeAllResults', { query: trimmed })}
                </span>
                <ArrowRight className="size-4 text-primary!" strokeWidth={1.75} aria-hidden />
              </CommandItem>
            </CommandGroup>
          </>
        )}
      </CommandList>
    </Command>
  )
}

/** Whether the Spotlight palette is open: the app chrome and Home may render triggers for it. */
const usePalette = create<{ open: boolean }>()(() => ({ open: false }))

function setOpen(open: boolean) {
  usePalette.setState({ open })
}

function close() {
  setOpen(false)
}

/**
 * Spotlight search trigger: a magnifier icon in the app chrome that opens `<QuickActionsHost/>`.
 * Styles with the `field` entry show `<QuickActionsField/>` instead.
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
      <Search strokeWidth={1.75} />
    </Button>
  )
}

/**
 * Spotlight trigger styled as a pill search field: used in chrome (`openai`, `google-md`) and in
 * the top-right corner of Home when the active style uses icon chrome (`apple`, `anthropic`).
 */
export function QuickActionsField({
  appearance = 'chrome',
  className,
}: {
  appearance?: 'chrome' | 'page'
  className?: string
}) {
  const { t } = useTranslation()
  const open = usePalette((s) => s.open)
  const label = t('quickActions.label')
  const pageSurface = appearance === 'page'

  return (
    <button
      type="button"
      aria-label={label}
      aria-keyshortcuts={SHORTCUT_KEYS}
      aria-haspopup="dialog"
      title={`${label} (${SHORTCUT_LABEL})`}
      data-state={open ? 'open' : 'closed'}
      onClick={() => setOpen(true)}
      className={cn(
        'flex h-9 min-w-0 items-center gap-2 rounded-full border px-3 text-left text-[13px] outline-none transition-colors',
        pageSurface
          ? 'border-border/70 bg-secondary/70 text-muted-foreground hover:bg-secondary hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50 data-[state=open]:bg-secondary'
          : 'border-sidebar-border bg-background/60 text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-foreground focus-visible:ring-2 focus-visible:ring-sidebar-ring data-[state=open]:bg-sidebar-accent',
        'group-data-[collapsible=icon]:size-9 group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:px-0',
        className,
      )}
    >
      <Search className="size-4 shrink-0" strokeWidth={1.75} aria-hidden />
      <span className="min-w-0 flex-1 truncate group-data-[collapsible=icon]:hidden">
        {t('quickActions.fieldPlaceholder')}
      </span>
      <kbd
        className={cn(
          'shrink-0 rounded-md border px-1.5 font-sans text-[11px] leading-5 font-medium group-data-[collapsible=icon]:hidden',
          pageSurface ? 'border-border/80 bg-background/70 text-muted-foreground' : 'border-sidebar-border',
        )}
      >
        {SHORTCUT_LABEL}
      </kbd>
    </button>
  )
}

/**
 * The Spotlight search & quick-actions palette, mounted once in the app shell: `<QuickActions/>`,
 * `<QuickActionsField/>`, and Ctrl/⌘K anywhere open a centered dialog on tablets/desktops or a
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
          <div className="flex min-h-0 flex-1 flex-col pt-2">{open ? <SpotlightPalette onClose={close} /> : null}</div>
        </DrawerContent>
      </Drawer>
    )
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent
        showCloseButton={false}
        className="top-[10vh] translate-y-0 gap-0 overflow-hidden rounded-2xl p-0 sm:max-w-xl"
      >
        <DialogTitle className="sr-only">{label}</DialogTitle>
        <DialogDescription className="sr-only">{t('quickActions.description')}</DialogDescription>
        <SpotlightPalette onClose={close} />
      </DialogContent>
    </Dialog>
  )
}
