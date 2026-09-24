import { Search, X } from 'lucide-react'
import { AnimatePresence, motion } from 'motion/react'
import { useState, type Ref } from 'react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

export interface SearchFieldProps {
  value: string
  onChange: (value: string) => void
  onSubmit?: (value: string) => void
  /** Phones: "Cancel" clears + blurs (iOS). */
  onCancel?: () => void
  placeholder: string
  autoFocus?: boolean
  inputRef?: Ref<HTMLInputElement>
  className?: string
}

/**
 * iOS-style search field: rounded grey capsule with a magnifier and a clear button; on phones a
 * "Cancel" button slides in while focused.
 */
export function SearchField({
  value,
  onChange,
  onSubmit,
  onCancel,
  placeholder,
  autoFocus,
  inputRef,
  className,
}: SearchFieldProps) {
  const { t } = useTranslation()
  const [focused, setFocused] = useState(false)
  const showCancel = !!onCancel && (focused || value.length > 0)

  return (
    <form
      role="search"
      className={cn('flex items-center gap-2', className)}
      onSubmit={(event) => {
        event.preventDefault()
        onSubmit?.(value)
        // Hide the on-screen keyboard after "Search" on phones.
        const input = event.currentTarget.querySelector('input')
        if (window.matchMedia('(pointer: coarse)').matches) input?.blur()
      }}
    >
      <label className="relative flex min-w-0 flex-1 items-center">
        <span className="sr-only">{t('common:actions.search')}</span>
        <Search
          className="pointer-events-none absolute left-2.5 size-[18px] text-muted-foreground md:left-3 md:size-4"
          strokeWidth={2}
          aria-hidden
        />
        <input
          ref={inputRef}
          type="search"
          enterKeyHint="search"
          autoComplete="off"
          autoCorrect="off"
          autoCapitalize="none"
          spellCheck={false}
          autoFocus={autoFocus}
          value={value}
          placeholder={placeholder}
          onChange={(event) => onChange(event.target.value)}
          onFocus={() => setFocused(true)}
          onBlur={() => setFocused(false)}
          onKeyDown={(event) => {
            if (event.key === 'Escape' && value) {
              event.preventDefault()
              onChange('')
            }
          }}
          className="h-9 w-full min-w-0 appearance-none rounded-[10px] bg-secondary pr-9 pl-9 text-[17px] outline-none placeholder:text-muted-foreground focus-visible:ring-2 focus-visible:ring-ring/40 md:h-10 md:rounded-xl md:pl-10 md:text-[15px] dark:bg-secondary/80 [&::-webkit-search-cancel-button]:hidden"
        />
        {value ? (
          <button
            type="button"
            aria-label={t('common:actions.clear')}
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => onChange('')}
            className="absolute right-1 grid size-8 place-items-center rounded-full text-muted-foreground hover:text-foreground"
          >
            <span className="grid size-[18px] place-items-center rounded-full bg-muted-foreground/60 text-background">
              <X className="size-3" strokeWidth={3} aria-hidden />
            </span>
          </button>
        ) : null}
      </label>
      <AnimatePresence initial={false}>
        {showCancel ? (
          <motion.button
            type="button"
            key="cancel"
            initial={{ opacity: 0, width: 0 }}
            animate={{ opacity: 1, width: 'auto' }}
            exit={{ opacity: 0, width: 0 }}
            transition={{ type: 'spring', stiffness: 400, damping: 36 }}
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => {
              onCancel?.()
              if (document.activeElement instanceof HTMLElement) document.activeElement.blur()
            }}
            className="h-9 shrink-0 overflow-hidden text-[17px] whitespace-nowrap text-primary md:hidden"
          >
            {t('common:actions.cancel')}
          </motion.button>
        ) : null}
      </AnimatePresence>
    </form>
  )
}
