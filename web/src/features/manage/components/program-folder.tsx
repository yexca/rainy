import { Podcast } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { DropdownMenuItem } from '@/components/ui/dropdown-menu'
import { cn } from '@/lib/utils'

import type { ProgramStatus } from '../lib/program-folders'

/**
 * Badge for a marked folder, or a quieter one for a folder inside a marked one or in a program
 * library. `compact` shows only the icon on phones (the label stays for assistive tech).
 */
export function ProgramFolderBadge({ status, compact = false }: { status: ProgramStatus; compact?: boolean }) {
  const { t } = useTranslation('manage')
  if (!status) return null
  const label =
    status === 'marked'
      ? t('folders.program.badge')
      : status === 'library'
        ? t('folders.program.library')
        : t('folders.program.inherited')
  return (
    <Badge
      variant="outline"
      className={cn('shrink-0 gap-1', status === 'marked' ? 'border-primary/40 text-primary' : 'text-muted-foreground')}
      title={label}
    >
      <Podcast aria-hidden />
      <span className={cn(compact && 'sr-only')}>{label}</span>
    </Badge>
  )
}

/**
 * Actions-menu entry marking the current folder, or unmarking it; none inside a marked folder or
 * a program library, where every folder already holds programs.
 */
export function ProgramFolderMenuItem({
  status,
  disabled,
  onToggle,
}: {
  status: ProgramStatus
  disabled?: boolean
  onToggle: (mark: boolean) => void
}) {
  const { t } = useTranslation('manage')
  if (status === 'inherited' || status === 'library') return null
  return (
    <DropdownMenuItem onSelect={() => onToggle(status !== 'marked')} disabled={disabled}>
      <Podcast />
      {status === 'marked' ? t('folders.program.unmark') : t('folders.program.mark')}
    </DropdownMenuItem>
  )
}
