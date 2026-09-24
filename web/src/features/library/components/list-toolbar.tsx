import { ArrowDownWideNarrow, ArrowUpNarrowWide, ChevronDown, X } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import type { SortOrder } from '@/lib/api/endpoints'
import { cn } from '@/lib/utils'

/** 36px pill; the `after:` box stretches the touch target to 44px on phones (§9.3). */
const PILL =
  "relative inline-flex h-9 shrink-0 items-center gap-1.5 rounded-full border border-border/70 bg-background px-3.5 text-[13px] font-medium whitespace-nowrap transition-colors outline-none after:absolute after:inset-x-0 after:-inset-y-1 after:content-[''] hover:bg-accent focus-visible:ring-[3px] focus-visible:ring-ring/50 data-[state=open]:bg-accent md:h-8 md:after:hidden"

/** Horizontal row of pills; scrolls sideways on phones instead of wrapping. */
export function ListToolbar({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div
      className={cn(
        'bleed-x page-x scrollbar-none -mt-2 mb-3 flex items-center gap-2 overflow-x-auto py-1 md:mx-0 md:mb-4 md:overflow-visible md:px-0',
        className,
      )}
    >
      {children}
    </div>
  )
}

export interface SortOption<T extends string> {
  value: T
  label: string
}

export interface SortMenuProps<T extends string> {
  options: readonly SortOption<T>[]
  value: T
  onChange: (value: T) => void
  order?: SortOrder
  onOrderChange?: (order: SortOrder) => void
}

/** "Sort: Name ▾" pill with a radio list of fields and ascending / descending. */
export function SortMenu<T extends string>({ options, value, onChange, order, onOrderChange }: SortMenuProps<T>) {
  const { t } = useTranslation()
  const current = options.find((o) => o.value === value) ?? options[0]
  const OrderIcon = order === 'desc' ? ArrowDownWideNarrow : ArrowUpNarrowWide
  return (
    <DropdownMenu>
      <DropdownMenuTrigger className={PILL}>
        <OrderIcon className="size-4 text-muted-foreground" strokeWidth={1.75} aria-hidden />
        <span>
          <span className="text-muted-foreground">{t('library:sort.labelWithColon')}</span>
          {current.label}
        </span>
        <ChevronDown className="size-3.5 text-muted-foreground" aria-hidden />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="min-w-52 rounded-xl">
        <DropdownMenuLabel className="text-xs text-muted-foreground">{t('library:sort.by')}</DropdownMenuLabel>
        <DropdownMenuRadioGroup value={value} onValueChange={(v) => onChange(v as T)}>
          {options.map((option) => (
            <DropdownMenuRadioItem key={option.value} value={option.value}>
              {option.label}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
        {order && onOrderChange ? (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuRadioGroup value={order} onValueChange={(v) => onOrderChange(v === 'desc' ? 'desc' : 'asc')}>
              <DropdownMenuRadioItem value="asc">{t('library:sort.asc')}</DropdownMenuRadioItem>
              <DropdownMenuRadioItem value="desc">{t('library:sort.desc')}</DropdownMenuRadioItem>
            </DropdownMenuRadioGroup>
          </>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

/** On/off filter pill (e.g. "Favorites"). */
export function FilterChip({
  active,
  onToggle,
  icon,
  children,
}: {
  active: boolean
  onToggle: () => void
  icon?: ReactNode
  children: ReactNode
}) {
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onToggle}
      className={cn(PILL, active && 'border-primary/40 bg-primary/10 text-primary hover:bg-primary/15')}
    >
      {icon}
      {children}
    </button>
  )
}

/** Pill with a scrollable single-choice dropdown (e.g. genre filter); `null` = no filter. */
export function ChoiceFilter({
  label,
  value,
  options,
  onChange,
  allLabel,
}: {
  label: string
  value: string | null
  options: readonly string[]
  onChange: (value: string | null) => void
  allLabel: string
}) {
  const { t } = useTranslation()
  const active = value !== null
  return (
    <div className="flex shrink-0 items-center">
      <DropdownMenu>
        <DropdownMenuTrigger
          className={cn(PILL, active && 'rounded-r-none border-r-0 border-primary/40 bg-primary/10 text-primary')}
        >
          <span className={cn(!active && 'text-muted-foreground')}>{label}</span>
          {active ? <span className="max-w-40 truncate">: {value}</span> : null}
          <ChevronDown className="size-3.5 opacity-70" aria-hidden />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="max-h-80 min-w-52 overflow-y-auto rounded-xl">
          <DropdownMenuRadioGroup value={value ?? ''} onValueChange={(v) => onChange(v || null)}>
            <DropdownMenuRadioItem value="">{allLabel}</DropdownMenuRadioItem>
            <DropdownMenuSeparator />
            {options.map((option) => (
              <DropdownMenuRadioItem key={option} value={option}>
                {option}
              </DropdownMenuRadioItem>
            ))}
          </DropdownMenuRadioGroup>
        </DropdownMenuContent>
      </DropdownMenu>
      {active ? (
        <button
          type="button"
          aria-label={t('library:filter.clear', { name: label })}
          onClick={() => onChange(null)}
          className={cn(PILL, 'rounded-l-none border-l-0 border-primary/40 bg-primary/10 px-2 text-primary hover:bg-primary/15')}
        >
          <X className="size-3.5" aria-hidden />
        </button>
      ) : null}
    </div>
  )
}
