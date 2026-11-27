import {
  AudioLines,
  ChevronRight,
  Folder,
  FolderOpen,
  FolderRoot,
  FolderTree,
  Loader2,
  Music,
  PanelLeftClose,
  PanelLeftOpen,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Drawer, DrawerContent, DrawerHeader, DrawerTitle, DrawerTrigger } from '@/components/ui/drawer'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { useIsMobile } from '@/hooks/use-media-query'
import { cn } from '@/lib/utils'

import { dirWithin, type FolderNavigatorRow } from '../lib/folder-explorer'

export interface FolderNavigatorListProps {
  rows: readonly FolderNavigatorRow[]
  currentDir: string
  /** Library-relative directory of the currently playing track (when in the same library), or `null`. */
  playingDir: string | null
  clampNames?: boolean
  onSelect: (dir: string) => void
  onToggleExpand: (dir: string) => void
}

export function FolderNavigatorList({
  rows,
  currentDir,
  playingDir,
  clampNames = true,
  onSelect,
  onToggleExpand,
}: FolderNavigatorListProps) {
  const { t } = useTranslation('manage')

  return (
    <nav aria-label={t('folders.treeTitle')} className="space-y-0.5 text-sm">
      {rows.map((row) => {
        const active = row.dir === currentDir
        const playingHere =
          playingDir !== null &&
          (playingDir === row.dir || (!row.isRoot && !row.expanded && dirWithin(playingDir, row.dir)))
        const Icon = playingHere ? AudioLines : row.isRoot ? FolderRoot : active || row.expanded ? FolderOpen : Folder

        return (
          <div
            key={row.dir || '__root__'}
            className={cn(
              'flex items-center rounded-md transition-colors',
              active
                ? 'bg-primary/12 font-medium text-foreground'
                : 'text-muted-foreground hover:bg-accent/60 hover:text-foreground',
            )}
            style={{ paddingLeft: `${Math.min(row.depth, 6) * 12}px` }}
          >
            {row.hasChildren && !row.isRoot ? (
              <button
                type="button"
                data-nav-toggle
                className="inline-flex size-7 shrink-0 items-center justify-center rounded text-muted-foreground hover:bg-accent hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
                aria-expanded={row.expanded}
                aria-label={t(row.expanded ? 'folders.collapseFolder' : 'folders.expandFolder', { name: row.name })}
                onClick={() => onToggleExpand(row.dir)}
              >
                {row.loading ? (
                  <Loader2 className="size-3.5 animate-spin" />
                ) : (
                  <ChevronRight className={cn('size-3.5 transition-transform duration-150', row.expanded && 'rotate-90')} />
                )}
              </button>
            ) : (
              <span className="size-4 shrink-0" aria-hidden="true" />
            )}
            <button
              type="button"
              data-nav-select
              aria-current={active ? 'page' : undefined}
              className="flex min-h-8 min-w-0 flex-1 items-center gap-1.5 rounded py-1 pr-2 text-left focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
              onClick={() => onSelect(row.dir)}
            >
              <Icon
                className={cn(
                  'size-4 shrink-0',
                  playingHere
                    ? 'text-primary motion-safe:animate-pulse'
                    : active
                      ? 'text-primary'
                      : 'text-muted-foreground',
                )}
                strokeWidth={1.75}
              />
              <span
                className={cn(
                  'min-w-0 flex-1 leading-snug break-words [overflow-wrap:anywhere]',
                  clampNames && 'line-clamp-2',
                )}
                title={row.name}
              >
                {row.name}
              </span>
              {row.trackCount > 0 ? (
                <span className="tnum inline-flex shrink-0 items-center gap-0.5 text-[11px] text-muted-foreground">
                  <Music className="size-3" />
                  {row.trackCount}
                </span>
              ) : null}
            </button>
          </div>
        )
      })}
    </nav>
  )
}

export function FolderRail({
  rows,
  currentDir,
  playingDir,
  clampNames = true,
  onSelect,
  onToggleExpand,
  onCollapse,
}: FolderNavigatorListProps & { onCollapse: () => void }) {
  return (
    <aside className="sticky top-[calc(var(--app-header-h)+var(--page-bar-h)+0.5rem)] flex max-h-[calc(100dvh-var(--app-header-h)-var(--player-clearance)-4rem)] flex-col self-start border-r pr-3">
      <div className="mb-1.5 flex shrink-0 items-center justify-end gap-1">
        <FolderRailToggle collapsed={false} onToggle={onCollapse} />
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto">
        <FolderNavigatorList
          rows={rows}
          currentDir={currentDir}
          playingDir={playingDir}
          clampNames={clampNames}
          onSelect={onSelect}
          onToggleExpand={onToggleExpand}
        />
      </div>
    </aside>
  )
}

export function FolderRailToggle({ collapsed, onToggle }: { collapsed: boolean; onToggle: () => void }) {
  const { t } = useTranslation('manage')
  const label = collapsed ? t('folders.showFolderRail') : t('folders.hideFolderRail')
  const Icon = collapsed ? PanelLeftOpen : PanelLeftClose
  return (
    <Button
      type="button"
      variant="ghost"
      size="icon"
      className="size-7 shrink-0 text-muted-foreground hover:text-foreground"
      aria-label={label}
      title={label}
      onClick={onToggle}
    >
      <Icon className="size-3.5" />
    </Button>
  )
}

export function FolderTreeButton({
  rows,
  currentDir,
  playingDir,
  clampNames = true,
  onSelect,
  onToggleExpand,
}: FolderNavigatorListProps) {
  const { t } = useTranslation('manage')
  const isMobile = useIsMobile()
  const [open, setOpen] = useState(false)

  const handleSelect = (dir: string) => {
    onSelect(dir)
    setOpen(false)
  }

  const trigger = (
    <Button
      type="button"
      variant="outline"
      size={isMobile ? 'icon' : 'sm'}
      className={cn('shrink-0', isMobile ? 'size-11' : 'h-8 gap-1.5 px-2.5 text-xs')}
      aria-label={t('folders.showFolders')}
    >
      <FolderTree className="size-4" />
      {isMobile ? null : <span>{t('folders.showFolders')}</span>}
    </Button>
  )

  if (isMobile) {
    return (
      <Drawer open={open} onOpenChange={setOpen}>
        <DrawerTrigger asChild>{trigger}</DrawerTrigger>
        <DrawerContent>
          <DrawerHeader className="pb-2">
            <DrawerTitle>{t('folders.treeTitle')}</DrawerTitle>
          </DrawerHeader>
          <div className="max-h-[65dvh] overflow-y-auto px-3 pb-[calc(1rem+var(--safe-bottom))] [&_button[data-nav-select]]:min-h-11 [&_button[data-nav-toggle]]:size-11">
            <FolderNavigatorList
              rows={rows}
              currentDir={currentDir}
              playingDir={playingDir}
              clampNames={clampNames}
              onSelect={handleSelect}
              onToggleExpand={onToggleExpand}
            />
          </div>
        </DrawerContent>
      </Drawer>
    )
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>{trigger}</PopoverTrigger>
      <PopoverContent align="start" className="w-80 max-w-[calc(100vw-2rem)] p-2">
        <div className="max-h-96 overflow-y-auto">
          <FolderNavigatorList
            rows={rows}
            currentDir={currentDir}
            playingDir={playingDir}
            clampNames={clampNames}
            onSelect={handleSelect}
            onToggleExpand={onToggleExpand}
          />
        </div>
      </PopoverContent>
    </Popover>
  )
}
