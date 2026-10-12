import { Timer } from 'lucide-react'
import { useEffect, useRef, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { formatDuration } from '@/lib/format'
import { cn } from '@/lib/utils'

import { parseMinutes, remainingSeconds, STOP_TOAST_ID, TIMER_MINUTES, TIMER_MODES, type TimerMode } from '../lib/stop-timer'
import { useStopTimer } from '../stop-timer'
import { ModeToggle } from './transport'

/** After the panel closes, ignore the tooltip opening from the focus Radix returns (ms). */
const TOOLTIP_QUIET = 400

/**
 * Auto-stop timer (§9.4): one button, tinted while a timer is on, that opens a panel with a tab
 * per mode (sleep / inactivity) and the times below, presets plus a field for any number of
 * minutes; picking or entering a time starts that mode. `bar`:
 * bottom bar, `sheet`: phone Now Playing and floating window action row, `stage`: desktop
 * full-screen header.
 */
export function StopTimerButton({
  tone,
  className,
  onOpenChange,
}: {
  tone: 'bar' | 'sheet' | 'stage'
  className?: string
  onOpenChange?: (open: boolean) => void
}) {
  const { t } = useTranslation('player')
  const active = useStopTimer((s) => s.mode !== null)
  const label = t('timer.title')
  const [tooltip, setTooltip] = useState(false)
  const quietUntil = useRef(0)

  const onPanelOpenChange = (open: boolean) => {
    setTooltip(false)
    // The "stopped playback" message sits where the bar's panel opens, and the panel says more.
    if (open) toast.dismiss(STOP_TOAST_ID)
    else quietUntil.current = Date.now() + TOOLTIP_QUIET
    onOpenChange?.(open)
  }

  const trigger = (
    <PopoverTrigger asChild>
      <ModeToggle
        tone={tone === 'bar' ? 'bar' : 'sheet'}
        active={active}
        aria-label={label}
        title={tone === 'bar' ? undefined : label}
        className={cn(
          tone === 'bar' ? 'size-8 rounded-md' : tone === 'stage' ? 'size-10 rounded-full hover:bg-white/10' : 'size-11',
          className,
        )}
      >
        <Timer className={tone === 'sheet' ? 'size-[22px]' : 'size-[18px]'} strokeWidth={1.9} />
      </ModeToggle>
    </PopoverTrigger>
  )

  return (
    <Popover onOpenChange={onPanelOpenChange}>
      {tone === 'bar' ? (
        <Tooltip open={tooltip} onOpenChange={(open) => setTooltip(open && Date.now() >= quietUntil.current)}>
          <TooltipTrigger asChild>{trigger}</TooltipTrigger>
          <TooltipContent side="top">{label}</TooltipContent>
        </Tooltip>
      ) : (
        trigger
      )}
      <PopoverContent
        aria-label={label}
        side={tone === 'stage' ? 'bottom' : 'top'}
        align="end"
        sideOffset={8}
        className="w-72 rounded-xl p-3"
      >
        <TimerPanel />
      </PopoverContent>
    </Popover>
  )
}

function isPreset(minutes: number): boolean {
  return (TIMER_MINUTES as readonly number[]).includes(minutes)
}

function TimerPanel() {
  const { t } = useTranslation('player')
  const mode = useStopTimer((s) => s.mode)
  const minutes = useStopTimer((s) => s.minutes)
  const endsAt = useStopTimer((s) => s.endsAt)
  const start = useStopTimer((s) => s.start)
  const cancel = useStopTimer((s) => s.cancel)
  const [tab, setTab] = useState<TimerMode>(mode ?? 'sleep')
  // A running custom time shows in the field when the panel opens.
  const [custom, setCustom] = useState(() => (mode && !isPreset(minutes) ? String(minutes) : ''))
  const customMinutes = parseMinutes(custom)
  const time = (m: number) => t('common:duration.minutes', { minutes: m })

  const startCustom = (event: FormEvent) => {
    event.preventDefault()
    if (customMinutes !== null) start(tab, customMinutes)
  }

  return (
    <div className="grid gap-3">
      <div role="tablist" aria-label={t('timer.title')} className="flex rounded-lg bg-muted p-0.5">
        {TIMER_MODES.map((m) => (
          <button
            key={m}
            type="button"
            role="tab"
            aria-selected={tab === m}
            onClick={() => setTab(m)}
            className={cn(
              'h-11 flex-1 rounded-md text-sm font-medium transition-[background-color,color,box-shadow] outline-none focus-visible:ring-2 focus-visible:ring-ring/50 md:h-8',
              tab === m ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground',
            )}
          >
            {t(`timer.${m}`)}
          </button>
        ))}
      </div>
      <p className="text-xs text-muted-foreground">{t(`timer.${tab}Hint`)}</p>
      <div className="grid grid-cols-3 gap-1.5">
        {TIMER_MINUTES.map((m) => {
          const selected = mode === tab && minutes === m
          return (
            <button
              key={m}
              type="button"
              aria-pressed={selected}
              onClick={() => start(tab, m)}
              className={cn(
                'tnum h-11 rounded-md border text-sm font-medium transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring/50 md:h-9',
                selected ? 'border-primary bg-primary text-primary-foreground' : 'border-border/70 hover:bg-accent',
              )}
            >
              {time(m)}
            </button>
          )
        })}
      </div>
      <form onSubmit={startCustom} className="flex gap-1.5">
        <Input
          value={custom}
          onChange={(event) => setCustom(event.target.value.replace(/\D/g, '').slice(0, 4))}
          inputMode="numeric"
          autoComplete="off"
          enterKeyHint="go"
          aria-label={t('timer.custom')}
          placeholder={t('timer.custom')}
          className={cn('tnum h-11 flex-1 md:h-9', mode === tab && minutes === customMinutes && !isPreset(minutes) && 'border-primary')}
        />
        <Button type="submit" variant="outline" disabled={customMinutes === null} className="h-11 shrink-0 md:h-9">
          {t('timer.start')}
        </Button>
      </form>
      {mode ? (
        <div className="flex items-center justify-between gap-2 border-t pt-3">
          <p className="tnum min-w-0 text-sm">
            {mode === 'sleep' && endsAt !== null ? (
              <Countdown key={endsAt} endsAt={endsAt} />
            ) : (
              t('timer.idleStatus', { time: time(minutes) })
            )}
          </p>
          <Button variant="ghost" size="sm" className="h-11 shrink-0 md:h-8" onClick={cancel}>
            {t('timer.off')}
          </Button>
        </div>
      ) : null}
    </div>
  )
}

/** Time left on the sleep timer; ticks while the panel is open. */
function Countdown({ endsAt }: { endsAt: number }) {
  const { t } = useTranslation('player')
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(id)
  }, [])
  return t('timer.sleepStatus', { time: formatDuration(remainingSeconds(endsAt, now)) })
}
