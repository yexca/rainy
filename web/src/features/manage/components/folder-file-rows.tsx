import {
  AlertCircle,
  AudioLines,
  Captions,
  Eye,
  File,
  FileArchive,
  FileAudio,
  FileText,
  Folder,
  Image as ImageIcon,
  Loader2,
  Pause,
  Play,
  Tags,
} from 'lucide-react'
import { useState, type MouseEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { TrackActionsMenu } from '@/features/library/components/track-actions'
import { api } from '@/lib/api/endpoints'
import type { FileEntry, FolderEntry } from '@/lib/api/types'
import { formatBitrate, formatBytes, formatDate, formatDuration } from '@/lib/format'
import { cn } from '@/lib/utils'

import { fileExtension, fileStem, isLyricsSidecarFile, type FolderTrackItem } from '../lib/folder-explorer'

const ARCHIVE_EXTS = new Set(['zip', 'rar', '7z', 'tar', 'gz', 'bz2', 'xz'])

export function FolderTile({
  folder,
  playingHere = false,
  playingLoading = false,
  clampNames = true,
  onOpen,
  onPlay,
}: {
  folder: FolderEntry
  playingHere?: boolean
  playingLoading?: boolean
  clampNames?: boolean
  onOpen: () => void
  onPlay?: () => void
}) {
  const { t } = useTranslation('manage')
  const FolderIcon = playingHere ? AudioLines : Folder

  return (
    <div
      className={cn(
        'group flex min-w-0 items-stretch overflow-hidden rounded-lg border border-border/75 bg-card/50 transition-colors hover:border-primary/35 hover:bg-accent/40',
        playingHere && 'border-primary/40 bg-primary/[0.05]',
      )}
    >
      <button
        type="button"
        className="flex min-h-14 min-w-0 flex-1 items-center gap-3 px-3 py-2.5 text-left focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none focus-visible:ring-inset"
        onClick={onOpen}
        title={folder.name}
      >
        <span
          className={cn(
            'flex size-9 shrink-0 items-center justify-center rounded-md',
            playingHere ? 'bg-primary/15 text-primary' : 'bg-primary/10 text-primary',
          )}
        >
          <FolderIcon
            className={cn('size-4', playingHere && 'motion-safe:animate-pulse')}
            fill={playingHere ? 'none' : 'currentColor'}
            fillOpacity={playingHere ? undefined : 0.14}
            strokeWidth={1.75}
          />
        </span>
        <span className="min-w-0 flex-1">
          <span
            className={cn(
              'block text-sm leading-snug font-medium break-words [overflow-wrap:anywhere]',
              clampNames && 'line-clamp-2',
            )}
          >
            {folder.name}
          </span>
          <span className="tnum mt-0.5 block truncate text-xs text-muted-foreground">
            {folder.trackCount > 0
              ? t('folders.subfolderTracks', { count: folder.trackCount })
              : t('folders.subfolderBrowse')}
          </span>
        </span>
      </button>

      {folder.trackCount > 0 && onPlay ? (
        <button
          type="button"
          disabled={playingLoading}
          className="inline-flex w-11 shrink-0 items-center justify-center border-l text-muted-foreground transition-colors hover:bg-primary/10 hover:text-primary focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none focus-visible:ring-inset disabled:opacity-60"
          aria-label={t('folders.playSubfolder', { name: folder.name })}
          title={t('folders.playSubfolder', { name: folder.name })}
          onClick={onPlay}
        >
          {playingLoading ? (
            <Loader2 className="size-4 animate-spin" />
          ) : (
            <Play className="size-3.5 fill-current" />
          )}
        </button>
      ) : null}
    </div>
  )
}

export function FolderTrackRow({
  item,
  selected,
  isCurrent,
  isPlaying,
  clampNames = true,
  onToggleSelect,
  onPlay,
  onTogglePlay,
  onEditTags,
  onPreviewSidecar,
}: {
  item: FolderTrackItem
  selected: boolean
  isCurrent: boolean
  isPlaying: boolean
  clampNames?: boolean
  onToggleSelect?: (shiftKey: boolean) => void
  onPlay?: () => void
  onTogglePlay?: () => void
  onEditTags?: () => void
  onPreviewSidecar?: (sidecar: FileEntry) => void
}) {
  const { t } = useTranslation('manage')
  const { file, playNumber, sidecars } = item
  const track = file.track
  const stem = fileStem(file.name)
  const showTagIdentity =
    !!track &&
    ((track.title && track.title.toLowerCase() !== stem.toLowerCase()) ||
      (track.artist && !stem.toLowerCase().includes(track.artist.toLowerCase())))

  const handleRowActivate = () => {
    if (isCurrent && onTogglePlay) {
      onTogglePlay()
      return
    }
    if (onPlay) {
      onPlay()
    }
  }

  const handleCheckboxClick = (e: MouseEvent<HTMLButtonElement>) => {
    e.stopPropagation()
    onToggleSelect?.(e.shiftKey)
  }

  const metaParts: string[] = []
  if (track) {
    if (showTagIdentity) {
      const tagSummary = [track.title, track.artist].filter(Boolean).join(' — ')
      if (tagSummary) metaParts.push(tagSummary)
    }
    const ext = (track.suffix || fileExtension(file.name)).toUpperCase()
    if (ext) metaParts.push(ext)
    const bitrate = formatBitrate(track.bitrate)
    if (bitrate) metaParts.push(bitrate)
    metaParts.push(formatBytes(file.size))
  } else {
    const ext = fileExtension(file.name).toUpperCase()
    if (ext) metaParts.push(ext)
    metaParts.push(formatBytes(file.size))
    metaParts.push(formatDate(file.mtime))
  }

  return (
    <div
      className={cn(
        'group flex min-h-13 items-center gap-1 rounded-md px-1 py-1 transition-colors hover:bg-accent/50 md:min-h-11 md:gap-1.5 md:px-2',
        selected && 'bg-primary/8 hover:bg-primary/12',
        isCurrent && !selected && 'bg-primary/[0.06]',
      )}
    >
      {/* Selection checkbox slot (44px touch target on mobile, aligned compact slot on desktop) */}
      {track && onToggleSelect ? (
        <span className="inline-flex size-11 shrink-0 items-center justify-center md:size-7">
          <Checkbox
            checked={selected}
            onClick={handleCheckboxClick}
            aria-label={t('table.selectTrack', { title: file.name })}
          />
        </span>
      ) : (
        <span className="size-11 shrink-0 md:size-7" aria-hidden="true" />
      )}

      {/* Leading play / track-index slot */}
      {track ? (
        <button
          type="button"
          onClick={handleRowActivate}
          aria-label={t(isCurrent && isPlaying ? 'folders.pauseTrack' : 'folders.playTrack', { name: file.name })}
          className="inline-flex size-8 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-primary/12 hover:text-primary focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
        >
          {isCurrent && isPlaying ? (
            <>
              <AudioLines className="size-4 text-primary motion-safe:animate-pulse group-hover:hidden" />
              <Pause className="hidden size-3.5 fill-current text-primary group-hover:block" />
            </>
          ) : isCurrent ? (
            <Play className="size-3.5 fill-current text-primary" />
          ) : (
            <>
              <span className="tnum text-xs text-muted-foreground group-hover:hidden">{playNumber}</span>
              <Play className="hidden size-3.5 fill-current text-foreground group-hover:block" />
            </>
          )}
        </button>
      ) : (
        <span className="inline-flex size-8 shrink-0 items-center justify-center text-muted-foreground">
          <FileAudio className="size-4" strokeWidth={1.75} />
        </span>
      )}

      {/* Main filename + metadata block */}
      <div className="min-w-0 flex-1 py-0.5">
        <div className="flex flex-wrap items-center gap-x-1.5 gap-y-0.5">
          {track ? (
            <button
              type="button"
              onClick={handleRowActivate}
              className={cn(
                'min-w-0 text-left text-sm leading-snug break-words [overflow-wrap:anywhere] focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
                clampNames && 'line-clamp-2',
                isCurrent ? 'font-medium text-primary' : 'text-foreground group-hover:text-foreground',
              )}
              title={file.name}
            >
              {file.name}
            </button>
          ) : (
            <span
              className={cn(
                'min-w-0 text-sm leading-snug break-words text-foreground [overflow-wrap:anywhere]',
                clampNames && 'line-clamp-2',
              )}
              title={file.name}
            >
              {file.name}
            </span>
          )}

          {sidecars.length > 0 && onPreviewSidecar ? (
            <button
              type="button"
              onClick={() => onPreviewSidecar(sidecars[0])}
              aria-label={t('folders.previewLyrics', { name: sidecars[0].name })}
              title={t('folders.previewLyrics', { name: sidecars[0].name })}
              className="inline-flex h-5 shrink-0 items-center gap-1 rounded border border-border/80 bg-muted/60 px-1.5 text-[10px] font-medium text-muted-foreground transition-colors hover:border-primary/40 hover:bg-primary/10 hover:text-primary focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
            >
              <Captions className="size-3" />
              <span>{t('folders.lyricsBadge')}</span>
            </button>
          ) : null}

          {!track ? (
            <span className="inline-flex h-5 shrink-0 items-center gap-1 rounded border border-amber-500/30 bg-amber-500/10 px-1.5 text-[10px] font-medium text-amber-700 dark:text-amber-300">
              <AlertCircle className="size-3" />
              <span>{t('folders.notIndexed')}</span>
            </span>
          ) : null}
        </div>

        <div className="tnum mt-0.5 truncate text-xs text-muted-foreground">{metaParts.join(' · ')}</div>
      </div>

      {/* Duration + quick tag editor + full per-track action menu */}
      {track ? (
        <span className="tnum shrink-0 px-1 text-xs text-muted-foreground">{formatDuration(track.duration)}</span>
      ) : null}

      {onEditTags ? (
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="hidden size-8 shrink-0 text-muted-foreground hover:text-foreground sm:inline-flex"
          aria-label={t('actions.editTags')}
          title={t('actions.editTags')}
          onClick={onEditTags}
        >
          <Tags className="size-4" />
        </Button>
      ) : null}

      {track ? <TrackActionsMenu tracks={[track]} className="shrink-0" /> : null}
    </div>
  )
}

export function FolderImageTile({
  libraryId,
  file,
  clampNames = true,
  onPreview,
}: {
  libraryId: number
  file: FileEntry
  clampNames?: boolean
  onPreview: () => void
}) {
  const [failed, setFailed] = useState(false)
  const src = api.manage.folders.imageUrl(libraryId, file.path, file.mtime)

  return (
    <button
      type="button"
      onClick={onPreview}
      title={file.name}
      className="group flex flex-col overflow-hidden rounded-lg border border-border/75 bg-card/50 text-left transition hover:border-primary/40 hover:bg-accent/40 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
    >
      <div className="relative aspect-square w-full overflow-hidden bg-muted/40">
        {failed ? (
          <div className="flex size-full items-center justify-center text-muted-foreground">
            <ImageIcon className="size-6" strokeWidth={1.5} />
          </div>
        ) : (
          <img
            src={src}
            alt={file.name}
            loading="lazy"
            decoding="async"
            onError={() => setFailed(true)}
            className="size-full object-cover transition-transform duration-200 group-hover:scale-[1.03]"
          />
        )}
      </div>
      <div className="min-w-0 px-2 py-1.5">
        <div
          className={cn(
            'text-xs leading-tight font-medium break-words [overflow-wrap:anywhere]',
            clampNames ? 'line-clamp-1' : 'line-clamp-3',
          )}
        >
          {file.name}
        </div>
        <div className="tnum mt-0.5 text-[11px] text-muted-foreground">{formatBytes(file.size)}</div>
      </div>
    </button>
  )
}

export function FolderDocRow({
  file,
  clampNames = true,
  onPreview,
}: {
  file: FileEntry
  clampNames?: boolean
  onPreview?: () => void
}) {
  const ext = fileExtension(file.name)
  const isLyrics = isLyricsSidecarFile(file.name)
  const Icon = isLyrics ? Captions : onPreview ? FileText : ARCHIVE_EXTS.has(ext) ? FileArchive : File
  const details = [ext ? ext.toUpperCase() : null, formatBytes(file.size), formatDate(file.mtime)]
    .filter(Boolean)
    .join(' · ')

  const content = (
    <>
      <span className="inline-flex size-8 shrink-0 items-center justify-center rounded-md bg-muted/60 text-muted-foreground">
        <Icon className="size-4" strokeWidth={1.75} />
      </span>
      <span className="min-w-0 flex-1">
        <span
          className={cn(
            'block text-sm leading-snug break-words [overflow-wrap:anywhere]',
            clampNames && 'line-clamp-2',
          )}
          title={file.name}
        >
          {file.name}
        </span>
        <span className="tnum mt-0.5 block truncate text-xs text-muted-foreground">{details}</span>
      </span>
      {onPreview ? (
        <Eye className="size-4 shrink-0 text-muted-foreground/70 transition-opacity sm:opacity-0 sm:group-hover:opacity-100" />
      ) : null}
    </>
  )

  if (onPreview) {
    return (
      <button
        type="button"
        onClick={onPreview}
        className="group flex min-h-12 w-full items-center gap-2.5 rounded-md px-2 py-1.5 text-left transition-colors hover:bg-accent/50 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
      >
        {content}
      </button>
    )
  }

  return <div className="flex min-h-12 items-center gap-2.5 rounded-md px-2 py-1.5">{content}</div>
}
