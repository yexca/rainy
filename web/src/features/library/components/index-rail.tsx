import { useRef, type PointerEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

export interface IndexRailProps {
  /** Letters in display order. */
  letters: readonly string[]
  /** Letters that have entries (others are dimmed and skipped). */
  available: ReadonlySet<string>
  onJump: (letter: string) => void
  className?: string
}

/**
 * A–Z jump rail (iOS contacts style). Tap or drag along it to jump; letters without entries are
 * dimmed. Keyboard users get regular buttons.
 */
export function IndexRail({ letters, available, onJump, className }: IndexRailProps) {
  const { t } = useTranslation('library')
  const last = useRef<string | null>(null)

  const jumpAt = (event: PointerEvent<HTMLElement>) => {
    const el = document.elementFromPoint(event.clientX, event.clientY)
    const letter = el instanceof HTMLElement ? el.dataset.letter : undefined
    if (!letter || letter === last.current || !available.has(letter)) return
    last.current = letter
    onJump(letter)
    if ('vibrate' in navigator && event.pointerType === 'touch') navigator.vibrate?.(5)
  }

  return (
    <nav
      aria-label={t('artists.index')}
      className={cn('ui-chrome touch-none select-none', className)}
      onPointerDown={(event) => {
        event.currentTarget.setPointerCapture(event.pointerId)
        last.current = null
        jumpAt(event)
      }}
      onPointerMove={(event) => {
        if (event.buttons === 0 && event.pointerType === 'mouse') return
        jumpAt(event)
      }}
      onPointerUp={() => {
        last.current = null
      }}
    >
      <ul className="flex flex-col items-center">
        {letters.map((letter) => {
          const enabled = available.has(letter)
          return (
            <li key={letter}>
              <button
                type="button"
                data-letter={letter}
                disabled={!enabled}
                tabIndex={enabled ? 0 : -1}
                aria-label={t('artists.jumpTo', { letter })}
                onClick={() => enabled && onJump(letter)}
                className={cn(
                  'grid h-[15px] w-6 place-items-center text-[11px] leading-none font-semibold outline-none md:h-[18px] md:text-xs',
                  enabled ? 'text-primary hover:scale-125 focus-visible:scale-125' : 'text-muted-foreground/40',
                )}
              >
                {letter}
              </button>
            </li>
          )
        })}
      </ul>
    </nav>
  )
}
