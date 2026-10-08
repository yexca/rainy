import { Monitor, Moon, Palette, Search, Settings, Sun } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'

import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { AccentPicker } from '@/features/settings/components/appearance-section'
import { Segmented } from '@/features/settings/components/settings-ui'
import { useTheme } from '@/hooks/use-theme'
import { LANGUAGES, currentLanguage, setLanguage } from '@/lib/i18n'
import type { Theme } from '@/lib/theme'

import { UserMenu } from './user-menu'

/** Icon buttons inside the header tray: round, muted until hovered or open. */
const TRAY_BUTTON_CLASS =
  'size-9 rounded-full text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-foreground data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-foreground'

function AppearanceMenu() {
  const { t, i18n } = useTranslation()
  const { theme, setTheme, accent, setAccent } = useTheme()
  // Reading i18n.language re-renders the picker when the language changes.
  const language = i18n.language ? currentLanguage() : 'en'
  const [open, setOpen] = useState(false)
  const label = t('theme.label')

  const themes: { value: Theme; label: string; icon: ReactNode }[] = [
    { value: 'light', label: t('theme.light'), icon: <Sun className="size-3.5" aria-hidden /> },
    { value: 'dark', label: t('theme.dark'), icon: <Moon className="size-3.5" aria-hidden /> },
    { value: 'system', label: t('theme.system'), icon: <Monitor className="size-3.5" aria-hidden /> },
  ]

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="icon" aria-label={label} title={label} className={TRAY_BUTTON_CLASS}>
          <Palette strokeWidth={1.75} />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" sideOffset={8} className="w-auto rounded-xl p-0">
        <div className="border-b px-4 py-3 text-sm font-semibold">{label}</div>
        <div className="grid gap-4 p-4">
          <Segmented label={t('theme.label')} value={theme} onChange={setTheme} options={themes} />
          <div className="grid gap-1.5">
            <span className="text-xs font-medium text-muted-foreground">{t('accent.label')}</span>
            <AccentPicker value={accent} onChange={setAccent} />
          </div>
          <div className="grid gap-1.5">
            <span className="text-xs font-medium text-muted-foreground">{t('language.label')}</span>
            <Segmented
              label={t('language.label')}
              value={language}
              onChange={(lng) => void setLanguage(lng)}
              options={LANGUAGES.map((l) => ({ value: l.code, label: <span lang={l.htmlLang}>{l.label}</span> }))}
            />
          </div>
        </div>
        <div className="border-t p-1.5">
          <Link
            to="/settings/appearance"
            onClick={() => setOpen(false)}
            className="flex h-9 items-center gap-2.5 rounded-md px-2.5 text-sm outline-none transition-colors hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring"
          >
            <Settings className="size-4 text-muted-foreground" aria-hidden />
            {t('theme.allSettings')}
          </Link>
        </div>
      </PopoverContent>
    </Popover>
  )
}

/**
 * Right side of the app header: a bordered tray of quick controls (search; appearance: theme,
 * accent and language), a divider, then the account button with the user's name and role.
 */
export function HeaderActions() {
  const { t } = useTranslation()
  const searchLabel = t('nav.search')

  return (
    <div className="ml-auto flex shrink-0 items-center gap-2">
      <div className="flex items-center gap-0.5 rounded-full border border-sidebar-border bg-background/60 p-0.5">
        <Button variant="ghost" size="icon" asChild className={TRAY_BUTTON_CLASS}>
          <Link to="/search" aria-label={searchLabel} title={searchLabel}>
            <Search strokeWidth={1.75} />
          </Link>
        </Button>
        <AppearanceMenu />
      </div>
      <span aria-hidden className="h-5 w-px bg-sidebar-border" />
      <UserMenu variant="header" />
    </div>
  )
}
