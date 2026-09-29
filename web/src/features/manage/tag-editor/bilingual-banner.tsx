import { Languages } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { guessLang, type BilingualAnalysis } from '@/lib/lyrics/bilingual'

/** Preview rows shown before "…and N more". */
const PREVIEW_LIMIT = 60

interface BilingualBannerProps {
  analysis: BilingualAnalysis
  onSplit: () => void
  onDismiss: () => void
  disabled?: boolean
}

/**
 * Shown when the lyrics text looks like space-separated bilingual lyrics ("original 中文"): a
 * preview of the detected split and, for timed lines, a button that rewrites them as paired LRC
 * lines. Nothing changes until the user asks for it.
 */
export function BilingualBanner({ analysis, onSplit, onDismiss, disabled }: BilingualBannerProps) {
  const { t } = useTranslation('manage')
  const [preview, setPreview] = useState(false)
  const count = analysis.lines.length
  const canSplit = analysis.timed > 0
  const shown = analysis.lines.slice(0, PREVIEW_LIMIT)

  return (
    <div role="status" className="rounded-lg border border-primary/25 bg-primary/5 p-3 text-sm">
      <div className="flex gap-2.5">
        <Languages className="mt-0.5 size-4 shrink-0 text-primary" strokeWidth={1.75} aria-hidden />
        <div className="min-w-0 flex-1 space-y-1">
          <p className="font-medium">{t('lyrics.bilingualDetected', { count })}</p>
          <p className="text-xs text-muted-foreground">
            {canSplit ? t('lyrics.bilingualTimedHint') : t('lyrics.bilingualPlainHint')}
          </p>
        </div>
      </div>

      {preview ? (
        <ol className="scrollbar-thin mt-3 max-h-60 divide-y divide-border/60 overflow-y-auto rounded-md border bg-background/80">
          {shown.map((line) => (
            <li key={line.line} className="grid gap-0.5 px-3 py-2">
              <span className="flex items-baseline gap-2">
                <span lang={guessLang(line.original)} className="min-w-0 flex-1 break-words">
                  {line.original}
                </span>
                {line.timed ? null : (
                  <Badge variant="outline" className="shrink-0 text-[10px] font-normal text-muted-foreground">
                    {t('lyrics.bilingualUntimed')}
                  </Badge>
                )}
              </span>
              <span lang="zh" className="break-words text-xs text-muted-foreground">
                {line.translation}
              </span>
            </li>
          ))}
          {count > shown.length ? (
            <li className="px-3 py-2 text-xs text-muted-foreground">
              {t('lyrics.bilingualMore', { count: count - shown.length })}
            </li>
          ) : null}
        </ol>
      ) : null}

      <div className="mt-3 flex flex-wrap justify-end gap-2">
        <Button type="button" variant="ghost" size="sm" className="text-muted-foreground" onClick={onDismiss}>
          {t('lyrics.bilingualDismiss')}
        </Button>
        <Button type="button" variant="outline" size="sm" aria-expanded={preview} onClick={() => setPreview(!preview)}>
          {preview ? t('lyrics.bilingualHidePreview') : t('lyrics.bilingualPreview')}
        </Button>
        {canSplit ? (
          <Button type="button" size="sm" disabled={disabled} onClick={onSplit}>
            {t('lyrics.bilingualSplit')}
          </Button>
        ) : null}
      </div>
    </div>
  )
}
