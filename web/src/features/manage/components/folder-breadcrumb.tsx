import { ChevronRight, CornerLeftUp, Folder, FolderRoot, MoreHorizontal } from 'lucide-react'
import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { useIsMobile } from '@/hooks/use-media-query'
import { cn } from '@/lib/utils'

import { splitDir } from '../lib/folder-explorer'

export interface FolderBreadcrumbProps {
  dir: string
  rootName: string
  onNavigate: (dir: string) => void
}

export function FolderBreadcrumb({ dir, rootName, onNavigate }: FolderBreadcrumbProps) {
  const { t } = useTranslation('manage')
  const isMobile = useIsMobile()
  const segmentsRef = useRef<HTMLDivElement>(null)
  const parts = splitDir(dir)
  const parent = parts.length > 1 ? parts.slice(0, -1).join('/') : ''
  const currentName = parts[parts.length - 1] ?? ''
  const collapsedAncestors =
    parts.length > 2
      ? parts.slice(0, -1).map((part, idx) => ({
          dir: parts.slice(0, idx + 1).join('/'),
          label: parts.slice(0, idx + 1).join(' / '),
          name: part,
        }))
      : []

  useEffect(() => {
    const el = segmentsRef.current
    if (!el) return
    el.scrollLeft = el.scrollWidth
  }, [dir])

  return (
    <nav aria-label={t('folders.breadcrumbs')} className="flex min-w-0 flex-1 items-center gap-1 text-sm">
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className={cn('shrink-0 text-muted-foreground hover:text-foreground', isMobile ? 'size-11' : 'size-7')}
        disabled={parts.length === 0}
        aria-label={t('folders.parentFolder')}
        title={t('folders.parentFolder')}
        onClick={() => onNavigate(parent)}
      >
        <CornerLeftUp className="size-4" />
      </Button>

      {/* Compact breadcrumb on phone & tablet: root + collapsed ancestor menu + current folder */}
      <div className="flex min-w-0 flex-1 items-center gap-0.5 lg:hidden">
        <button
          type="button"
          className={cn(
            'inline-flex min-h-11 shrink-0 items-center gap-1.5 rounded-md px-2 text-left transition-colors hover:bg-accent md:min-h-7 md:py-1',
            parts.length === 0 ? 'max-w-full font-medium text-foreground' : 'max-w-[8.5rem] text-muted-foreground hover:text-foreground',
          )}
          aria-current={parts.length === 0 ? 'page' : undefined}
          onClick={() => onNavigate('')}
          title={rootName}
        >
          <FolderRoot className="size-3.5 shrink-0" />
          <span className="truncate">{rootName}</span>
        </button>

        {parts.length === 2 ? (
          <>
            <ChevronRight className="size-3.5 shrink-0 text-muted-foreground/60" />
            <button
              type="button"
              className="min-h-11 max-w-[7rem] shrink truncate rounded-md px-1.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground md:min-h-7 md:py-1"
              onClick={() => onNavigate(parts[0])}
              title={parts[0]}
            >
              {parts[0]}
            </button>
          </>
        ) : null}

        {collapsedAncestors.length > 0 ? (
          <>
            <ChevronRight className="size-3.5 shrink-0 text-muted-foreground/60" />
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className={cn('shrink-0 text-muted-foreground', isMobile ? 'size-11' : 'size-7')}
                  aria-label={t('folders.showParentFolders')}
                >
                  <MoreHorizontal className="size-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="start" className="max-w-[calc(100vw-2rem)] min-w-52 rounded-xl">
                <DropdownMenuItem onSelect={() => onNavigate('')}>
                  <FolderRoot className="size-4 shrink-0" />
                  <span className="truncate">{rootName}</span>
                </DropdownMenuItem>
                {collapsedAncestors.map((item) => (
                  <DropdownMenuItem key={item.dir} onSelect={() => onNavigate(item.dir)}>
                    <Folder className="size-4 shrink-0" />
                    <span className="truncate" title={item.label}>
                      {item.label}
                    </span>
                  </DropdownMenuItem>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>
          </>
        ) : null}

        {parts.length >= 1 ? (
          <>
            <ChevronRight className="size-3.5 shrink-0 text-muted-foreground/60" />
            <span
              className="min-w-0 flex-1 truncate px-1.5 py-1 font-medium text-foreground"
              aria-current="page"
              title={currentName}
            >
              {currentName}
            </span>
          </>
        ) : null}
      </div>

      {/* Full auto-scrolling breadcrumb on wide viewports */}
      <div
        ref={segmentsRef}
        className="scrollbar-none hidden min-w-0 flex-1 items-center gap-0.5 overflow-x-auto whitespace-nowrap lg:flex"
      >
        <button
          type="button"
          className={cn(
            'inline-flex shrink-0 items-center gap-1.5 rounded-md px-2 py-1 transition-colors hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
            parts.length === 0 ? 'font-medium text-foreground' : 'text-muted-foreground hover:text-foreground',
          )}
          aria-current={parts.length === 0 ? 'page' : undefined}
          onClick={() => onNavigate('')}
          title={rootName}
        >
          <FolderRoot className="size-3.5 shrink-0" />
          <span className="max-w-44 truncate">{rootName}</span>
        </button>
        {parts.map((part, idx) => {
          const target = parts.slice(0, idx + 1).join('/')
          const last = idx === parts.length - 1
          return (
            <span key={target} className="inline-flex shrink-0 items-center gap-0.5">
              <ChevronRight className="size-3.5 shrink-0 text-muted-foreground/60" />
              <button
                type="button"
                className={cn(
                  'max-w-56 truncate rounded-md px-2 py-1 transition-colors hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
                  last ? 'font-medium text-foreground' : 'text-muted-foreground hover:text-foreground',
                )}
                aria-current={last ? 'page' : undefined}
                onClick={() => onNavigate(target)}
                title={part}
              >
                {part}
              </button>
            </span>
          )
        })}
      </div>
    </nav>
  )
}
