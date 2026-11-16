import { useQuery } from '@tanstack/react-query'
import {
  Check,
  ChevronLeft,
  ChevronRight,
  Copy,
  Image as ImageIcon,
  X,
  ZoomIn,
  ZoomOut,
} from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ErrorState } from '@/components/error-state'
import { Spinner } from '@/components/spinner'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog'
import { useIsMobile } from '@/hooks/use-media-query'
import { api } from '@/lib/api/endpoints'
import { formatBytes, formatDate } from '@/lib/format'
import { cn } from '@/lib/utils'

import { parsePreviewLyrics, type FolderPreviewItem } from '../lib/folder-explorer'
import { manageKeys } from '../queries'

export interface FolderPreviewDialogProps {
  libraryId: number
  items: readonly FolderPreviewItem[]
  index: number | null
  onIndexChange: (next: number | null) => void
}

export function FolderPreviewDialog({ libraryId, items, index, onIndexChange }: FolderPreviewDialogProps) {
  const { t } = useTranslation('manage')
  const isMobile = useIsMobile()
  const open = index !== null && index >= 0 && index < items.length
  const item = open ? items[index] : null
  const [zoomed, setZoomed] = useState(false)
  const [copied, setCopied] = useState(false)
  const [failedPath, setFailedPath] = useState<string | null>(null)
  const [dimensions, setDimensions] = useState<{ path: string; w: number; h: number } | null>(null)
  const activeThumbRef = useRef<HTMLButtonElement>(null)

  const step = (delta: number) => {
    if (!open || items.length <= 1) return
    const next = (index + delta + items.length) % items.length
    setZoomed(false)
    setCopied(false)
    onIndexChange(next)
  }

  useEffect(() => {
    if (!open || items.length <= 1) return
    const onKeyDown = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement | null
      if (target && (target.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName))) return
      if (e.key === 'ArrowLeft') {
        e.preventDefault()
        onIndexChange((index - 1 + items.length) % items.length)
        setZoomed(false)
      } else if (e.key === 'ArrowRight') {
        e.preventDefault()
        onIndexChange((index + 1) % items.length)
        setZoomed(false)
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [open, index, items.length, onIndexChange])

  useEffect(() => {
    activeThumbRef.current?.scrollIntoView({ block: 'nearest', inline: 'center' })
  }, [index])

  const textQuery = useQuery({
    queryKey: manageKeys.folderText(libraryId, item?.path ?? ''),
    queryFn: ({ signal }) => api.manage.folders.text({ libraryId, path: item!.path }, { signal }),
    enabled: open && item?.kind === 'text',
    staleTime: 60_000,
  })

  const isLyrics = item?.isLyrics ?? false
  const textContent = textQuery.data?.content
  const parsedLyrics = useMemo(
    () => (isLyrics && textContent ? parsePreviewLyrics(textContent) : null),
    [isLyrics, textContent],
  )

  const copyContent = async () => {
    const content = textQuery.data?.content
    if (!content) return
    try {
      await navigator.clipboard.writeText(content)
      setCopied(true)
      toast.success(t('common:actions.copied'))
      window.setTimeout(() => setCopied(false), 1800)
    } catch {
      // Clipboard access can be denied in non-secure contexts; fail quietly.
    }
  }

  const dimsLabel =
    item && dimensions?.path === item.path && dimensions.w > 0 ? `${dimensions.w} × ${dimensions.h}` : null
  const subtitleParts = item ? [dimsLabel, formatBytes(item.size), formatDate(item.mtime)].filter(Boolean) : []
  const btnSize = isMobile ? 'size-11' : 'size-8'

  return (
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        if (!nextOpen) {
          setZoomed(false)
          setCopied(false)
          onIndexChange(null)
        }
      }}
    >
      <DialogContent
        showCloseButton={false}
        className={cn(
          'flex max-h-[calc(100dvh-1.5rem)] flex-col gap-0 overflow-hidden p-0',
          item?.kind === 'image' ? 'sm:max-w-4xl' : 'sm:max-w-2xl',
        )}
      >
        {item ? (
          <>
            <div className="flex items-center gap-2 border-b px-3 py-2 sm:px-4">
              {items.length > 1 ? (
                <div className="flex shrink-0 items-center gap-0.5">
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className={btnSize}
                    aria-label={t('folders.preview.previous')}
                    onClick={() => step(-1)}
                  >
                    <ChevronLeft className="size-4" />
                  </Button>
                  <span className="tnum px-1 text-xs text-muted-foreground">
                    {index! + 1} / {items.length}
                  </span>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className={btnSize}
                    aria-label={t('folders.preview.next')}
                    onClick={() => step(1)}
                  >
                    <ChevronRight className="size-4" />
                  </Button>
                </div>
              ) : null}

              <div className="min-w-0 flex-1">
                <DialogTitle className="truncate text-sm font-semibold" title={item.name}>
                  {item.name}
                </DialogTitle>
                <DialogDescription className="tnum mt-0.5 truncate text-xs">
                  {subtitleParts.join(' · ')}
                </DialogDescription>
              </div>

              <div className="flex shrink-0 items-center gap-1">
                {item.kind === 'image' ? (
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className={btnSize}
                    aria-label={t(zoomed ? 'folders.preview.zoomOut' : 'folders.preview.zoomIn')}
                    title={t(zoomed ? 'folders.preview.zoomOut' : 'folders.preview.zoomIn')}
                    onClick={() => setZoomed((z) => !z)}
                  >
                    {zoomed ? <ZoomOut className="size-4" /> : <ZoomIn className="size-4" />}
                  </Button>
                ) : textQuery.data?.content ? (
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className={btnSize}
                    aria-label={t('common:actions.copy')}
                    title={t('common:actions.copy')}
                    onClick={() => void copyContent()}
                  >
                    {copied ? <Check className="size-4 text-primary" /> : <Copy className="size-4" />}
                  </Button>
                ) : null}

                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className={btnSize}
                  aria-label={t('common:actions.close')}
                  onClick={() => onIndexChange(null)}
                >
                  <X className="size-4" />
                </Button>
              </div>
            </div>

            {item.kind === 'image' ? (
              <div className="flex min-h-0 flex-1 flex-col">
                <div className="flex min-h-[16rem] flex-1 items-center justify-center overflow-auto bg-muted/30 p-3 sm:p-4">
                  {failedPath === item.path ? (
                    <div className="flex flex-col items-center gap-2 py-12 text-center text-sm text-muted-foreground">
                      <ImageIcon className="size-8 stroke-[1.5]" />
                      <p>{t('folders.preview.imageFailed')}</p>
                    </div>
                  ) : (
                    <img
                      key={item.path}
                      src={api.manage.folders.imageUrl(libraryId, item.path, item.mtime)}
                      alt={item.name}
                      onLoad={(e) => {
                        const img = e.currentTarget
                        setDimensions({ path: item.path, w: img.naturalWidth, h: img.naturalHeight })
                      }}
                      onError={() => setFailedPath(item.path)}
                      onClick={() => setZoomed((z) => !z)}
                      className={cn(
                        'rounded-md select-none',
                        zoomed
                          ? 'max-w-none cursor-zoom-out'
                          : 'max-h-[65dvh] max-w-full cursor-zoom-in object-contain',
                      )}
                    />
                  )}
                </div>

                {items.length > 1 && items.every((entry) => entry.kind === 'image') ? (
                  <div className="scrollbar-none flex gap-1.5 overflow-x-auto border-t bg-card px-3 py-2">
                    {items.map((entry, idx) => {
                      const active = idx === index
                      return (
                        <button
                          key={entry.path}
                          ref={active ? activeThumbRef : undefined}
                          type="button"
                          onClick={() => {
                            setZoomed(false)
                            onIndexChange(idx)
                          }}
                          aria-label={entry.name}
                          aria-current={active ? 'true' : undefined}
                          className={cn(
                            'size-12 shrink-0 overflow-hidden rounded-md border transition focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
                            active ? 'border-primary ring-2 ring-primary/30' : 'opacity-65 hover:opacity-100',
                          )}
                        >
                          <img
                            src={api.manage.folders.imageUrl(libraryId, entry.path, entry.mtime)}
                            alt=""
                            loading="lazy"
                            decoding="async"
                            className="size-full object-cover"
                          />
                        </button>
                      )
                    })}
                  </div>
                ) : null}
              </div>
            ) : (
              <div className="min-h-[14rem] flex-1 overflow-y-auto p-4">
                {textQuery.isPending ? (
                  <div className="flex min-h-48 items-center justify-center">
                    <Spinner />
                  </div>
                ) : textQuery.isError ? (
                  <ErrorState
                    error={textQuery.error}
                    onRetry={() => void textQuery.refetch()}
                    retrying={textQuery.isFetching}
                  />
                ) : !textQuery.data.content.trim() ? (
                  <p className="py-12 text-center text-sm text-muted-foreground">{t('folders.preview.emptyText')}</p>
                ) : parsedLyrics ? (
                  <div className="space-y-1.5">
                    {parsedLyrics.map((line, i) => (
                      <div key={i} className="flex items-baseline gap-3 text-sm leading-relaxed">
                        <span className="tnum w-12 shrink-0 text-right text-xs text-muted-foreground select-none">
                          {line.timeLabel ?? ''}
                        </span>
                        <span className="min-w-0 flex-1 break-words">{line.text || '♪'}</span>
                      </div>
                    ))}
                  </div>
                ) : (
                  <pre className="font-mono text-xs leading-relaxed break-words whitespace-pre-wrap text-foreground">
                    {textQuery.data.content}
                  </pre>
                )}

                {textQuery.data?.truncated ? (
                  <p className="mt-4 border-t pt-3 text-xs text-muted-foreground">{t('folders.preview.truncated')}</p>
                ) : null}
              </div>
            )}
          </>
        ) : null}
      </DialogContent>
    </Dialog>
  )
}
