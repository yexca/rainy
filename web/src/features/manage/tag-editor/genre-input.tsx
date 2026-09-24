import { useQuery } from '@tanstack/react-query'
import { X } from 'lucide-react'
import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { api } from '@/lib/api/endpoints'
import { queryKeys } from '@/lib/query-keys'
import { cn } from '@/lib/utils'

import { MULTI_SEPARATOR, splitMulti } from '../lib/tag-fields'

export interface GenreInputProps {
  id?: string
  /** `"Rock; Pop"` */
  value: string
  onChange: (value: string) => void
  placeholder?: string
  disabled?: boolean
}

/** Multi-value genre editor: chips + free text with suggestions from the library's genres. */
export function GenreInput({ id, value, onChange, placeholder, disabled }: GenreInputProps) {
  const { t } = useTranslation('manage')
  const listId = useId()
  const [text, setText] = useState('')
  const values = splitMulti(value)
  const genres = useQuery({
    queryKey: queryKeys.genres,
    queryFn: ({ signal }) => api.genres({ signal }),
    staleTime: 5 * 60_000,
  })

  const commit = (raw: string) => {
    const added = splitMulti(raw.replace(/,/g, ';'))
    if (added.length === 0) return
    const lower = new Set(values.map((v) => v.toLowerCase()))
    const next = [...values, ...added.filter((v) => !lower.has(v.toLowerCase()))]
    onChange(next.join(MULTI_SEPARATOR))
    setText('')
  }

  const remove = (index: number) => onChange(values.filter((_, i) => i !== index).join(MULTI_SEPARATOR))

  return (
    <div
      className={cn(
        'flex min-h-9 w-full flex-wrap items-center gap-1.5 rounded-md border border-input bg-transparent px-2 py-1.5 shadow-xs transition-[color,box-shadow] dark:bg-input/30',
        'focus-within:border-ring focus-within:ring-[3px] focus-within:ring-ring/50',
        disabled && 'pointer-events-none opacity-50',
      )}
    >
      {values.map((genre, i) => (
        <span
          key={`${genre}-${i}`}
          className="inline-flex h-6 items-center gap-1 rounded-full bg-secondary pr-1 pl-2.5 text-xs font-medium text-secondary-foreground"
        >
          {genre}
          <button
            type="button"
            onClick={() => remove(i)}
            aria-label={t('editor.removeValue', { value: genre })}
            className="grid size-5 place-items-center rounded-full text-muted-foreground hover:bg-background/80 hover:text-foreground"
          >
            <X className="size-3" />
          </button>
        </span>
      ))}
      <input
        id={id}
        value={text}
        list={listId}
        disabled={disabled}
        placeholder={values.length === 0 ? placeholder : t('editor.addGenre')}
        onChange={(e) => {
          const v = e.target.value
          if (/[;,]$/.test(v)) commit(v)
          else setText(v)
        }}
        onKeyDown={(e) => {
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
      <datalist id={listId}>
        {(genres.data ?? []).map((g) => (
          <option key={g.id} value={g.name} />
        ))}
      </datalist>
    </div>
  )
}
