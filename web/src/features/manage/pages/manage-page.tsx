import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query'
import { useWindowVirtualizer } from '@tanstack/react-virtual'
import {
  FolderSync,
  ImagePlus,
  Languages,
  LibraryBig,
  ListChecks,
  MoreHorizontal,
  PencilLine,
  SearchX,
  RefreshCw,
  Tags,
  Trash2,
  Upload,
  type LucideIcon,
} from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState, type MouseEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useSearchParams } from 'react-router'
import { toast } from 'sonner'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { PageLoader, Spinner } from '@/components/spinner'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { useLibraries } from '@/features/admin/queries'
import { useIsMobile } from '@/hooks/use-media-query'
import { api, type TrackSort } from '@/lib/api/endpoints'
import type { Track } from '@/lib/api/types'
import { formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useUI } from '@/stores/ui'

import { BatchDialogs, type BatchDialogKind, type BatchDialogState } from '../components/batch-dialogs'
import { TracksTabs } from '../components/tracks-tabs'
import { ManageToolbar } from '../components/manage-toolbar'
import { MOBILE_ROW_HEIGHT, MobileTrackRows } from '../components/track-list-mobile'
import { DESKTOP_ROW_HEIGHT, TrackTableHeader, TrackTableRows } from '../components/track-table'
import { toastError } from '../lib/batch'
import { COLUMNS, loadVisibleColumns, saveVisibleColumns, type ColumnId } from '../lib/columns'
import { activeFilterCount, readFilters, toTrackParams, writeFilters, type ManageFilters } from '../lib/filters'
import { fetchAllMatching, trackPageQuery, usePagedRows } from '../lib/paged-tracks'
import { useSelection } from '../lib/selection'
import { useScrollMargin } from '../lib/use-scroll-margin'

/** The tag editor loads every selected track's tags; beyond this, ask to narrow the selection. */
const MAX_EDITOR_TRACKS = 2000

/** Rescans of more distinct folders than this collapse to their top-level folders. */
const MAX_RESCAN_DIRS = 20

type Action = 'edit' | BatchDialogKind | 'rescan'

export default function ManagePage() {
  const { t } = useTranslation('manage')
  const isMobile = useIsMobile()
  const queryClient = useQueryClient()
  const openTagEditor = useUI((s) => s.openTagEditor)
  const [searchParams, setSearchParams] = useSearchParams()
  const filters = useMemo(() => readFilters(searchParams), [searchParams])
  const params = useMemo(() => toTrackParams(filters), [filters])
  const { libraries } = useLibraries()

  const selection = useSelection()
  const known = useRef(new Map<string, { track: Track; index: number }>())
  const [selecting, setSelecting] = useState(false)
  const [dialog, setDialog] = useState<BatchDialogState | null>(null)
  const [busy, setBusy] = useState<Action | null>(null)
  const [visibleColumns, setVisibleColumns] = useState(loadVisibleColumns)
  const columns = useMemo(() => COLUMNS.filter((c) => visibleColumns.has(c.id)), [visibleColumns])

  const clearSelection = selection.clear
  const update = useCallback(
    (patch: Partial<ManageFilters>) => {
      setSearchParams((prev) => writeFilters(prev, patch), { replace: true })
      clearSelection()
    },
    [setSearchParams, clearSelection],
  )

  const albumLabel = useQuery({
    queryKey: ['albums', filters.albumId],
    queryFn: ({ signal }) => api.albums.get(filters.albumId, { signal }),
    enabled: !!filters.albumId,
    select: (a) => a.name,
  }).data
  const artistLabel = useQuery({
    queryKey: ['artists', filters.artistId],
    queryFn: ({ signal }) => api.artists.get(filters.artistId, { signal }),
    enabled: !!filters.artistId,
    select: (a) => a.name,
  }).data

  // ---- data + virtualization
  const first = useQuery({ ...trackPageQuery(params, 0), placeholderData: keepPreviousData })
  const total = first.data?.total ?? 0
  const [listRef, scrollMargin] = useScrollMargin<HTMLDivElement>()
  const rowHeight = isMobile ? MOBILE_ROW_HEIGHT : DESKTOP_ROW_HEIGHT
  const virtualizer = useWindowVirtualizer({
    count: total,
    estimateSize: () => rowHeight,
    overscan: 12,
    scrollMargin,
  })
  const virtualItems = virtualizer.getVirtualItems()
  const { rowAt, ensureRange } = usePagedRows(
    params,
    virtualItems.map((v) => v.index),
  )

  useEffect(() => {
    virtualizer.measure()
  }, [rowHeight, virtualizer])

  // ---- selection
  const count = selection.count(total)
  const remember = (track: Track, index: number) => known.current.set(track.id, { track, index })

  const rangeSelect = async (index: number) => {
    const from = selection.anchor ?? index
    try {
      const rows = await ensureRange(from, index)
      rows.forEach((r) => remember(r.track, r.index))
      selection.add(rows.map((r) => r.track.id))
    } catch (error) {
      toastError(error)
    }
  }

  const onRowClick = (index: number, track: Track, e: MouseEvent) => {
    remember(track, index)
    if (e.shiftKey && selection.anchor !== null) void rangeSelect(index)
    else if (e.metaKey || e.ctrlKey) selection.toggle(track.id, index)
    else selection.only(track.id, index)
  }

  const onToggle = (index: number, track: Track, e: MouseEvent) => {
    e.preventDefault()
    remember(track, index)
    if (e.shiftKey && selection.anchor !== null) void rangeSelect(index)
    else selection.toggle(track.id, index)
  }

  /** Selected tracks in table order (fetches everything for "all matching"). */
  const resolveSelection = async (): Promise<Track[]> => {
    const sel = selection.selection
    if (sel.kind === 'all') {
      const all = await fetchAllMatching(queryClient, params)
      return all.filter((track) => !sel.excluded.has(track.id))
    }
    return [...sel.ids]
      .map((id) => known.current.get(id))
      .filter((v): v is { track: Track; index: number } => v !== undefined)
      .sort((a, b) => a.index - b.index)
      .map((v) => v.track)
  }

  const run = async (action: Action, only?: Track[]) => {
    if (busy) return
    setBusy(action)
    try {
      const tracks = only ?? (count > 0 ? await resolveSelection() : [])
      const ids = tracks.map((track) => track.id)
      if (action === 'rescan') await rescan(tracks)
      else if (ids.length === 0) toast(t('table.selectFirst'))
      else if (action === 'edit' && ids.length > MAX_EDITOR_TRACKS)
        toast.error(t('table.tooManyToEdit', { count: ids.length, max: formatNumber(MAX_EDITOR_TRACKS) }))
      else if (action === 'edit') openTagEditor(ids)
      else setDialog({ kind: action, trackIds: ids })
    } catch (error) {
      toastError(error)
    } finally {
      setBusy(null)
    }
  }

  const rescan = async (tracks: Track[]) => {
    let targets: { libraryId: number; dir: string }[]
    if (tracks.length === 0) {
      targets = [{ libraryId: filters.libraryId ?? libraries[0]?.id ?? 1, dir: filters.dirPrefix }]
    } else {
      const dirs = new Map<string, { libraryId: number; dir: string }>()
      for (const track of tracks) dirs.set(`${track.libraryId}:${track.dir}`, { libraryId: track.libraryId, dir: track.dir })
      targets = [...dirs.values()]
      if (targets.length > MAX_RESCAN_DIRS) {
        const top = new Map<string, { libraryId: number; dir: string }>()
        for (const d of targets) {
          const root = d.dir.split('/')[0] ?? ''
          top.set(`${d.libraryId}:${root}`, { libraryId: d.libraryId, dir: root })
        }
        targets = [...top.values()]
      }
    }
    await Promise.all(targets.map((target) => api.manage.folders.rescan(target)))
    toast.success(t('rescan.started', { count: targets.length }))
  }

  const onDone = (kind: BatchDialogKind) => {
    if (kind === 'delete' || kind === 'rename') selection.clear()
  }

  // Desktop keyboard shortcuts: Esc clears, Ctrl/Cmd+A selects all matching.
  const selectAll = selection.selectAll
  useEffect(() => {
    if (isMobile) return
    const onKey = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement | null
      if (target?.closest('input, textarea, select, [contenteditable="true"], [role="dialog"], [role="menu"]')) return
      if (document.querySelector('[role="dialog"]')) return
      if (e.key === 'Escape') clearSelection()
      else if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'a') {
        e.preventDefault()
        selectAll()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [isMobile, clearSelection, selectAll])

  const sortBy = (sort: TrackSort) =>
    update({ sort, order: filters.sort === sort && filters.order === 'asc' ? 'desc' : 'asc' })

  const headerChecked: boolean | 'indeterminate' =
    total > 0 && count === total ? true : count > 0 ? 'indeterminate' : false

  const filtered = activeFilterCount(filters) > 0 || !!filters.q

  let content
  if (first.isPending) {
    content = <PageLoader />
  } else if (first.isError && !first.data) {
    content = <ErrorState error={first.error} onRetry={() => void first.refetch()} retrying={first.isFetching} />
  } else if (total === 0) {
    content = filtered ? (
      <EmptyState
        icon={SearchX}
        title={t('table.noResults')}
        description={t('table.noResultsDescription')}
        action={
          <Button variant="outline" onClick={() => update({ q: '', albumId: '', artistId: '', genre: '', dirPrefix: '', libraryId: undefined, missing: '' })}>
            {t('filters.clearAll')}
          </Button>
        }
      />
    ) : (
      <EmptyState
        icon={LibraryBig}
        title={t('table.empty')}
        description={t('table.emptyDescription')}
        action={
          <Button asChild>
            <Link to="/manage/upload">
              <Upload />
              {t('upload.title')}
            </Link>
          </Button>
        }
      />
    )
  } else if (isMobile) {
    content = (
      <div ref={listRef}>
        <MobileTrackRows
          items={virtualItems}
          totalSize={virtualizer.getTotalSize()}
          scrollMargin={scrollMargin}
          rowAt={rowAt}
          selecting={selecting}
          isSelected={selection.isSelected}
          onTap={(index, track) => {
            remember(track, index)
            if (selecting) selection.toggle(track.id, index)
            else openTagEditor([track.id])
          }}
        />
      </div>
    )
  } else {
    content = (
      <div ref={listRef} role="grid" aria-rowcount={total} aria-multiselectable className="bleed-x px-[calc(var(--page-px)-0.5rem)]">
        <TrackTableRows
          columns={columns}
          items={virtualItems}
          totalSize={virtualizer.getTotalSize()}
          scrollMargin={scrollMargin}
          rowAt={rowAt}
          isSelected={selection.isSelected}
          onRowClick={onRowClick}
          onToggle={onToggle}
          onRowDoubleClick={(index, track) => {
            remember(track, index)
            if (selection.isSelected(track.id) && count > 1) void run('edit')
            else openTagEditor([track.id])
          }}
        />
      </div>
    )
  }

  const actions: { id: Action; icon: LucideIcon; label: string; short?: string; destructive?: boolean }[] = [
    { id: 'edit', icon: Tags, label: t('actions.editTags') },
    { id: 'cover', icon: ImagePlus, label: t('actions.cover') },
    { id: 'rename', icon: PencilLine, label: t('actions.rename'), short: t('actions.renameShort') },
    { id: 'encoding', icon: Languages, label: t('actions.fixEncoding') },
    { id: 'rebuild', icon: RefreshCw, label: t('actions.rebuildTags') },
    { id: 'rescan', icon: FolderSync, label: t('actions.rescan') },
    { id: 'delete', icon: Trash2, label: t('actions.delete'), destructive: true },
  ]

  return (
    <Page>
      <PageHeader
        title={t('title')}
        subtitle={first.data ? t('table.count', { count: total, formatted: formatNumber(total) }) : undefined}
        navActions={
          isMobile && total > 0 ? (
            <Button
              variant="ghost"
              className="h-11 px-2 text-[17px] font-normal text-primary hover:bg-transparent"
              onClick={() => {
                if (selecting) selection.clear()
                setSelecting(!selecting)
              }}
            >
              {selecting ? t('common:actions.done') : t('common:actions.select')}
            </Button>
          ) : null
        }
      >
        <TracksTabs />
        <ManageToolbar
          filters={filters}
          onChange={update}
          libraries={libraries}
          albumLabel={albumLabel}
          artistLabel={artistLabel}
          visibleColumns={visibleColumns}
          onVisibleColumnsChange={(next: ReadonlySet<ColumnId>) => {
            setVisibleColumns(next)
            saveVisibleColumns(next)
          }}
          isMobile={isMobile}
        />
      </PageHeader>

      {!isMobile && total > 0 ? (
        <div className="bleed-x page-x hairline-b sticky top-[calc(var(--safe-top)+3rem)] z-20 bg-background/90 backdrop-blur-xl">
          <div className="flex h-12 items-center gap-2">
            <p className="tnum min-w-0 flex-1 truncate text-sm">
              {count > 0 ? (
                <>
                  <span className="font-medium">{t('selection.count', { count, formatted: formatNumber(count) })}</span>
                  {selection.selection.kind === 'ids' && count < total ? (
                    <Button variant="link" size="sm" className="h-auto px-2" onClick={selection.selectAll}>
                      {t('selection.selectAllMatching', { count: total, formatted: formatNumber(total) })}
                    </Button>
                  ) : null}
                  <Button variant="link" size="sm" className="h-auto px-2 text-muted-foreground" onClick={selection.clear}>
                    {t('common:actions.clear')}
                  </Button>
                </>
              ) : (
                <span className="text-muted-foreground">{t('selection.hint')}</span>
              )}
            </p>
            {actions.map((action) => {
              const disabled = busy !== null || (action.id !== 'rescan' && count === 0)
              return (
                <Tooltip key={action.id}>
                  <TooltipTrigger asChild>
                    <Button
                      variant="ghost"
                      size="sm"
                      disabled={disabled}
                      onClick={() => void run(action.id)}
                      className={cn('gap-1.5 max-xl:size-8 max-xl:px-0', action.destructive && 'text-destructive hover:text-destructive')}
                      aria-label={action.label}
                    >
                      {busy === action.id ? <Spinner size="sm" className="text-current" /> : <action.icon />}
                      <span className="max-xl:sr-only">{action.label}</span>
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent className="xl:hidden">{action.label}</TooltipContent>
                </Tooltip>
              )
            })}
          </div>
          <TrackTableHeader
            columns={columns}
            sort={filters.sort}
            order={filters.order}
            onSort={sortBy}
            checked={headerChecked}
            onCheckedChange={(checked) => (checked ? selection.selectAll() : selection.clear())}
          />
        </div>
      ) : null}

      {isMobile && selecting && total > 0 ? (
        <div className="-mt-1 mb-1 flex h-10 items-center justify-between text-[15px]">
          <span className="tnum text-muted-foreground">{t('selection.count', { count, formatted: formatNumber(count) })}</span>
          <Button
            variant="ghost"
            className="h-10 px-2 text-[15px] font-normal text-primary hover:bg-transparent"
            onClick={count === total ? selection.clear : selection.selectAll}
          >
            {count === total ? t('selection.deselectAll') : t('common:actions.selectAll')}
          </Button>
        </div>
      ) : null}

      {content}

      {isMobile && selecting ? (
        <MobileActionBar
          count={count}
          busy={busy}
          onAction={(a) => void run(a)}
          actions={actions}
        />
      ) : null}

      <BatchDialogs dialog={dialog} onClose={() => setDialog(null)} onDone={onDone} />
    </Page>
  )
}

interface MobileActionBarProps {
  count: number
  busy: Action | null
  onAction: (action: Action) => void
  actions: { id: Action; icon: LucideIcon; label: string; short?: string; destructive?: boolean }[]
}

/** Bottom action bar of the phone selection mode (covers the mini player while selecting). */
function MobileActionBar({ count, busy, onAction, actions }: MobileActionBarProps) {
  const { t } = useTranslation('manage')
  const primary = actions.filter((a) => a.id === 'edit' || a.id === 'rename' || a.id === 'delete')
  const secondary = actions.filter((a) => !primary.includes(a))
  const disabled = count === 0 || busy !== null
  return (
    <div className="ui-chrome glass hairline-t fixed inset-x-0 bottom-0 z-50 pb-safe pl-safe pr-safe">
      <div className="mx-auto flex h-[calc(var(--tabbar-h)+8px)] max-w-lg items-stretch px-2">
        {primary.map((a) => (
          <button
            key={a.id}
            type="button"
            disabled={disabled}
            onClick={() => onAction(a.id)}
            className={cn(
              'flex flex-1 flex-col items-center justify-center gap-0.5 text-[11px] font-medium transition-[opacity,transform] active:scale-[0.94] disabled:opacity-40',
              a.destructive ? 'text-destructive' : 'text-primary',
            )}
          >
            {busy === a.id ? <Spinner size="md" className="text-current" /> : <a.icon className="size-6" strokeWidth={1.75} />}
            {a.short ?? a.label}
          </button>
        ))}
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button
              type="button"
              disabled={busy !== null}
              className="flex flex-1 flex-col items-center justify-center gap-0.5 text-[11px] font-medium text-primary transition-[opacity,transform] active:scale-[0.94] disabled:opacity-40"
            >
              {busy && secondary.some((a) => a.id === busy) ? <Spinner size="md" className="text-current" /> : <MoreHorizontal className="size-6" strokeWidth={1.75} />}
              {t('common:actions.more')}
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent side="top" align="end" className="min-w-56 rounded-xl">
            {secondary.map((a, i) => (
              <div key={a.id}>
                {i > 0 && a.id === 'rescan' ? <DropdownMenuSeparator /> : null}
                <DropdownMenuItem className="h-11" disabled={a.id !== 'rescan' && count === 0} onSelect={() => onAction(a.id)}>
                  <a.icon />
                  {a.label}
                </DropdownMenuItem>
              </div>
            ))}
            <DropdownMenuSeparator />
            <DropdownMenuItem className="h-11" disabled>
              <ListChecks />
              {t('selection.count', { count, formatted: formatNumber(count) })}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </div>
  )
}
