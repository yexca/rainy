import { Loader2, Play, Shuffle } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

export interface PlayShuffleButtonsProps {
  onPlay: () => void | Promise<unknown>
  onShuffle: () => void | Promise<unknown>
  disabled?: boolean
  /** `stretch`: two equal-width iOS buttons (phones); `inline`: compact pills (default). */
  layout?: 'inline' | 'stretch'
  className?: string
}

/** Play + Shuffle pair. Async handlers show a spinner on the pressed button. */
export function PlayShuffleButtons({ onPlay, onShuffle, disabled, layout = 'inline', className }: PlayShuffleButtonsProps) {
  const { t } = useTranslation()
  const [busy, setBusy] = useState<'play' | 'shuffle' | null>(null)

  const run = (which: 'play' | 'shuffle') => {
    const result = which === 'play' ? onPlay() : onShuffle()
    if (result instanceof Promise) {
      setBusy(which)
      void result.finally(() => setBusy(null))
    }
  }

  const icon = (which: 'play' | 'shuffle') =>
    busy === which ? (
      <Loader2 className="size-[18px] animate-spin" aria-hidden />
    ) : which === 'play' ? (
      <Play className="size-[18px]" fill="currentColor" aria-hidden />
    ) : (
      <Shuffle className="size-[18px]" strokeWidth={2} aria-hidden />
    )

  if (layout === 'stretch') {
    const cls =
      'h-12 flex-1 gap-2 rounded-xl bg-secondary text-[17px] font-semibold text-primary shadow-none transition-transform hover:bg-secondary/80 active:scale-[0.97] dark:bg-secondary/80'
    return (
      <div className={cn('flex w-full gap-3', className)}>
        <Button className={cls} disabled={disabled || busy !== null} onClick={() => run('play')}>
          {icon('play')}
          {t('common:actions.play')}
        </Button>
        <Button className={cls} disabled={disabled || busy !== null} onClick={() => run('shuffle')}>
          {icon('shuffle')}
          {t('common:actions.shuffle')}
        </Button>
      </div>
    )
  }

  return (
    <div className={cn('flex items-center gap-2', className)}>
      <Button
        className="h-9 min-w-24 gap-2 rounded-full px-5 font-semibold shadow-sm active:scale-[0.97]"
        disabled={disabled || busy !== null}
        onClick={() => run('play')}
      >
        {icon('play')}
        {t('common:actions.play')}
      </Button>
      <Button
        variant="secondary"
        className="h-9 min-w-24 gap-2 rounded-full px-5 font-semibold text-primary active:scale-[0.97]"
        disabled={disabled || busy !== null}
        onClick={() => run('shuffle')}
      >
        {icon('shuffle')}
        {t('common:actions.shuffle')}
      </Button>
    </div>
  )
}
