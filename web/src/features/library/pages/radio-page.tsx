import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ExternalLink, Loader2, Pencil, Plus, Radio, Trash2 } from 'lucide-react'
import { useId, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Page } from '@/components/page'
import { PageHeader } from '@/components/page-header'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { usePlayer } from '@/features/player/store'
import { useAuth } from '@/hooks/use-auth'
import { useIsMobile } from '@/hooks/use-media-query'
import { api } from '@/lib/api/endpoints'
import type { RadioStation } from '@/lib/api/types'
import { errorMessage } from '@/lib/errors'
import { queryKeys } from '@/lib/query-keys'
import { cn } from '@/lib/utils'

import { ActionList, ActionMenu, ActionSheetHeader } from '../components/action-menu'
import { PlayingIndicator } from '../components/playing-indicator'
import { ResponsiveDialog } from '../components/responsive-dialog'
import type { ActionGroups } from '../lib/actions'
import { seedGradient } from '../lib/colors'
import { radiosQuery } from '../lib/queries'
import { RADIO_ID_PREFIX, isHttpUrl, radioToTrack, urlHost } from '../lib/radio'

type Editing = { mode: 'create' } | { mode: 'edit'; station: RadioStation } | null

export default function RadioPage() {
  const { t } = useTranslation('library')
  const { isAdmin } = useAuth()
  const isMobile = useIsMobile()
  const query = useQuery(radiosQuery())
  const [editing, setEditing] = useState<Editing>(null)
  const [deleting, setDeleting] = useState<RadioStation | null>(null)
  const stations = [...(query.data ?? [])].sort((a, b) => a.name.localeCompare(b.name))

  const play = (station: RadioStation) => {
    usePlayer.getState().playTracks([radioToTrack(station, t('radio.live'))], 0, { shuffle: false })
  }

  const addButton = isAdmin ? (
    <Button
      onClick={() => setEditing({ mode: 'create' })}
      variant={isMobile ? 'ghost' : 'default'}
      size={isMobile ? 'icon' : 'default'}
      aria-label={t('radio.add')}
      className={isMobile ? 'size-11 rounded-full text-primary hover:text-primary' : 'h-9 gap-2 rounded-full px-4 font-semibold'}
    >
      <Plus className={isMobile ? 'size-6' : 'size-4'} strokeWidth={2} aria-hidden />
      {isMobile ? null : t('radio.add')}
    </Button>
  ) : undefined

  return (
    <Page>
      <PageHeader
        title={t('common:nav.radio')}
        back={isMobile}
        subtitle={query.data && stations.length > 0 ? t('radio.count', { count: stations.length }) : undefined}
        navActions={isMobile ? addButton : undefined}
        actions={isMobile ? undefined : addButton}
      />

      {query.isPending ? (
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3" aria-hidden>
          {Array.from({ length: 6 }, (_, i) => (
            <div key={i} className="flex items-center gap-3 p-2">
              <Skeleton className="size-14 rounded-lg" />
              <div className="flex flex-1 flex-col gap-2">
                <Skeleton className="h-4 w-1/2" />
                <Skeleton className="h-3 w-1/3" />
              </div>
            </div>
          ))}
        </div>
      ) : query.isError ? (
        <ErrorState error={query.error} onRetry={() => void query.refetch()} retrying={query.isFetching} />
      ) : stations.length === 0 ? (
        <EmptyState
          icon={Radio}
          title={t('radio.emptyTitle')}
          description={isAdmin ? t('radio.emptyAdmin') : t('radio.emptyListener')}
          action={
            isAdmin ? (
              <Button onClick={() => setEditing({ mode: 'create' })}>
                <Plus aria-hidden />
                {t('radio.add')}
              </Button>
            ) : null
          }
        />
      ) : (
        <ul className="grid gap-x-4 md:grid-cols-2 md:gap-y-2 xl:grid-cols-3">
          {stations.map((station, i) => (
            <StationRow
              key={station.id}
              station={station}
              last={i === stations.length - 1}
              isAdmin={isAdmin}
              onPlay={() => play(station)}
              onEdit={() => setEditing({ mode: 'edit', station })}
              onDelete={() => setDeleting(station)}
            />
          ))}
        </ul>
      )}

      {isAdmin ? (
        <>
          <StationDialog editing={editing} onClose={() => setEditing(null)} />
          <DeleteStationDialog station={deleting} onClose={() => setDeleting(null)} />
        </>
      ) : null}
    </Page>
  )
}

function StationArt({ station, size = 56 }: { station: RadioStation; size?: number }) {
  return (
    <span
      aria-hidden
      className="grid shrink-0 place-items-center rounded-lg text-white shadow-sm ring-1 ring-black/5 dark:ring-white/10"
      style={{ width: size, height: size, backgroundImage: seedGradient(station.name) }}
    >
      <Radio style={{ width: size * 0.42, height: size * 0.42 }} strokeWidth={1.75} />
    </span>
  )
}

function StationRow({
  station,
  last,
  isAdmin,
  onPlay,
  onEdit,
  onDelete,
}: {
  station: RadioStation
  last: boolean
  isAdmin: boolean
  onPlay: () => void
  onEdit: () => void
  onDelete: () => void
}) {
  const { t } = useTranslation('library')
  const state = usePlayer((s) => {
    const current = s.index >= 0 ? s.queue[s.index] : undefined
    if (!current || current.id !== `${RADIO_ID_PREFIX}${station.id}`) return 'none'
    return s.isPlaying ? 'playing' : 'paused'
  })
  const current = state !== 'none'
  const host = urlHost(station.homepageUrl || station.streamUrl)

  const groups: ActionGroups = [
    station.homepageUrl && isHttpUrl(station.homepageUrl)
      ? [{ key: 'homepage', label: t('radio.openHomepage'), icon: ExternalLink, onSelect: () => window.open(station.homepageUrl, '_blank', 'noopener,noreferrer') }]
      : [],
    isAdmin
      ? [
          { key: 'edit', label: t('common:actions.edit'), icon: Pencil, deferred: true, onSelect: onEdit },
          { key: 'delete', label: t('common:actions.delete'), icon: Trash2, destructive: true, deferred: true, onSelect: onDelete },
        ]
      : [],
  ]
  const hasActions = groups.some((g) => g.length > 0)

  return (
    <li className={cn('relative flex items-center gap-1 md:rounded-xl md:hover:bg-accent/60', !last && 'hairline-inset [--hairline-inset:4.25rem] md:after:hidden')}>
      <button
        type="button"
        onClick={() => (current ? usePlayer.getState().togglePlay() : onPlay())}
        aria-label={t('radio.play', { name: station.name })}
        aria-current={current ? 'true' : undefined}
        className="flex min-w-0 flex-1 items-center gap-3 py-2 text-left outline-none active:opacity-70 md:px-2 focus-visible:[&>span:first-child]:ring-[3px] focus-visible:[&>span:first-child]:ring-ring/60"
      >
        <span className="relative">
          <StationArt station={station} />
          {current ? (
            <span className="absolute inset-0 grid place-items-center rounded-lg bg-black/30">
              <PlayingIndicator playing={state === 'playing'} className="text-white" />
            </span>
          ) : null}
        </span>
        <span className="min-w-0 flex-1">
          <span className={cn('block truncate text-[17px] font-medium md:text-[15px]', current && 'text-primary')}>
            {station.name}
          </span>
          <span className="block truncate text-[13px] text-muted-foreground">{host || t('radio.live')}</span>
        </span>
      </button>
      {hasActions ? (
        <ActionMenu
          label={t('common:actions.more')}
          className="-mr-2.5 md:mr-1"
          sheetHeader={<ActionSheetHeader art={<StationArt station={station} size={48} />} title={station.name} subtitle={host} />}
        >
          <ActionList groups={groups} />
        </ActionMenu>
      ) : null}
    </li>
  )
}

function StationDialog({ editing, onClose }: { editing: Editing; onClose: () => void }) {
  const { t } = useTranslation('library')
  const station = editing?.mode === 'edit' ? editing.station : undefined
  return (
    <ResponsiveDialog
      open={editing !== null}
      onOpenChange={(open) => (open ? undefined : onClose())}
      title={station ? t('radio.editTitle') : t('radio.addTitle')}
      description={t('radio.formDescription')}
    >
      <StationForm key={station?.id ?? 'new'} station={station} onDone={onClose} />
    </ResponsiveDialog>
  )
}

function StationForm({ station, onDone }: { station?: RadioStation; onDone: () => void }) {
  const { t } = useTranslation('library')
  const queryClient = useQueryClient()
  const id = useId()
  const [name, setName] = useState(station?.name ?? '')
  const [streamUrl, setStreamUrl] = useState(station?.streamUrl ?? '')
  const [homepageUrl, setHomepageUrl] = useState(station?.homepageUrl ?? '')
  const [touched, setTouched] = useState(false)

  const errors = {
    name: name.trim() ? '' : t('radio.nameRequired'),
    streamUrl: isHttpUrl(streamUrl.trim()) ? '' : t('radio.urlInvalid'),
    homepageUrl: !homepageUrl.trim() || isHttpUrl(homepageUrl.trim()) ? '' : t('radio.urlInvalid'),
  }
  const invalid = Object.values(errors).some(Boolean)

  const save = useMutation({
    mutationFn: () => {
      const body = { name: name.trim(), streamUrl: streamUrl.trim(), homepageUrl: homepageUrl.trim() }
      return station ? api.radios.update(station.id, body) : api.radios.create(body)
    },
    onSuccess: (saved) => {
      toast.success(station ? t('toast.stationSaved') : t('toast.stationAdded', { name: saved.name }))
      void queryClient.invalidateQueries({ queryKey: queryKeys.radios })
      onDone()
    },
    onError: (error) => toast.error(errorMessage(error, t)),
  })

  const submit = (event: FormEvent) => {
    event.preventDefault()
    setTouched(true)
    if (!invalid && !save.isPending) save.mutate()
  }

  const field = (key: keyof typeof errors, label: string, value: string, set: (v: string) => void, props: object) => (
    <div className="grid gap-2">
      <Label htmlFor={`${id}-${key}`}>{label}</Label>
      <Input
        id={`${id}-${key}`}
        value={value}
        onChange={(event) => set(event.target.value)}
        aria-invalid={touched && !!errors[key]}
        className="h-11 text-base md:h-9 md:text-sm"
        {...props}
      />
      {touched && errors[key] ? <p className="text-xs text-destructive">{errors[key]}</p> : null}
    </div>
  )

  return (
    <form onSubmit={submit} noValidate className="flex flex-col gap-5">
      {field('name', t('radio.name'), name, setName, { autoFocus: true, maxLength: 200, autoComplete: 'off' })}
      {field('streamUrl', t('radio.streamUrl'), streamUrl, setStreamUrl, {
        type: 'url',
        inputMode: 'url',
        placeholder: 'https://',
        autoComplete: 'off',
      })}
      {field('homepageUrl', t('radio.homepageUrl'), homepageUrl, setHomepageUrl, {
        type: 'url',
        inputMode: 'url',
        placeholder: t('radio.optional'),
        autoComplete: 'off',
      })}
      <div className="flex flex-col-reverse gap-2 pb-1 sm:flex-row sm:justify-end">
        <Button type="button" variant="outline" onClick={onDone} className="h-11 md:h-9">
          {t('common:actions.cancel')}
        </Button>
        <Button type="submit" disabled={save.isPending || (touched && invalid)} className="h-11 md:h-9">
          {save.isPending ? <Loader2 className="animate-spin" aria-hidden /> : null}
          {station ? t('common:actions.save') : t('common:actions.add')}
        </Button>
      </div>
    </form>
  )
}

function DeleteStationDialog({ station, onClose }: { station: RadioStation | null; onClose: () => void }) {
  const { t } = useTranslation('library')
  const queryClient = useQueryClient()
  const remove = useMutation({
    mutationFn: (target: RadioStation) => api.radios.delete(target.id),
    onSuccess: (_, target) => {
      toast.success(t('toast.stationDeleted', { name: target.name }))
      void queryClient.invalidateQueries({ queryKey: queryKeys.radios })
      onClose()
    },
    onError: (error) => toast.error(errorMessage(error, t)),
  })

  return (
    <AlertDialog open={station !== null} onOpenChange={(open) => (open ? undefined : onClose())}>
      <AlertDialogContent className="rounded-2xl">
        <AlertDialogHeader>
          <AlertDialogTitle>{t('radio.deleteTitle', { name: station?.name ?? '' })}</AlertDialogTitle>
          <AlertDialogDescription>{t('radio.deleteDescription')}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={remove.isPending}>{t('common:actions.cancel')}</AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            disabled={remove.isPending}
            onClick={(event) => {
              event.preventDefault()
              if (station) remove.mutate(station)
            }}
          >
            {t('common:actions.delete')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
