import { X } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { MascotArt, type MascotPose } from '@/components/mascot'
import { cn } from '@/lib/utils'
import { useUI } from '@/stores/ui'

import { useCurrentTrack, usePlayer } from '../store'

/** Paused this long, she dozes off. */
const SLEEP_AFTER_MS = 2 * 60_000
const BUBBLE_MS = 4_000
const POSES: MascotPose[] = ['idle', 'listening', 'sleeping']

/**
 * The mascot in the bottom-right corner (tablet and desktop; docs/development/design.md#mascot).
 * She listens while music plays, waves when paused and dozes off after a while. Clicking her
 * shows a short line; the × hides her (Settings › Mascot brings her back).
 *
 * Sits on top of the player chrome: `--player-clearance` above the bottom bar or the mini bar,
 * and beside the floating window (`--mascot-right`, index.css).
 */
export function MascotCompanion() {
  const { t } = useTranslation()
  const setCompanion = useUI((s) => s.setMascotCompanion)
  const isPlaying = usePlayer((s) => s.isPlaying)
  const track = useCurrentTrack()
  const [sleepy, setSleepy] = useState(false)
  const [line, setLine] = useState<string | null>(null)
  const bubbleTimer = useRef<number | undefined>(undefined)

  useEffect(() => {
    if (isPlaying) return
    const id = window.setTimeout(() => setSleepy(true), SLEEP_AFTER_MS)
    return () => {
      window.clearTimeout(id)
      setSleepy(false)
    }
  }, [isPlaying])

  useEffect(() => () => window.clearTimeout(bubbleTimer.current), [])

  const pose: MascotPose = isPlaying ? 'listening' : sleepy ? 'sleeping' : 'idle'

  const speak = () => {
    let next: string
    if (isPlaying && track) next = t('mascot.lines.playing', { title: track.title })
    else if (sleepy) next = t('mascot.lines.sleeping')
    else {
      const idle = t('mascot.lines.idle', { returnObjects: true }) as string[]
      next = idle[Math.floor(Math.random() * idle.length)]
    }
    setLine(next)
    window.clearTimeout(bubbleTimer.current)
    bubbleTimer.current = window.setTimeout(() => setLine(null), BUBBLE_MS)
  }

  const hide = () => {
    setCompanion(false)
    toast(t('mascot.hidden'))
  }

  return (
    <div
      data-slot="mascot-companion"
      className="group pointer-events-none fixed right-(--mascot-right) bottom-(--player-clearance) z-30 transition-[bottom,right] duration-300 ease-out xl:right-[calc(var(--mascot-right)+var(--player-panel-w,0px))]"
    >
      {line ? (
        <p
          role="status"
          className="absolute right-3 bottom-full mb-1 w-max max-w-56 rounded-2xl rounded-br-sm border bg-popover px-3 py-2 text-xs text-popover-foreground shadow-lg animate-in fade-in slide-in-from-bottom-1"
        >
          {line}
        </p>
      ) : null}
      <button
        type="button"
        onClick={speak}
        aria-label={t('mascot.companionLabel')}
        className="pointer-events-auto relative block h-28 w-[7.25rem] rounded-xl outline-none focus-visible:ring-2 focus-visible:ring-ring lg:h-32 lg:w-[8.25rem]"
      >
        {POSES.map((p) => (
          <MascotArt
            key={p}
            pose={p}
            className={cn(
              'absolute right-0 bottom-0 h-full origin-bottom transition-opacity duration-500',
              p === pose ? 'opacity-100' : 'opacity-0',
              p === 'listening' && 'motion-safe:animate-mascot-sway',
            )}
          />
        ))}
      </button>
      <button
        type="button"
        onClick={hide}
        aria-label={t('mascot.hide')}
        title={t('mascot.hide')}
        className="pointer-events-auto absolute top-0 left-0 grid size-7 place-items-center rounded-full border bg-background/90 text-muted-foreground opacity-0 shadow-sm backdrop-blur transition-opacity group-hover:opacity-100 hover:text-foreground focus-visible:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <X className="size-3.5" aria-hidden />
      </button>
    </div>
  )
}
