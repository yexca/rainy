import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { api } from '@/lib/api/endpoints'
import { queryKeys } from '@/lib/query-keys'

import { MultiValueInput } from './multi-value-input'

export interface GenreInputProps {
  id?: string
  /** `"Rock; Pop"` */
  value: string
  onChange: (value: string) => void
  placeholder?: string
  disabled?: boolean
}

/** Multi-value genre editor with suggestions from the library's genres. */
export function GenreInput({ id, value, onChange, placeholder, disabled }: GenreInputProps) {
  const { t } = useTranslation('manage')
  const genres = useQuery({
    queryKey: queryKeys.genres,
    queryFn: ({ signal }) => api.genres({ signal }),
    staleTime: 5 * 60_000,
  })

  return (
    <MultiValueInput
      id={id}
      value={value}
      onChange={onChange}
      placeholder={placeholder}
      addPlaceholder={t('editor.addGenre')}
      suggestions={(genres.data ?? []).map((g) => g.name)}
      splitComma
      disabled={disabled}
    />
  )
}
