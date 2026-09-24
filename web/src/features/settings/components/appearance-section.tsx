import { Check, Monitor, Moon, Sun } from 'lucide-react'
import { useRef, type KeyboardEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { useTheme } from '@/hooks/use-theme'
import { LANGUAGES, currentLanguage, setLanguage, type Language } from '@/lib/i18n'
import { ACCENTS, ACCENT_SWATCHES, type Accent, type Theme } from '@/lib/theme'
import { cn } from '@/lib/utils'

import { Segmented, SettingsRow, SettingsSection } from './settings-ui'

export function AppearanceSection() {
  const { t, i18n } = useTranslation('settings')
  const { theme, setTheme, accent, setAccent } = useTheme()
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
      <SettingsRow label={t('common:accent.label')} description={t('appearance.accentHint')} stacked>
        <AccentPicker value={accent} onChange={setAccent} />
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

function AccentPicker({ value, onChange }: { value: Accent; onChange: (accent: Accent) => void }) {
  const { t } = useTranslation()
  const refs = useRef<(HTMLButtonElement | null)[]>([])

  const onKeyDown = (event: KeyboardEvent<HTMLButtonElement>, index: number) => {
    const dir = event.key === 'ArrowRight' || event.key === 'ArrowDown' ? 1 : event.key === 'ArrowLeft' || event.key === 'ArrowUp' ? -1 : 0
    if (!dir) return
    event.preventDefault()
    const next = (index + dir + ACCENTS.length) % ACCENTS.length
    onChange(ACCENTS[next])
    refs.current[next]?.focus()
  }

  return (
    <div role="radiogroup" aria-label={t('accent.label')} className="-mx-1 flex flex-wrap gap-0.5">
      {ACCENTS.map((accent, index) => {
        const selected = accent === value
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
              style={{ backgroundColor: ACCENT_SWATCHES[accent], ...(selected ? { ['--tw-ring-color' as string]: ACCENT_SWATCHES[accent] } : {}) }}
            >
              {selected ? <Check className={cn('size-4', accent === 'amber' ? 'text-black/75' : 'text-white drop-shadow-sm')} strokeWidth={3} aria-hidden /> : null}
            </span>
          </button>
        )
      })}
    </div>
  )
}
