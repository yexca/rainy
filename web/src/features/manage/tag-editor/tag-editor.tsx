import { useQueryClient } from '@tanstack/react-query'
import {
  CaseSensitive,
  ChevronDown,
  Copy,
  Eraser,
  FileText,
  ListOrdered,
  Lock,
  Replace,
  TriangleAlert,
  Wand2,
  X,
} from 'lucide-react'
import { useEffect, useMemo, useState, type KeyboardEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { CoverArt } from '@/components/cover-art'
import { ErrorState } from '@/components/error-state'
import { Spinner } from '@/components/spinner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Progress } from '@/components/ui/progress'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { api } from '@/lib/api/endpoints'
import type { BatchResult, ItemError, TrackTags } from '@/lib/api/types'
import { cn } from '@/lib/utils'

import { isReadonlyMessage, toastError } from '../lib/batch'
import { FIELDS } from '../lib/tag-fields'
import { buildTagEdits, keepEdits, setFieldValues, type Edits } from '../lib/tag-state'
import type { FieldValues, ToolId } from '../lib/tag-tools'
import { useTrackTags } from '../lib/use-track-tags'
import { invalidateLibrary } from '../queries'
import { CoverTab } from './cover-tab'
import { DetailsTab } from './details-tab'
import { FileTab } from './file-tab'
import { LyricsTab } from './lyrics-tab'
import { RawTab } from './raw-tab'
import { ToolsDialog } from './tools-dialog'
import type { EditorTab, LyricsDraft, PendingCover } from './types'

const TOOL_ICONS = {
  autoNumber: ListOrdered,
  fromFilename: FileText,
  replace: Replace,
  case: CaseSensitive,
  copy: Copy,
  clear: Eraser,
} as const

const DETAIL_KEYS = new Set(FIELDS.flatMap((f) => f.keys))

export interface TagEditorProps {
  trackIds: readonly string[]
  /** Close immediately (after a successful save or a confirmed discard). */
  onClose: () => void
  /** Close requested by the user (asks to discard when dirty). */
  onRequestClose: () => void
  onDirtyChange: (dirty: boolean) => void
  /** Title element id for the surrounding Sheet/Drawer (a11y). */
  titleId?: string
}

function initialLyrics(item: TrackTags | undefined): LyricsDraft {
  if (!item) return { text: '', target: 'embedded' }
  return item.lrc !== null ? { text: item.lrc, target: 'lrc' } : { text: item.lyrics ?? '', target: 'embedded' }
}

export function TagEditor({ trackIds, onClose, onRequestClose, onDirtyChange }: TagEditorProps) {
  const { t } = useTranslation('manage')
  const queryClient = useQueryClient()
  const { status, items, loaded, total, failures, reload } = useTrackTags(trackIds)
  const [edits, setEdits] = useState<Edits>({})
  const [cover, setCover] = useState<PendingCover | null>(null)
  const [lyricsDraft, setLyricsDraft] = useState<LyricsDraft | null>(null)
  const [tab, setTab] = useState<EditorTab>('details')
  const [tool, setTool] = useState<ToolId | null>(null)
  const [saving, setSaving] = useState(false)
  const [saveErrors, setSaveErrors] = useState<ItemError[]>([])

  const single = items.length === 1 ? items[0] : undefined
  const lyricsInitial = useMemo(() => initialLyrics(single), [single])
  const lyrics = lyricsDraft ?? lyricsInitial
  const lyricsDirty = !!single && lyricsDraft !== null && (lyricsDraft.text !== lyricsInitial.text || lyricsDraft.target !== lyricsInitial.target)

  const tagEdits = useMemo(() => buildTagEdits(items, edits), [items, edits])
  const coverInvalid = cover?.kind === 'set' && !cover.embed && !cover.saveToFolder
  const dirty = tagEdits.length > 0 || cover !== null || lyricsDirty
  const readonlyCount = items.filter((i) => !i.writable).length
  const allReadonly = items.length > 0 && readonlyCount === items.length
  const busy = saving || status === 'loading'

  useEffect(() => onDirtyChange(dirty), [dirty, onDirtyChange])

  // Release the cover preview's object URL when it is replaced or the editor unmounts.
  const previewUrl = cover?.kind === 'set' ? cover.previewUrl : null
  useEffect(
    () => () => {
      if (previewUrl) URL.revokeObjectURL(previewUrl)
    },
    [previewUrl],
  )

  const detailsDirty = Object.values(edits).some((patch) => Object.keys(patch).some((k) => DETAIL_KEYS.has(k)))

  const applyTool = (values: Record<string, FieldValues>) => setEdits((current) => setFieldValues(items, current, values))

  const save = async () => {
    if (!dirty || busy || coverInvalid) return
    setSaving(true)
    setSaveErrors([])
    const errors: ItemError[] = []
    let failed = false
    let tagsDone = tagEdits.length === 0
    let coverDone = cover === null
    let lyricsDone = !lyricsDirty
    const ids = items.map((i) => i.track.id)
    try {
      if (!tagsDone) {
        const result = await api.manage.tags.save(tagEdits)
        errors.push(...(result.errors ?? []))
        tagsDone = true
      }
      if (cover) {
        const albumId = items[0]?.track.albumId
        const target = cover.scope === 'album' && albumId ? { albumId } : { trackIds: ids }
        let result: BatchResult
        if (cover.kind === 'set') {
          result = await api.manage.cover.set({ file: cover.file, ...target, embed: cover.embed, saveToFolder: cover.saveToFolder })
        } else {
          result = await api.manage.cover.remove({ ...target, removeFolderImage: cover.removeFolderImage })
        }
        errors.push(...(result.errors ?? []))
        coverDone = true
      }
      if (lyricsDirty && single) {
        await api.manage.setLyrics(single.track.id, { text: lyrics.text, target: lyrics.target })
        lyricsDone = true
      }
    } catch (error) {
      failed = true
      toastError(error)
    }

    void invalidateLibrary(queryClient)

    if (!failed && errors.length === 0) {
      setSaving(false)
      toast.success(t('editor.saved', { count: Math.max(tagEdits.length, cover ? ids.length : 0, lyricsDirty ? 1 : 0) }))
      onClose()
      return
    }

    // Partial success: refresh the snapshot, keep what still needs saving.
    await reload(ids)
    const failedIds = new Set(errors.map((e) => e.trackId))
    setEdits((current) => (tagsDone ? keepEdits(current, failedIds) : current))
    if (coverDone && errors.length === 0) setCover(null)
    if (lyricsDone) setLyricsDraft(null)
    setSaveErrors(errors)
    setSaving(false)
    if (errors.length > 0) toast.error(t('batch.failed', { count: errors.length, ok: ids.length - failedIds.size }))
  }

  const onKeyDown = (e: KeyboardEvent) => {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 's') {
      e.preventDefault()
      void save()
    }
  }

  return (
    <div className="flex h-full min-h-0 flex-col" onKeyDown={onKeyDown}>
      <EditorHeader items={items} trackCount={trackIds.length} onClose={onRequestClose} onTool={setTool} toolsDisabled={status !== 'ready' || saving || allReadonly} />

      {status === 'loading' ? (
        <div className="flex flex-1 flex-col items-center justify-center gap-3 px-8">
          <Spinner size="lg" />
          <p className="tnum text-sm text-muted-foreground">{t('editor.loading', { loaded, total })}</p>
          {total > 1 ? <Progress value={(loaded / Math.max(1, total)) * 100} className="h-1 max-w-56" /> : null}
        </div>
      ) : status === 'error' || items.length === 0 ? (
        <div className="flex flex-1 items-center justify-center">
          <ErrorState error={failures[0]?.error} onRetry={() => void reload()} size="compact" />
        </div>
      ) : (
        <Tabs value={tab} onValueChange={(v) => setTab(v as EditorTab)} className="min-h-0 flex-1 gap-0">
          <div className="hairline-b px-4 pb-3 sm:px-5">
            <TabsList className="scrollbar-none w-full justify-start overflow-x-auto">
              <EditorTabTrigger value="details" dirty={detailsDirty}>
                {t('tabs.details')}
              </EditorTabTrigger>
              <EditorTabTrigger value="cover" dirty={cover !== null}>
                {t('tabs.cover')}
              </EditorTabTrigger>
              <EditorTabTrigger value="lyrics" dirty={lyricsDirty}>
                {t('tabs.lyrics')}
              </EditorTabTrigger>
              <EditorTabTrigger value="raw" dirty={tagEdits.length > 0}>
                {t('tabs.raw')}
              </EditorTabTrigger>
              <EditorTabTrigger value="file">{t('tabs.file')}</EditorTabTrigger>
            </TabsList>
          </div>

          <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain px-4 pt-4 sm:px-5">
            <Notices failures={failures.length} readonly={readonlyCount} total={items.length} saveErrors={saveErrors} items={items} />
            <TabsContent value="details">
              <DetailsTab items={items} edits={edits} onEditsChange={setEdits} disabled={saving || allReadonly} />
            </TabsContent>
            <TabsContent value="cover">
              <CoverTab items={items} pending={cover} onPendingChange={setCover} disabled={saving || allReadonly} />
            </TabsContent>
            <TabsContent value="lyrics">
              {single ? (
                <LyricsTab item={single} draft={lyrics} onDraftChange={setLyricsDraft} disabled={saving || allReadonly} />
              ) : (
                <p className="py-10 text-center text-sm text-balance text-muted-foreground">{t('lyrics.singleOnly')}</p>
              )}
            </TabsContent>
            <TabsContent value="raw">
              <RawTab items={items} edits={edits} onEditsChange={setEdits} disabled={saving || allReadonly} />
            </TabsContent>
            <TabsContent value="file">
              <FileTab items={items} />
            </TabsContent>
          </div>
        </Tabs>
      )}

      <footer className="hairline-t flex items-center gap-3 px-4 pt-3 pb-[calc(var(--safe-bottom)+0.75rem)] sm:px-5 sm:pb-4">
        <p className="min-w-0 flex-1 truncate text-sm text-muted-foreground">
          {dirty ? t('editor.pending', { count: Math.max(tagEdits.length, cover || lyricsDirty ? 1 : 0) }) : t('editor.noChanges')}
        </p>
        {dirty ? (
          <Button
            variant="ghost"
            className="max-sm:h-11"
            disabled={saving}
            onClick={() => {
              setEdits({})
              setCover(null)
              setLyricsDraft(null)
              setSaveErrors([])
            }}
          >
            {t('editor.revertAll')}
          </Button>
        ) : null}
        <Button className="min-w-24 max-sm:h-11" onClick={() => void save()} disabled={!dirty || busy || coverInvalid || allReadonly}>
          {saving ? <Spinner size="sm" className="text-current" /> : null}
          {t('common:actions.save')}
        </Button>
      </footer>

      <ToolsDialog tool={tool} items={items} edits={edits} onApply={applyTool} onClose={() => setTool(null)} />
    </div>
  )
}

function EditorTabTrigger({ value, dirty, children }: { value: EditorTab; dirty?: boolean; children: string }) {
  return (
    <TabsTrigger value={value} className="flex-none gap-1.5 px-3">
      {children}
      {dirty ? <span className="size-1.5 rounded-full bg-primary" aria-hidden /> : null}
    </TabsTrigger>
  )
}

interface EditorHeaderProps {
  items: readonly TrackTags[]
  trackCount: number
  onClose: () => void
  onTool: (tool: ToolId) => void
  toolsDisabled: boolean
}

function EditorHeader({ items, trackCount, onClose, onTool, toolsDisabled }: EditorHeaderProps) {
  const { t } = useTranslation('manage')
  const first = items[0]?.track
  const albums = new Set(items.map((i) => i.track.albumId))
  const subtitle =
    trackCount === 1 && first
      ? [first.artist, first.album].filter(Boolean).join(' — ')
      : albums.size === 1 && first
        ? first.album
        : t('editor.variousAlbums')

  return (
    <header className="flex items-center gap-3 px-4 pt-4 pb-3 sm:px-5 sm:pt-5">
      <div className="relative shrink-0">
        <CoverArt coverArt={first?.coverArt} size={44} />
        {trackCount > 1 ? (
          <span className="tnum absolute -right-1.5 -bottom-1.5 min-w-5 rounded-full bg-primary px-1 text-center text-[11px] leading-5 font-semibold text-primary-foreground ring-2 ring-background">
            {trackCount > 999 ? '999+' : trackCount}
          </span>
        ) : null}
      </div>
      <div className="min-w-0 flex-1">
        <h2 id="tag-editor-title" className="truncate text-[17px] font-semibold tracking-tight">
          {trackCount === 1 && first ? first.title : t('editor.titleMany', { count: trackCount })}
        </h2>
        <p className="truncate text-[13px] text-muted-foreground">{items.length > 0 ? subtitle : t('common:tagEditor.title')}</p>
      </div>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="outline" size="sm" disabled={toolsDisabled} className="max-sm:size-11 max-sm:px-0">
            <Wand2 />
            <span className="max-sm:sr-only">{t('tools.menu')}</span>
            <ChevronDown className="opacity-60 max-sm:hidden" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="min-w-56 rounded-xl">
          <DropdownMenuLabel className="text-xs text-muted-foreground">{t('tools.menuHint')}</DropdownMenuLabel>
          {(Object.keys(TOOL_ICONS) as ToolId[]).map((id) => {
            const Icon = TOOL_ICONS[id]
            return (
              <DropdownMenuItem key={id} onSelect={() => onTool(id)}>
                <Icon />
                {t(`tools.${id}.title`)}
              </DropdownMenuItem>
            )
          })}
        </DropdownMenuContent>
      </DropdownMenu>
      <Button variant="ghost" size="icon" onClick={onClose} aria-label={t('common:actions.close')} className="-mr-2 max-sm:size-11">
        <X className="size-5" />
      </Button>
    </header>
  )
}

interface NoticesProps {
  failures: number
  readonly: number
  total: number
  saveErrors: ItemError[]
  items: readonly TrackTags[]
}

function Notices({ failures, readonly, total, saveErrors, items }: NoticesProps) {
  const { t } = useTranslation('manage')
  const readonlySave = saveErrors.some((e) => isReadonlyMessage(e.error))
  const names = new Map(items.map((i) => [i.track.id, i.track.path]))
  return (
    <div className={cn('grid gap-3', (failures > 0 || readonly > 0 || saveErrors.length > 0) && 'mb-4')}>
      {readonly > 0 || readonlySave ? (
        <Alert className="border-amber-500/30 bg-amber-500/5">
          <Lock />
          <AlertTitle>{readonly === total ? t('readonly.title') : t('readonly.some', { count: readonly })}</AlertTitle>
          <AlertDescription>{t('readonly.description')}</AlertDescription>
        </Alert>
      ) : null}
      {failures > 0 ? (
        <Alert>
          <TriangleAlert />
          <AlertTitle>{t('editor.loadFailed', { count: failures })}</AlertTitle>
        </Alert>
      ) : null}
      {saveErrors.length > 0 ? (
        <Alert variant="destructive">
          <TriangleAlert />
          <AlertTitle>{t('editor.saveFailed', { count: saveErrors.length })}</AlertTitle>
          <AlertDescription>
            <ul className="grid max-h-40 w-full gap-1 overflow-y-auto text-xs">
              {saveErrors.map((e, i) => (
                <li key={`${e.trackId}-${i}`} className="break-all">
                  <span className="font-mono">{e.path || names.get(e.trackId) || e.trackId}</span>: {e.error}
                </li>
              ))}
            </ul>
          </AlertDescription>
        </Alert>
      ) : null}
    </div>
  )
}
