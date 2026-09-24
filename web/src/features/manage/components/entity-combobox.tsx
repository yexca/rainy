import { useQuery } from '@tanstack/react-query'
import { Check, ChevronsUpDown } from 'lucide-react'
import { useDeferredValue, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { CoverArt } from '@/components/cover-art'
import { Spinner } from '@/components/spinner'
import { Button } from '@/components/ui/button'
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { api } from '@/lib/api/endpoints'
import { cn } from '@/lib/utils'

export interface EntityOption {
  id: string
  name: string
  detail?: string
  coverArt?: string
}

export interface EntityComboboxProps {
  kind: 'album' | 'artist'
  value: string
  /** Label of the current value (when known). */
  valueLabel?: string
  onChange: (option: EntityOption | null) => void
  placeholder: string
  id?: string
}

/** Searchable album / artist picker backed by the library API. */
export function EntityCombobox({ kind, value, valueLabel, onChange, placeholder, id }: EntityComboboxProps) {
  const { t } = useTranslation('manage')
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const q = useDeferredValue(search.trim())

  const query = useQuery({
    queryKey: [kind === 'album' ? 'albums' : 'artists', 'picker', q],
    enabled: open,
    staleTime: 60_000,
    queryFn: async ({ signal }): Promise<EntityOption[]> => {
      if (kind === 'album') {
        const page = await api.albums.list({ q: q || undefined, sort: 'name', limit: 30 }, { signal })
        return page.items.map((a) => ({ id: a.id, name: a.name, detail: a.artist, coverArt: a.coverArt }))
      }
      const page = await api.artists.list({ q: q || undefined, sort: 'name', limit: 30, all: true }, { signal })
      return page.items.map((a) => ({ id: a.id, name: a.name, coverArt: a.coverArt }))
    },
  })

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button id={id} variant="outline" role="combobox" aria-expanded={open} className="w-full justify-between font-normal">
          <span className={cn('truncate', !value && 'text-muted-foreground')}>{value ? valueLabel || value : placeholder}</span>
          <ChevronsUpDown className="opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-(--radix-popover-trigger-width) min-w-64 p-0" align="start">
        <Command shouldFilter={false}>
          <CommandInput value={search} onValueChange={setSearch} placeholder={t('filters.searchPlaceholder')} />
          <CommandList>
            {query.isPending ? (
              <div className="flex justify-center py-6">
                <Spinner size="sm" />
              </div>
            ) : (
              <CommandEmpty>{t('filters.noResults')}</CommandEmpty>
            )}
            <CommandGroup>
              {value ? (
                <CommandItem value="__clear" onSelect={() => {
                    onChange(null)
                    setOpen(false)
                  }} className="text-muted-foreground">
                  {t('filters.any')}
                </CommandItem>
              ) : null}
              {(query.data ?? []).map((option) => (
                <CommandItem
                  key={option.id}
                  value={option.id}
                  onSelect={() => {
                    onChange(option)
                    setOpen(false)
                  }}
                >
                  <CoverArt coverArt={option.coverArt} size={28} shape={kind === 'artist' ? 'circle' : 'square'} flat />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate">{option.name}</span>
                    {option.detail ? <span className="block truncate text-xs text-muted-foreground">{option.detail}</span> : null}
                  </span>
                  {option.id === value ? <Check className="size-4" /> : null}
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
