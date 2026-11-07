import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ClipboardCheck, CopyPlus, ExternalLink, FileWarning, History, Import, Music, Pencil, Trash2, type LucideIcon } from 'lucide-react'
import { useMemo, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { PageLoader, Spinner } from '@/components/spinner'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { useLibraries } from '@/features/admin/queries'
import { api } from '@/lib/api/endpoints'
import type { StagedConflictPolicy, StagedImportResult, StagedItem } from '@/lib/api/types'
import { formatBytes, formatDateTime, formatDuration, formatRelative } from '@/lib/format'
import i18n from '@/lib/i18n'
import { cn } from '@/lib/utils'

import { ConfirmDialog } from '../components/confirm-dialog'
import { ResponsiveDialog } from '../components/responsive-dialog'
import { TracksTabs } from '../components/tracks-tabs'
import { toastError } from '../lib/batch'
import { cleanDir } from '../lib/destination'
import { REVIEW_FIELDS, conflictCount, formFromTags, stagedName, tagPatch, type ReviewForm } from '../lib/review'
import { promptUntagged } from '../lib/untagged'
import { invalidateLibrary, manageKeys, stagingQuery } from '../queries'

const POLICY_KEY = 'rainy.manage.reviewConflict'

function loadPolicy(): StagedConflictPolicy {
  try {
    return localStorage.getItem(POLICY_KEY) === 'skip' ? 'skip' : 'keep'
  } catch {
    return 'keep'
  }
}

/**
 * Tracks › Review: downloads held for review (links and online music started with "Review
 * before importing"). Their tags can be corrected and their destination changed; importing
 * places them like any other added music.
 */
export default function ReviewPage() {
  const { t } = useTranslation('manage')
  const queryClient = useQueryClient()
  const staging = useQuery(stagingQuery)
  const items = useMemo(() => staging.data ?? [], [staging.data])
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set())
  const [policy, setPolicy] = useState<StagedConflictPolicy>(loadPolicy)
  const [editing, setEditing] = useState<StagedItem | null>(null)
  const [confirmDiscard, setConfirmDiscard] = useState<string[] | null>(null)

  const selectedIds = items.filter((i) => selected.has(i.id)).map((i) => i.id)
  const totalSize = items.reduce((n, i) => n + i.size, 0)
  const allChecked = items.length > 0 && selectedIds.length === items.length

  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: manageKeys.staging })
  }

  const importItems = useMutation({
    mutationFn: (ids: string[]) => api.manage.staging.import(ids, policy),
    onSuccess: (res) => {
      reportImport(res)
      setSelected((prev) => new Set([...prev].filter((id) => !res.imported.includes(id))))
      refresh()
      if (res.imported.length > 0) void invalidateLibrary(queryClient)
    },
    onError: toastError,
  })

  const discard = async (ids: string[]) => {
    try {
      const res = await api.manage.staging.discard(ids)
      toast.success(t('review.discarded', { count: res.discarded }))
      setSelected(new Set())
      refresh()
    } catch (error) {
      toastError(error)
      throw error
    }
  }

  const toggle = (id: string) =>
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })

  return (
    <Page>
      <PageHeader
        title={t('title')}
        subtitle={items.length > 0 ? t('review.summary', { count: items.length, size: formatBytes(totalSize) }) : t('review.subtitle')}
      >
        <TracksTabs />
      </PageHeader>

      {staging.isPending ? (
        <PageLoader />
      ) : staging.isError ? (
        <ErrorState error={staging.error} onRetry={() => void staging.refetch()} retrying={staging.isFetching} />
      ) : items.length === 0 ? (
        <EmptyState icon={ClipboardCheck} art="happy" title={t('review.emptyTitle')} description={t('review.emptyDescription')} />
      ) : (
        <div className="grid gap-3">
          <div className="bleed-x page-x hairline-b sticky top-[calc(var(--app-header-h)+2.75rem)] z-20 flex min-h-12 flex-wrap items-center gap-x-3 gap-y-1 py-1.5 sticky-surface md:top-(--app-header-h)">
            <Checkbox
              checked={allChecked ? true : selectedIds.length > 0 ? 'indeterminate' : false}
              onCheckedChange={(v) => setSelected(v === true ? new Set(items.map((i) => i.id)) : new Set())}
              aria-label={t('common:actions.selectAll')}
            />
            <span className="tnum min-w-0 flex-1 truncate text-sm text-muted-foreground">
              {selectedIds.length > 0 ? (
                t('selection.count', { count: selectedIds.length, formatted: selectedIds.length })
              ) : (
                <span className="max-sm:sr-only">{t('review.selectHint')}</span>
              )}
            </span>
            <Select
              value={policy}
              onValueChange={(v) => {
                const next: StagedConflictPolicy = v === 'skip' ? 'skip' : 'keep'
                setPolicy(next)
                try {
                  localStorage.setItem(POLICY_KEY, next)
                } catch {
                  // not remembered
                }
              }}
            >
              <SelectTrigger size="sm" className="w-auto max-sm:h-10" aria-label={t('review.policy')}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="keep">{t('review.policies.keep')}</SelectItem>
                <SelectItem value="skip">{t('review.policies.skip')}</SelectItem>
              </SelectContent>
            </Select>
            <Button size="sm" className="max-sm:h-10" disabled={selectedIds.length === 0 || importItems.isPending} onClick={() => importItems.mutate(selectedIds)}>
              {importItems.isPending ? <Spinner size="sm" className="text-current" /> : <Import />}
              {t('review.import')}
            </Button>
            <Button
              variant="ghost"
              size="sm"
              className="text-destructive hover:text-destructive max-sm:h-10"
              disabled={selectedIds.length === 0}
              onClick={() => setConfirmDiscard(selectedIds)}
              aria-label={t('review.discard')}
            >
              <Trash2 />
              <span className="max-sm:sr-only">{t('review.discard')}</span>
            </Button>
          </div>
          <ul className="bleed-x md:mx-0">
            {items.map((item) => (
              <ReviewRow
                key={item.id}
                item={item}
                selected={selected.has(item.id)}
                onToggle={() => toggle(item.id)}
                onEdit={() => setEditing(item)}
                onImport={() => importItems.mutate([item.id])}
                importing={importItems.isPending}
              />
            ))}
          </ul>
        </div>
      )}

      {editing ? (
        <ReviewDialog
          key={editing.id}
          item={items.find((i) => i.id === editing.id) ?? editing}
          onClose={() => setEditing(null)}
          onImport={(id) => importItems.mutateAsync([id])}
          onDiscard={(id) => setConfirmDiscard([id])}
        />
      ) : null}

      <ConfirmDialog
        open={confirmDiscard !== null}
        onOpenChange={(open) => (open ? undefined : setConfirmDiscard(null))}
        title={t('review.discardTitle', { count: confirmDiscard?.length ?? 0 })}
        description={t('review.discardDescription')}
        confirmLabel={t('review.discard')}
        destructive
        onConfirm={async () => {
          const ids = confirmDiscard ?? []
          await discard(ids)
          if (editing && ids.includes(editing.id)) setEditing(null)
        }}
      />
    </Page>
  )
}

/** Toasts the outcome of an import and asks for the tags of tracks that arrived without them. */
function reportImport(res: StagedImportResult) {
  const t = i18n.t.bind(i18n)
  if (res.imported.length > 0) toast.success(t('manage:review.imported', { count: res.imported.length }))
  if (res.skipped.length > 0) toast.warning(t('manage:review.skipped', { count: res.skipped.length }), { description: t('manage:review.skippedHint') })
  if (res.errors.length > 0) {
    toast.error(t('manage:review.failed', { count: res.errors.length }), {
      description: res.errors.slice(0, 3).map((e) => e.error).join('\n'),
      duration: 8000,
    })
  }
  promptUntagged(res.untagged)
}

function ConflictBadges({ item }: { item: StagedItem }) {
  const { t } = useTranslation('manage')
  const c = item.conflict
  if (!c.exists && c.sameSource.length === 0 && c.similar.length === 0) return null
  return (
    <span className="flex flex-wrap gap-1.5">
      {c.exists ? (
        <Chip icon={FileWarning} tone="warn">
          {t('review.conflicts.exists')}
        </Chip>
      ) : null}
      {c.sameSource.length > 0 ? (
        <Chip icon={History} tone="warn">
          {t('review.conflicts.sameSource')}
        </Chip>
      ) : null}
      {c.similar.length > 0 ? (
        <Chip icon={CopyPlus} tone="muted">
          {t('review.conflicts.similar', { count: c.similar.length })}
        </Chip>
      ) : null}
    </span>
  )
}

function Chip({ icon: Icon, tone, children }: { icon: LucideIcon; tone: 'warn' | 'muted'; children: ReactNode }) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-[11px] font-medium',
        tone === 'warn' ? 'bg-amber-500/12 text-amber-700 dark:text-amber-400' : 'bg-muted text-muted-foreground',
      )}
    >
      <Icon className="size-3" aria-hidden />
      {children}
    </span>
  )
}

function Cover({ item, className }: { item: StagedItem; className?: string }) {
  const [failed, setFailed] = useState(false)
  return (
    <span className={cn('grid shrink-0 place-items-center overflow-hidden rounded-md bg-muted text-muted-foreground', className)}>
      {item.hasCover && !failed ? (
        <img src={api.manage.staging.pictureUrl(item.id, item.size)} alt="" className="size-full object-cover" loading="lazy" onError={() => setFailed(true)} />
      ) : (
        <Music className="size-1/2" strokeWidth={1.5} aria-hidden />
      )}
    </span>
  )
}

function ReviewRow({
  item,
  selected,
  onToggle,
  onEdit,
  onImport,
  importing,
}: {
  item: StagedItem
  selected: boolean
  onToggle: () => void
  onEdit: () => void
  onImport: () => void
  importing: boolean
}) {
  const { t } = useTranslation('manage')
  const name = stagedName(item)
  const album = item.tags.ALBUM?.[0]
  return (
    <li
      className={cn(
        'hairline-inset page-x flex items-center gap-3 py-2.5 [--hairline-inset:calc(var(--page-px)+2.25rem)] md:rounded-md md:px-2 md:[--hairline-inset:2.25rem]',
        selected && 'bg-primary/8',
      )}
    >
      <Checkbox checked={selected} onCheckedChange={onToggle} aria-label={t('table.selectTrack', { title: name })} />
      <Cover item={item} className="size-11 max-sm:hidden" />
      <button
        type="button"
        onClick={onEdit}
        className="grid min-w-0 flex-1 gap-0.5 rounded-md text-left outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
      >
        <span className="truncate text-[15px] md:text-sm">
          <span className="font-medium">{name}</span>
          {album ? <span className="text-muted-foreground"> · {album}</span> : null}
        </span>
        <span className={cn('truncate font-mono text-[11px]', item.error ? 'text-destructive' : 'text-muted-foreground')} title={item.error || item.target}>
          {item.error || `→ ${item.target}`}
        </span>
        <span className="tnum truncate text-xs text-muted-foreground" title={formatDateTime(item.createdAt)}>
          {[
            item.format.toUpperCase(),
            formatBytes(item.size),
            item.duration > 0 ? formatDuration(item.duration) : '',
            t(`review.kinds.${item.kind}`),
            formatRelative(item.createdAt),
            item.createdBy,
          ]
            .filter(Boolean)
            .join(' · ')}
        </span>
        <ConflictBadges item={item} />
      </button>
      <Button variant="ghost" size="icon" className="size-10 shrink-0 text-muted-foreground" onClick={onEdit} aria-label={t('review.edit')} title={t('review.edit')}>
        <Pencil />
      </Button>
      <Button
        variant="ghost"
        size="icon"
        className="size-10 shrink-0 text-muted-foreground"
        onClick={onImport}
        disabled={importing || !!item.error}
        aria-label={t('review.importOne', { name })}
        title={t('review.import')}
      >
        <Import />
      </Button>
    </li>
  )
}

/** Edits the tags and destination of one staged download; saving shows the new target. */
function ReviewDialog({
  item,
  onClose,
  onImport,
  onDiscard,
}: {
  item: StagedItem
  onClose: () => void
  onImport: (id: string) => Promise<StagedImportResult>
  onDiscard: (id: string) => void
}) {
  const { t } = useTranslation('manage')
  const queryClient = useQueryClient()
  const { libraries, limited } = useLibraries()
  const [form, setForm] = useState<ReviewForm>(() => formFromTags(item.tags))
  const [libraryId, setLibraryId] = useState(item.libraryId)
  const [dir, setDir] = useState(item.dir)
  const [organize, setOrganize] = useState(item.organize)

  const patch = tagPatch(form, item.tags)
  const cleanedDir = cleanDir(dir)
  const invalidDir = cleanedDir.split('/').some((p) => p === '..' || p === '.')
  const destinationChanged = libraryId !== item.libraryId || cleanedDir !== item.dir || organize !== item.organize
  const dirty = Object.keys(patch).length > 0 || destinationChanged

  const save = useMutation({
    mutationFn: () =>
      api.manage.staging.update(item.id, {
        tags: Object.keys(patch).length > 0 ? patch : undefined,
        ...(destinationChanged ? { libraryId, dir: cleanedDir, organize } : {}),
      }),
    onSuccess: (updated) => {
      queryClient.setQueryData<StagedItem[]>(manageKeys.staging, (prev) => prev?.map((i) => (i.id === updated.id ? updated : i)))
      setForm(formFromTags(updated.tags))
      setDir(updated.dir)
    },
    onError: toastError,
  })

  const saveAndImport = useMutation({
    mutationFn: async () => {
      if (dirty) await save.mutateAsync()
      return onImport(item.id)
    },
    onSuccess: (res) => {
      if (res.imported.includes(item.id)) onClose()
    },
  })

  const busy = save.isPending || saveAndImport.isPending
  const c = item.conflict

  return (
    <ResponsiveDialog
      open
      onOpenChange={(open) => (open ? undefined : onClose())}
      title={t('review.editTitle')}
      description={item.title || item.source}
      size="lg"
      dismissible={!busy}
      footer={
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="ghost" className="text-destructive hover:text-destructive max-sm:h-11" onClick={() => onDiscard(item.id)} disabled={busy}>
            <Trash2 />
            {t('review.discard')}
          </Button>
          <span className="flex-1" />
          <Button variant="outline" className="max-sm:h-11" onClick={() => save.mutate()} disabled={!dirty || busy || invalidDir}>
            {save.isPending && !saveAndImport.isPending ? <Spinner size="sm" /> : null}
            {t('review.save')}
          </Button>
          <Button className="max-sm:h-11" onClick={() => saveAndImport.mutate()} disabled={busy || invalidDir || (!dirty && !!item.error)}>
            {saveAndImport.isPending ? <Spinner size="sm" className="text-current" /> : <Import />}
            {dirty ? t('review.saveAndImport') : t('review.import')}
          </Button>
        </div>
      }
    >
      <div className="grid gap-5">
        <div className="flex items-start gap-3">
          <Cover item={item} className="size-16" />
          <div className="grid min-w-0 flex-1 gap-1 text-xs text-muted-foreground">
            <p className="tnum">
              {[item.format.toUpperCase(), formatBytes(item.size), item.duration > 0 ? formatDuration(item.duration) : ''].filter(Boolean).join(' · ')}
            </p>
            {item.source ? (
              <a href={item.source} target="_blank" rel="noreferrer noopener" className="inline-flex min-w-0 items-center gap-1 underline-offset-4 hover:underline">
                <ExternalLink className="size-3 shrink-0" aria-hidden />
                <span className="truncate">{item.source}</span>
              </a>
            ) : null}
          </div>
        </div>

        <div className="grid gap-3 sm:grid-cols-2">
          {REVIEW_FIELDS.map((field) => (
            <div key={field} className={cn('grid gap-1.5', (field === 'title' || field === 'artist') && 'sm:col-span-2')}>
              <Label htmlFor={`review-${field}`} className="text-[13px] font-medium text-foreground/75">
                {t(`review.fields.${field}`)}
              </Label>
              <Input
                id={`review-${field}`}
                value={form[field]}
                inputMode={field === 'track' || field === 'disc' ? 'numeric' : undefined}
                onChange={(e) => setForm((prev) => ({ ...prev, [field]: e.target.value }))}
                placeholder={field === 'artist' || field === 'genre' ? t('review.multiHint') : undefined}
                autoComplete="off"
              />
            </div>
          ))}
        </div>

        <section className="grid gap-3 rounded-xl border p-4">
          <h3 className="text-sm font-semibold">{t('upload.target')}</h3>
          {!limited && libraries.length > 1 ? (
            <div className="grid gap-1.5">
              <Label className="text-[13px] font-medium text-foreground/75">{t('filters.library')}</Label>
              <Select value={String(libraryId)} onValueChange={(v) => setLibraryId(Number(v))}>
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
            <Label htmlFor="review-dir" className="text-[13px] font-medium text-foreground/75">
              {t('upload.folder')}
            </Label>
            <Input
              id="review-dir"
              value={dir}
              onChange={(e) => setDir(e.target.value)}
              placeholder={t('upload.folderPlaceholder')}
              aria-invalid={invalidDir || undefined}
              className="font-mono text-sm"
              autoComplete="off"
              spellCheck={false}
            />
            {invalidDir ? <p className="text-xs text-destructive">{t('upload.invalidDir')}</p> : null}
          </div>
          <Label className="flex items-center gap-3 font-normal">
            <Switch checked={organize} onCheckedChange={setOrganize} />
            <span className="text-sm font-medium">{t('upload.organize')}</span>
          </Label>
          <div className="grid gap-1 rounded-lg bg-muted/50 px-3 py-2.5">
            <span className="text-xs text-muted-foreground">{dirty ? t('review.targetAfterSave') : t('review.target')}</span>
            <code className={cn('font-mono text-xs break-all', item.error && 'text-destructive')}>{item.error || item.target}</code>
          </div>
        </section>

        {conflictCount(item) > 0 ? (
          <section className="grid gap-2">
            <h3 className="text-sm font-semibold">{t('review.conflictsTitle')}</h3>
            {c.exists ? <p className="text-xs text-amber-700 dark:text-amber-400">{t('review.conflicts.existsHint')}</p> : null}
            {c.sameSource.length > 0 ? (
              <MatchList title={t('review.conflicts.sameSourceHint')} paths={c.sameSource.map((m) => m.path)} />
            ) : null}
            {c.similar.length > 0 ? <MatchList title={t('review.conflicts.similarHint')} paths={c.similar.map((m) => m.path)} /> : null}
          </section>
        ) : null}
      </div>
    </ResponsiveDialog>
  )
}

function MatchList({ title, paths }: { title: string; paths: string[] }) {
  return (
    <div className="grid gap-1">
      <p className="text-xs text-muted-foreground">{title}</p>
      <ul className="grid gap-0.5">
        {paths.map((p) => (
          <li key={p} className="truncate font-mono text-[11px]" title={p}>
            {p}
          </li>
        ))}
      </ul>
    </div>
  )
}
