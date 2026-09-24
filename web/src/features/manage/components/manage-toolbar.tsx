import { useQuery } from '@tanstack/react-query'
import { ArrowDownUp, Columns3, Filter, Search, X } from 'lucide-react'
import { useEffect, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import type { SortOrder, TrackSort } from '@/lib/api/endpoints'
import { api } from '@/lib/api/endpoints'
import type { LibraryInfo } from '@/lib/api/types'
import { queryKeys } from '@/lib/query-keys'
import { cn } from '@/lib/utils'

import { COLUMNS, type ColumnId } from '../lib/columns'
import { activeFilterCount, type ManageFilters, type MissingFilter } from '../lib/filters'
import { EntityCombobox } from './entity-combobox'

const ANY = '__any'
const SEARCH_DEBOUNCE_MS = 250

export interface ManageToolbarProps {
  filters: ManageFilters
  onChange: (patch: Partial<ManageFilters>) => void
  libraries: LibraryInfo[]
  albumLabel?: string
  artistLabel?: string
  visibleColumns: ReadonlySet<ColumnId>
  onVisibleColumnsChange: (columns: ReadonlySet<ColumnId>) => void
  isMobile: boolean
}

/** Search field, filter popover, sort (mobile) / columns (desktop) menus and active-filter chips. */
export function ManageToolbar({
  filters,
  onChange,
  libraries,
  albumLabel,
  artistLabel,
  visibleColumns,
  onVisibleColumnsChange,
  isMobile,
}: ManageToolbarProps) {
  const { t } = useTranslation('manage')
  const [search, setSearch] = useState(filters.q)
  const [prevQ, setPrevQ] = useState(filters.q)
  if (filters.q !== prevQ) {
    // URL changed from elsewhere (back button, chip removal) — follow it.
    setPrevQ(filters.q)
    setSearch(filters.q)
  }

  useEffect(() => {
    if (search === filters.q) return
    const id = setTimeout(() => onChange({ q: search }), SEARCH_DEBOUNCE_MS)
    return () => clearTimeout(id)
  }, [search, filters.q, onChange])

  const count = activeFilterCount(filters)
  const chips: { key: string; label: ReactNode; clear: Partial<ManageFilters> }[] = []
  if (filters.libraryId && libraries.length > 1) {
    const lib = libraries.find((l) => l.id === filters.libraryId)
    chips.push({ key: 'lib', label: `${t('filters.library')}: ${lib?.name || filters.libraryId}`, clear: { libraryId: undefined } })
  }
  if (filters.albumId) chips.push({ key: 'album', label: `${t('fields.album')}: ${albumLabel || '…'}`, clear: { albumId: '' } })
  if (filters.artistId) chips.push({ key: 'artist', label: `${t('fields.artist')}: ${artistLabel || '…'}`, clear: { artistId: '' } })
  if (filters.genre) chips.push({ key: 'genre', label: `${t('fields.genre')}: ${filters.genre}`, clear: { genre: '' } })
  if (filters.dirPrefix) chips.push({ key: 'dir', label: `${t('filters.folder')}: ${filters.dirPrefix}`, clear: { dirPrefix: '' } })
  if (filters.missing) chips.push({ key: 'missing', label: t(`filters.missing_${filters.missing}`), clear: { missing: '' } })

  return (
    <div className="grid gap-3 pb-3">
      <div className="flex items-center gap-2">
        <div className="relative min-w-0 flex-1 md:max-w-sm">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            type="search"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t('filters.search')}
            aria-label={t('filters.search')}
            className="h-10 rounded-lg pl-9 md:h-9"
          />
        </div>
        <FilterPopover filters={filters} onChange={onChange} libraries={libraries} albumLabel={albumLabel} artistLabel={artistLabel}>
          <Button variant="outline" className="h-10 gap-2 md:h-9" aria-label={t('filters.title')}>
            <Filter />
            <span className="max-sm:sr-only">{t('filters.title')}</span>
            {count > 0 ? <Badge className="h-5 min-w-5 px-1.5 tabular-nums">{count}</Badge> : null}
          </Button>
        </FilterPopover>
        {isMobile ? (
          <SortMenu sort={filters.sort} order={filters.order} onChange={onChange} />
        ) : (
          <ColumnsMenu visible={visibleColumns} onChange={onVisibleColumnsChange} />
        )}
      </div>
      {chips.length > 0 ? (
        <div className="flex flex-wrap items-center gap-1.5">
          {chips.map((chip) => (
            <button
              key={chip.key}
              type="button"
              onClick={() => onChange(chip.clear)}
              className="inline-flex h-7 max-w-full items-center gap-1 rounded-full bg-secondary pr-1.5 pl-3 text-xs font-medium text-secondary-foreground transition-colors hover:bg-secondary/70"
              aria-label={t('filters.remove', { filter: String(chip.label) })}
            >
              <span className="truncate">{chip.label}</span>
              <X className="size-3.5 shrink-0 opacity-60" />
            </button>
          ))}
          {chips.length > 1 ? (
            <Button
              variant="link"
              size="xs"
              className="text-muted-foreground"
              onClick={() =>
                onChange({ albumId: '', artistId: '', genre: '', dirPrefix: '', libraryId: undefined, missing: '' })
              }
            >
              {t('filters.clearAll')}
            </Button>
          ) : null}
        </div>
      ) : null}
    </div>
  )
}

interface FilterPopoverProps {
  filters: ManageFilters
  onChange: (patch: Partial<ManageFilters>) => void
  libraries: LibraryInfo[]
  albumLabel?: string
  artistLabel?: string
  children: ReactNode
}

function FilterPopover({ filters, onChange, libraries, albumLabel, artistLabel, children }: FilterPopoverProps) {
  const { t } = useTranslation('manage')
  const [dir, setDir] = useState(filters.dirPrefix)
  const [open, setOpen] = useState(false)
  const genres = useQuery({ queryKey: queryKeys.genres, queryFn: ({ signal }) => api.genres({ signal }), enabled: open })

  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        setOpen(next)
        if (next) setDir(filters.dirPrefix)
      }}
    >
      <PopoverTrigger asChild>{children}</PopoverTrigger>
      <PopoverContent align="end" className="grid w-[min(20rem,calc(100vw-2rem))] gap-3 rounded-xl">
        {libraries.length > 1 ? (
          <FilterField label={t('filters.library')}>
            <Select
              value={filters.libraryId ? String(filters.libraryId) : ANY}
              onValueChange={(v) => onChange({ libraryId: v === ANY ? undefined : Number(v) })}
            >
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ANY}>{t('filters.any')}</SelectItem>
                {libraries.map((lib) => (
                  <SelectItem key={lib.id} value={String(lib.id)}>
                    {lib.name || `#${lib.id}`}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </FilterField>
        ) : null}
        <FilterField label={t('fields.album')}>
          <EntityCombobox
            kind="album"
            value={filters.albumId}
            valueLabel={albumLabel}
            onChange={(o) => onChange({ albumId: o?.id ?? '' })}
            placeholder={t('filters.any')}
          />
        </FilterField>
        <FilterField label={t('fields.artist')}>
          <EntityCombobox
            kind="artist"
            value={filters.artistId}
            valueLabel={artistLabel}
            onChange={(o) => onChange({ artistId: o?.id ?? '' })}
            placeholder={t('filters.any')}
          />
        </FilterField>
        <FilterField label={t('fields.genre')}>
          <Select value={filters.genre || ANY} onValueChange={(v) => onChange({ genre: v === ANY ? '' : v })}>
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent className="max-h-72">
              <SelectItem value={ANY}>{t('filters.any')}</SelectItem>
              {filters.genre && !genres.data?.some((g) => g.name === filters.genre) ? (
                <SelectItem value={filters.genre}>{filters.genre}</SelectItem>
              ) : null}
              {(genres.data ?? []).map((g) => (
                <SelectItem key={g.id} value={g.name}>
                  {g.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </FilterField>
        <FilterField label={t('filters.folder')}>
          <form
            onSubmit={(e) => {
              e.preventDefault()
              onChange({ dirPrefix: dir.trim().replace(/^\/+|\/+$/g, '') })
            }}
          >
            <Input
              value={dir}
              onChange={(e) => setDir(e.target.value)}
              onBlur={() => onChange({ dirPrefix: dir.trim().replace(/^\/+|\/+$/g, '') })}
              placeholder={t('filters.folderPlaceholder')}
              className="font-mono text-xs md:text-xs"
            />
          </form>
        </FilterField>
        <FilterField label={t('filters.missing')}>
          <Select value={filters.missing || ANY} onValueChange={(v) => onChange({ missing: v === ANY ? '' : (v as MissingFilter) })}>
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={ANY}>{t('filters.missing_exclude')}</SelectItem>
              <SelectItem value="include">{t('filters.missing_include')}</SelectItem>
              <SelectItem value="only">{t('filters.missing_only')}</SelectItem>
            </SelectContent>
          </Select>
        </FilterField>
      </PopoverContent>
    </Popover>
  )
}

function FilterField({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="grid gap-1.5">
      <Label className="text-xs font-medium text-muted-foreground">{label}</Label>
      {children}
    </div>
  )
}

const MOBILE_SORTS: readonly TrackSort[] = ['path', 'title', 'artist', 'album', 'year', 'recent', 'updated', 'duration', 'size']

function SortMenu({ sort, order, onChange }: { sort: TrackSort; order: SortOrder; onChange: (patch: Partial<ManageFilters>) => void }) {
  const { t } = useTranslation('manage')
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="icon" className="size-10" aria-label={t('table.sort')}>
          <ArrowDownUp />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-52 rounded-xl">
        <DropdownMenuLabel className="text-xs text-muted-foreground">{t('table.sort')}</DropdownMenuLabel>
        <DropdownMenuRadioGroup value={sort} onValueChange={(v) => onChange({ sort: v as TrackSort })}>
          {MOBILE_SORTS.map((s) => (
            <DropdownMenuRadioItem key={s} value={s} className="h-10">
              {t(`sort.${s}`)}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
        <DropdownMenuSeparator />
        <DropdownMenuRadioGroup value={order} onValueChange={(v) => onChange({ order: v as SortOrder })}>
          <DropdownMenuRadioItem value="asc" className="h-10">
            {t('table.ascending')}
          </DropdownMenuRadioItem>
          <DropdownMenuRadioItem value="desc" className="h-10">
            {t('table.descending')}
          </DropdownMenuRadioItem>
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function ColumnsMenu({ visible, onChange }: { visible: ReadonlySet<ColumnId>; onChange: (columns: ReadonlySet<ColumnId>) => void }) {
  const { t } = useTranslation('manage')
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" className={cn('gap-2')}>
          <Columns3 />
          {t('table.columns')}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-48 rounded-xl">
        {COLUMNS.filter((c) => !c.fixed).map((c) => (
          <DropdownMenuCheckboxItem
            key={c.id}
            checked={visible.has(c.id)}
            onSelect={(e) => e.preventDefault()}
            onCheckedChange={(checked) => {
              const next = new Set(visible)
              if (checked) next.add(c.id)
              else next.delete(c.id)
              onChange(next)
            }}
          >
            {t(`columns.${c.id}`)}
          </DropdownMenuCheckboxItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
