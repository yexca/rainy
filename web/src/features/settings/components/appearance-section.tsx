import { Check, Circle, Feather, Layers3, Monitor, Moon, Shapes, Sun, type LucideIcon } from 'lucide-react'
import { useRef, type CSSProperties, type KeyboardEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Switch } from '@/components/ui/switch'
import { useTheme } from '@/hooks/use-theme'
import { LANGUAGES, currentLanguage, setLanguage, type Language } from '@/lib/i18n'
import {
  IMMERSIVE_SWATCH,
  STYLES,
  STYLE_PREVIEWS,
  accentSwatch,
  immersiveForStyle,
  showsCoverWheel,
  styleAccents,
  type Accent,
  type AccentSwatch,
  type Style,
  type Theme,
} from '@/lib/theme'
import { cn } from '@/lib/utils'

import { Segmented, SettingsRow, SettingsSection } from './settings-ui'

export function AppearanceSection() {
  const { t, i18n } = useTranslation('settings')
  const { theme, setTheme, style, setStyle, accent, savedAccent, setAccent, immersive, setImmersive } = useTheme()
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
        <StylePicker value={style} accent={savedAccent} immersive={immersive} onChange={setStyle} className="w-full sm:w-80" />
      </SettingsRow>
      {/* The immersive switch sits beside the label; the swatches below follow it. */}
      <div>
        <SettingsRow
          label={t('common:accent.label')}
          description={immersive ? t('appearance.immersiveHint') : t('appearance.accentHint')}
          className="pb-1"
        >
          <ImmersiveSwitch id="settings-immersive" checked={immersive} onChange={setImmersive} />
        </SettingsRow>
        <div className="px-4 pb-2">
          <AccentPicker value={accent} style={style} immersive={immersive} onChange={setAccent} />
        </div>
      </div>
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

const PREVIEW_BAR = 'h-1.5 flex-1 rounded-full ring-1 ring-black/10 dark:ring-white/15'

/**
 * Inline properties of an `accent-swatch` element (index.css): the accent's light and dark
 * colours, so the swatch shows the current theme's; the cover wheel paints over the fill.
 */
function swatchStyle({ light, dark }: AccentSwatch, wheel: boolean): CSSProperties {
  return {
    '--swatch-light': light.primary,
    '--swatch-dark': dark.primary,
    '--swatch-fg-light': light.foreground,
    '--swatch-fg-dark': dark.foreground,
    ...(wheel ? { backgroundImage: IMMERSIVE_SWATCH } : {}),
  } as CSSProperties
}

/**
 * Style presets as cards: icon, name, and the accent / light / dark colours it gives (the accent
 * is a colour wheel where choosing the card leaves you immersive, see `immersiveForStyle`).
 */
export function StylePicker({
  value,
  accent,
  immersive,
  onChange,
  className,
}: {
  value: Style
  accent: Accent
  immersive: boolean
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
                <span
                  className={cn(PREVIEW_BAR, 'accent-swatch bg-(--swatch)')}
                  style={swatchStyle(accentSwatch(accent, style), showsCoverWheel(accent, style, immersiveForStyle(immersive, value, style)))}
                />
                {[background.light, background.dark].map((fill) => (
                  <span key={fill} className={PREVIEW_BAR} style={{ background: fill }} />
                ))}
              </span>
            </span>
          </button>
        )
      })}
    </div>
  )
}

/** The immersive mode switch with its name, shown beside the accent label. */
export function ImmersiveSwitch({
  id,
  checked,
  onChange,
  className,
}: {
  id: string
  checked: boolean
  onChange: (enabled: boolean) => void
  className?: string
}) {
  const { t } = useTranslation()
  return (
    <span className={cn('flex shrink-0 items-center gap-2', className)}>
      <label htmlFor={id} className="text-[13px] font-medium text-muted-foreground">
        {t('immersive.label')}
      </label>
      <Switch id={id} checked={checked} onCheckedChange={onChange} />
    </span>
  )
}

/**
 * The highlight colours the style offers, in the current theme. In immersive mode the first
 * swatch (`original`) is a colour wheel: the highlight follows the cover like everything else;
 * the others stay fixed highlights.
 */
export function AccentPicker({
  value,
  style,
  immersive,
  onChange,
}: {
  value: Accent
  style: Style
  immersive: boolean
  onChange: (accent: Accent) => void
}) {
  const { t } = useTranslation()
  const accents = styleAccents(style)
  const { refs, onKeyDown } = useRadioKeys(accents, onChange)

  return (
    <div role="radiogroup" aria-label={t('accent.label')} className="-mx-1 flex flex-wrap">
      {accents.map((accent, index) => {
        const selected = accent === value
        const wheel = showsCoverWheel(accent, style, immersive)
        const label = wheel ? t('accent.cover') : t(`accent.${accent}`)
        return (
          <button
            key={accent}
            ref={(el) => {
              refs.current[index] = el
            }}
            type="button"
            role="radio"
            aria-checked={selected}
            aria-label={label}
            title={label}
            tabIndex={selected ? 0 : -1}
            onClick={() => onChange(accent)}
            onKeyDown={(event) => onKeyDown(event, index)}
            className="group grid size-11 place-items-center rounded-full outline-none"
          >
            <span
              className={cn(
                'accent-swatch grid size-8 place-items-center rounded-full bg-(--swatch) shadow-sm ring-1 ring-black/10 transition-[box-shadow,scale] duration-150 group-active:scale-90 dark:ring-white/15',
                'group-focus-visible:ring-2 group-focus-visible:ring-ring group-focus-visible:ring-offset-2 group-focus-visible:ring-offset-background',
                selected && 'ring-2 ring-offset-2 ring-offset-card',
              )}
              style={{ ...swatchStyle(accentSwatch(accent, style), wheel), ...(selected ? { ['--tw-ring-color' as string]: 'var(--swatch)' } : {}) }}
            >
              {selected ? (
                <Check className={cn('size-4', wheel ? 'text-white drop-shadow-sm' : 'text-(--swatch-fg)')} strokeWidth={3} aria-hidden />
              ) : null}
            </span>
          </button>
        )
      })}
    </div>
  )
}
