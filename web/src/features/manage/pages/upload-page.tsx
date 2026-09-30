import { useWindowVirtualizer } from '@tanstack/react-virtual'
import {
  CircleAlert,
  CircleCheck,
  FileAudio,
  FolderOpen,
  FolderUp,
  RotateCw,
  Tags,
  Upload,
  X,
} from 'lucide-react'
import { useRef, useState, type CSSProperties, type DragEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useSearchParams } from 'react-router'
import { toast } from 'sonner'

import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Progress } from '@/components/ui/progress'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { useLibraries, useDefaultRenamePattern } from '@/features/admin/queries'
import { useIsMobile } from '@/hooks/use-media-query'
import { formatBytes, formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useUI } from '@/stores/ui'

import { FolderPicker } from '../components/folder-picker'
import { LinkDownload } from '../components/link-download'
import { toastError } from '../lib/batch'
import { filesFromDataTransfer, filesFromInput, isUploadable, type PickedFile } from '../lib/upload-files'
import { useUploadQueue, type UploadEntry } from '../lib/upload-queue'
import { useScrollMargin } from '../lib/use-scroll-margin'

const ORGANIZE_KEY = 'rainy.manage.uploadOrganize'

function loadOrganize(): boolean {
  try {
    return localStorage.getItem(ORGANIZE_KEY) === '1'
  } catch {
    return false
  }
}

export default function UploadPage() {
  const { t } = useTranslation('manage')
  const [searchParams] = useSearchParams()
  const { libraries, limited } = useLibraries()
  const renamePattern = useDefaultRenamePattern()
  const add = useUploadQueue((s) => s.add)
  const isMobile = useIsMobile()

  const urlLib = Number.parseInt(searchParams.get('libraryId') ?? '', 10)
  const [libraryId, setLibraryId] = useState<number | null>(Number.isFinite(urlLib) ? urlLib : null)
  const [dir, setDir] = useState(searchParams.get('dir') ?? '')
  const [organize, setOrganize] = useState(loadOrganize)
  const [pickerOpen, setPickerOpen] = useState(false)
  const [dragging, setDragging] = useState(false)
  const filesRef = useRef<HTMLInputElement>(null)
  const folderRef = useRef<HTMLInputElement>(null)
  const dragDepth = useRef(0)

  const effectiveLibraryId = libraryId ?? libraries[0]?.id ?? 1
  const library = libraries.find((l) => l.id === effectiveLibraryId)
  const cleanDir = dir.trim().replace(/\\/g, '/').replace(/^\/+|\/+$/g, '')
  const invalidDir = cleanDir.split('/').some((p) => p === '..' || p === '.')

  const enqueue = (files: PickedFile[]) => {
    if (files.length === 0) return
    if (invalidDir) {
      toast.error(t('upload.invalidDir'))
      return
    }
    const skipped = files.filter((f) => !isUploadable(f.path)).length
    if (skipped === files.length) {
      toast.error(t('upload.nothingToUpload'), { description: t('upload.audioOnly') })
      return
    }
    add(files, { libraryId: effectiveLibraryId, dir: cleanDir, organize })
    toast(t('upload.queued', { count: files.length - skipped }), {
      description: skipped > 0 ? t('upload.skippedCount', { count: skipped }) : undefined,
    })
  }

  const onDrop = async (e: DragEvent) => {
    e.preventDefault()
    dragDepth.current = 0
    setDragging(false)
    try {
      enqueue(await filesFromDataTransfer(e.dataTransfer))
    } catch (error) {
      toastError(error)
    }
  }

  return (
    <Page>
      <PageHeader title={t('upload.title')} subtitle={t('upload.subtitle')} back={isMobile ? '/manage' : undefined} />

      <div className="mx-auto grid max-w-3xl gap-6">
        <section className="grid gap-4 rounded-xl border p-4 sm:p-5">
          <h2 className="text-sm font-semibold">{t('upload.target')}</h2>
          {!limited && libraries.length > 1 ? (
            <div className="grid gap-1.5">
              <Label className="text-[13px] font-medium text-foreground/75">{t('filters.library')}</Label>
              <Select value={String(effectiveLibraryId)} onValueChange={(v) => setLibraryId(Number(v))}>
                <SelectTrigger className="w-full sm:w-72">
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
            </div>
          ) : null}
          <div className="grid gap-1.5">
            <Label htmlFor="upload-dir" className="text-[13px] font-medium text-foreground/75">
              {t('upload.folder')}
            </Label>
            <div className="flex gap-2">
              <Input
                id="upload-dir"
                value={dir}
                onChange={(e) => setDir(e.target.value)}
                placeholder={t('upload.folderPlaceholder')}
                aria-invalid={invalidDir || undefined}
                className="font-mono text-sm"
                autoComplete="off"
                spellCheck={false}
              />
              <Button variant="outline" onClick={() => setPickerOpen(true)} className="shrink-0">
                <FolderOpen />
                <span className="max-sm:sr-only">{t('upload.browse')}</span>
              </Button>
            </div>
            {invalidDir ? (
              <p className="text-xs text-destructive">{t('upload.invalidDir')}</p>
            ) : organize ? (
              <p className="text-xs text-muted-foreground">{t('upload.folderOrganizeHint')}</p>
            ) : null}
          </div>
          <Label className="flex items-start gap-3 font-normal">
            <Switch
              className="mt-0.5"
              checked={organize}
              onCheckedChange={(v) => {
                setOrganize(v)
                try {
                  localStorage.setItem(ORGANIZE_KEY, v ? '1' : '0')
                } catch {
                  // not remembered
                }
              }}
            />
            <span className="grid gap-1">
              <span className="text-sm font-medium">{t('upload.organize')}</span>
              <span className="text-xs text-muted-foreground">
                {t('upload.organizeHint')} <code className="font-mono break-all">{renamePattern}</code>
              </span>
            </span>
          </Label>
        </section>

        <section
          onDragEnter={(e) => {
            e.preventDefault()
            dragDepth.current++
            setDragging(true)
          }}
          onDragOver={(e) => e.preventDefault()}
          onDragLeave={() => {
            dragDepth.current = Math.max(0, dragDepth.current - 1)
            if (dragDepth.current === 0) setDragging(false)
          }}
          onDrop={(e) => void onDrop(e)}
          className={cn(
            'flex flex-col items-center gap-4 rounded-2xl border-2 border-dashed px-6 py-10 text-center transition-colors sm:py-14',
            dragging ? 'border-primary bg-primary/5' : 'border-border',
          )}
        >
          <div className="grid size-14 place-items-center rounded-2xl bg-primary/10 text-primary">
            <Upload className="size-6" strokeWidth={1.75} />
          </div>
          <div className="grid gap-1">
            <p className="text-base font-semibold">{t('upload.dropTitle')}</p>
            <p className="text-sm text-muted-foreground">{t('upload.dropHint')}</p>
          </div>
          <div className="flex flex-wrap justify-center gap-2">
            <Button onClick={() => filesRef.current?.click()} className="max-sm:h-11">
              <FileAudio />
              {t('upload.chooseFiles')}
            </Button>
            <Button variant="outline" onClick={() => folderRef.current?.click()} className="max-sm:h-11">
              <FolderUp />
              {t('upload.chooseFolder')}
            </Button>
          </div>
          <input
            ref={filesRef}
            type="file"
            multiple
            accept="audio/*,.flac,.ape,.wv,.mpc,.dsf,.dff,.tta,.opus,.m4a,.m4b,.wma,.aif,.aiff,.spx"
            className="hidden"
            onChange={(e) => {
              enqueue(filesFromInput(e.target.files))
              e.target.value = ''
            }}
          />
          <input
            ref={(el) => {
              folderRef.current = el
              el?.setAttribute('webkitdirectory', '')
            }}
            type="file"
            multiple
            className="hidden"
            onChange={(e) => {
              enqueue(filesFromInput(e.target.files))
              e.target.value = ''
            }}
          />
        </section>

        <LinkDownload libraryId={effectiveLibraryId} dir={cleanDir} organize={organize} invalidDir={invalidDir} />

        <UploadQueue />
      </div>

      <FolderPicker
        open={pickerOpen}
        onOpenChange={setPickerOpen}
        libraryId={effectiveLibraryId}
        libraryName={library?.name ?? ''}
        initialDir={cleanDir}
        onSelect={setDir}
      />
    </Page>
  )
}

function UploadQueue() {
  const { t } = useTranslation('manage')
  const entries = useUploadQueue((s) => s.entries)
  const { cancelAll, retryFailed, clearFinished } = useUploadQueue.getState()
  const [listRef, scrollMargin] = useScrollMargin<HTMLUListElement>()
  const virtualizer = useWindowVirtualizer({ count: entries.length, estimateSize: () => 56, overscan: 10, scrollMargin })

  const activeCount = entries.filter((e) => e.status === 'queued' || e.status === 'uploading').length

  if (entries.length === 0) return null

  const relevant = entries
  const done = relevant.filter((e) => e.status === 'done').length
  const failed = relevant.filter((e) => e.status === 'error' || e.status === 'canceled').length
  const totalBytes = relevant.reduce((sum, e) => sum + e.size, 0)
  const loadedBytes = relevant.reduce((sum, e) => sum + (e.status === 'done' ? e.size : e.status === 'uploading' ? e.loaded : 0), 0)

  return (
    <section className="grid gap-3">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <div className="min-w-0 flex-1">
          <h2 className="text-sm font-semibold">{t('upload.queue')}</h2>
          <p className="tnum text-xs text-muted-foreground">
            {t('upload.progress', { done: formatNumber(done), total: formatNumber(relevant.length) })} ·{' '}
            {formatBytes(loadedBytes)} / {formatBytes(totalBytes)}
          </p>
        </div>
        <div className="flex flex-wrap gap-1">
          {failed > 0 ? (
            <Button variant="ghost" size="sm" onClick={retryFailed}>
              <RotateCw />
              {t('upload.retryFailed')}
            </Button>
          ) : null}
          {activeCount > 0 ? (
            <Button variant="ghost" size="sm" onClick={cancelAll}>
              <X />
              {t('upload.cancelAll')}
            </Button>
          ) : null}
          {entries.length > activeCount ? (
            <Button variant="ghost" size="sm" onClick={clearFinished}>
              {t('upload.clearFinished')}
            </Button>
          ) : null}
        </div>
      </div>
      {activeCount > 0 ? <Progress value={totalBytes > 0 ? (loadedBytes / totalBytes) * 100 : 0} className="h-1" /> : null}
      <ul ref={listRef} className="relative rounded-xl border" style={{ height: virtualizer.getTotalSize() }}>
        {virtualizer.getVirtualItems().map((item) => (
          <UploadRow
            key={entries[item.index].id}
            entry={entries[item.index]}
            style={{ height: item.size, transform: `translateY(${item.start - scrollMargin}px)` }}
          />
        ))}
      </ul>
    </section>
  )
}

function UploadRow({ entry, style }: { entry: UploadEntry; style: CSSProperties }) {
  const { t } = useTranslation('manage')
  const openTagEditor = useUI((s) => s.openTagEditor)
  const { cancel, retry, remove } = useUploadQueue.getState()
  const fraction = entry.size > 0 ? entry.loaded / entry.size : 0

  return (
    <li className="absolute inset-x-0 top-0 flex items-center gap-3 border-b px-3 last:border-b-0" style={style}>
      <FileAudio className="size-5 shrink-0 text-muted-foreground" strokeWidth={1.75} />
      <div className="grid min-w-0 flex-1 gap-1">
        <p className="truncate text-sm" title={entry.path}>
          {entry.path}
        </p>
        {entry.status === 'uploading' ? (
          <Progress value={fraction * 100} className="h-1" />
        ) : (
          <p
            className={cn(
              'truncate text-xs',
              entry.status === 'error' ? 'text-destructive' : 'text-muted-foreground',
            )}
            title={entry.error}
          >
            {formatBytes(entry.size)} · {entry.status === 'error' ? entry.error || t('upload.status.error') : t(`upload.status.${entry.status}`)}
          </p>
        )}
      </div>
      {entry.status === 'done' ? (
        <>
          {entry.trackIds.length > 0 ? (
            <Button variant="ghost" size="icon-sm" aria-label={t('actions.editTags')} onClick={() => openTagEditor(entry.trackIds)}>
              <Tags />
            </Button>
          ) : null}
          <CircleCheck className="size-5 shrink-0 text-emerald-500" aria-label={t('upload.status.done')} />
        </>
      ) : null}
      {entry.status === 'error' || entry.status === 'canceled' ? (
        <>
          <CircleAlert className="size-4 shrink-0 text-destructive" aria-hidden />
          <Button variant="ghost" size="icon-sm" aria-label={t('upload.retry')} onClick={() => retry(entry.id)}>
            <RotateCw />
          </Button>
        </>
      ) : null}
      {entry.status === 'queued' || entry.status === 'uploading' ? (
        <Button variant="ghost" size="icon-sm" aria-label={t('common:actions.cancel')} onClick={() => cancel(entry.id)}>
          <X />
        </Button>
      ) : (
        <Button variant="ghost" size="icon-sm" className="text-muted-foreground" aria-label={t('upload.removeEntry')} onClick={() => remove(entry.id)}>
          <X />
        </Button>
      )}
    </li>
  )
}
