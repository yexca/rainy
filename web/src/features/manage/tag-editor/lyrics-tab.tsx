import { Clock, Eraser, Pause, Play } from 'lucide-react'
import { useDeferredValue, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { usePlayback, usePlayer } from '@/features/player/store'
import type { TrackTags } from '@/lib/api/types'
import { analyzeBilingual, toPairedLrc } from '@/lib/lyrics/bilingual'
import { cn } from '@/lib/utils'

import { formatLrcTimestamp, highlightLine, isSyncedLrc, stampLine } from '../lib/lrc'
import { BilingualBanner } from './bilingual-banner'
import type { LyricsDraft, LyricsTarget } from './types'

export interface LyricsTabProps {
  item: TrackTags
  draft: LyricsDraft
  onDraftChange: (draft: LyricsDraft) => void
  disabled?: boolean
}

/** Shared box metrics of the textarea and its highlight layer (must match exactly). */
const BOX = 'px-3 py-2.5 font-mono text-[13px] leading-6 whitespace-pre-wrap break-words [overflow-wrap:anywhere] [tab-size:4]'

export function LyricsTab({ item, draft, onDraftChange, disabled }: LyricsTabProps) {
  const { t } = useTranslation('manage')
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const layerRef = useRef<HTMLDivElement>(null)
  const synced = isSyncedLrc(draft.text)
  // Analyse the text as the user types or pastes it; suggest a split, never apply one silently.
  const deferredText = useDeferredValue(draft.text)
  const bilingual = useMemo(() => analyzeBilingual(deferredText), [deferredText])
  const [bilingualDismissed, setBilingualDismissed] = useState(false)

  const splitBilingual = () => {
    const analysis = analyzeBilingual(draft.text)
    if (analysis.timed === 0) return
    onDraftChange({ ...draft, text: toPairedLrc(draft.text, analysis) })
    toast.success(t('lyrics.bilingualDone', { count: analysis.timed }))
  }

  const stamp = (seconds: number) => {
    const el = textareaRef.current
    const caret = el ? el.selectionStart : draft.text.length
    const result = stampLine(draft.text, caret, seconds)
    onDraftChange({ ...draft, text: result.text })
    requestAnimationFrame(() => {
      if (!el) return
      el.focus()
      el.setSelectionRange(result.caret, result.caret)
      // keep the caret line visible
      const line = result.text.slice(0, result.caret).split('\n').length
      const lineHeight = 24
      if (line * lineHeight > el.scrollTop + el.clientHeight - lineHeight) el.scrollTop = line * lineHeight - el.clientHeight / 2
    })
  }

  return (
    <div className="grid gap-4 pb-2">
      <div className="flex flex-wrap items-center gap-3">
        <ToggleGroup
          type="single"
          variant="outline"
          size="sm"
          value={draft.target}
          onValueChange={(v) => v && onDraftChange({ ...draft, target: v as LyricsTarget })}
          disabled={disabled}
          aria-label={t('lyrics.target')}
        >
          <ToggleGroupItem value="embedded">{t('lyrics.embedded')}</ToggleGroupItem>
          <ToggleGroupItem value="lrc">{t('lyrics.lrcFile')}</ToggleGroupItem>
        </ToggleGroup>
        {draft.text.trim() ? (
          <Badge variant={synced ? 'default' : 'secondary'}>{synced ? t('lyrics.synced') : t('lyrics.plain')}</Badge>
        ) : null}
        {bilingual.paired > 0 ? (
          <Badge variant="secondary" title={t('lyrics.bilingualPairedHint')}>
            {t('lyrics.bilingual')}
          </Badge>
        ) : null}
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="ml-auto text-muted-foreground"
          disabled={disabled || !draft.text}
          onClick={() => onDraftChange({ ...draft, text: '' })}
        >
          <Eraser />
          {t('lyrics.clear')}
        </Button>
      </div>
      <p className="-mt-2 text-xs text-muted-foreground">
        {draft.target === 'lrc' ? t('lyrics.lrcHint') : t('lyrics.embeddedHint')}
      </p>

      {bilingual.detected && !bilingualDismissed ? (
        <BilingualBanner
          analysis={bilingual}
          onSplit={splitBilingual}
          onDismiss={() => setBilingualDismissed(true)}
          disabled={disabled}
        />
      ) : null}

      <PlaybackStrip item={item} onStamp={stamp} disabled={disabled} />

      <div className="relative overflow-hidden rounded-md border border-input shadow-xs focus-within:border-ring focus-within:ring-[3px] focus-within:ring-ring/50 dark:bg-input/30">
        <div ref={layerRef} aria-hidden className={cn(BOX, 'pointer-events-none absolute inset-0 overflow-hidden text-foreground')}>
          {draft.text.split('\n').map((line, i) => (
            <div key={i} className="min-h-6">
              {highlightLine(line).map((seg, j) => (
                <span
                  key={j}
                  className={cn(
                    seg.kind === 'time' && 'font-medium text-primary',
                    seg.kind === 'meta' && 'text-muted-foreground italic',
                  )}
                >
                  {seg.text}
                </span>
              ))}
            </div>
          ))}
        </div>
        <textarea
          ref={textareaRef}
          value={draft.text}
          disabled={disabled}
          spellCheck={false}
          placeholder={t('lyrics.placeholder')}
          aria-label={t('lyrics.label')}
          onChange={(e) => onDraftChange({ ...draft, text: e.target.value })}
          onScroll={(e) => {
            if (layerRef.current) layerRef.current.scrollTop = e.currentTarget.scrollTop
          }}
          className={cn(
            BOX,
            'scrollbar-none relative block h-[min(50dvh,26rem)] w-full resize-none bg-transparent text-transparent caret-foreground outline-none selection:bg-primary/25 selection:text-transparent placeholder:text-muted-foreground',
          )}
        />
      </div>
    </div>
  )
}

interface PlaybackStripProps {
  item: TrackTags
  onStamp: (seconds: number) => void
  disabled?: boolean
}

/** Mini transport for syncing: play this track, see the position, stamp the current line. */
function PlaybackStrip({ item, onStamp, disabled }: PlaybackStripProps) {
  const { t } = useTranslation('manage')
  const isCurrent = usePlayer((s) => s.index >= 0 && s.queue[s.index]?.id === item.track.id)
  const isPlaying = usePlayer((s) => s.isPlaying)
  const togglePlay = usePlayer((s) => s.togglePlay)
  const playTracks = usePlayer((s) => s.playTracks)
  const currentTime = usePlayback((s) => (isCurrent ? s.currentTime : 0))

  return (
    <div className="flex items-center gap-2 rounded-lg bg-muted/60 p-1.5 pl-2">
      <Button
        type="button"
        variant="ghost"
        size="icon-sm"
        aria-label={isCurrent && isPlaying ? t('common:actions.pause') : t('common:actions.play')}
        onClick={() => (isCurrent ? togglePlay() : playTracks([item.track], 0))}
      >
        {isCurrent && isPlaying ? <Pause fill="currentColor" /> : <Play fill="currentColor" />}
      </Button>
      <span className="tnum min-w-12 text-sm text-muted-foreground">
        {isCurrent ? formatLrcTimestamp(currentTime).slice(1, -1) : t('lyrics.notPlaying')}
      </span>
      <Button
        type="button"
        size="sm"
        variant="secondary"
        className="ml-auto"
        disabled={disabled || !isCurrent}
        title={isCurrent ? t('lyrics.stampHint') : t('lyrics.playToStamp')}
        onMouseDown={(e) => e.preventDefault() /* keep the textarea caret */}
        onClick={() => onStamp(usePlayback.getState().currentTime)}
      >
        <Clock />
        {t('lyrics.stamp')}
      </Button>
    </div>
  )
}
