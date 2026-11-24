import { X } from 'lucide-react'
import { useId, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import { MULTI_SEPARATOR, splitMulti } from '../lib/tag-fields'

export interface MultiValueInputProps {
  id?: string
  /** `"Artist A; Artist B"` */
  value: string
  onChange: (value: string) => void
  placeholder?: string
  /** Placeholder of the text field once there is at least one value. */
  addPlaceholder?: string
  /** Offered as completions. */
  suggestions?: readonly string[]
  /** Also split typed text on commas (genres; artist names may contain them). */
  splitComma?: boolean
  disabled?: boolean
}

/**
 * Multi-value tag editor: every value is a chip, so the field shows that it holds several
 * values without a caption. Typing `;` (or Enter, or leaving the field) adds a chip; a click on
 * a chip takes it back into the text field for editing; Backspace on an empty field removes the
 * last one.
 */
export function MultiValueInput({
  id,
  value,
  onChange,
  placeholder,
  addPlaceholder,
  suggestions,
  splitComma = false,
  disabled,
}: MultiValueInputProps) {
  const { t } = useTranslation('manage')
  const listId = useId()
  const inputRef = useRef<HTMLInputElement>(null)
  const composing = useRef(false)
  const [text, setText] = useState('')
  const values = splitMulti(value)
  const separator = splitComma ? /[;,；，]$/ : /[;；]$/

  const commit = (raw: string, base: readonly string[] = values) => {
    const added = splitMulti((splitComma ? raw.replace(/[,，]/g, ';') : raw).replace(/；/g, ';'))
    if (added.length === 0) {
      setText('')
      return
    }
    const lower = new Set(base.map((v) => v.toLowerCase()))
    const next = [...base, ...added.filter((v) => !lower.has(v.toLowerCase()))]
    onChange(next.join(MULTI_SEPARATOR))
    setText('')
  }

  const remove = (index: number) => onChange(values.filter((_, i) => i !== index).join(MULTI_SEPARATOR))

  const edit = (index: number) => {
    const rest = values.filter((_, i) => i !== index)
    // Keep what was being typed as its own value rather than losing it.
    if (text.trim()) commit(text, rest)
    else onChange(rest.join(MULTI_SEPARATOR))
    setText(values[index] ?? '')
    inputRef.current?.focus()
  }

  return (
    <div
      className={cn(
        'flex min-h-9 w-full flex-wrap items-center gap-1.5 rounded-md border border-input bg-transparent px-2 py-1.5 shadow-xs transition-[color,box-shadow] dark:bg-input/30',
        'focus-within:border-ring focus-within:ring-[3px] focus-within:ring-ring/50',
        disabled && 'pointer-events-none opacity-50',
      )}
    >
      {values.map((v, i) => (
        <span
          key={`${v}-${i}`}
          className="inline-flex h-6 min-w-0 max-w-full items-center rounded-full bg-secondary text-xs font-medium text-secondary-foreground"
        >
          <button
            type="button"
            onClick={() => edit(i)}
            aria-label={t('editor.editValue', { value: v })}
            className="min-w-0 truncate rounded-full py-0.5 pl-2.5 hover:text-foreground"
          >
            {v}
          </button>
          <button
            type="button"
            onClick={() => remove(i)}
            aria-label={t('editor.removeValue', { value: v })}
            className="mr-0.5 grid size-5 shrink-0 place-items-center rounded-full text-muted-foreground hover:bg-background/80 hover:text-foreground"
          >
            <X className="size-3" />
          </button>
        </span>
      ))}
      <input
        ref={inputRef}
        id={id}
        value={text}
        list={suggestions ? listId : undefined}
        disabled={disabled}
        autoComplete="off"
        placeholder={values.length === 0 ? placeholder : addPlaceholder}
        onCompositionStart={() => (composing.current = true)}
        onCompositionEnd={(e) => {
          composing.current = false
          const v = e.currentTarget.value
          if (separator.test(v)) commit(v)
          else setText(v)
        }}
        onChange={(e) => {
          const v = e.target.value
          if (!composing.current && !(e.nativeEvent as InputEvent).isComposing && separator.test(v)) commit(v)
          else setText(v)
        }}
        onKeyDown={(e) => {
          if (e.nativeEvent.isComposing || composing.current) return
          if (e.key === 'Enter') {
            e.preventDefault()
            commit(text)
          } else if (e.key === 'Backspace' && !text && values.length > 0) {
            remove(values.length - 1)
          }
        }}
        onBlur={() => commit(text)}
        className="h-6 min-w-24 flex-1 bg-transparent px-1 text-base outline-none placeholder:text-muted-foreground md:text-sm"
      />
      {suggestions ? (
        <datalist id={listId}>
          {suggestions.map((s) => (
            <option key={s} value={s} />
          ))}
        </datalist>
      ) : null}
    </div>
  )
}
