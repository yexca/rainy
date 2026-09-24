import { useMutation, useQueryClient } from '@tanstack/react-query'
import { ImagePlus } from 'lucide-react'
import { useEffect, useRef, useState, type DragEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Spinner } from '@/components/spinner'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { api } from '@/lib/api/endpoints'
import { formatBytes } from '@/lib/format'
import { cn } from '@/lib/utils'

import { reportBatch, toastError } from '../lib/batch'
import { invalidateLibrary } from '../queries'
import { ResponsiveDialog } from './responsive-dialog'

const MAX_IMAGE_BYTES = 20 * 1024 * 1024

export interface CoverDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  trackIds: readonly string[]
  onDone?: () => void
}

/** Set or remove the cover of many tracks at once (without loading every tag set). */
export function CoverDialog({ open, onOpenChange, trackIds, onDone }: CoverDialogProps) {
  const { t } = useTranslation('manage')
  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('coverDialog.title')}
      description={t('coverDialog.description', { count: trackIds.length })}
    >
      {open ? <CoverBody trackIds={trackIds} onClose={() => onOpenChange(false)} onDone={onDone} /> : null}
    </ResponsiveDialog>
  )
}

function CoverBody({ trackIds, onClose, onDone }: { trackIds: readonly string[]; onClose: () => void; onDone?: () => void }) {
  const { t } = useTranslation('manage')
  const queryClient = useQueryClient()
  const inputRef = useRef<HTMLInputElement>(null)
  const [mode, setMode] = useState<'set' | 'remove'>('set')
  const [file, setFile] = useState<File | null>(null)
  const [embed, setEmbed] = useState(true)
  const [saveToFolder, setSaveToFolder] = useState(false)
  const [removeFolderImage, setRemoveFolderImage] = useState(false)
  const [dragging, setDragging] = useState(false)
  const [preview, setPreview] = useState<string | null>(null)

  useEffect(
    () => () => {
      if (preview) URL.revokeObjectURL(preview)
    },
    [preview],
  )

  const choose = (f: File | undefined | null) => {
    if (!f || !f.type.startsWith('image/')) {
      toast.error(t('cover.notImage'))
      return
    }
    if (f.size > MAX_IMAGE_BYTES) {
      toast.error(t('cover.tooLarge', { size: formatBytes(MAX_IMAGE_BYTES) }))
      return
    }
    setFile(f)
    setPreview(URL.createObjectURL(f))
  }

  const run = useMutation({
    mutationFn: () =>
      mode === 'set' && file
        ? api.manage.cover.set({ file, trackIds: [...trackIds], embed, saveToFolder })
        : api.manage.cover.remove({ trackIds: [...trackIds], removeFolderImage }),
    onSuccess: (result) => {
      reportBatch(result, mode === 'set' ? 'manage:coverDialog.done' : 'manage:coverDialog.removed')
      void invalidateLibrary(queryClient)
      onDone?.()
      onClose()
    },
    onError: toastError,
  })

  const onDrop = (e: DragEvent) => {
    e.preventDefault()
    setDragging(false)
    choose(Array.from(e.dataTransfer.files).find((f) => f.type.startsWith('image/')))
  }

  const canRun = mode === 'remove' || (!!file && (embed || saveToFolder))

  return (
    <div className="grid gap-4" onPaste={(e) => choose(Array.from(e.clipboardData.files).find((f) => f.type.startsWith('image/')))}>
      <ToggleGroup type="single" variant="outline" value={mode} onValueChange={(v) => v && setMode(v as 'set' | 'remove')} className="w-full">
        <ToggleGroupItem value="set" className="flex-1">
          {t('coverDialog.set')}
        </ToggleGroupItem>
        <ToggleGroupItem value="remove" className="flex-1">
          {t('coverDialog.remove')}
        </ToggleGroupItem>
      </ToggleGroup>

      {mode === 'set' ? (
        <>
          <button
            type="button"
            onClick={() => inputRef.current?.click()}
            onDragOver={(e) => {
              e.preventDefault()
              setDragging(true)
            }}
            onDragLeave={() => setDragging(false)}
            onDrop={onDrop}
            className={cn(
              'flex items-center gap-4 rounded-xl border border-dashed p-4 text-left transition-colors hover:bg-accent/40',
              dragging && 'border-primary bg-primary/5',
            )}
          >
            <span className="grid size-24 shrink-0 place-items-center overflow-hidden rounded-lg bg-muted">
              {preview ? <img src={preview} alt="" className="size-full object-cover" /> : <ImagePlus className="size-7 text-muted-foreground" strokeWidth={1.5} />}
            </span>
            <span className="grid min-w-0 gap-1 text-sm">
              <span className="font-medium">{file ? file.name : t('cover.choose')}</span>
              <span className="text-xs text-muted-foreground">{file ? formatBytes(file.size) : t('cover.dropHint')}</span>
            </span>
          </button>
          <input
            ref={inputRef}
            type="file"
            accept="image/jpeg,image/png,image/webp,image/gif"
            className="hidden"
            onChange={(e) => {
              choose(e.target.files?.[0])
              e.target.value = ''
            }}
          />
          <div className="grid gap-2.5">
            <Label className="flex items-center gap-3 font-normal">
              <Checkbox checked={embed} onCheckedChange={(v) => setEmbed(v === true)} />
              {t('cover.embed')}
            </Label>
            <Label className="flex items-center gap-3 font-normal">
              <Checkbox checked={saveToFolder} onCheckedChange={(v) => setSaveToFolder(v === true)} />
              {t('cover.saveToFolder')}
            </Label>
          </div>
        </>
      ) : (
        <div className="grid gap-3">
          <p className="text-sm text-muted-foreground">{t('coverDialog.removeHint')}</p>
          <Label className="flex items-center gap-3 font-normal">
            <Checkbox checked={removeFolderImage} onCheckedChange={(v) => setRemoveFolderImage(v === true)} />
            {t('cover.removeFolderImage')}
          </Label>
        </div>
      )}

      <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end [&_button]:max-sm:h-11">
        <Button variant="outline" onClick={onClose} disabled={run.isPending}>
          {t('common:actions.cancel')}
        </Button>
        <Button variant={mode === 'remove' ? 'destructive' : 'default'} onClick={() => run.mutate()} disabled={!canRun || run.isPending}>
          {run.isPending ? <Spinner size="sm" className="text-current" /> : null}
          {mode === 'set' ? t('coverDialog.apply', { count: trackIds.length }) : t('coverDialog.removeApply', { count: trackIds.length })}
        </Button>
      </div>
    </div>
  )
}
