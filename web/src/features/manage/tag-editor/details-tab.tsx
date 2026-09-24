import { RotateCcw } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import type { TrackTags } from '@/lib/api/types'
import { cn } from '@/lib/utils'

import { FIELD_BY_ID, type FieldDef, type FieldId } from '../lib/tag-fields'
import { changeField, revertKeys, summarizeField, type Edits, type FieldSummary } from '../lib/tag-state'
import { DraftInput, DraftTextarea } from './draft-input'
import { GenreInput } from './genre-input'

export interface DetailsTabProps {
  items: readonly TrackTags[]
  edits: Edits
  onEditsChange: (edits: Edits) => void
  disabled?: boolean
}

const DATE_RE = /^\d{4}(-\d{2}(-\d{2})?)?$/

/** Common fields; with several tracks, differing values show a "Multiple values" placeholder. */
export function DetailsTab({ items, edits, onEditsChange, disabled }: DetailsTabProps) {
  const { t } = useTranslation('manage')

  const field = (id: FieldId, extra?: { className?: string; hint?: ReactNode }) => {
    const def = FIELD_BY_ID[id]
    const summary = summarizeField(items, edits, def)
    return (
      <FieldRow
        key={id}
        def={def}
        summary={summary}
        className={extra?.className}
        hint={extra?.hint}
        disabled={disabled}
        onChange={(value) => onEditsChange(changeField(items, edits, def, value))}
        onRevert={() => onEditsChange(revertKeys(edits, def.keys))}
        multipleLabel={t('editor.multipleValues')}
        label={t(`fields.${id}`)}
        revertLabel={t('editor.revertField', { field: t(`fields.${id}`) })}
      />
    )
  }

  const dateSummary = summarizeField(items, edits, FIELD_BY_ID.date)
  const dateInvalid = !!dateSummary.value && !DATE_RE.test(dateSummary.value)

  return (
    <div className="grid gap-4 pb-2">
      {items.length > 1 ? <p className="-mb-1 text-xs text-muted-foreground">{t('editor.batchHint', { count: items.length })}</p> : null}
      {field('title')}
      {field('artist', { hint: t('editor.multiHint') })}
      {field('album')}
      {field('albumArtist')}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        {field('trackNumber')}
        {field('trackTotal')}
        {field('discNumber')}
        {field('discTotal')}
      </div>
      <div className="grid grid-cols-2 gap-3">
        {field('date', { hint: dateInvalid ? <span className="text-destructive">{t('editor.dateHint')}</span> : undefined })}
        {field('bpm')}
      </div>
      {field('genre')}
      {field('composer')}
      {field('discSubtitle')}
      {field('compilation')}
      {field('comment')}
    </div>
  )
}

interface FieldRowProps {
  def: FieldDef
  summary: FieldSummary
  label: string
  multipleLabel: string
  revertLabel: string
  hint?: ReactNode
  className?: string
  disabled?: boolean
  onChange: (value: string) => void
  onRevert: () => void
}

function FieldRow({ def, summary, label, multipleLabel, revertLabel, hint, className, disabled, onChange, onRevert }: FieldRowProps) {
  const id = `tag-field-${def.id}`
  const placeholder = summary.mixed ? multipleLabel : undefined
  const mixedClass = summary.mixed ? 'placeholder:italic placeholder:text-muted-foreground/80' : undefined

  let control: ReactNode
  switch (def.kind) {
    case 'bool':
      control = (
        <div className="flex h-9 items-center gap-3">
          <Switch
            id={id}
            checked={summary.value === '1'}
            disabled={disabled}
            onCheckedChange={(checked) => onChange(checked ? '1' : '')}
          />
          {summary.mixed ? <span className="text-sm text-muted-foreground italic">{multipleLabel}</span> : null}
        </div>
      )
      break
    case 'genre':
      control = <GenreInput id={id} value={summary.value} onChange={onChange} placeholder={placeholder} disabled={disabled} />
      break
    case 'textarea':
      control = (
        <DraftTextarea
          id={id}
          value={summary.value}
          onValueChange={onChange}
          placeholder={placeholder}
          disabled={disabled}
          rows={2}
          className={cn('max-h-48 min-h-16', mixedClass)}
        />
      )
      break
    default:
      control = (
        <DraftInput
          id={id}
          value={summary.value}
          onValueChange={onChange}
          placeholder={placeholder}
          disabled={disabled}
          inputMode={def.kind === 'number' ? 'numeric' : undefined}
          autoComplete="off"
          spellCheck={def.kind === 'text' || def.kind === 'multi'}
          className={cn(def.kind === 'number' && 'tnum', mixedClass)}
        />
      )
  }

  return (
    <div className={cn('grid min-w-0 gap-1.5', className)}>
      <div className="flex h-5 items-center gap-1.5">
        <Label htmlFor={id} className="min-w-0 truncate text-[13px] font-medium text-foreground/75">
          {label}
        </Label>
        {summary.dirty ? <span className="size-1.5 shrink-0 rounded-full bg-primary" aria-hidden /> : null}
        {summary.dirty ? (
          <Button
            type="button"
            variant="ghost"
            size="icon-xs"
            className="-my-1 ml-auto text-muted-foreground"
            onClick={onRevert}
            aria-label={revertLabel}
            title={revertLabel}
          >
            <RotateCcw />
          </Button>
        ) : null}
      </div>
      {control}
      {hint ? <p className="text-xs text-muted-foreground">{hint}</p> : null}
    </div>
  )
}
