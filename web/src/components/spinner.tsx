import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Logo } from '@/components/logo'
import { cn } from '@/lib/utils'

const SIZES = {
  sm: 'size-4',
  md: 'size-5',
  lg: 'size-8',
} as const

export interface SpinnerProps {
  size?: keyof typeof SIZES
  className?: string
  /** Accessible label (defaults to "Loading"). */
  label?: string
}

/** Indeterminate activity indicator. */
export function Spinner({ size = 'md', className, label }: SpinnerProps) {
  const { t } = useTranslation()
  return (
    <Loader2
      role="status"
      aria-label={label ?? t('a11y.loading')}
      strokeWidth={1.75}
      className={cn('animate-spin text-muted-foreground', SIZES[size], className)}
    />
  )
}

/** Centered spinner filling the page area (route / section loading). */
export function PageLoader({ className }: { className?: string }) {
  return (
    <div className={cn('flex min-h-[40vh] w-full items-center justify-center', className)}>
      <Spinner size="lg" />
    </div>
  )
}

/** Full-viewport splash (initial auth check, lazy route bootstrap). */
export function FullscreenLoader() {
  const { t } = useTranslation()
  return (
    <div
      role="status"
      aria-label={t('a11y.loading')}
      className="fixed inset-0 z-50 grid place-items-center bg-background"
    >
      <Logo size={56} className="animate-pulse drop-shadow-sm" />
    </div>
  )
}
