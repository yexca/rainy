import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { api } from '@/lib/api/endpoints'
import type { Playlist, PlaylistImportPreview } from '@/lib/api/types'
import { errorMessage } from '@/lib/errors'
import { queryKeys } from '@/lib/query-keys'

import { ResponsiveDialog } from './responsive-dialog'

export function PlaylistImportDialog({ open, onOpenChange, onSaved }: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSaved: (playlist: Playlist) => void
}) {
  const { t } = useTranslation('library')
  return (
    <ResponsiveDialog open={open} onOpenChange={onOpenChange} title={t('playlistFiles.import')} description={t('playlistFiles.description')} className="md:max-h-[92dvh] md:overflow-y-auto">
      {open ? <ImportForm onSaved={(playlist) => { onOpenChange(false); onSaved(playlist) }} onCancel={() => onOpenChange(false)} /> : null}
    </ResponsiveDialog>
  )
}

function ImportForm({ onSaved, onCancel }: { onSaved: (playlist: Playlist) => void; onCancel: () => void }) {
  const { t } = useTranslation('library')
  const id = useId()
  const queryClient = useQueryClient()
  const [text, setText] = useState('')
  const [name, setName] = useState('')
  const [baseDir, setBaseDir] = useState('')
  const [libraryId, setLibraryId] = useState(0)
  const [libraries, setLibraries] = useState<PlaylistImportPreview['libraries']>([])
  const [reading, setReading] = useState(false)
  const [skip, setSkip] = useState(false)
  const [visible, setVisible] = useState(100)
  const preview = useMutation({
    mutationFn: () => api.playlists.previewImport({ text, baseDir, libraryId }),
    onSuccess: (result) => { setLibraries(result.libraries); setSkip(false); setVisible(100) },
    onError: (error) => toast.error(errorMessage(error, t)),
  })
  const entries = preview.data?.entries ?? []
  const matched = entries.filter((entry) => entry.status === 'matched')
  const skipped = entries.length - matched.length
  const create = useMutation({
    mutationFn: () => api.playlists.create({ name: name.trim(), trackIds: matched.map((entry) => entry.trackId) }),
    onSuccess: (playlist) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.playlists })
      toast.success(t('toast.playlistCreated', { name: playlist.name }))
      onSaved(playlist)
    },
    onError: (error) => toast.error(errorMessage(error, t)),
  })
  const busy = reading || preview.isPending || create.isPending
  const readFile = async (file: File | undefined) => {
    preview.reset()
    setText('')
    if (!file) return
    if (file.size > 512 * 1024) { toast.error(t('playlistFiles.fileError')); return }
    setReading(true)
    try {
      const content = new TextDecoder('utf-8', { fatal: true }).decode(await file.arrayBuffer())
      setText(content)
      setName(file.name.replace(/\.m3u8?$/i, '').slice(0, 200))
    } catch {
      toast.error(t('playlistFiles.fileError'))
    } finally {
      setReading(false)
    }
  }
  return (
    <div className="grid min-w-0 gap-4">
      <div className="grid gap-2">
        <Label htmlFor={id + '-file'}>{t('playlistFiles.file')}</Label>
        <Input id={id + '-file'} type="file" accept=".m3u,.m3u8" disabled={busy} className="h-11" onChange={(event) => void readFile(event.target.files?.[0])} />
      </div>
      <div className="grid gap-2">
        <Label htmlFor={id + '-name'}>{t('playlist.name')}</Label>
        <Input id={id + '-name'} value={name} maxLength={200} disabled={create.isPending} className="h-11" onChange={(event) => setName(event.target.value)} />
      </div>
      <div className="grid gap-2">
        <Label htmlFor={id + '-folder'}>{t('playlistFiles.folder')}</Label>
        <Input id={id + '-folder'} value={baseDir} disabled={busy} className="h-11" onChange={(event) => { setBaseDir(event.target.value); preview.reset() }} />
        <p className="text-xs text-muted-foreground">{t('playlistFiles.folderHint')}</p>
      </div>
      {libraries.length > 1 ? (
        <div className="grid gap-2">
          <Label htmlFor={id + '-library'}>{t('playlistFiles.library')}</Label>
          <select id={id + '-library'} value={libraryId} disabled={busy} className="h-11 w-full rounded-md border bg-background px-3 text-sm" onChange={(event) => { setLibraryId(Number(event.target.value)); preview.reset() }}>
            <option value={0}>{t('playlistFiles.allLibraries')}</option>
            {libraries.map((library) => <option key={library.id} value={library.id}>{library.name}</option>)}
          </select>
        </div>
      ) : null}
      <Button variant="outline" className="h-11" disabled={!text || busy} onClick={() => preview.mutate()}>{t('playlistFiles.preview')}</Button>
      {preview.data ? (
        <div className="grid gap-3" aria-live="polite">
          <p className="text-sm">{t('playlistFiles.summary', { matched: matched.length, skipped })}</p>
          {entries.length > 0 ? (
            <ol className="max-h-40 overflow-y-auto rounded-md border text-xs">
              {entries.slice(0, visible).map((entry) => (
                <li key={entry.line} className="flex gap-2 border-b p-2 last:border-0">
                  <span className="shrink-0 text-muted-foreground">{entry.line}</span>
                  <span className="min-w-0 flex-1 break-all">{entry.path}</span>
                  <span className={entry.status === 'matched' ? 'text-muted-foreground' : 'text-destructive'}>{t(`playlistFiles.status.${entry.status}`)}</span>
                </li>
              ))}
            </ol>
          ) : null}
          {entries.length > visible ? <Button variant="ghost" className="h-11" onClick={() => setVisible(visible + 100)}>{t('playlistFiles.showMore')}</Button> : null}
          {skipped > 0 ? (
            <label className="flex min-h-11 items-center gap-3 text-sm">
              <input type="checkbox" checked={skip} disabled={create.isPending} onChange={(event) => setSkip(event.target.checked)} className="size-5 shrink-0 accent-primary" />
              {t('playlistFiles.skip', { count: skipped })}
            </label>
          ) : null}
        </div>
      ) : null}
      <div className="flex justify-end gap-2 pb-1">
        <Button variant="outline" className="h-11" disabled={create.isPending} onClick={onCancel}>{t('common:actions.cancel')}</Button>
        <Button className="h-11" disabled={busy || !name.trim() || matched.length === 0 || (skipped > 0 && !skip)} onClick={() => create.mutate()}>{t('playlistFiles.create')}</Button>
      </div>
    </div>
  )
}
