import { MicVocal, Radio } from 'lucide-react'
import { useReducedMotion } from 'motion/react'
import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Spinner } from '@/components/spinner'
import type { LyricsLine } from '@/lib/api/types'
import { errorMessage } from '@/lib/errors'
import { groupBilingual, guessLang, type DisplayLine } from '@/lib/lyrics/bilingual'
import { cn } from '@/lib/utils'

import { activeLineIndex, useLyrics } from '../hooks/use-lyrics'
import { usePlaybackPrefs } from '../prefs'
import { usePlayback, usePlayer } from '../store'
import type { PlayableTrack } from '../types'

/** After the user scrolls the lyrics, wait this long before auto-centring again (ms). */
const USER_SCROLL_PAUSE = 3500

export type LyricsTone = 'sheet' | 'panel' | 'stage'

interface LyricsViewProps {
  track: PlayableTrack | undefined
  /** `sheet`: mobile Now Playing (white on artwork). `stage`: desktop full-screen (white, larger). `panel`: side panel (themed). */
  tone: LyricsTone
  className?: string
}

export function LyricsView({ track, tone, className }: LyricsViewProps) {
  const { t } = useTranslation('player')
  const query = useLyrics(track)
  const onArt = tone !== 'panel'

  let content: ReactNode
  if (!track) {
    content = null
  } else if (track.isRadio) {
    content = <LyricsMessage icon={Radio} title={t('lyrics.radio')} onArt={onArt} />
  } else if (query.isPending) {
    content = (
      <div className="grid h-full place-items-center">
        <Spinner className={onArt ? 'text-white/70' : undefined} />
      </div>
    )
  } else if (query.isError) {
    content = <LyricsMessage icon={MicVocal} title={t('lyrics.error')} description={errorMessage(query.error, t)} onArt={onArt} />
  } else if (query.data.lines.length === 0) {
    content = <LyricsMessage icon={MicVocal} title={t('lyrics.none')} description={t('lyrics.noneHint')} onArt={onArt} />
  } else {
    content = <Lyrics key={track.id} lines={query.data.lines} synced={query.data.synced} tone={tone} />
  }

  return <div className={cn('relative h-full min-h-0', className)}>{content}</div>
}

function LyricsMessage({
  icon: Icon,
  title,
  description,
  onArt,
}: {
  icon: typeof MicVocal
  title: string
  description?: string
  onArt: boolean
}) {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-2 px-8 text-center">
      <Icon className={cn('size-8', onArt ? 'text-white/50' : 'text-muted-foreground/60')} strokeWidth={1.5} />
      <p className={cn('text-base font-semibold', onArt ? 'text-white/85' : 'text-foreground')}>{title}</p>
      {description ? (
        <p className={cn('max-w-xs text-sm', onArt ? 'text-white/55' : 'text-muted-foreground')}>{description}</p>
      ) : null}
    </div>
  )
}

const LINE_CLASS: Record<LyricsTone, string> = {
  sheet: 'text-[26px] leading-[1.25] font-bold tracking-tight',
  stage: 'text-[34px] leading-[1.2] font-bold tracking-tight',
  panel: 'text-xl leading-snug font-semibold',
}

/** Translation lines under a synced bilingual line: about two thirds of the line size. */
const TRANSLATION_CLASS: Record<LyricsTone, string> = {
  sheet: 'mt-1 text-[0.68em] leading-snug font-semibold tracking-normal',
  stage: 'mt-1.5 text-[0.68em] leading-snug font-semibold tracking-normal',
  panel: 'mt-0.5 text-[0.75em] leading-snug font-medium',
}
/** Plain lyrics are smaller already. */
const PLAIN_TRANSLATION_CLASS = 'mt-0.5 text-[0.82em] leading-snug font-medium'

const SCROLLER_CLASS =
  'scrollbar-none relative h-full touch-pan-y overflow-y-auto overscroll-contain [mask-image:linear-gradient(to_bottom,transparent,#000_6%,#000_88%,transparent)]'

/** Groups bilingual lines (original + translation, lib/lyrics/bilingual); `TranslationToggle` hides them. */
function Lyrics({ lines, synced, tone }: { lines: LyricsLine[]; synced: boolean; tone: LyricsTone }) {
  const display = useMemo(() => groupBilingual(lines, synced), [lines, synced])
  const translate = usePlaybackPrefs((s) => s.lyricsTranslation)
  return synced ? (
    <SyncedLyrics lines={display} tone={tone} translate={translate} />
  ) : (
    <PlainLyrics lines={display} tone={tone} translate={translate} />
  )
}

/** One lyric line: the original with its translations beneath, each with a language hint. */
function LineText({ line, translationClass }: { line: DisplayLine; translationClass: string | null }) {
  return (
    <>
      <span lang={guessLang(line.text)} className="block">
        {line.text}
      </span>
      {translationClass !== null
        ? line.translations.map((text, i) => (
            <span key={i} lang={guessLang(text)} className={cn('block opacity-75', translationClass)}>
              {text}
            </span>
          ))
        : null}
    </>
  )
}

function SyncedLyrics({ lines, tone, translate }: { lines: DisplayLine[]; tone: LyricsTone; translate: boolean }) {
  const { t } = useTranslation('player')
  const reduceMotion = useReducedMotion()
  const scrollerRef = useRef<HTMLDivElement>(null)
  const lineRefs = useRef<(HTMLButtonElement | null)[]>([])
  const pausedUntil = useRef(0)
  const resumeTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const [active, setActive] = useState(() => activeLineIndex(lines, usePlayback.getState().currentTime * 1000))
  const onArt = tone !== 'panel'

  // Follow playback: only re-render when the active line changes.
  useEffect(() => {
    let current = -2
    return usePlayback.subscribe((s) => {
      const next = activeLineIndex(lines, s.currentTime * 1000)
      if (next !== current) {
        current = next
        setActive(next)
      }
    })
  }, [lines])

  const centre = (index: number, smooth: boolean) => {
    const scroller = scrollerRef.current
    const line = lineRefs.current[Math.max(0, index)]
    if (!scroller || !line) return
    const top = line.offsetTop - scroller.clientHeight * 0.36 + line.offsetHeight / 2
    scroller.scrollTo({ top: Math.max(0, top), behavior: smooth && !reduceMotion ? 'smooth' : 'auto' })
  }

  useEffect(() => {
    if (Date.now() < pausedUntil.current) return
    centre(active, true)
    // `centre` only reads refs; re-run on line changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [active])

  // First paint: jump straight to the current line.
  useEffect(() => {
    centre(active, false)
    return () => clearTimeout(resumeTimer.current)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const onUserScroll = () => {
    pausedUntil.current = Date.now() + USER_SCROLL_PAUSE
    clearTimeout(resumeTimer.current)
    resumeTimer.current = setTimeout(() => {
      pausedUntil.current = 0
      const s = usePlayback.getState()
      centre(activeLineIndex(lines, s.currentTime * 1000), true)
    }, USER_SCROLL_PAUSE)
  }

  // Showing or hiding translations moves every line: re-centre without animation.
  useEffect(() => {
    if (Date.now() >= pausedUntil.current) centre(activeLineIndex(lines, usePlayback.getState().currentTime * 1000), false)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [translate])

  const seekTo = (line: DisplayLine) => {
    pausedUntil.current = 0
    usePlayback.getState().seek(line.start / 1000)
    usePlayer.getState().play()
  }

  return (
    <div
      ref={scrollerRef}
      className={SCROLLER_CLASS}
      onWheel={onUserScroll}
      onTouchMove={onUserScroll}
      role="list"
      aria-label={t('lyrics.title')}
    >
      <div className={cn('pt-[18vh] pb-[45vh]', tone === 'panel' ? 'px-5' : tone === 'stage' ? 'px-2' : 'px-7')}>
        {lines.map((line, i) => {
          const isActive = i === active
          return (
            <div role="listitem" key={i}>
              <button
                ref={(el) => {
                  lineRefs.current[i] = el
                }}
                type="button"
                onClick={() => seekTo(line)}
                aria-current={isActive ? 'true' : undefined}
                className={cn(
                  'block w-full origin-left rounded-lg py-2.5 text-left transition-[opacity,scale] duration-500 ease-out outline-none select-text',
                  LINE_CLASS[tone],
                  onArt ? 'text-white focus-visible:bg-white/10' : 'text-foreground focus-visible:bg-accent',
                  isActive ? 'scale-100 opacity-100' : cn('scale-[0.96]', onArt ? 'opacity-35 hover:opacity-60' : 'opacity-30 hover:opacity-55'),
                )}
              >
                {line.text ? <LineText line={line} translationClass={translate ? TRANSLATION_CLASS[tone] : null} /> : <span aria-label={t('lyrics.instrumental')}>♪</span>}
              </button>
            </div>
          )
        })}
      </div>
    </div>
  )
}

function PlainLyrics({ lines, tone, translate }: { lines: DisplayLine[]; tone: LyricsTone; translate: boolean }) {
  const onArt = tone !== 'panel'
  return (
    <div className={SCROLLER_CLASS}>
      <div className={cn('py-[6vh]', tone === 'panel' ? 'px-5' : tone === 'stage' ? 'px-2' : 'px-7')}>
        {lines.map((line, i) => (
          <p
            key={i}
            className={cn(
              'min-h-[1em] py-1 select-text',
              tone === 'panel' ? 'text-base leading-relaxed' : 'text-xl leading-snug font-semibold',
              onArt ? 'text-white/85' : 'text-foreground/85',
            )}
          >
            <LineText line={line} translationClass={translate ? PLAIN_TRANSLATION_CLASS : null} />
          </p>
        ))}
      </div>
    </div>
  )
}
