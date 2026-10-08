import { Check, Circle, Feather, Layers3, Monitor, Moon, Shapes, Sun, type LucideIcon } from 'lucide-react'
import { useRef, type KeyboardEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Switch } from '@/components/ui/switch'
import { useTheme } from '@/hooks/use-theme'
import { LANGUAGES, currentLanguage, setLanguage, type Language } from '@/lib/i18n'
import { ACCENTS, STYLES, STYLE_PREVIEWS, accentSwatch, type Accent, type Style, type Theme } from '@/lib/theme'
import { cn } from '@/lib/utils'

import { Segmented, SettingsRow, SettingsSection } from './settings-ui'

export function AppearanceSection() {
  const { t, i18n } = useTranslation('settings')
  const { theme, setTheme, style, setStyle, accent, setAccent, albumColors, setAlbumColors, immersive, setImmersive } = useTheme()
  // Re-render on language change (i18n.language is read for the current value).
  const language: Language = i18n.language ? currentLanguage() : 'en'

  const themes: { value: Theme; label: string; icon: ReactNode }[] = [
    { value: 'light', label: t('common:theme.light'), icon: <Sun className="size-3.5" aria-hidden /> },
    { value: 'dark', label: t('common:theme.dark'), icon: <Moon className="size-3.5" aria-hidden /> },
    { value: 'system', label: t('common:theme.system'), icon: <Monitor className="size-3.5" aria-hidden /> },
  ]

  return (
    <SettingsSection id="appearance" title={t('appearance.title')}>
      <SettingsRow label={t('appearance.theme')} stacked>
        <Segmented label={t('appearance.theme')} value={theme} onChange={setTheme} options={themes} />
      </SettingsRow>
      <SettingsRow label={t('common:style.label')} description={t('appearance.styleHint')} stacked>
        <StylePicker value={style} accent={accent} onChange={setStyle} className="w-full sm:w-80" />
      </SettingsRow>
      <SettingsRow label={t('common:accent.label')} description={t('appearance.accentHint')} stacked>
        <AccentPicker value={accent} style={style} onChange={setAccent} />
      </SettingsRow>
      <SettingsRow label={t('appearance.albumColors')} description={t('appearance.albumColorsHint')} htmlFor="settings-album-colors">
        <Switch id="settings-album-colors" checked={albumColors} onCheckedChange={setAlbumColors} />
      </SettingsRow>
      <SettingsRow
        label={t('appearance.immersive')}
        description={albumColors ? t('appearance.immersiveHint') : t('appearance.immersiveNeedsAlbumColors')}
        htmlFor="settings-immersive"
      >
        <Switch id="settings-immersive" checked={albumColors && immersive} onCheckedChange={setImmersive} disabled={!albumColors} />
      </SettingsRow>
      <SettingsRow label={t('common:language.label')} stacked>
        <Segmented
          label={t('common:language.label')}
          value={language}
          onChange={(lng) => void setLanguage(lng)}
          options={LANGUAGES.map((l) => ({ value: l.code, label: <span lang={l.htmlLang}>{l.label}</span> }))}
        />
      </SettingsRow>
    </SettingsSection>
  )
}

const STYLE_ICONS: Record<Style, LucideIcon> = {
  anthropic: Feather,
  openai: Circle,
  apple: Layers3,
  'google-md': Shapes,
}

/** Arrow keys move the selection of a radio group of buttons (roving tabindex). */
function useRadioKeys<T>(values: readonly T[], onChange: (value: T) => void) {
  const refs = useRef<(HTMLButtonElement | null)[]>([])
  const onKeyDown = (event: KeyboardEvent<HTMLButtonElement>, index: number) => {
    const dir = event.key === 'ArrowRight' || event.key === 'ArrowDown' ? 1 : event.key === 'ArrowLeft' || event.key === 'ArrowUp' ? -1 : 0
    if (!dir) return
    event.preventDefault()
    const next = (index + dir + values.length) % values.length
    onChange(values[next])
    refs.current[next]?.focus()
  }
  return { refs, onKeyDown }
}

/** Style presets as cards: icon, name, and the accent / light / dark colours it gives. */
export function StylePicker({
  value,
  accent,
  onChange,
  className,
}: {
  value: Style
  accent: Accent
  onChange: (style: Style) => void
  className?: string
}) {
  const { t } = useTranslation()
  const { refs, onKeyDown } = useRadioKeys(STYLES, onChange)

  return (
    <div role="radiogroup" aria-label={t('style.label')} className={cn('grid grid-cols-2 gap-2', className)}>
      {STYLES.map((style, index) => {
        const Icon = STYLE_ICONS[style]
        const selected = style === value
        const { background } = STYLE_PREVIEWS[style]
        return (
          <button
            key={style}
            ref={(el) => {
              refs.current[index] = el
            }}
            type="button"
            role="radio"
            aria-checked={selected}
            tabIndex={selected ? 0 : -1}
            onClick={() => onChange(style)}
            onKeyDown={(event) => onKeyDown(event, index)}
            className={cn(
              'flex min-h-14 min-w-0 items-center gap-2.5 rounded-lg border px-3 py-2 text-left outline-none transition-[color,background-color,border-color,scale] duration-150 active:scale-[0.97]',
              'focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background',
              selected
                ? 'border-primary bg-primary/10 text-primary'
                : 'bg-background text-muted-foreground hover:bg-muted hover:text-foreground',
            )}
          >
            <Icon className="size-4 shrink-0" strokeWidth={1.75} aria-hidden />
            <span className="min-w-0 flex-1">
              <span className="block truncate text-sm font-medium">{t(`style.${style}`)}</span>
              <span className="mt-1.5 flex gap-1" aria-hidden>
                {[accentSwatch(accent, style), background.light, background.dark].map((color, i) => (
                  <span key={i} className="h-1.5 flex-1 rounded-full ring-1 ring-black/10 dark:ring-white/15" style={{ backgroundColor: color }} />
                ))}
              </span>
            </span>
          </button>
        )
      })}
    </div>
  )
}

export function AccentPicker({ value, style, onChange }: { value: Accent; style: Style; onChange: (accent: Accent) => void }) {
  const { t } = useTranslation()
  const { refs, onKeyDown } = useRadioKeys(ACCENTS, onChange)

  return (
    <div role="radiogroup" aria-label={t('accent.label')} className="-mx-1 flex flex-wrap">
      {ACCENTS.map((accent, index) => {
        const selected = accent === value
        const swatch = accentSwatch(accent, style)
        return (
          <button
            key={accent}
            ref={(el) => {
              refs.current[index] = el
            }}
            type="button"
            role="radio"
            aria-checked={selected}
            aria-label={t(`accent.${accent}`)}
            title={t(`accent.${accent}`)}
            tabIndex={selected ? 0 : -1}
            onClick={() => onChange(accent)}
            onKeyDown={(event) => onKeyDown(event, index)}
            className="group grid size-11 place-items-center rounded-full outline-none"
          >
            <span
              className={cn(
                'grid size-8 place-items-center rounded-full shadow-sm ring-1 ring-black/10 transition-[box-shadow,scale] duration-150 group-active:scale-90 dark:ring-white/15',
                'group-focus-visible:ring-2 group-focus-visible:ring-ring group-focus-visible:ring-offset-2 group-focus-visible:ring-offset-background',
                selected && 'ring-2 ring-offset-2 ring-offset-card',
              )}
              style={{ backgroundColor: swatch, ...(selected ? { ['--tw-ring-color' as string]: swatch } : {}) }}
            >
              {selected ? <Check className={cn('size-4', accent === 'amber' ? 'text-black/75' : 'text-white drop-shadow-sm')} strokeWidth={3} aria-hidden /> : null}
            </span>
          </button>
        )
      })}
    </div>
  )
}
