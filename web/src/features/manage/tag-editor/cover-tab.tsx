import { ImagePlus, ImageOff, Trash2, Undo2 } from 'lucide-react'
import { useRef, useState, type ClipboardEvent, type DragEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { CoverArt } from '@/components/cover-art'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { api } from '@/lib/api/endpoints'
import type { TrackTags } from '@/lib/api/types'
import { formatBytes } from '@/lib/format'
import { cn } from '@/lib/utils'

import type { CoverScope, PendingCover } from './types'

const MAX_IMAGE_BYTES = 20 * 1024 * 1024
const PREVIEW = 232

export interface CoverTabProps {
  items: readonly TrackTags[]
  pending: PendingCover | null
  onPendingChange: (pending: PendingCover | null) => void
  disabled?: boolean
}

/** Pick the first image file from a drop / paste / file input. */
function firstImage(files: FileList | readonly File[] | null | undefined): File | null {
  if (!files) return null
  for (const file of Array.from(files)) if (file.type.startsWith('image/')) return file
  return null
}

export function CoverTab({ items, pending, onPendingChange, disabled }: CoverTabProps) {
  const { t } = useTranslation('manage')
  const inputRef = useRef<HTMLInputElement>(null)
  const [dragging, setDragging] = useState(false)
  const [pictureFailed, setPictureFailed] = useState(false)
  // Remember the scope choice even before an image is picked.
  const [idleScope, setIdleScope] = useState<CoverScope>('tracks')

  const first = items[0]
  const albumId = first?.track.albumId ?? ''
  const sameAlbum = !!albumId && items.every((i) => i.track.albumId === albumId)
  const withPicture = items.filter((i) => i.pictures.length > 0).length
  const effectiveScope: CoverScope = pending?.scope ?? idleScope

  const choose = (file: File | null) => {
    if (!file) {
      toast.error(t('cover.notImage'))
      return
    }
    if (file.size > MAX_IMAGE_BYTES) {
      toast.error(t('cover.tooLarge', { size: formatBytes(MAX_IMAGE_BYTES) }))
      return
    }
    // The editor revokes replaced preview URLs.
    onPendingChange({
      kind: 'set',
      file,
      previewUrl: URL.createObjectURL(file),
      scope: effectiveScope,
      embed: pending?.kind === 'set' ? pending.embed : true,
      saveToFolder: pending?.kind === 'set' ? pending.saveToFolder : false,
    })
  }

  const clear = () => onPendingChange(null)

  const onDrop = (e: DragEvent) => {
    e.preventDefault()
    setDragging(false)
    if (disabled) return
    choose(firstImage(e.dataTransfer.files))
  }

  const onPaste = (e: ClipboardEvent) => {
    if (disabled) return
    const file = firstImage(Array.from(e.clipboardData.items).flatMap((i) => {
        const f = i.kind === 'file' ? i.getAsFile() : null
        return f ? [f] : []
      }))
    if (file) {
      e.preventDefault()
      choose(file)
    }
  }

  const setScope = (next: CoverScope) => {
    setIdleScope(next)
    if (pending) onPendingChange({ ...pending, scope: next })
  }

  let preview
  if (pending?.kind === 'set') {
    preview = <img src={pending.previewUrl} alt="" className="size-full object-cover" />
  } else if (pending?.kind === 'remove') {
    preview = (
      <div className="grid size-full place-items-center bg-muted text-muted-foreground">
        <div className="flex flex-col items-center gap-2 text-sm">
          <ImageOff className="size-8" strokeWidth={1.5} />
          {t('cover.willRemove')}
        </div>
      </div>
    )
  } else if (first && first.pictures.length > 0 && !pictureFailed) {
    preview = (
      <img
        src={api.manage.pictureUrl(first.track.id, first.track.updatedAt)}
        alt=""
        className="size-full object-cover"
        onError={() => setPictureFailed(true)}
      />
    )
  } else {
    preview = <CoverArt coverArt={first?.track.coverArt} size={PREVIEW} flat rounded="none" fluid className="size-full" />
  }

  return (
    <div className="grid gap-5 pb-2" onPaste={onPaste}>
      <div
        onDragOver={(e) => {
          e.preventDefault()
          if (!disabled) setDragging(true)
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={onDrop}
        className={cn(
          'flex flex-col items-center gap-4 rounded-xl border border-dashed p-5 transition-colors sm:flex-row sm:items-start',
          dragging ? 'border-primary bg-primary/5' : 'border-border',
        )}
      >
        <div
          className="relative aspect-square w-[min(100%,232px)] shrink-0 overflow-hidden rounded-lg bg-muted shadow-sm ring-1 ring-black/5 dark:ring-white/10"
          style={{ maxWidth: PREVIEW }}
        >
          {preview}
        </div>
        <div className="flex min-w-0 flex-1 flex-col gap-3 text-center sm:text-left">
          <div className="text-sm text-muted-foreground">
            {pending?.kind === 'set' ? (
              <p className="break-all">
                <span className="font-medium text-foreground">{pending.file.name}</span> · {formatBytes(pending.file.size)}
              </p>
            ) : items.length > 1 ? (
              <p>{t('cover.embeddedCount', { count: withPicture, total: items.length })}</p>
            ) : (
              <p>{withPicture > 0 ? t('cover.hasEmbedded') : t('cover.noEmbedded')}</p>
            )}
            <p className="mt-1 hidden text-xs sm:block">{t('cover.dropHint')}</p>
          </div>
          <div className="flex flex-wrap justify-center gap-2 sm:justify-start">
            <Button type="button" variant="outline" size="sm" onClick={() => inputRef.current?.click()} disabled={disabled}>
              <ImagePlus />
              {t('cover.choose')}
            </Button>
            {pending ? (
              <Button type="button" variant="ghost" size="sm" onClick={clear}>
                <Undo2 />
                {t('cover.undo')}
              </Button>
            ) : (
              <Button
                type="button"
                variant="ghost"
                size="sm"
                className="text-destructive hover:text-destructive"
                disabled={disabled || (withPicture === 0 && effectiveScope === 'tracks')}
                onClick={() => onPendingChange({ kind: 'remove', scope: effectiveScope, removeFolderImage: false })}
              >
                <Trash2 />
                {t('cover.remove')}
              </Button>
            )}
          </div>
          <input
            ref={inputRef}
            type="file"
            accept="image/jpeg,image/png,image/webp,image/gif"
            className="hidden"
            onChange={(e) => {
              choose(firstImage(e.target.files))
              e.target.value = ''
            }}
          />
        </div>
      </div>

      <fieldset className="grid gap-3" disabled={disabled}>
        <legend className="mb-2 text-[13px] font-medium text-foreground/75">{t('cover.applyTo')}</legend>
        <RadioGroup value={effectiveScope} onValueChange={(v) => setScope(v as CoverScope)} className="gap-2.5">
          <Label className="flex items-center gap-3 font-normal">
            <RadioGroupItem value="tracks" />
            {t('cover.scopeTracks', { count: items.length })}
          </Label>
          <Label className={cn('flex items-center gap-3 font-normal', !sameAlbum && 'opacity-50')}>
            <RadioGroupItem value="album" disabled={!sameAlbum} />
            <span className="min-w-0 truncate">
              {sameAlbum ? t('cover.scopeAlbum', { album: first?.track.album || '—' }) : t('cover.scopeAlbumDisabled')}
            </span>
          </Label>
        </RadioGroup>

        {pending?.kind === 'set' ? (
          <div className="mt-1 grid gap-2.5">
            <Label className="flex items-center gap-3 font-normal">
              <Checkbox checked={pending.embed} onCheckedChange={(v) => onPendingChange({ ...pending, embed: v === true })} />
              {t('cover.embed')}
            </Label>
            <Label className="flex items-center gap-3 font-normal">
              <Checkbox
                checked={pending.saveToFolder}
                onCheckedChange={(v) => onPendingChange({ ...pending, saveToFolder: v === true })}
              />
              {t('cover.saveToFolder')}
            </Label>
            {!pending.embed && !pending.saveToFolder ? (
              <p className="text-xs text-destructive">{t('cover.nothingSelected')}</p>
            ) : null}
          </div>
        ) : null}
        {pending?.kind === 'remove' ? (
          <Label className="mt-1 flex items-center gap-3 font-normal">
            <Checkbox
              checked={pending.removeFolderImage}
              onCheckedChange={(v) => onPendingChange({ ...pending, removeFolderImage: v === true })}
            />
            {t('cover.removeFolderImage')}
          </Label>
        ) : null}
      </fieldset>

      {first && first.pictures.length > 0 ? (
        <div className="grid gap-1.5">
          <p className="text-[13px] font-medium text-foreground/75">{t('cover.pictures')}</p>
          <ul className="grid gap-1 text-xs text-muted-foreground">
            {first.pictures.map((p, i) => (
              <li key={i} className="flex gap-2">
                <span className="font-medium text-foreground/80">{p.type || t('cover.pictureType')}</span>
                <span>{p.mimeType}</span>
                {p.description ? <span className="truncate">“{p.description}”</span> : null}
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </div>
  )
}
