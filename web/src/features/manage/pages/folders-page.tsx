import { useMutation, useQueries, useQuery } from '@tanstack/react-query'
import {
  AlertCircle,
  Captions,
  CheckSquare,
  FolderOpen,
  FolderSync,
  HardDrive,
  Image as ImageIcon,
  LibraryBig,
  Lock,
  MoreHorizontal,
  PencilLine,
  RefreshCw,
  Tags,
  Trash2,
  Upload,
  WrapText,
  X,
} from 'lucide-react'
import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useSearchParams } from 'react-router'
import { toast } from 'sonner'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { PageLoader } from '@/components/spinner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useCurrentTrack, usePlayer } from '@/features/player/store'
import { useLibraries } from '@/hooks/use-libraries'
import { useIsMobile } from '@/hooks/use-media-query'
import { TOOLS_SECTION } from '@/layouts/nav'
import { SectionTabs } from '@/layouts/section-tabs'
import { api } from '@/lib/api/endpoints'
import type { FileEntry, FolderListing } from '@/lib/api/types'
import { toastError } from '@/lib/batch'
import { formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useUI } from '@/stores/ui'

import { BatchDialogs, type BatchDialogState } from '../components/batch-dialogs'
import { FolderBreadcrumb } from '../components/folder-breadcrumb'
import { FolderDocRow, FolderImageTile, FolderTile, FolderTrackRow } from '../components/folder-file-rows'
import { FolderRail, FolderRailToggle, FolderTreeButton } from '../components/folder-navigator'
import { FolderPreviewDialog } from '../components/folder-preview-dialog'
import {
  ancestorDirs,
  buildFolderNavigatorRows,
  dirWithin,
  groupFolderFiles,
  joinDir,
  splitDir,
  toPreviewItem,
  type FolderPreviewItem,
} from '../lib/folder-explorer'
import { manageKeys } from '../queries'

const RAIL_STORAGE_KEY = 'rainy:folders-rail-hidden'
const FULL_NAMES_STORAGE_KEY = 'rainy:folders-full-names'
/** 38rem = 608px: wide enough for a 13–15.5rem folder tree + comfortable file rows. */
const WIDE_PANEL_PX = 608
const INITIAL_TRACK_LIMIT = 160
const INITIAL_IMAGE_LIMIT = 24
const EMPTY_STRING_SET: ReadonlySet<string> = new Set()

function readStoredFlag(key: string): boolean {
  try {
    return window.localStorage.getItem(key) === '1'
  } catch {
    return false
  }
}

function writeStoredFlag(key: string, enabled: boolean): void {
  try {
    if (enabled) window.localStorage.setItem(key, '1')
    else window.localStorage.removeItem(key)
  } catch {
    // Ignore storage quota or privacy-mode failures.
  }
}

export default function FoldersPage() {
  const { t } = useTranslation('manage')
  const isMobile = useIsMobile()
  const [searchParams, setSearchParams] = useSearchParams()
  const { libraries, limited } = useLibraries()
  const openTagEditor = useUI((s) => s.openTagEditor)
  const currentTrack = useCurrentTrack()
  const isPlaying = usePlayer((s) => s.isPlaying)
  const playTracks = usePlayer((s) => s.playTracks)
  const togglePlay = usePlayer((s) => s.togglePlay)

  const urlLib = Number.parseInt(searchParams.get('libraryId') ?? '', 10)
  const libraryId = Number.isFinite(urlLib) && urlLib > 0 ? urlLib : (libraries[0]?.id ?? 1)
  const library = libraries.find((l) => l.id === libraryId)
  const rootLabel = t('folders.root')
  const rawDir = searchParams.get('dir') ?? ''
  const dir = joinDir(splitDir(rawDir))
  const folderKey = `${libraryId}:${dir}`

  const [railCollapsed, setRailCollapsed] = useState(() => readStoredFlag(RAIL_STORAGE_KEY))
  const [fullNames, setFullNames] = useState(() => readStoredFlag(FULL_NAMES_STORAGE_KEY))
  const [showAttachedLyrics, setShowAttachedLyrics] = useState(false)
  const [toggledDirs, setToggledDirs] = useState<ReadonlyMap<string, boolean>>(() => new Map())
  const [selectionState, setSelectionState] = useState<{
    key: string
    ids: ReadonlySet<string>
    anchorId: string | null
  }>({ key: folderKey, ids: EMPTY_STRING_SET, anchorId: null })
  const [trackLimitState, setTrackLimitState] = useState<{ key: string; limit: number }>({
    key: folderKey,
    limit: INITIAL_TRACK_LIMIT,
  })
  const [allImagesKey, setAllImagesKey] = useState<string | null>(null)
  const [previewState, setPreviewState] = useState<{
    key: string
    items: readonly FolderPreviewItem[]
    index: number
  } | null>(null)
  const [playingFolderPath, setPlayingFolderPath] = useState<string | null>(null)
  const [dialog, setDialog] = useState<BatchDialogState | null>(null)

  const panelRef = useRef<HTMLDivElement>(null)
  const [wide, setWide] = useState(() => typeof window !== 'undefined' && window.innerWidth >= 768)

  useEffect(() => {
    const el = panelRef.current
    if (!el || typeof ResizeObserver === 'undefined') return
    const ro = new ResizeObserver((entries) => {
      const entry = entries[0]
      if (entry) setWide(entry.contentRect.width >= WIDE_PANEL_PX)
    })
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  const playingDir =
    currentTrack && currentTrack.libraryId === libraryId ? joinDir(splitDir(currentTrack.dir)) : null

  const expandedDirs = useMemo(() => {
    const next = new Set<string>(ancestorDirs(dir))
    if (playingDir !== null) {
      for (const d of ancestorDirs(playingDir)) next.add(d)
    }
    const prefix = `${libraryId}:`
    for (const [k, expanded] of toggledDirs) {
      if (!k.startsWith(prefix)) continue
      const target = k.slice(prefix.length)
      if (expanded) next.add(target)
      else next.delete(target)
    }
    next.add('')
    return next
  }, [dir, libraryId, playingDir, toggledDirs])

  const navigate = (next: { libraryId?: number; dir?: string }) => {
    const targetLib = next.libraryId ?? libraryId
    const targetDir = next.dir !== undefined ? joinDir(splitDir(next.dir)) : dir
    setSelectionState({ key: `${targetLib}:${targetDir}`, ids: EMPTY_STRING_SET, anchorId: null })
    setPreviewState(null)
    if (next.dir !== undefined) {
      setToggledDirs((prev) => {
        const updated = new Map(prev)
        for (const anc of ancestorDirs(targetDir)) {
          updated.delete(`${targetLib}:${anc}`)
        }
        return updated
      })
    }
    setSearchParams((prev) => {
      const sp = new URLSearchParams(prev)
      if (next.libraryId !== undefined) sp.set('libraryId', String(next.libraryId))
      if (next.dir !== undefined) {
        if (targetDir) sp.set('dir', targetDir)
        else sp.delete('dir')
      }
      return sp
    })
  }

  const toggleExpand = (targetDir: string) => {
    if (!targetDir) return
    const currentlyExpanded = expandedDirs.has(targetDir)
    setToggledDirs((prev) => {
      const next = new Map(prev)
      next.set(`${libraryId}:${targetDir}`, !currentlyExpanded)
      return next
    })
  }

  const toggleRail = () => {
    setRailCollapsed((prev) => {
      const next = !prev
      writeStoredFlag(RAIL_STORAGE_KEY, next)
      return next
    })
  }

  const toggleFullNames = () => {
    setFullNames((prev) => {
      const next = !prev
      writeStoredFlag(FULL_NAMES_STORAGE_KEY, next)
      return next
    })
  }

  // Query the current folder and every expanded tree node (deduplicated, cached for 30s).
  const queriedDirs = useMemo(() => {
    const all = new Set<string>(expandedDirs)
    all.add('')
    all.add(dir)
    return Array.from(all).sort()
  }, [dir, expandedDirs])

  const dirQueries = useQueries({
    queries: queriedDirs.map((d) => ({
      queryKey: manageKeys.folders(libraryId, d),
      queryFn: ({ signal }: { signal: AbortSignal }) => api.manage.folders.list({ libraryId, dir: d }, { signal }),
      staleTime: 30_000,
    })),
  })

  const { listingsByDir, loadingDirs } = useMemo(() => {
    const map = new Map<string, FolderListing>()
    const loading = new Set<string>()
    queriedDirs.forEach((d, idx) => {
      const q = dirQueries[idx]
      if (q?.data) map.set(d, q.data)
      if (q?.isPending || q?.isFetching) loading.add(d)
    })
    return { listingsByDir: map, loadingDirs: loading }
  }, [dirQueries, queriedDirs])

  const listing = useQuery({
    queryKey: manageKeys.folders(libraryId, dir),
    queryFn: ({ signal }) => api.manage.folders.list({ libraryId, dir }, { signal }),
    staleTime: 30_000,
  })
  const data = listing.data

  const navigatorRows = useMemo(
    () =>
      buildFolderNavigatorRows({
        rootName: rootLabel,
        listings: listingsByDir,
        loadingDirs,
        expandedDirs,
      }),
    [expandedDirs, rootLabel, listingsByDir, loadingDirs],
  )

  const rootListing = listingsByDir.get('')
  const hasTree = (rootListing?.folders.length ?? 0) > 0 || dir !== '' || (data?.folders.length ?? 0) > 0
  const showRail = wide && !railCollapsed && hasTree
  const clampNames = isMobile ? !fullNames : true

  const sections = useMemo(
    () => groupFolderFiles(data?.files ?? [], showAttachedLyrics),
    [data?.files, showAttachedLyrics],
  )
  // Section headings only separate kinds of content; a folder with one kind needs none.
  const sectionCount = [
    data?.folders.length ?? 0,
    sections.audio.length,
    sections.images.length,
    sections.documents.length,
    sections.others.length,
  ].filter((n) => n > 0).length

  const selected = selectionState.key === folderKey ? selectionState.ids : EMPTY_STRING_SET
  const trackLimit = trackLimitState.key === folderKey ? trackLimitState.limit : INITIAL_TRACK_LIMIT
  const showAllImages = allImagesKey === folderKey

  const indexedTrackIds = useMemo(() => sections.playableTracks.map((tr) => tr.id), [sections.playableTracks])
  const selectedIds = useMemo(
    () => indexedTrackIds.filter((id) => selected.has(id)),
    [indexedTrackIds, selected],
  )
  const targetIds = selectedIds.length > 0 ? selectedIds : indexedTrackIds

  const toggleSelectTrack = (id: string, shiftKey: boolean) => {
    setSelectionState((prev) => {
      const currentIds = prev.key === folderKey ? prev.ids : EMPTY_STRING_SET
      const anchor = prev.key === folderKey ? prev.anchorId : null
      const next = new Set(currentIds)

      if (shiftKey && anchor && anchor !== id) {
        const start = indexedTrackIds.indexOf(anchor)
        const end = indexedTrackIds.indexOf(id)
        if (start >= 0 && end >= 0) {
          const [lo, hi] = start < end ? [start, end] : [end, start]
          const shouldSelect = !currentIds.has(id)
          for (let i = lo; i <= hi; i++) {
            if (shouldSelect) next.add(indexedTrackIds[i])
            else next.delete(indexedTrackIds[i])
          }
          return { key: folderKey, ids: next, anchorId: id }
        }
      }

      if (next.has(id)) next.delete(id)
      else next.add(id)
      return { key: folderKey, ids: next, anchorId: id }
    })
  }

  const toggleSelectAll = () => {
    if (indexedTrackIds.length === 0) return
    if (selectedIds.length === indexedTrackIds.length) {
      setSelectionState({ key: folderKey, ids: EMPTY_STRING_SET, anchorId: null })
    } else {
      setSelectionState({ key: folderKey, ids: new Set(indexedTrackIds), anchorId: null })
    }
  }

  const clearSelection = () => {
    setSelectionState({ key: folderKey, ids: EMPTY_STRING_SET, anchorId: null })
  }

  const playFolder = async (targetDir: string) => {
    if (playingFolderPath !== null) return
    setPlayingFolderPath(targetDir)
    try {
      const res = await api.tracks.list({
        libraryId,
        ...(targetDir ? { dirPrefix: targetDir } : {}),
        sort: 'path',
        order: 'asc',
        limit: 1000,
      })
      if (res.items.length > 0) {
        playTracks(res.items, 0)
      }
    } catch (error) {
      toastError(error)
    } finally {
      setPlayingFolderPath(null)
    }
  }

  const openImagePreview = (idx: number) => {
    setPreviewState({ key: folderKey, items: sections.imagePreviews, index: idx })
  }

  const openDocPreview = (file: FileEntry) => {
    const idx = sections.documentPreviews.findIndex((p) => p.path === file.path)
    if (idx >= 0) {
      setPreviewState({ key: folderKey, items: sections.documentPreviews, index: idx })
      return
    }
    const single = toPreviewItem(file)
    if (single) {
      setPreviewState({ key: folderKey, items: [single], index: 0 })
    }
  }

  const rescan = useMutation({
    mutationFn: () => api.manage.folders.rescan({ libraryId, dir }),
    onSuccess: () => toast.success(t('rescan.started', { count: 1 })),
    onError: toastError,
  })

  const writable = data ? data.writable : (library?.writable ?? true)
  const managerLink = `/manage?${new URLSearchParams({
    ...(dir ? { dirPrefix: dir } : {}),
    ...(limited ? {} : { libraryId: String(libraryId) }),
  })}`
  const uploadLink = `/manage/upload?${new URLSearchParams({
    libraryId: String(libraryId),
    ...(dir ? { dir } : {}),
  })}`

  const visibleTracks = sections.audio.slice(0, trackLimit)
  const visibleImages = showAllImages ? sections.images : sections.images.slice(0, INITIAL_IMAGE_LIMIT)
  const activePreview = previewState?.key === folderKey ? previewState : null

  const allSelected = indexedTrackIds.length > 0 && selectedIds.length === indexedTrackIds.length
  const someSelected = selectedIds.length > 0 && !allSelected

  return (
    <Page centered>
      <PageHeader
        title={t('folders.title')}
        back={isMobile ? '/manage' : undefined}
      >
        <SectionTabs section={TOOLS_SECTION} />
      </PageHeader>

      <div ref={panelRef} className="space-y-4">
        {/* Top bar: path breadcrumb (with optional multi-library select) + folder actions */}
        <div className="flex items-center justify-between gap-2 border-b pb-2.5">
          <div className="flex min-w-0 flex-1 items-center gap-1.5">
            {wide && railCollapsed && hasTree ? (
              <FolderRailToggle collapsed onToggle={toggleRail} />
            ) : null}
            {(!wide || railCollapsed) && hasTree ? (
              <FolderTreeButton
                rows={navigatorRows}
                currentDir={dir}
                playingDir={playingDir}
                clampNames={clampNames}
                onSelect={(nextDir) => navigate({ dir: nextDir })}
                onToggleExpand={toggleExpand}
              />
            ) : null}
            {!limited && libraries.length > 1 ? (
              <Select value={String(libraryId)} onValueChange={(v) => navigate({ libraryId: Number(v), dir: '' })}>
                <SelectTrigger className="h-8 w-auto max-w-36 shrink-0 gap-1.5 px-2.5 text-xs">
                  <HardDrive className="size-3.5 shrink-0" />
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {libraries.map((lib) => (
                    <SelectItem key={lib.id} value={String(lib.id)}>
                      {lib.name || `#${lib.id}`}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            ) : null}
            <FolderBreadcrumb
              dir={dir}
              rootName={rootLabel}
              onNavigate={(nextDir) => navigate({ dir: nextDir })}
            />
            {!writable ? <ReadonlyBadge /> : null}
            {library && !library.exists ? (
              <Badge variant="outline" className="shrink-0 text-destructive">
                {t('folders.libraryUnavailable')}
              </Badge>
            ) : null}
          </div>

          <div className="flex shrink-0 items-center gap-1">
            {isMobile ? (
              <Button
                type="button"
                variant="ghost"
                size="icon"
                aria-pressed={fullNames}
                aria-label={t('folders.fullNames')}
                title={fullNames ? t('folders.fullNamesOnHint') : t('folders.fullNamesOffHint')}
                className={cn('size-11', fullNames ? 'bg-primary/12 text-primary' : 'text-muted-foreground')}
                onClick={toggleFullNames}
              >
                <WrapText className="size-4" />
              </Button>
            ) : null}

            {writable ? (
              <Button
                asChild
                variant="ghost"
                size={isMobile ? 'icon' : 'sm'}
                className={cn(isMobile ? 'size-11 text-muted-foreground' : 'h-8 gap-1.5 px-2.5 text-xs')}
              >
                <Link to={uploadLink} aria-label={t('folders.uploadHere')} title={t('folders.uploadHere')}>
                  <Upload className="size-4" />
                  {isMobile ? null : <span>{t('folders.uploadHere')}</span>}
                </Link>
              </Button>
            ) : null}

            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  type="button"
                  variant="outline"
                  size={isMobile ? 'icon' : 'sm'}
                  className={cn(isMobile ? 'size-11' : 'h-8 gap-1.5 px-2.5 text-xs')}
                  aria-label={t('folders.actions')}
                >
                  {rescan.isPending ? <RefreshCw className="size-4 animate-spin" /> : <MoreHorizontal className="size-4" />}
                  {isMobile ? null : <span>{t('folders.actions')}</span>}
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="min-w-56 rounded-xl">
                <DropdownMenuItem onSelect={() => rescan.mutate()} disabled={rescan.isPending}>
                  <FolderSync />
                  {t('folders.rescan')}
                </DropdownMenuItem>
                <DropdownMenuItem asChild>
                  <Link to={managerLink}>
                    <LibraryBig />
                    {t('folders.openInManager')}
                  </Link>
                </DropdownMenuItem>
                {indexedTrackIds.length > 0 ? (
                  <>
                    <DropdownMenuSeparator />
                    {!allSelected ? (
                      <DropdownMenuItem onSelect={toggleSelectAll}>
                        <CheckSquare />
                        {t('folders.selectAllInFolder')}
                      </DropdownMenuItem>
                    ) : null}
                    <DropdownMenuItem onSelect={() => openTagEditor(targetIds)}>
                      <Tags />
                      {selectedIds.length > 0
                        ? t('folders.editSelected', { count: selectedIds.length })
                        : t('folders.editAll', { count: indexedTrackIds.length })}
                    </DropdownMenuItem>
                    {writable ? (
                      <>
                        <DropdownMenuItem onSelect={() => setDialog({ kind: 'cover', trackIds: targetIds })}>
                          <ImageIcon />
                          {t('actions.cover')}
                        </DropdownMenuItem>
                        <DropdownMenuItem onSelect={() => setDialog({ kind: 'rename', trackIds: targetIds })}>
                          <PencilLine />
                          {t('actions.rename')}
                        </DropdownMenuItem>
                      </>
                    ) : null}
                    {writable && selectedIds.length > 0 ? (
                      <DropdownMenuItem
                        variant="destructive"
                        onSelect={() => setDialog({ kind: 'delete', trackIds: selectedIds })}
                      >
                        <Trash2 />
                        {t('actions.delete')}
                      </DropdownMenuItem>
                    ) : null}
                  </>
                ) : null}
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </div>

        {/* Explorer body: responsive split rail + folder contents */}
        <div
          className={cn(
            showRail && 'grid grid-cols-[minmax(12.5rem,15.5rem)_minmax(0,1fr)] items-start gap-5',
          )}
        >
          {showRail ? (
            <FolderRail
              rows={navigatorRows}
              currentDir={dir}
              playingDir={playingDir}
              clampNames={clampNames}
              onSelect={(nextDir) => navigate({ dir: nextDir })}
              onToggleExpand={toggleExpand}
              onCollapse={toggleRail}
            />
          ) : null}

          <div className="min-w-0 space-y-4">
            {/* Contextual batch toolbar shown only when tracks are selected */}
            {selectedIds.length > 0 ? (
              <div className="flex flex-wrap items-center justify-between gap-2 rounded-lg bg-primary/8 px-2.5 py-1.5">
                <div className="flex items-center gap-2 text-xs font-medium">
                  <Checkbox
                    checked={allSelected ? true : someSelected ? 'indeterminate' : false}
                    onCheckedChange={toggleSelectAll}
                    aria-label={t('folders.selectAllInFolder')}
                  />
                  <span className="tnum">
                    {t('selection.count', {
                      count: selectedIds.length,
                      formatted: formatNumber(selectedIds.length),
                    })}
                  </span>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="size-7 text-muted-foreground hover:text-foreground"
                    aria-label={t('selection.deselectAll')}
                    title={t('selection.deselectAll')}
                    onClick={clearSelection}
                  >
                    <X className="size-3.5" />
                  </Button>
                </div>

                <div className="flex flex-wrap items-center gap-1.5">
                  <Button
                    type="button"
                    size="sm"
                    variant="secondary"
                    className="h-8 gap-1.5 px-2.5 text-xs"
                    onClick={() => openTagEditor(selectedIds)}
                  >
                    <Tags className="size-3.5" />
                    <span>{t('actions.editTags')}</span>
                  </Button>
                  {writable ? (
                    <>
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        className="h-8 gap-1.5 px-2.5 text-xs"
                        onClick={() => setDialog({ kind: 'rename', trackIds: selectedIds })}
                      >
                        <PencilLine className="size-3.5" />
                        <span>{t('actions.renameShort')}</span>
                      </Button>
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        className="h-8 gap-1.5 px-2.5 text-xs text-destructive hover:text-destructive"
                        onClick={() => setDialog({ kind: 'delete', trackIds: selectedIds })}
                      >
                        <Trash2 className="size-3.5" />
                        <span>{t('actions.delete')}</span>
                      </Button>
                    </>
                  ) : null}
                </div>
              </div>
            ) : null}

            {/* Folder contents */}
            {listing.isPending ? (
              <PageLoader className="min-h-64" />
            ) : listing.isError ? (
              <ErrorState
                error={listing.error}
                onRetry={() => void listing.refetch()}
                retrying={listing.isFetching}
              />
            ) : !data || (data.folders.length === 0 && data.files.length === 0) ? (
              <EmptyState
                icon={FolderOpen}
                title={t('folders.empty')}
                action={
                  writable ? (
                    <Button asChild variant="outline">
                      <Link to={uploadLink}>
                        <Upload />
                        {t('folders.uploadHere')}
                      </Link>
                    </Button>
                  ) : undefined
                }
              />
            ) : (
              <div className="space-y-6">
                {/* Subfolders grid */}
                {data.folders.length > 0 ? (
                  <ExplorerSection title={t('folders.sections.folders')} count={data.folders.length} showTitle={sectionCount > 1}>
                    <div className="grid gap-2 [grid-template-columns:repeat(auto-fill,minmax(min(100%,15rem),1fr))]">
                      {data.folders.map((f) => (
                        <FolderTile
                          key={f.path}
                          folder={f}
                          playingHere={playingDir !== null && dirWithin(playingDir, f.path)}
                          playingLoading={playingFolderPath === f.path}
                          clampNames={clampNames}
                          onOpen={() => navigate({ dir: f.path })}
                          onPlay={f.trackCount > 0 ? () => void playFolder(f.path) : undefined}
                        />
                      ))}
                    </div>
                  </ExplorerSection>
                ) : null}

                {/* Audio tracks */}
                {sections.audio.length > 0 ? (
                  <ExplorerSection
                    title={t('folders.sections.tracks')}
                    count={sections.audio.length}
                    showTitle={sectionCount > 1}
                    badge={
                      sections.unindexedAudioCount > 0 ? (
                        <button
                          type="button"
                          onClick={() => rescan.mutate()}
                          disabled={rescan.isPending}
                          className="inline-flex items-center gap-1 rounded-full border border-amber-500/35 bg-amber-500/10 px-2 py-0.5 text-[11px] font-medium text-amber-700 transition-colors hover:bg-amber-500/15 dark:text-amber-300"
                          title={t('folders.rescan')}
                        >
                          <AlertCircle className="size-3" />
                          <span>{t('folders.unindexedBadge', { count: sections.unindexedAudioCount })}</span>
                        </button>
                      ) : undefined
                    }
                    actions={
                      sections.attachedLyricsCount > 0 ? (
                        <Button
                          type="button"
                          variant={showAttachedLyrics ? 'secondary' : 'ghost'}
                          size="sm"
                          className="h-7 gap-1 px-2 text-xs"
                          aria-pressed={showAttachedLyrics}
                          onClick={() => setShowAttachedLyrics((v) => !v)}
                        >
                          <Captions className="size-3.5" />
                          <span>
                            {showAttachedLyrics
                              ? t('folders.hideLyricsFiles')
                              : t('folders.showLyricsFiles', { count: sections.attachedLyricsCount })}
                          </span>
                        </Button>
                      ) : undefined
                    }
                  >
                    <div className="divide-y divide-border/50">
                      {visibleTracks.map((item) => {
                        const track = item.file.track
                        const isCurrent = !!track && currentTrack?.id === track.id
                        return (
                          <FolderTrackRow
                            key={item.file.path}
                            item={item}
                            selected={!!track && selected.has(track.id)}
                            isCurrent={isCurrent}
                            isPlaying={isCurrent && isPlaying}
                            clampNames={clampNames}
                            onToggleSelect={
                              track ? (shiftKey) => toggleSelectTrack(track.id, shiftKey) : undefined
                            }
                            onPlay={
                              track && item.playIndex >= 0
                                ? () => playTracks(sections.playableTracks, item.playIndex)
                                : undefined
                            }
                            onTogglePlay={track ? togglePlay : undefined}
                            onEditTags={track ? () => openTagEditor([track.id]) : undefined}
                            onPreviewSidecar={openDocPreview}
                          />
                        )
                      })}
                    </div>
                    {sections.audio.length > visibleTracks.length ? (
                      <div className="pt-2">
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          className="w-full text-xs sm:w-auto"
                          onClick={() =>
                            setTrackLimitState({
                              key: folderKey,
                              limit: trackLimit + INITIAL_TRACK_LIMIT,
                            })
                          }
                        >
                          {t('folders.showMoreTracks', {
                            shown: visibleTracks.length,
                            total: sections.audio.length,
                          })}
                        </Button>
                      </div>
                    ) : null}
                  </ExplorerSection>
                ) : null}

                {/* Images gallery */}
                {sections.images.length > 0 ? (
                  <ExplorerSection title={t('folders.sections.images')} count={sections.images.length} showTitle={sectionCount > 1}>
                    <div className="grid gap-2.5 [grid-template-columns:repeat(auto-fill,minmax(6.75rem,1fr))] sm:[grid-template-columns:repeat(auto-fill,minmax(7.75rem,1fr))]">
                      {visibleImages.map((img, idx) => (
                        <FolderImageTile
                          key={img.path}
                          libraryId={libraryId}
                          file={img}
                          clampNames={clampNames}
                          onPreview={() => openImagePreview(idx)}
                        />
                      ))}
                    </div>
                    {sections.images.length > visibleImages.length ? (
                      <div className="pt-2">
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          className="w-full text-xs sm:w-auto"
                          onClick={() => setAllImagesKey(folderKey)}
                        >
                          {t('folders.showAllImages', { count: sections.images.length })}
                        </Button>
                      </div>
                    ) : null}
                  </ExplorerSection>
                ) : null}

                {/* Lyrics & text documents */}
                {sections.documents.length > 0 ? (
                  <ExplorerSection title={t('folders.sections.documents')} count={sections.documents.length} showTitle={sectionCount > 1}>
                    <div className="divide-y divide-border/50">
                      {sections.documents.map((doc) => (
                        <FolderDocRow
                          key={doc.path}
                          file={doc}
                          clampNames={clampNames}
                          onPreview={() => openDocPreview(doc)}
                        />
                      ))}
                    </div>
                  </ExplorerSection>
                ) : null}

                {/* Other non-media files */}
                {sections.others.length > 0 ? (
                  <ExplorerSection title={t('folders.sections.otherFiles')} count={sections.others.length} showTitle={sectionCount > 1}>
                    <div className="divide-y divide-border/50">
                      {sections.others.map((other) => (
                        <FolderDocRow key={other.path} file={other} clampNames={clampNames} />
                      ))}
                    </div>
                  </ExplorerSection>
                ) : null}
              </div>
            )}
          </div>
        </div>
      </div>

      <FolderPreviewDialog
        libraryId={libraryId}
        items={activePreview?.items ?? []}
        index={activePreview ? activePreview.index : null}
        onIndexChange={(nextIdx) => {
          if (nextIdx === null || !activePreview) {
            setPreviewState(null)
          } else {
            setPreviewState({ ...activePreview, index: nextIdx })
          }
        }}
      />

      <BatchDialogs dialog={dialog} onClose={() => setDialog(null)} onDone={clearSelection} />
    </Page>
  )
}

function ExplorerSection({
  title,
  count,
  showTitle = true,
  badge,
  actions,
  children,
}: {
  title: string
  count: number
  /** `false` keeps the heading for assistive tech only (the folder holds one kind of content). */
  showTitle?: boolean
  badge?: ReactNode
  actions?: ReactNode
  children: ReactNode
}) {
  const bar = showTitle || badge || actions
  return (
    <section className="space-y-1.5" aria-label={showTitle ? undefined : title}>
      {bar ? (
        <div className="flex min-h-7 flex-wrap items-center justify-between gap-2">
          <div className="flex items-center gap-1.5">
            {showTitle ? (
              <>
                <h2 className="text-xs font-medium tracking-wide text-muted-foreground uppercase">{title}</h2>
                <span className="tnum text-xs text-muted-foreground/75">({count})</span>
              </>
            ) : null}
            {badge}
          </div>
          {actions ? <div className="ml-auto flex flex-wrap items-center gap-1.5">{actions}</div> : null}
        </div>
      ) : null}
      {children}
    </section>
  )
}

function ReadonlyBadge() {
  const { t } = useTranslation('manage')
  return (
    <Badge variant="outline" className="shrink-0 gap-1 text-amber-600 dark:text-amber-400">
      <Lock />
      {t('file.readonly')}
    </Badge>
  )
}
