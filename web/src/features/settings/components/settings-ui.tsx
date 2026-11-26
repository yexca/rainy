import { Check, Copy } from 'lucide-react'
import { useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { cn } from '@/lib/utils'

/** iOS-style grouped section: small title, rounded card of rows, optional footnote. */
export function SettingsSection({
  id,
  title,
  description,
  footer,
  children,
  className,
}: {
  id: string
  title: string
  description?: ReactNode
  footer?: ReactNode
  children: ReactNode
  className?: string
}) {
  return (
    <section id={id} aria-labelledby={`${id}-title`} className={cn('scroll-mt-[calc(var(--app-header-h)+4rem)]', className)}>
      <div className="px-1 pb-2">
        <h2 id={`${id}-title`} className="text-[13px] font-semibold tracking-wide text-muted-foreground uppercase">
          {title}
        </h2>
        {description ? <p className="mt-0.5 text-[13px] text-muted-foreground">{description}</p> : null}
      </div>
      <div className="divide-y divide-border/70 overflow-hidden rounded-xl border bg-card text-card-foreground shadow-xs">
        {children}
      </div>
      {footer ? <div className="px-1 pt-2 text-[13px] leading-relaxed text-muted-foreground">{footer}</div> : null}
    </section>
  )
}

/** One row: label (+ description) and a control; `stacked` puts the control below (mobile-friendly). */
export function SettingsRow({
  label,
  description,
  htmlFor,
  children,
  stacked,
  className,
}: {
  label: ReactNode
  description?: ReactNode
  htmlFor?: string
  children?: ReactNode
  stacked?: boolean
  className?: string
}) {
  const Label = htmlFor ? 'label' : 'div'
  return (
    <div
      className={cn(
        'flex min-h-12 gap-x-4 gap-y-3 px-4 py-3',
        stacked ? 'flex-col items-stretch sm:flex-row sm:items-center' : 'items-center',
        className,
      )}
    >
      <div className="min-w-0 flex-1">
        <Label htmlFor={htmlFor} className="block text-[15px] font-medium md:text-sm">
          {label}
        </Label>
        {description ? <p className="mt-0.5 text-[13px] leading-snug text-muted-foreground">{description}</p> : null}
      </div>
      {children !== undefined ? <div className={cn('flex shrink-0 items-center gap-2', stacked && 'sm:justify-end')}>{children}</div> : null}
    </div>
  )
}

async function writeClipboard(text: string): Promise<void> {
  if (navigator.clipboard && window.isSecureContext) {
    await navigator.clipboard.writeText(text)
    return
  }
  // Plain-HTTP NAS installs have no async clipboard: fall back to a hidden textarea.
  const area = document.createElement('textarea')
  area.value = text
  area.setAttribute('readonly', '')
  area.style.position = 'fixed'
  area.style.opacity = '0'
  document.body.appendChild(area)
  area.select()
  try {
    if (!document.execCommand('copy')) throw new Error('copy failed')
  } finally {
    area.remove()
  }
}

/** Icon button that copies `value` and briefly shows a check mark. */
export function CopyButton({ value, label, className }: { value: string; label: string; className?: string }) {
  const { t } = useTranslation()
  const [copied, setCopied] = useState(false)
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

  const copy = async () => {
    try {
      await writeClipboard(value)
      setCopied(true)
      clearTimeout(timer.current)
      timer.current = setTimeout(() => setCopied(false), 1600)
      toast.success(t('actions.copied'), { id: 'copied' })
    } catch {
      toast.error(t('errors.unknown'))
    }
  }

  return (
    <button
      type="button"
      onClick={() => void copy()}
      aria-label={label}
      title={label}
      className={cn(
        'grid size-9 shrink-0 place-items-center rounded-md text-muted-foreground transition-colors outline-none hover:bg-accent hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50',
        copied && 'text-primary hover:text-primary',
        className,
      )}
    >
      {copied ? <Check className="size-4" /> : <Copy className="size-4" />}
    </button>
  )
}

/** Monospace value with a copy button (server URL, username, API key). */
export function CopyField({ value, label, className }: { value: string; label: string; className?: string }) {
  const { t } = useTranslation('settings')
  return (
    <div className={cn('flex min-w-0 items-center gap-1 rounded-lg border bg-muted/40 py-0.5 pr-0.5 pl-3', className)}>
      <code className="min-w-0 flex-1 truncate font-mono text-[13px] select-all" title={value}>
        {value}
      </code>
      <CopyButton value={value} label={t('copyValue', { label })} />
    </div>
  )
}
