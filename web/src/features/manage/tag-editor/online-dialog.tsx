import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, ArrowRight, Globe, ImageOff, Search } from 'lucide-react'
import { useMemo, useState, type FormEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Spinner } from '@/components/spinner'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { useAuth } from '@/hooks/use-auth'
import { ApiError } from '@/lib/api/client'
import { api } from '@/lib/api/endpoints'
import type { MetadataProvider, MetadataProviderId, MetadataResult, TrackTags } from '@/lib/api/types'
import { formatDuration } from '@/lib/format'
import { cn } from '@/lib/utils'

import { ResponsiveDialog } from '../components/responsive-dialog'
import {
  coverFileName,
  defaultQuery,
  durationMatch,
  lyricsText,
  onlineFieldValues,
  resultFields,
  type OnlineFieldId,
} from '../lib/online'
import { FIELD_BY_ID, type FieldId } from '../lib/tag-fields'
import { summarizeField, type Edits } from '../lib/tag-state'

const PROVIDER_KEY = 'rainy.manage.onlineProvider'
const REGION_KEY = 'rainy.manage.onlineRegion'
const LYRICS_PREVIEW_LINES = 6

export interface OnlineApply {
  values: Record<string, Partial<Record<FieldId, string>>>
  cover: File | null
  lyrics: string | null
}

export interface OnlineDialogProps {
  open: boolean
  items: readonly TrackTags[]
  edits: Edits
  /** The single track's current lyrics draft (decides whether lyrics are pre-selected). */
  currentLyrics: string
  onApply: (apply: OnlineApply) => void
  onClose: () => void
}

function load(key: string): string {
  try {
    return localStorage.getItem(key) ?? ''
  } catch {
    return ''
  }
}

function save(key: string, value: string) {
  try {
    localStorage.setItem(key, value)
  } catch {
    // storage unavailable — the choice just isn't remembered
  }
}

const statusQueryKey = ['manage', 'metadata', 'status'] as const

/**
 * Search online catalogues and copy a result into the tag editor's draft. Nothing is written
 * until the editor saves; searches only run when an admin enabled online lookup.
 */
export function OnlineDialog({ open, items, edits, currentLyrics, onApply, onClose }: OnlineDialogProps) {
  const { t } = useTranslation('manage')
  const [selected, setSelected] = useState<MetadataResult | null>(null)
  const status = useQuery({
    queryKey: statusQueryKey,
    queryFn: ({ signal }) => api.manage.metadata.status({ signal }),
    enabled: open,
    // Re-checked on every opening, so a setting an admin just changed applies right away.
    staleTime: 0,
  })

  // Every opening starts from the result list.
  const close = () => {
    setSelected(null)
    onClose()
  }

  const providers = status.data?.providers ?? []
  const provider = selected ? providers.find((p) => p.id === selected.provider) : undefined

  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={(next) => (next ? undefined : close())}
      title={t('online.title')}
      description={t('online.description')}
      size="lg"
      dialogOnly
      bodyClassName="flex min-h-[min(60dvh,28rem)] flex-col"
    >
      {status.isPending ? (
        <div className="flex flex-1 items-center justify-center py-10">
          <Spinner />
        </div>
      ) : status.isError ? (
        <ErrorState error={status.error} onRetry={() => void status.refetch()} size="compact" />
      ) : !status.data.enabled ? (
        <Disabled />
      ) : (
        <>
          {/* The list stays mounted under the detail view so going back keeps the results and scroll. */}
          <SearchView items={items} providers={providers} onPick={setSelected} hidden={selected !== null} />
          {selected && provider ? (
            <DetailView
              key={`${selected.provider}:${selected.id}`}
              result={selected}
              provider={provider}
              items={items}
              edits={edits}
              currentLyrics={currentLyrics}
              onBack={() => setSelected(null)}
              onApply={(apply) => {
                onApply(apply)
                close()
              }}
            />
          ) : null}
        </>
      )}
    </ResponsiveDialog>
  )
}

function Disabled() {
  const { t } = useTranslation('manage')
  const { isAdmin } = useAuth()
  return (
    <EmptyState
      icon={Globe}
      size="compact"
      title={t('online.disabledTitle')}
      description={isAdmin ? t('online.disabledAdmin') : t('online.disabledManager')}
    />
  )
}

interface SearchViewProps {
  items: readonly TrackTags[]
  providers: MetadataProvider[]
  onPick: (result: MetadataResult) => void
  hidden: boolean
}

function SearchView({ items, providers, onPick, hidden }: SearchViewProps) {
  const { t, i18n } = useTranslation('manage')
  const [providerId, setProviderId] = useState<MetadataProviderId>(() => {
    const stored = load(PROVIDER_KEY)
    return (providers.find((p) => p.id === stored) ?? providers[0]).id
  })
  const provider = providers.find((p) => p.id === providerId) ?? providers[0]
  const [region, setRegion] = useState(() => load(REGION_KEY))
  const effectiveRegion = provider.regions.includes(region) ? region : (provider.regions[0] ?? '')
  const [input, setInput] = useState(() => defaultQuery(items))
  // The submitted query; searching starts right away with the track's own title and artist.
  const [query, setQuery] = useState(() => defaultQuery(items))

  const search = useQuery({
    queryKey: ['manage', 'metadata', 'search', provider.id, query, effectiveRegion],
    queryFn: ({ signal }) =>
      api.manage.metadata.search({ provider: provider.id, q: query, region: effectiveRegion || undefined }, { signal }),
    enabled: query.trim() !== '',
    staleTime: 5 * 60_000,
    retry: false,
  })

  const regionNames = useMemo(() => {
    try {
      return new Intl.DisplayNames([i18n.language], { type: 'region' })
    } catch {
      return null
    }
  }, [i18n.language])

  const submit = (e: FormEvent) => {
    e.preventDefault()
    const q = input.trim()
    if (q === query) void search.refetch()
    else setQuery(q)
  }

  const trackDuration = items.length === 1 ? (items[0]?.track.duration ?? 0) : 0
  const results = search.data?.items ?? []

  return (
    <div className={cn('flex flex-1 flex-col gap-3', hidden && 'hidden')}>
      <ToggleGroup
        type="single"
        variant="outline"
        value={provider.id}
        onValueChange={(v) => {
          if (!v) return
          setProviderId(v as MetadataProviderId)
          save(PROVIDER_KEY, v)
        }}
        className="scrollbar-none w-full justify-start overflow-x-auto"
        aria-label={t('online.provider')}
      >
        {providers.map((p) => (
          <ToggleGroupItem key={p.id} value={p.id} className="flex-none px-3 max-sm:h-11">
            {t(`online.providers.${p.id}`)}
          </ToggleGroupItem>
        ))}
      </ToggleGroup>

      <form onSubmit={submit} className="flex gap-2">
        <Input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder={t('online.placeholder')}
          aria-label={t('online.query')}
          maxLength={200}
          enterKeyHint="search"
          className="min-w-0 flex-1 max-sm:h-11"
        />
        {provider.regions.length > 0 ? (
          <Select
            value={effectiveRegion}
            onValueChange={(v) => {
              setRegion(v)
              save(REGION_KEY, v)
            }}
          >
            <SelectTrigger className="w-28 shrink-0 max-sm:h-11" aria-label={t('online.region')}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {provider.regions.map((r) => (
                <SelectItem key={r} value={r}>
                  {regionNames?.of(r.toUpperCase()) ?? r.toUpperCase()}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        ) : null}
        <Button type="submit" disabled={input.trim() === ''} className="shrink-0 max-sm:size-11 max-sm:px-0">
          <Search />
          <span className="max-sm:sr-only">{t('online.search')}</span>
        </Button>
      </form>

      <div className="flex flex-1 flex-col">
        {query.trim() === '' ? (
          <p className="py-10 text-center text-sm text-muted-foreground">{t('online.enterQuery')}</p>
        ) : search.isFetching && !search.data ? (
          <div className="flex flex-1 items-center justify-center py-10">
            <Spinner />
          </div>
        ) : search.isError ? (
          <ErrorState
            error={search.error}
            description={
              search.error instanceof ApiError && search.error.status === 503
                ? t('online.providerFailed', { provider: t(`online.providers.${provider.id}`) })
                : undefined
            }
            onRetry={() => void search.refetch()}
            retrying={search.isFetching}
            size="compact"
          />
        ) : results.length === 0 ? (
          <EmptyState icon={Search} size="compact" title={t('online.noResults')} description={t('online.noResultsHint')} />
        ) : (
          <ul className={cn('-mx-2 grid grid-cols-1 gap-0.5 transition-opacity', search.isFetching && 'opacity-60')}>
            {results.map((r) => (
              <li key={`${r.provider}:${r.id}`}>
                <ResultRow result={r} trackDuration={trackDuration} onPick={() => onPick(r)} />
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}

function Thumb({ url, size }: { url: string; size: number }) {
  const [failed, setFailed] = useState(false)
  return (
    <div
      className="grid shrink-0 place-items-center overflow-hidden rounded-md bg-muted text-muted-foreground ring-1 ring-black/5 dark:ring-white/10"
      style={{ width: size, height: size }}
    >
      {url && !failed ? (
        <img
          src={api.manage.metadata.coverUrl(url)}
          alt=""
          loading="lazy"
          decoding="async"
          className="size-full object-cover"
          onError={() => setFailed(true)}
        />
      ) : (
        <ImageOff className="size-4" strokeWidth={1.5} aria-hidden />
      )}
    </div>
  )
}

function ResultRow({ result, trackDuration, onPick }: { result: MetadataResult; trackDuration: number; onPick: () => void }) {
  const { t } = useTranslation('manage')
  const match = durationMatch(trackDuration, result.duration)
  const details = [
    result.date.slice(0, 4),
    result.trackNumber > 0 ? t('online.trackNo', { n: result.trackNumber }) : '',
    result.genre,
  ].filter(Boolean)
  return (
    <button
      type="button"
      onClick={onPick}
      className="flex min-h-14 w-full items-center gap-3 rounded-lg px-2 py-2 text-left transition-colors hover:bg-accent focus-visible:bg-accent focus-visible:outline-none"
    >
      <Thumb url={result.thumbUrl} size={48} />
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium">{result.title}</p>
        <p className="truncate text-xs text-muted-foreground">
          {[result.artists.join(' / '), result.album].filter(Boolean).join(' — ')}
        </p>
        {details.length > 0 ? <p className="truncate text-xs text-muted-foreground/80">{details.join(' · ')}</p> : null}
      </div>
      {result.duration > 0 ? (
        <span
          className={cn(
            'tnum shrink-0 rounded px-1.5 py-0.5 text-xs',
            match === 'match' && 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400',
            match === 'mismatch' && 'bg-amber-500/10 text-amber-700 dark:text-amber-400',
            (match === 'near' || match === 'unknown') && 'text-muted-foreground',
          )}
          title={match === 'mismatch' ? t('online.lengthMismatch') : undefined}
        >
          {formatDuration(result.duration)}
        </span>
      ) : null}
      <ArrowRight className="size-4 shrink-0 text-muted-foreground" aria-hidden />
    </button>
  )
}

interface DetailViewProps {
  result: MetadataResult
  provider: MetadataProvider
  items: readonly TrackTags[]
  edits: Edits
  currentLyrics: string
  onBack: () => void
  onApply: (apply: OnlineApply) => void
}

function DetailView({ result, provider, items, edits, currentLyrics, onBack, onApply }: DetailViewProps) {
  const { t } = useTranslation('manage')
  const single = items.length === 1
  const values = useMemo(() => resultFields(result, single), [result, single])
  const fieldIds = Object.keys(values) as OnlineFieldId[]

  const current = useMemo(() => {
    const out: Partial<Record<OnlineFieldId, { value: string; mixed: boolean }>> = {}
    for (const id of Object.keys(values) as OnlineFieldId[]) out[id] = summarizeField(items, edits, FIELD_BY_ID[id])
    return out
  }, [values, items, edits])

  // Pre-select the fields that would change.
  const [chosen, setChosen] = useState<Set<OnlineFieldId>>(
    () => new Set(fieldIds.filter((id) => current[id]?.mixed || current[id]?.value !== values[id])),
  )
  const hasCover = result.coverUrl !== ''
  const anyPicture = items.some((i) => i.pictures.length > 0)
  const [coverChosen, setCoverChosen] = useState(hasCover && !anyPicture)

  const lyricsEnabled = single && provider.lyrics
  const lyrics = useQuery({
    queryKey: ['manage', 'metadata', 'lyrics', result.provider, result.id],
    queryFn: ({ signal }) => api.manage.metadata.lyrics(result.provider, result.id, { signal }),
    enabled: lyricsEnabled,
    staleTime: 5 * 60_000,
    retry: false,
  })
  const [lyricsChoice, setLyricsChoice] = useState<boolean | null>(null)
  const lyricsChosen = lyrics.data ? (lyricsChoice ?? currentLyrics.trim() === '') : false
  const [withTranslation, setWithTranslation] = useState(true)
  const [applying, setApplying] = useState(false)

  const nothing = chosen.size === 0 && !coverChosen && !lyricsChosen

  const toggle = (id: OnlineFieldId, on: boolean) =>
    setChosen((prev) => {
      const next = new Set(prev)
      if (on) next.add(id)
      else next.delete(id)
      return next
    })

  const apply = async () => {
    setApplying(true)
    let cover: File | null = null
    if (coverChosen) {
      try {
        const res = await fetch(api.manage.metadata.coverUrl(result.coverUrl), { credentials: 'same-origin' })
        if (!res.ok) throw new Error(String(res.status))
        const blob = await res.blob()
        cover = new File([blob], coverFileName(blob.type), { type: blob.type || 'image/jpeg' })
      } catch {
        setApplying(false)
        toast.error(t('online.coverFailed'))
        return
      }
    }
    const text = lyricsChosen && lyrics.data ? lyricsText(lyrics.data, withTranslation) : null
    onApply({ values: onlineFieldValues(items, values, chosen), cover, lyrics: text })
    toast.success(t('online.applied'))
  }

  const lyricsPreview = lyrics.data
    ? lyricsText(lyrics.data, withTranslation)
        .split('\n')
        .map((l) => l.replace(/^(\s*\[[^\]]*\])+/, '').trim())
        .filter(Boolean)
        .slice(0, LYRICS_PREVIEW_LINES)
    : []

  return (
    <div className="flex flex-1 flex-col gap-4">
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="icon" onClick={onBack} aria-label={t('online.back')} className="-ml-2 shrink-0 max-sm:size-11">
          <ArrowLeft className="size-5" />
        </Button>
        <Thumb url={result.thumbUrl} size={56} />
        <div className="min-w-0 flex-1">
          <p className="truncate font-medium">{result.title}</p>
          <p className="truncate text-sm text-muted-foreground">{t(`online.providers.${result.provider}`)}</p>
        </div>
      </div>

      {!single ? <p className="text-xs text-muted-foreground">{t('online.batchHint')}</p> : null}

      <ul className="grid grid-cols-1 gap-1">
        {fieldIds.map((id) => {
          const cur = current[id]
          const shown = cur?.mixed ? t('editor.multipleValues') : cur?.value || t('tools.empty')
          return (
            <li key={id}>
              <ChoiceRow id={`online-${id}`} checked={chosen.has(id)} onChange={(on) => toggle(id, on)} label={t(`fields.${id}`)}>
                <span className={cn('truncate text-muted-foreground', !cur?.value && !cur?.mixed && 'italic')}>{shown}</span>
                <ArrowRight className="size-3.5 shrink-0 text-muted-foreground" aria-hidden />
                <span className="truncate font-medium">{values[id]}</span>
              </ChoiceRow>
            </li>
          )
        })}
        {hasCover ? (
          <li>
            <ChoiceRow id="online-cover" checked={coverChosen} onChange={setCoverChosen} label={t('tabs.cover')}>
              <Thumb url={result.coverUrl} size={64} />
              <span className="text-xs text-muted-foreground">{anyPicture ? t('online.coverReplaces') : t('online.coverAdds')}</span>
            </ChoiceRow>
          </li>
        ) : null}
        {lyricsEnabled ? (
          <li>
            {lyrics.isPending ? (
              <div className="flex items-center gap-2 px-2 py-3 text-sm text-muted-foreground">
                <Spinner size="sm" />
                {t('online.lyricsLoading')}
              </div>
            ) : lyrics.isError ? (
              <p className="px-2 py-3 text-sm text-muted-foreground">
                {lyrics.error instanceof ApiError && lyrics.error.status === 404 ? t('online.noLyrics') : t('online.lyricsFailed')}
              </p>
            ) : (
              <ChoiceRow id="online-lyrics" checked={lyricsChosen} onChange={setLyricsChoice} label={t('tabs.lyrics')} align="start">
                <div className="grid min-w-0 flex-1 grid-cols-1 gap-2">
                  <div className="rounded-md bg-muted/60 px-2.5 py-2 text-xs leading-relaxed text-muted-foreground">
                    {lyricsPreview.map((line, i) => (
                      <p key={i} className="truncate">
                        {line}
                      </p>
                    ))}
                  </div>
                  {lyrics.data.translation ? (
                    <label className="flex items-center gap-2 text-xs">
                      <Checkbox checked={withTranslation} onCheckedChange={(v) => setWithTranslation(v === true)} />
                      {t('online.withTranslation')}
                    </label>
                  ) : null}
                  {currentLyrics.trim() ? <p className="text-xs text-muted-foreground">{t('online.lyricsReplace')}</p> : null}
                </div>
              </ChoiceRow>
            )}
          </li>
        ) : null}
      </ul>

      <div className="sticky -bottom-4 -mx-6 mt-auto -mb-4 flex justify-end gap-2 border-t bg-background px-6 pt-3 pb-4">
        <Button variant="ghost" onClick={onBack} className="max-sm:h-11">
          {t('online.back')}
        </Button>
        <Button onClick={() => void apply()} disabled={nothing || applying} className="max-sm:h-11">
          {applying ? <Spinner size="sm" className="text-current" /> : null}
          {t('online.apply')}
        </Button>
      </div>
    </div>
  )
}

interface ChoiceRowProps {
  id: string
  checked: boolean
  onChange: (checked: boolean) => void
  label: string
  align?: 'center' | 'start'
  children: ReactNode
}

function ChoiceRow({ id, checked, onChange, label, align = 'center', children }: ChoiceRowProps) {
  return (
    <div
      className={cn(
        'flex min-h-11 gap-3 rounded-lg px-2 py-1.5 transition-colors',
        align === 'start' ? 'items-start' : 'items-center',
        checked && 'bg-primary/5',
      )}
    >
      <Checkbox id={id} checked={checked} onCheckedChange={(v) => onChange(v === true)} className={cn(align === 'start' && 'mt-0.5')} />
      <Label htmlFor={id} className="w-24 shrink-0 text-sm font-normal text-muted-foreground sm:w-28">
        {label}
      </Label>
      <div className="flex min-w-0 flex-1 items-center gap-2 text-sm">{children}</div>
    </div>
  )
}
