import { ClipboardCheck, FolderOpen, FolderTree, HardDrive, Settings2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Toggle } from '@/components/ui/toggle'
import { useLibraries } from '@/features/admin/queries'
import { useAuth } from '@/hooks/use-auth'
import { groupedPages, TRACKS_SECTION } from '@/layouts/nav'
import { SubTabs } from '@/layouts/sub-tabs'
import { cn } from '@/lib/utils'

import { useDestination, useDestinationTarget } from '../lib/destination'
import { FolderPicker } from './folder-picker'

/** The pages grouped under Tracks → Add (Upload, Links, Online). */
const SOURCES = groupedPages(TRACKS_SECTION.tabs, '/manage/upload')

const chip = 'h-8 max-w-full gap-1.5 rounded-full px-3 text-xs font-medium max-sm:h-11'

/**
 * Top of Tracks → Add: where the music comes from (Upload / Links / Online) on the left, where it
 * goes on the right — library, folder, whether the server organizes it by tags, and (with
 * `review`) whether downloads wait in Tracks → Review. The destination is shared by the three
 * pages (`useDestination`); details open on demand from each chip.
 */
export function AddBar({ destination = true, review = false }: { destination?: boolean; review?: boolean }) {
  const { t } = useTranslation('manage')
  const { t: tc } = useTranslation()
  const target = useDestinationTarget()
  return (
    <div className="grid gap-2">
      <div className="flex flex-wrap items-center gap-2">
        <SubTabs items={SOURCES} label={tc('nav.add')} />
        {destination ? (
          <div className="flex min-w-0 flex-wrap items-center gap-1.5 sm:ml-auto">
            <LibrarySelect />
            <FolderChip />
            <OrganizeChip />
            {review ? <ReviewToggle /> : null}
          </div>
        ) : null}
      </div>
      {destination && target.invalidDir ? <p className="text-xs text-destructive sm:text-right">{t('upload.invalidDir')}</p> : null}
    </div>
  )
}

function LibrarySelect() {
  const { t } = useTranslation('manage')
  const { libraries, limited } = useLibraries()
  const setLibraryId = useDestination((s) => s.setLibraryId)
  const target = useDestinationTarget()
  if (limited || libraries.length < 2) return null
  return (
    <Select value={String(target.libraryId)} onValueChange={(v) => setLibraryId(Number(v))}>
      <SelectTrigger aria-label={t('filters.library')} className={cn(chip, 'w-auto max-w-44 shadow-none')}>
        <HardDrive className="size-3.5 shrink-0" />
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
  )
}

function FolderChip() {
  const { t } = useTranslation('manage')
  const dir = useDestination((s) => s.dir)
  const setDir = useDestination((s) => s.setDir)
  const target = useDestinationTarget()
  const [open, setOpen] = useState(false)
  const [pickerOpen, setPickerOpen] = useState(false)
  const label = target.dir || t('upload.folderPlaceholder')

  return (
    <>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <Button
            variant="outline"
            size="sm"
            aria-label={`${t('upload.folder')}: ${label}`}
            aria-invalid={target.invalidDir || undefined}
            className={cn(chip, target.invalidDir && 'border-destructive text-destructive')}
          >
            <FolderOpen className="size-3.5" />
            <span className={cn('min-w-0 truncate', target.dir && 'font-mono')}>{label}</span>
          </Button>
        </PopoverTrigger>
        <PopoverContent align="end" className="grid w-80 gap-2">
          <Label htmlFor="destination-dir" className="text-[13px] font-medium text-foreground/75">
            {t('upload.folder')}
          </Label>
          <div className="flex gap-2">
            <Input
              id="destination-dir"
              value={dir}
              onChange={(e) => setDir(e.target.value)}
              placeholder={t('upload.folderPlaceholder')}
              aria-invalid={target.invalidDir || undefined}
              className="font-mono text-sm"
              autoComplete="off"
              spellCheck={false}
            />
            <Button
              variant="outline"
              size="icon"
              className="shrink-0 max-sm:size-11"
              aria-label={t('upload.browse')}
              title={t('upload.browse')}
              onClick={() => {
                setOpen(false)
                setPickerOpen(true)
              }}
            >
              <FolderOpen />
            </Button>
          </div>
        </PopoverContent>
      </Popover>
      <FolderPicker
        open={pickerOpen}
        onOpenChange={setPickerOpen}
        libraryId={target.libraryId}
        libraryName={target.library?.name ?? ''}
        initialDir={target.dir}
        onSelect={setDir}
      />
    </>
  )
}

function OrganizeChip() {
  const { t } = useTranslation('manage')
  const { isAdmin } = useAuth()
  const target = useDestinationTarget()
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="sm" className={cn(chip, 'bg-muted/60 text-muted-foreground hover:text-foreground')}>
          <FolderTree className="size-3.5" />
          {target.organize ? t('upload.byTags') : t('upload.asIs')}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="grid w-80 gap-3 text-sm">
        <p className="font-medium">{target.organize ? t('upload.organize') : t('upload.organizeOff')}</p>
        {target.organize ? (
          <code className="rounded-md bg-muted px-2 py-1.5 font-mono text-xs break-all">{target.renamePattern}</code>
        ) : (
          <p className="text-xs text-muted-foreground">{t('upload.organizeOffHint')}</p>
        )}
        {isAdmin ? (
          <Button asChild variant="outline" size="sm" className="justify-self-start max-sm:h-11">
            <Link to="/admin/settings">
              <Settings2 />
              {t('upload.organizeSettings')}
            </Link>
          </Button>
        ) : null}
      </PopoverContent>
    </Popover>
  )
}

function ReviewToggle() {
  const { t } = useTranslation('manage')
  const setReview = useDestination((s) => s.setReview)
  const target = useDestinationTarget()
  return (
    <Toggle
      variant="outline"
      size="sm"
      pressed={target.review}
      onPressedChange={setReview}
      title={t('review.optionHint')}
      aria-label={t('review.option')}
      className={cn(chip, 'shadow-none data-[state=on]:border-primary/50 data-[state=on]:bg-primary/10 data-[state=on]:text-primary')}
    >
      <ClipboardCheck className="size-3.5" />
      {t('review.chip')}
    </Toggle>
  )
}
