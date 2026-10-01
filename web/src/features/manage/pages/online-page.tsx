import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ChevronDown, Download, ExternalLink, Globe, Music, Search, Settings2, SlidersHorizontal } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { toast } from 'sonner'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import { PageLoader, Spinner } from '@/components/spinner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { useAuth } from '@/hooks/use-auth'
import { api } from '@/lib/api/endpoints'
import type { DownloadsStatus, OnlinePlatform, OnlineQualityType, OnlineSong, OnlineStatus } from '@/lib/api/types'
import { formatDuration, formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'

import { DestinationSection } from '../components/destination-section'
import { DownloadJobs } from '../components/download-jobs'
import { TracksTabs } from '../components/tracks-tabs'
import { toastError } from '../lib/batch'
import { useDestinationTarget } from '../lib/destination'
import { PLATFORMS, QUALITIES, bestQuality, chunk, expectedQuality, isPlatform, isQuality, songKey, uniqueSongs } from '../lib/online-music'
import { useJobNotifications } from '../lib/use-job-notifications'
import { downloadsQuery, manageKeys, onlineStatusQuery } from '../queries'

const PAGE_SIZE = 30
/** Songs per download request (the server accepts 50). */
const BATCH = 50

const PLATFORM_KEY = 'rainy.manage.onlinePlatform'
const QUALITY_KEY = 'rainy.manage.onlineQuality'
const LYRICS_KEY = 'rainy.manage.onlineLyrics'
const COVER_KEY = 'rainy.manage.onlineCover'

function load<T>(key: string, valid: (v: unknown) => v is T, fallback: T): T {
  try {
    const v = localStorage.getItem(key)
    return valid(v) ? v : fallback
  } catch {
    return fallback
  }
}

function save(key: string, value: string) {
  try {
    localStorage.setItem(key, value)
  } catch {
    // not remembered
  }
}

const isFlag = (v: unknown): v is '0' | '1' => v === '0' || v === '1'

/**
 * Tracks → Online: search the online catalogues lx-music knows and download songs into the
 * library through the music sources an administrator imported. Downloads run on the server.
 */
export default function OnlinePage() {
  const { t } = useTranslation('manage')
  const status = useQuery(onlineStatusQuery)
  const downloads = useQuery(downloadsQuery)
  useJobNotifications(downloads.data?.jobs)
  const onlineJobs = downloads.data?.jobs.filter((j) => j.kind === 'online') ?? []

  let content
  if (!status.data) {
    content = status.isError ? (
      <ErrorState error={status.error} onRetry={() => void status.refetch()} retrying={status.isFetching} />
    ) : (
      <PageLoader />
    )
  } else if (!status.data.enabled || status.data.sources === 0) {
    content = <Unavailable status={status.data} />
  } else {
    content = <OnlineSearch />
  }

  return (
    <Page>
      <PageHeader title={t('title')} subtitle={t('onlineMusic.subtitle')}>
        <TracksTabs />
      </PageHeader>
      <div className="mx-auto grid max-w-4xl gap-6">
        {content}
        <DownloadJobs jobs={onlineJobs} />
      </div>
    </Page>
  )
}

function Unavailable({ status }: { status: OnlineStatus }) {
  const { t } = useTranslation('manage')
  const { isAdmin } = useAuth()
  const off = !status.enabled
  return (
    <EmptyState
      icon={Globe}
      title={off ? t('onlineMusic.disabledTitle') : t('onlineMusic.noSourcesTitle')}
      description={
        isAdmin
          ? off
            ? t('onlineMusic.disabledAdmin')
            : t('onlineMusic.noSourcesAdmin')
          : off
            ? t('onlineMusic.disabledManager')
            : t('onlineMusic.noSourcesManager')
      }
      action={
        isAdmin ? (
          <Button asChild variant="outline" className="max-sm:h-11">
            <Link to="/admin/settings/sources">
              <Settings2 />
              {t('onlineMusic.openSettings')}
            </Link>
          </Button>
        ) : null
      }
    />
  )
}

function OnlineSearch() {
  const { t } = useTranslation('manage')
  const queryClient = useQueryClient()
  const target = useDestinationTarget()
  const [platform, setPlatform] = useState<OnlinePlatform>(() => load(PLATFORM_KEY, isPlatform, 'kw'))
  const [input, setInput] = useState('')
  const [query, setQuery] = useState('')
  const [selected, setSelected] = useState<ReadonlyMap<string, OnlineSong>>(new Map())
  const [quality, setQuality] = useState<OnlineQualityType>(() => load(QUALITY_KEY, isQuality, 'flac'))
  const [lyrics, setLyrics] = useState(() => load(LYRICS_KEY, isFlag, '1') === '1')
  const [cover, setCover] = useState(() => load(COVER_KEY, isFlag, '1') === '1')
  const [optionsOpen, setOptionsOpen] = useState(false)

  const results = useInfiniteQuery({
    queryKey: manageKeys.onlineSearch(platform, query),
    queryFn: ({ pageParam, signal }) => api.manage.online.search({ platform, q: query, page: pageParam, limit: PAGE_SIZE }, { signal }),
    initialPageParam: 1,
    getNextPageParam: (last) => (last.items.length > 0 && last.page * last.limit < last.total && last.page < 100 ? last.page + 1 : undefined),
    enabled: query !== '',
    staleTime: 5 * 60_000,
  })
  const songs = uniqueSongs(results.data?.pages.flatMap((p) => p.items) ?? [])
  const total = results.data?.pages[0]?.total ?? 0

  const start = useMutation({
    mutationFn: async (list: OnlineSong[]) => {
      const jobs = []
      for (const part of chunk(list, BATCH)) {
        const res = await api.manage.online.download({
          songs: part, quality, lyrics, cover, libraryId: target.libraryId, dir: target.dir || undefined, organize: target.organize,
        })
        jobs.push(...res.jobs)
      }
      return jobs
    },
    onSuccess: (jobs, list) => {
      queryClient.setQueryData<DownloadsStatus>(manageKeys.downloads, (prev) =>
        prev ? { ...prev, jobs: [...[...jobs].reverse(), ...prev.jobs] } : prev,
      )
      void queryClient.invalidateQueries({ queryKey: manageKeys.downloads })
      setSelected((prev) => {
        const next = new Map(prev)
        for (const song of list) next.delete(songKey(song))
        return next
      })
      toast(t('onlineMusic.queued', { count: jobs.length }))
    },
    onError: toastError,
  })

  const submit = (e: FormEvent) => {
    e.preventDefault()
    const q = input.trim().replace(/\s+/g, ' ')
    if (q) setQuery(q)
  }

  const toggle = (song: OnlineSong) =>
    setSelected((prev) => {
      const next = new Map(prev)
      const key = songKey(song)
      if (next.has(key)) next.delete(key)
      else next.set(key, song)
      return next
    })

  const allSelected = songs.length > 0 && songs.every((s) => selected.has(songKey(s)))
  const someSelected = songs.some((s) => selected.has(songKey(s)))
  const download = (list: OnlineSong[]) => {
    if (target.invalidDir) {
      toast.error(t('upload.invalidDir'))
      setOptionsOpen(true)
      return
    }
    start.mutate(list)
  }

  const summary = [
    `${target.library?.name || t('library.default')} › ${target.organize ? t('onlineMusic.organized') : target.dir || t('upload.folderPlaceholder')}`,
    t(`onlineMusic.qualities.${quality}`),
    lyrics ? t('onlineMusic.withLyrics') : '',
    cover ? t('onlineMusic.withCover') : '',
  ].filter(Boolean)

  return (
    <>
      <Collapsible open={optionsOpen || target.invalidDir} onOpenChange={setOptionsOpen} className="grid gap-3">
        <CollapsibleTrigger asChild>
          <button
            type="button"
            className="flex min-h-14 items-center gap-3 rounded-xl border px-4 py-2.5 text-left outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
          >
            <SlidersHorizontal className="size-[18px] shrink-0 text-primary" strokeWidth={1.75} aria-hidden />
            <span className="grid min-w-0 flex-1 gap-0.5">
              <span className="text-sm font-medium">{t('onlineMusic.options')}</span>
              <span className={cn('truncate text-xs', target.invalidDir ? 'text-destructive' : 'text-muted-foreground')}>
                {target.invalidDir ? t('upload.invalidDir') : summary.join(' · ')}
              </span>
            </span>
            <ChevronDown className={cn('size-4 shrink-0 text-muted-foreground transition-transform', (optionsOpen || target.invalidDir) && 'rotate-180')} aria-hidden />
          </button>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <DestinationSection>
            <div className="grid gap-4 border-t pt-4">
              <div className="grid gap-1.5">
                <Label className="text-[13px] font-medium text-foreground/75">{t('onlineMusic.quality')}</Label>
                <Select
                  value={quality}
                  onValueChange={(v) => {
                    if (!isQuality(v)) return
                    setQuality(v)
                    save(QUALITY_KEY, v)
                  }}
                >
                  <SelectTrigger className="w-full sm:w-72">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {[...QUALITIES].reverse().map((q) => (
                      <SelectItem key={q} value={q}>
                        {t(`onlineMusic.qualities.${q}`)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <p className="text-xs text-muted-foreground">{t('onlineMusic.qualityHint')}</p>
              </div>
              <Label className="flex items-start gap-3 font-normal">
                <Switch
                  className="mt-0.5"
                  checked={lyrics}
                  onCheckedChange={(v) => {
                    setLyrics(v)
                    save(LYRICS_KEY, v ? '1' : '0')
                  }}
                />
                <span className="grid gap-1">
                  <span className="text-sm font-medium">{t('onlineMusic.lyrics')}</span>
                  <span className="text-xs text-muted-foreground">{t('onlineMusic.lyricsHint')}</span>
                </span>
              </Label>
              <Label className="flex items-center gap-3 font-normal">
                <Switch
                  checked={cover}
                  onCheckedChange={(v) => {
                    setCover(v)
                    save(COVER_KEY, v ? '1' : '0')
                  }}
                />
                <span className="text-sm font-medium">{t('onlineMusic.cover')}</span>
              </Label>
            </div>
          </DestinationSection>
        </CollapsibleContent>
      </Collapsible>

      <section className="grid gap-3">
        <form onSubmit={submit} className="flex flex-wrap gap-2" role="search">
          <Select
            value={platform}
            onValueChange={(v) => {
              if (!isPlatform(v)) return
              setPlatform(v)
              save(PLATFORM_KEY, v)
            }}
          >
            <SelectTrigger className="w-full max-sm:h-11 sm:w-44" aria-label={t('onlineMusic.platform')}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {PLATFORMS.map((p) => (
                <SelectItem key={p} value={p}>
                  {t(`onlineMusic.platforms.${p}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <div className="flex min-w-0 flex-1 gap-2">
            <Input
              type="search"
              value={input}
              onChange={(e) => setInput(e.target.value)}
              placeholder={t('onlineMusic.searchPlaceholder')}
              aria-label={t('onlineMusic.searchPlaceholder')}
              enterKeyHint="search"
              maxLength={200}
              autoComplete="off"
              className="min-w-0 flex-1 max-sm:h-11"
            />
            <Button type="submit" disabled={!input.trim()} className="max-sm:size-11 max-sm:px-0">
              <Search />
              <span className="max-sm:sr-only">{t('onlineMusic.search')}</span>
            </Button>
          </div>
        </form>

        {query === '' ? (
          <p className="px-1 text-sm text-muted-foreground">{t('onlineMusic.enterQuery')}</p>
        ) : results.isPending ? (
          <PageLoader />
        ) : results.isError ? (
          <ErrorState
            size="compact"
            error={results.error}
            title={t('onlineMusic.searchFailed', { platform: t(`onlineMusic.platforms.${platform}`) })}
            onRetry={() => void results.refetch()}
            retrying={results.isFetching}
          />
        ) : songs.length === 0 ? (
          <EmptyState size="compact" icon={Search} title={t('onlineMusic.noResults')} description={t('onlineMusic.noResultsHint')} />
        ) : (
          <>
            <div className="flex min-h-11 items-center gap-3 px-3">
              <Checkbox
                checked={allSelected ? true : someSelected ? 'indeterminate' : false}
                onCheckedChange={(checked) =>
                  setSelected((prev) => {
                    const next = new Map(prev)
                    for (const song of songs) {
                      if (checked) next.set(songKey(song), song)
                      else next.delete(songKey(song))
                    }
                    return next
                  })
                }
                aria-label={t('onlineMusic.selectAll')}
              />
              <p className="tnum min-w-0 flex-1 truncate text-sm text-muted-foreground">
                {t('onlineMusic.total', { count: total, formatted: formatNumber(total) })}
              </p>
            </div>
            <ul className="divide-y rounded-xl border">
              {songs.map((song) => (
                <SongRow
                  key={songKey(song)}
                  song={song}
                  quality={quality}
                  selected={selected.has(songKey(song))}
                  onToggle={() => toggle(song)}
                  onDownload={() => download([song])}
                  busy={start.isPending}
                />
              ))}
            </ul>
            {results.hasNextPage ? (
              <Button variant="outline" className="justify-self-center max-sm:h-11" onClick={() => void results.fetchNextPage()} disabled={results.isFetchingNextPage}>
                {results.isFetchingNextPage ? <Spinner size="sm" className="text-current" /> : null}
                {t('onlineMusic.loadMore')}
              </Button>
            ) : null}
          </>
        )}
        <p className="px-1 text-xs text-muted-foreground">{t('onlineMusic.rights')}</p>
      </section>

      <div
        className={cn(
          'sticky bottom-[calc(var(--tabbar-h)+var(--miniplayer-h)+var(--safe-bottom)+1rem)] z-20 flex justify-center transition-opacity md:bottom-[calc(var(--player-clearance)+1rem)]',
          selected.size > 0 ? 'opacity-100' : 'pointer-events-none opacity-0',
        )}
        aria-hidden={selected.size === 0}
      >
        <div className="glass flex max-w-full items-center gap-2 rounded-2xl border p-2 shadow-lg">
          <span className="tnum truncate px-2 text-sm">{t('onlineMusic.selected', { count: selected.size })}</span>
          <Button variant="ghost" className="max-sm:h-11" onClick={() => setSelected(new Map())} tabIndex={selected.size > 0 ? 0 : -1}>
            {t('onlineMusic.clear')}
          </Button>
          <Button className="max-sm:h-11" onClick={() => download([...selected.values()])} disabled={start.isPending} tabIndex={selected.size > 0 ? 0 : -1}>
            {start.isPending ? <Spinner size="sm" className="text-current" /> : <Download />}
            {t('onlineMusic.download')}
          </Button>
        </div>
      </div>
    </>
  )
}

function SongRow({
  song,
  quality,
  selected,
  onToggle,
  onDownload,
  busy,
}: {
  song: OnlineSong
  quality: OnlineQualityType
  selected: boolean
  onToggle: () => void
  onDownload: () => void
  busy: boolean
}) {
  const { t } = useTranslation('manage')
  const best = bestQuality(song)
  const expected = expectedQuality(song, quality)
  const sizes = song.qualities.map((q) => `${t(`onlineMusic.badges.${q.type}`)}${q.size ? ` ${q.size}` : ''}`).join(' · ')
  const artists = song.artists.join(' / ')
  return (
    <li className={cn('flex min-h-15 items-center gap-3 px-3 py-2 transition-colors', selected && 'bg-primary/5')}>
      <Checkbox checked={selected} onCheckedChange={onToggle} aria-label={t('onlineMusic.select', { title: song.title })} className="shrink-0" />
      <button type="button" onClick={onToggle} className="flex min-w-0 flex-1 items-center gap-3 text-left outline-none">
        <SongCover url={song.coverUrl} />
        <span className="grid min-w-0 flex-1 gap-0.5">
          <span className="truncate text-sm font-medium" title={song.title}>
            {song.title}
          </span>
          <span className="truncate text-xs text-muted-foreground" title={[artists, song.album].filter(Boolean).join(' · ')}>
            {[artists, song.album].filter(Boolean).join(' · ')}
          </span>
        </span>
      </button>
      <span className="tnum hidden shrink-0 text-xs text-muted-foreground sm:block">{song.duration > 0 ? formatDuration(song.duration) : ''}</span>
      {best ? (
        <Badge
          variant={expected === 'flac' || expected === 'flac24bit' ? 'secondary' : 'outline'}
          className="shrink-0 tabular-nums"
          title={sizes}
        >
          {t(`onlineMusic.badges.${expected ?? best}`)}
        </Badge>
      ) : null}
      {song.pageUrl ? (
        <Button asChild variant="ghost" size="icon-sm" className="hidden shrink-0 text-muted-foreground sm:inline-flex" aria-label={t('onlineMusic.openPage')} title={t('onlineMusic.openPage')}>
          <a href={song.pageUrl} target="_blank" rel="noreferrer noopener">
            <ExternalLink />
          </a>
        </Button>
      ) : null}
      <Button
        variant="ghost"
        size="icon-sm"
        className="shrink-0 max-sm:size-10"
        onClick={onDownload}
        disabled={busy}
        aria-label={t('onlineMusic.downloadOne', { title: song.title })}
        title={t('onlineMusic.downloadOne', { title: song.title })}
      >
        <Download />
      </Button>
    </li>
  )
}

function SongCover({ url }: { url: string }) {
  const [failed, setFailed] = useState(false)
  return (
    <span className="grid size-11 shrink-0 place-items-center overflow-hidden rounded-md bg-muted ring-1 ring-black/5 dark:ring-white/10">
      {url && !failed ? (
        <img src={api.manage.online.coverUrl(url)} alt="" loading="lazy" decoding="async" className="size-full object-cover" onError={() => setFailed(true)} />
      ) : (
        <Music className="size-5 text-muted-foreground" strokeWidth={1.75} aria-hidden />
      )}
    </span>
  )
}
