import { Plus, RotateCcw, Trash2, X } from 'lucide-react'
import { useMemo, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { TrackTags } from '@/lib/api/types'
import { cn } from '@/lib/utils'

import { HIDDEN_RAW_KEYS, TAG_KEY_PATTERN, normalizeTagKey } from '../lib/tag-fields'
import { revertKeys, setRawKey, summarizeRawKeys, type Edits, type RawKeySummary } from '../lib/tag-state'

export interface RawTabProps {
  items: readonly TrackTags[]
  edits: Edits
  onEditsChange: (edits: Edits) => void
  disabled?: boolean
}

/** Every tag key as stored in the files (incl. custom keys): edit values, add or remove keys. */
export function RawTab({ items, edits, onEditsChange, disabled }: RawTabProps) {
  const { t } = useTranslation('manage')
  const rows = summarizeRawKeys(items, edits, HIDDEN_RAW_KEYS)
  // Keys whose values differ between the loaded tracks (before any edit).
  const originallyMixed = useMemo(
    () => new Set(summarizeRawKeys(items, {}, HIDDEN_RAW_KEYS).filter((r) => r.mixed).map((r) => r.key)),
    [items],
  )
  const [newKey, setNewKey] = useState('')
  const [newValue, setNewValue] = useState('')
  const key = normalizeTagKey(newKey)
  const keyInvalid = !!newKey && (!TAG_KEY_PATTERN.test(key) || HIDDEN_RAW_KEYS.has(key))
  const exists = rows.some((r) => r.key === key && (r.values.length > 0 || r.mixed))

  const add = (e: FormEvent) => {
    e.preventDefault()
    if (!key || keyInvalid || !newValue.trim()) return
    onEditsChange(setRawKey(items, edits, key, [newValue.trim()]))
    setNewKey('')
    setNewValue('')
  }

  return (
    <div className="grid gap-4 pb-2">
      <p className="text-xs text-muted-foreground">{t('raw.hint')}</p>
      {rows.length === 0 ? <p className="py-6 text-center text-sm text-muted-foreground">{t('raw.empty')}</p> : null}
      <ul className="grid gap-px overflow-hidden rounded-lg border bg-border">
        {rows.map((row) => (
          <RawRow
            key={row.key}
            row={row}
            disabled={disabled}
            onChange={(values) =>
              // Emptying the input of a key whose tracks disagree restores their own values (like
              // the Details tab); deleting the key everywhere is the explicit "Delete key" button.
              onEditsChange(
                originallyMixed.has(row.key) && values.length > 0 && values.every((v) => v.trim() === '')
                  ? revertKeys(edits, [row.key])
                  : setRawKey(items, edits, row.key, values),
              )
            }
            onRevert={() => onEditsChange(revertKeys(edits, [row.key]))}
          />
        ))}
      </ul>

      <form onSubmit={add} className="grid gap-2 rounded-lg border border-dashed p-3">
        <p className="text-[13px] font-medium text-foreground/75">{t('raw.addTitle')}</p>
        <div className="grid gap-2 sm:grid-cols-[minmax(0,2fr)_minmax(0,3fr)_auto]">
          <Input
            value={newKey}
            onChange={(e) => setNewKey(e.target.value)}
            placeholder={t('raw.keyPlaceholder')}
            aria-label={t('raw.key')}
            aria-invalid={keyInvalid || undefined}
            autoCapitalize="characters"
            autoComplete="off"
            spellCheck={false}
            className="font-mono uppercase"
            disabled={disabled}
          />
          <Input
            value={newValue}
            onChange={(e) => setNewValue(e.target.value)}
            placeholder={t('raw.valuePlaceholder')}
            aria-label={t('raw.value')}
            disabled={disabled}
          />
          <Button type="submit" variant="secondary" disabled={disabled || !key || keyInvalid || !newValue.trim()}>
            <Plus />
            {exists ? t('raw.replace') : t('common:actions.add')}
          </Button>
        </div>
        {keyInvalid ? <p className="text-xs text-destructive">{t('raw.invalidKey')}</p> : null}
      </form>
    </div>
  )
}

interface RawRowProps {
  row: RawKeySummary
  disabled?: boolean
  onChange: (values: string[]) => void
  onRevert: () => void
}

function RawRow({ row, disabled, onChange, onRevert }: RawRowProps) {
  const { t } = useTranslation('manage')
  // Empty strings are kept while editing (so inputs don't vanish) and stripped on save.
  const deleted = !row.mixed && row.values.every((v) => v === '')
  // Always show at least one input so a cleared value can be typed again.
  const inputs = row.values.length > 0 ? row.values : ['']

  const setAt = (index: number, value: string) => {
    const next = [...inputs]
    next[index] = value
    onChange(next)
  }

  return (
    <li className="grid gap-2 bg-background p-3 sm:grid-cols-[10rem_minmax(0,1fr)] sm:gap-3">
      <div className="flex min-w-0 items-start gap-1.5 sm:pt-2">
        <span
          className={cn(
            'min-w-0 font-mono text-xs font-medium break-all text-foreground/80',
            deleted && 'text-muted-foreground line-through',
          )}
        >
          {row.key}
        </span>
        {row.dirty ? <span className="mt-1 size-1.5 shrink-0 rounded-full bg-primary" aria-hidden /> : null}
        {row.partial ? (
          <Badge variant="outline" className="ml-auto h-5 text-[10px] sm:ml-0">
            {t('raw.partial')}
          </Badge>
        ) : null}
      </div>
      <div className="grid min-w-0 gap-1.5">
        {inputs.map((value, i) => (
          <div key={i} className="flex items-center gap-1">
            <Input
              value={value}
              onChange={(e) => setAt(i, e.target.value)}
              placeholder={row.mixed ? t('editor.multipleValues') : deleted ? t('raw.deleted') : undefined}
              aria-label={t('raw.valueOf', { key: row.key })}
              disabled={disabled}
              className={cn('h-8', row.mixed && 'placeholder:italic')}
            />
            {inputs.length > 1 ? (
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                className="text-muted-foreground"
                aria-label={t('raw.removeValue')}
                disabled={disabled}
                onClick={() => onChange(inputs.filter((_, j) => j !== i))}
              >
                <X />
              </Button>
            ) : null}
          </div>
        ))}
        <div className="flex items-center gap-1">
          <Button
            type="button"
            variant="ghost"
            size="xs"
            className="text-muted-foreground"
            disabled={disabled || row.mixed || deleted}
            onClick={() => onChange([...inputs, ''])}
          >
            <Plus />
            {t('raw.addValue')}
          </Button>
          {row.dirty ? (
            <Button type="button" variant="ghost" size="xs" className="text-muted-foreground" onClick={onRevert}>
              <RotateCcw />
              {t('editor.revert')}
            </Button>
          ) : null}
          {!deleted ? (
            <Button
              type="button"
              variant="ghost"
              size="xs"
              className="ml-auto text-destructive hover:text-destructive"
              disabled={disabled}
              onClick={() => onChange([])}
            >
              <Trash2 />
              {t('raw.deleteKey')}
            </Button>
          ) : null}
        </div>
      </div>
    </li>
  )
}
