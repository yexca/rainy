import { AnimatePresence, motion } from 'motion/react'
import { useEffect, useMemo, useState } from 'react'
import { createPortal } from 'react-dom'
import { useTranslation } from 'react-i18next'

import { CoverArt } from '@/components/cover-art'
import { groupBilingual, guessLang, type DisplayLine } from '@/lib/lyrics/bilingual'

import { NowPlayingBackground } from '../components/now-playing-background'
import { NextButton, PlayPauseButton, PrevButton } from '../components/transport'
import { activeLineIndex, useLyrics } from '../hooks/use-lyrics'
import { pipLyricWindow } from '../lib/pip-lyrics'
import { usePlaybackPrefs } from '../prefs'
import { useCurrentTrack, usePlayback, usePlayer } from '../store'
import { usePipLyrics } from './pip-lyrics'

/** Renders the lyrics into the Document Picture-in-Picture window while one is open (mounted by `<AudioEngine/>`). */
export function PipLyricsHost() {
  const win = usePipLyrics((s) => s.window)
  return win ? createPortal(<PipLyricsWindow />, win.document.body) : null
}

/**
 * The picture-in-picture lyrics window: the track with prev / play / next on top, the line
 * being sung (with its translation when shown) large in the middle, the next line dimmed below.
 */
function PipLyricsWindow() {
  const { t } = useTranslation('player')
  const track = useCurrentTrack()
  const isPlaying = usePlayer((s) => s.isPlaying)
  const query = useLyrics(track)

  let body: React.ReactNode
  if (!track) body = <Message text={t('idle.title')} />
  else if (track.isRadio) body = <Message text={t('lyrics.radio')} />
  else if (query.isPending) body = <Message text="…" />
  else if (query.isError) body = <Message text={t('lyrics.error')} />
  else if (query.data.lines.length === 0) body = <Message text={t('lyrics.none')} />
  else if (!query.data.synced) body = <Message text={t('pip.notSynced')} />
  else body = <SyncedLines key={track.id} lines={query.data.lines} />

  return (
    <div className="relative flex h-dvh flex-col overflow-hidden text-white select-none">
      <NowPlayingBackground coverArt={track?.coverArt} playing={isPlaying} />
      <header className="relative flex shrink-0 items-center gap-3 px-4 pt-3">
        {track ? <CoverArt coverArt={track.coverArt} size={40} flat className="size-10 shrink-0" alt="" /> : null}
        <div className="min-w-0 flex-1 leading-tight">
          <p className="truncate text-sm font-semibold">{track?.title ?? 'Rainy'}</p>
          <p className="truncate text-xs text-white/60">{track?.artist}</p>
        </div>
        <div className="flex shrink-0 items-center">
          <PrevButton tone="sheet" size="sm" className="size-9" />
          <PlayPauseButton tone="sheet" size="sm" className="size-10" />
          <NextButton tone="sheet" size="sm" className="size-9" />
        </div>
      </header>
      <div className="relative flex min-h-0 flex-1 flex-col justify-center px-5 pb-3 text-center">{body}</div>
    </div>
  )
}

function Message({ text }: { text: string }) {
  return <p className="text-base font-semibold text-white/75">{text}</p>
}

function SyncedLines({ lines }: { lines: { start: number; text: string }[] }) {
  const { t } = useTranslation('player')
  const display = useMemo(() => groupBilingual(lines, true), [lines])
  const translate = usePlaybackPrefs((s) => s.lyricsTranslation)
  const [active, setActive] = useState(() => activeLineIndex(display, usePlayback.getState().currentTime * 1000))

  // Only re-render when the active line changes.
  useEffect(() => {
    let current = -2
    return usePlayback.subscribe((s) => {
      const next = activeLineIndex(display, s.currentTime * 1000)
      if (next !== current) {
        current = next
        setActive(next)
      }
    })
  }, [display])

  const { current, next, index } = pipLyricWindow(display, active)
  return (
    <>
      <AnimatePresence mode="popLayout" initial={false}>
        <motion.div
          key={index}
          initial={{ opacity: 0, y: 14 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: -14 }}
          transition={{ type: 'spring', stiffness: 400, damping: 36 }}
          aria-live="polite"
        >
          <LineText line={current} translate={translate} instrumental={t('lyrics.instrumental')} />
        </motion.div>
      </AnimatePresence>
      {next ? (
        <p lang={guessLang(next.text)} className="mt-2 line-clamp-1 text-[clamp(13px,4vw,20px)] font-semibold text-white/40">
          {next.text}
        </p>
      ) : null}
    </>
  )
}

function LineText({ line, translate, instrumental }: { line: DisplayLine | null; translate: boolean; instrumental: string }) {
  if (!line || line.text === '') {
    return (
      <p className="text-[clamp(20px,8vw,40px)] font-bold" aria-label={instrumental}>
        ♪
      </p>
    )
  }
  return (
    <>
      <p lang={guessLang(line.text)} className="line-clamp-2 text-[clamp(18px,6.5vw,36px)] leading-tight font-bold tracking-tight text-balance">
        {line.text}
      </p>
      {translate && line.translations[0] ? (
        <p lang={guessLang(line.translations[0])} className="mt-1 line-clamp-1 text-[clamp(13px,4.2vw,22px)] font-semibold text-white/75">
          {line.translations[0]}
        </p>
      ) : null}
    </>
  )
}
