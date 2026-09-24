import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { useId, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { api } from '@/lib/api/endpoints'
import type { Playlist } from '@/lib/api/types'
import { errorMessage } from '@/lib/errors'
import { queryKeys } from '@/lib/query-keys'

import { ResponsiveDialog } from './responsive-dialog'

const PLAYLIST_NAME_MAX = 200
const PLAYLIST_COMMENT_MAX = 2000

export interface PlaylistFormDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Edit this playlist; omit to create a new one. */
  playlist?: Playlist
  /** Tracks for a new playlist. */
  trackIds?: string[]
  onSaved?: (playlist: Playlist) => void
}

/** Create a playlist or edit its name / description / visibility. */
export function PlaylistFormDialog({ open, onOpenChange, playlist, trackIds, onSaved }: PlaylistFormDialogProps) {
  const { t } = useTranslation()
  const editing = !!playlist
  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={onOpenChange}
      title={editing ? t('library:playlist.editTitle') : t('library:playlist.createTitle')}
      description={
        editing
          ? t('library:playlist.editDescription')
          : trackIds?.length
            ? t('library:playlist.createWithSongs', { count: trackIds.length })
            : t('library:playlist.createDescription')
      }
    >
      <PlaylistForm
        playlist={playlist}
        trackIds={trackIds}
        onCancel={() => onOpenChange(false)}
        onSaved={(saved) => {
          onOpenChange(false)
          onSaved?.(saved)
        }}
      />
    </ResponsiveDialog>
  )
}

function PlaylistForm({
  playlist,
  trackIds,
  onCancel,
  onSaved,
}: {
  playlist?: Playlist
  trackIds?: string[]
  onCancel: () => void
  onSaved: (playlist: Playlist) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const id = useId()
  const [name, setName] = useState(playlist?.name ?? '')
  const [comment, setComment] = useState(playlist?.comment ?? '')
  const [isPublic, setIsPublic] = useState(playlist?.public ?? false)
  const [touched, setTouched] = useState(false)
  const trimmed = name.trim()
  const invalid = trimmed.length === 0

  const save = useMutation({
    mutationFn: () =>
      playlist
        ? api.playlists.update(playlist.id, { name: trimmed, comment: comment.trim(), public: isPublic })
        : api.playlists.create({ name: trimmed, comment: comment.trim(), public: isPublic, trackIds }),
    onSuccess: (saved) => {
      toast.success(playlist ? t('library:toast.playlistSaved') : t('library:toast.playlistCreated', { name: saved.name }))
      void queryClient.invalidateQueries({ queryKey: queryKeys.playlists })
      onSaved(saved)
    },
    onError: (error) => toast.error(errorMessage(error, t)),
  })

  const onSubmit = (event: FormEvent) => {
    event.preventDefault()
    setTouched(true)
    if (invalid || save.isPending) return
    save.mutate()
  }

  return (
    <form onSubmit={onSubmit} className="flex flex-col gap-5" noValidate>
      <div className="grid gap-2">
        <Label htmlFor={`${id}-name`}>{t('library:playlist.name')}</Label>
        <Input
          id={`${id}-name`}
          value={name}
          maxLength={PLAYLIST_NAME_MAX}
          autoFocus
          autoComplete="off"
          placeholder={t('library:playlist.namePlaceholder')}
          aria-invalid={touched && invalid}
          onChange={(event) => setName(event.target.value)}
          className="h-11 text-base md:h-9 md:text-sm"
        />
        {touched && invalid ? <p className="text-xs text-destructive">{t('library:playlist.nameRequired')}</p> : null}
      </div>
      <div className="grid gap-2">
        <Label htmlFor={`${id}-comment`}>{t('library:playlist.description')}</Label>
        <Textarea
          id={`${id}-comment`}
          value={comment}
          maxLength={PLAYLIST_COMMENT_MAX}
          rows={3}
          placeholder={t('library:playlist.descriptionPlaceholder')}
          onChange={(event) => setComment(event.target.value)}
          className="max-h-40 resize-none"
        />
      </div>
      <div className="flex items-center justify-between gap-4 rounded-xl bg-muted/60 px-4 py-3">
        <div className="min-w-0">
          <Label htmlFor={`${id}-public`} className="text-sm">
            {t('library:playlist.public')}
          </Label>
          <p className="mt-1 text-xs text-muted-foreground">{t('library:playlist.publicHint')}</p>
        </div>
        <Switch id={`${id}-public`} checked={isPublic} onCheckedChange={setIsPublic} />
      </div>
      <div className="flex flex-col-reverse gap-2 pb-1 sm:flex-row sm:justify-end">
        <Button type="button" variant="outline" onClick={onCancel} className="h-11 md:h-9">
          {t('common:actions.cancel')}
        </Button>
        <Button type="submit" disabled={save.isPending || (touched && invalid)} className="h-11 md:h-9">
          {save.isPending ? <Loader2 className="animate-spin" aria-hidden /> : null}
          {playlist ? t('common:actions.save') : t('common:actions.create')}
        </Button>
      </div>
    </form>
  )
}
