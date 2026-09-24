import { ArrowRight, TriangleAlert } from 'lucide-react'
import { useMemo, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import type { TrackTags } from '@/lib/api/types'
import { cn } from '@/lib/utils'

import { ResponsiveDialog } from '../components/responsive-dialog'
import { FIELDS, TEXT_FIELDS, type FieldId } from '../lib/tag-fields'
import type { Edits } from '../lib/tag-state'
import {
  autoNumber,
  changeCase,
  clearField,
  copyField,
  defaultAutoNumber,
  defaultCase,
  defaultClear,
  defaultCopy,
  defaultReplace,
  findReplace,
  tagsFromFilename,
  type AutoNumberOptions,
  type CaseMode,
  type CaseOptions,
  type ClearOptions,
  type CopyOptions,
  type FieldValues,
  type ReplaceOptions,
  type ToolId,
  type ToolResult,
} from '../lib/tag-tools'

const PREVIEW_LIMIT = 200
const FILENAME_PRESETS = ['{track} {title}', '{track} - {title}', '{artist} - {title}', '{track} - {artist} - {title}', '{album}/{track} {title}']
const FILENAME_PATTERN_KEY = 'rainy.manage.filenamePattern'

function loadPattern(): string {
  try {
    return localStorage.getItem(FILENAME_PATTERN_KEY) || FILENAME_PRESETS[0]
  } catch {
    return FILENAME_PRESETS[0]
  }
}

export interface ToolsDialogProps {
  tool: ToolId | null
  items: readonly TrackTags[]
  edits: Edits
  onApply: (values: Record<string, FieldValues>) => void
  onClose: () => void
}

/** Options + live preview for one batch tool; nothing changes until "Apply". */
export function ToolsDialog({ tool, items, edits, onApply, onClose }: ToolsDialogProps) {
  const { t } = useTranslation('manage')
  return (
    <ResponsiveDialog
      open={tool !== null}
      onOpenChange={(open) => (open ? undefined : onClose())}
      title={tool ? t(`tools.${tool}.title`) : ''}
      description={tool ? t(`tools.${tool}.description`) : undefined}
      size="lg"
      dialogOnly
    >
      {tool ? <ToolBody key={tool} tool={tool} items={items} edits={edits} onApply={onApply} onClose={onClose} /> : null}
    </ResponsiveDialog>
  )
}

function ToolBody({ tool, items, edits, onApply, onClose }: ToolsDialogProps & { tool: ToolId }) {
  const { t } = useTranslation('manage')
  const [autoOpts, setAutoOpts] = useState<AutoNumberOptions>(defaultAutoNumber)
  const [pattern, setPattern] = useState(loadPattern)
  const [replaceOpts, setReplaceOpts] = useState<ReplaceOptions>(defaultReplace)
  const [caseOpts, setCaseOpts] = useState<CaseOptions>(defaultCase)
  const [copyOpts, setCopyOpts] = useState<CopyOptions>(defaultCopy)
  const [clearOpts, setClearOpts] = useState<ClearOptions>(defaultClear)

  const result: ToolResult = useMemo(() => {
    switch (tool) {
      case 'autoNumber':
        return autoNumber(items, edits, autoOpts)
      case 'fromFilename':
        return tagsFromFilename(items, edits, pattern)
      case 'replace':
        return findReplace(items, edits, replaceOpts)
      case 'case':
        return changeCase(items, edits, caseOpts)
      case 'copy':
        return copyField(items, edits, copyOpts)
      case 'clear':
        return clearField(items, edits, clearOpts)
    }
  }, [tool, items, edits, autoOpts, pattern, replaceOpts, caseOpts, copyOpts, clearOpts])

  const apply = () => {
    if (tool === 'fromFilename') {
      try {
        localStorage.setItem(FILENAME_PATTERN_KEY, pattern)
      } catch {
        // storage unavailable — the pattern just isn't remembered
      }
    }
    onApply(result.values)
    onClose()
  }

  let options: ReactNode
  switch (tool) {
    case 'autoNumber':
      options = (
        <div className="grid gap-3 sm:grid-cols-2">
          <Option label={t('tools.autoNumber.start')} htmlFor="tool-start">
            <Input
              id="tool-start"
              type="number"
              min={0}
              inputMode="numeric"
              value={autoOpts.start}
              onChange={(e) => setAutoOpts({ ...autoOpts, start: Number.parseInt(e.target.value, 10) || 0 })}
            />
          </Option>
          <Option label={t('tools.autoNumber.order')}>
            <Select value={autoOpts.order} onValueChange={(v) => setAutoOpts({ ...autoOpts, order: v as AutoNumberOptions['order'] })}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="selection">{t('tools.autoNumber.orderSelection')}</SelectItem>
                <SelectItem value="filename">{t('tools.autoNumber.orderFilename')}</SelectItem>
                <SelectItem value="path">{t('tools.autoNumber.orderPath')}</SelectItem>
              </SelectContent>
            </Select>
          </Option>
          <Check checked={autoOpts.setTotal} onChange={(v) => setAutoOpts({ ...autoOpts, setTotal: v })} label={t('tools.autoNumber.setTotal')} />
          <Check checked={autoOpts.perDisc} onChange={(v) => setAutoOpts({ ...autoOpts, perDisc: v })} label={t('tools.autoNumber.perDisc')} />
        </div>
      )
      break
    case 'fromFilename':
      options = (
        <div className="grid gap-2">
          <Option label={t('tools.fromFilename.pattern')} htmlFor="tool-pattern">
            <Input
              id="tool-pattern"
              value={pattern}
              onChange={(e) => setPattern(e.target.value)}
              className="font-mono"
              autoComplete="off"
              spellCheck={false}
            />
          </Option>
          <div className="flex flex-wrap gap-1.5">
            {FILENAME_PRESETS.map((p) => (
              <button
                key={p}
                type="button"
                onClick={() => setPattern(p)}
                className={cn(
                  'rounded-full border px-2.5 py-1 font-mono text-xs transition-colors hover:bg-accent',
                  p === pattern && 'border-primary bg-primary/10 text-primary',
                )}
              >
                {p}
              </button>
            ))}
          </div>
          <p className="text-xs text-muted-foreground">{t('tools.fromFilename.tokens')}</p>
        </div>
      )
      break
    case 'replace':
      options = (
        <div className="grid gap-3 sm:grid-cols-2">
          <Option label={t('tools.field')} className="sm:col-span-2">
            <FieldSelect value={replaceOpts.field} onChange={(field) => setReplaceOpts({ ...replaceOpts, field })} allowAll textOnly />
          </Option>
          <Option label={t('tools.replace.find')} htmlFor="tool-find">
            <Input
              id="tool-find"
              value={replaceOpts.find}
              onChange={(e) => setReplaceOpts({ ...replaceOpts, find: e.target.value })}
              className={cn(replaceOpts.regex && 'font-mono')}
              autoComplete="off"
              spellCheck={false}
            />
          </Option>
          <Option label={t('tools.replace.with')} htmlFor="tool-with">
            <Input
              id="tool-with"
              value={replaceOpts.replace}
              onChange={(e) => setReplaceOpts({ ...replaceOpts, replace: e.target.value })}
              className={cn(replaceOpts.regex && 'font-mono')}
              autoComplete="off"
              spellCheck={false}
            />
          </Option>
          <Check checked={replaceOpts.regex} onChange={(regex) => setReplaceOpts({ ...replaceOpts, regex })} label={t('tools.replace.regex')} />
          <Check
            checked={replaceOpts.caseSensitive}
            onChange={(caseSensitive) => setReplaceOpts({ ...replaceOpts, caseSensitive })}
            label={t('tools.replace.caseSensitive')}
          />
        </div>
      )
      break
    case 'case':
      options = (
        <div className="grid gap-3 sm:grid-cols-2">
          <Option label={t('tools.field')}>
            <FieldSelect value={caseOpts.field} onChange={(field) => setCaseOpts({ ...caseOpts, field })} allowAll textOnly />
          </Option>
          <Option label={t('tools.case.mode')}>
            <Select value={caseOpts.mode} onValueChange={(v) => setCaseOpts({ ...caseOpts, mode: v as CaseMode })}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(['title', 'sentence', 'upper', 'lower'] as const).map((m) => (
                  <SelectItem key={m} value={m}>
                    {t(`tools.case.modes.${m}`)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Option>
        </div>
      )
      break
    case 'copy':
      options = (
        <div className="grid gap-3 sm:grid-cols-2">
          <Option label={t('tools.copy.from')}>
            <FieldSelect value={copyOpts.from} onChange={(from) => setCopyOpts({ ...copyOpts, from: from as FieldId })} textOnly />
          </Option>
          <Option label={t('tools.copy.to')}>
            <FieldSelect value={copyOpts.to} onChange={(to) => setCopyOpts({ ...copyOpts, to: to as FieldId })} textOnly />
          </Option>
          <Check checked={copyOpts.onlyEmpty} onChange={(onlyEmpty) => setCopyOpts({ ...copyOpts, onlyEmpty })} label={t('tools.copy.onlyEmpty')} />
        </div>
      )
      break
    case 'clear':
      options = (
        <Option label={t('tools.field')}>
          <FieldSelect value={clearOpts.field} onChange={(field) => setClearOpts({ field: field as FieldId })} />
        </Option>
      )
      break
  }

  const noMatch = result.rows.filter((r) => r.error === 'noMatch').length

  return (
    <div className="grid gap-5">
      {options}

      <div className="grid gap-2">
        <div className="flex items-baseline justify-between gap-2">
          <p className="text-[13px] font-medium text-foreground/75">{t('tools.preview')}</p>
          <p className="text-xs text-muted-foreground">
            {t('tools.willChange', { count: result.changed })}
            {noMatch > 0 ? ` · ${t('tools.noMatchCount', { count: noMatch })}` : ''}
          </p>
        </div>
        {result.error ? (
          <p className="flex items-center gap-2 rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">
            <TriangleAlert className="size-4 shrink-0" />
            {t(`tools.errors.${result.error}`)}
          </p>
        ) : result.rows.length === 0 ? (
          <p className="rounded-lg bg-muted/60 px-3 py-6 text-center text-sm text-muted-foreground">{t('tools.nothingToChange')}</p>
        ) : (
          <ul className="max-h-[40dvh] divide-y overflow-y-auto rounded-lg border text-[13px] sm:max-h-80">
            {result.rows.slice(0, PREVIEW_LIMIT).map((row) => (
              <li key={row.trackId} className="grid gap-1 px-3 py-2">
                <p className="truncate font-mono text-xs text-muted-foreground">{row.name}</p>
                {row.error ? (
                  <p className="text-xs text-amber-600 dark:text-amber-400">{t('tools.errors.noMatch')}</p>
                ) : (
                  row.changes.map((c) => (
                    <div key={c.field} className="grid grid-cols-[6.5rem_minmax(0,1fr)] items-baseline gap-2">
                      <span className="truncate text-xs text-muted-foreground">{t(`fields.${c.field}`)}</span>
                      <span className="flex min-w-0 flex-wrap items-baseline gap-x-1.5">
                        <span className="break-all text-muted-foreground line-through decoration-destructive/50">
                          {c.before || t('tools.empty')}
                        </span>
                        <ArrowRight className="size-3 shrink-0 self-center text-muted-foreground" aria-hidden />
                        <span className="font-medium break-all">{c.after || t('tools.empty')}</span>
                      </span>
                    </div>
                  ))
                )}
              </li>
            ))}
            {result.rows.length > PREVIEW_LIMIT ? (
              <li className="px-3 py-2 text-center text-xs text-muted-foreground">
                {t('tools.moreRows', { count: result.rows.length - PREVIEW_LIMIT })}
              </li>
            ) : null}
          </ul>
        )}
      </div>

      <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end [&_button]:max-sm:h-11">
        <Button variant="outline" onClick={onClose}>
          {t('common:actions.cancel')}
        </Button>
        <Button onClick={apply} disabled={result.changed === 0}>
          {t('tools.apply', { count: result.changed })}
        </Button>
      </div>
    </div>
  )
}

function Option({ label, htmlFor, className, children }: { label: string; htmlFor?: string; className?: string; children: ReactNode }) {
  return (
    <div className={cn('grid gap-1.5', className)}>
      <Label htmlFor={htmlFor} className="text-[13px] font-medium text-foreground/75">
        {label}
      </Label>
      {children}
    </div>
  )
}

function Check({ checked, onChange, label }: { checked: boolean; onChange: (checked: boolean) => void; label: string }) {
  return (
    <Label className="flex min-h-9 items-center gap-3 font-normal">
      <Checkbox checked={checked} onCheckedChange={(v) => onChange(v === true)} />
      {label}
    </Label>
  )
}

function FieldSelect({
  value,
  onChange,
  allowAll,
  textOnly,
}: {
  value: FieldId | 'all'
  onChange: (value: FieldId | 'all') => void
  allowAll?: boolean
  textOnly?: boolean
}) {
  const { t } = useTranslation('manage')
  const fields = textOnly ? TEXT_FIELDS : FIELDS
  return (
    <Select value={value} onValueChange={(v) => onChange(v as FieldId | 'all')}>
      <SelectTrigger className="w-full">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {allowAll ? <SelectItem value="all">{t('tools.allTextFields')}</SelectItem> : null}
        {fields.map((f) => (
          <SelectItem key={f.id} value={f.id}>
            {t(`fields.${f.id}`)}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
