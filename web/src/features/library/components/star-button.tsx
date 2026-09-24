import { Star } from 'lucide-react'
import { motion } from 'motion/react'
import { useTranslation } from 'react-i18next'

import type { StarType } from '@/lib/api/types'
import { cn } from '@/lib/utils'

import { useToggleStar } from '../lib/star'

export interface StarButtonProps {
  type: StarType
  item: { id: string; starred: boolean }
  /** `sm` for table rows, `md` default, `lg` for headers. */
  size?: 'sm' | 'md' | 'lg'
  className?: string
  /** Don't toast on success (dense lists). */
  silent?: boolean
}

const SIZES = {
  sm: { button: 'size-8', icon: 'size-4' },
  md: { button: 'size-11 md:size-9', icon: 'size-5' },
  lg: { button: 'size-11', icon: 'size-[22px]' },
} as const

/** Favorite toggle with an optimistic update and a small spring "pop". */
export function StarButton({ type, item, size = 'md', className, silent }: StarButtonProps) {
  const { t } = useTranslation()
  const toggleStar = useToggleStar()
  // A toggle keeps one name; `aria-pressed` announces the state (the tooltip says what a click does).
  const label = t('common:actions.favorite')
  const hint = item.starred ? t('common:actions.unfavorite') : label
  const s = SIZES[size]

  return (
    <button
      type="button"
      aria-label={label}
      aria-pressed={item.starred}
      title={hint}
      onClick={(event) => {
        event.stopPropagation()
        void toggleStar(type, [item], !item.starred, { silent })
      }}
      onDoubleClick={(event) => event.stopPropagation()}
      className={cn(
        'inline-grid shrink-0 place-items-center rounded-full transition-colors outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50',
        item.starred ? 'text-primary' : 'text-muted-foreground hover:text-foreground',
        s.button,
        className,
      )}
    >
      <motion.span
        key={item.starred ? 'on' : 'off'}
        initial={{ scale: item.starred ? 0.6 : 1 }}
        animate={{ scale: 1 }}
        transition={{ type: 'spring', stiffness: 500, damping: 18 }}
        className="grid place-items-center"
      >
        <Star className={s.icon} strokeWidth={1.75} fill={item.starred ? 'currentColor' : 'none'} aria-hidden />
      </motion.span>
    </button>
  )
}
