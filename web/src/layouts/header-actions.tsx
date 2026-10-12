import { Monitor, Moon, Palette, Settings, Sun } from 'lucide-react'
import { useId, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'

import { Button } from '@/components/ui/button'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { AccentPicker, ImmersiveSwitch, StylePicker } from '@/features/settings/components/appearance-section'
import { Segmented } from '@/features/settings/components/settings-ui'
import { useIsMobile } from '@/hooks/use-media-query'
import { useTheme } from '@/hooks/use-theme'
import { LANGUAGES, currentLanguage, setLanguage } from '@/lib/i18n'
import { STYLE_CHROME, type Style, type Theme } from '@/lib/theme'

import { QuickActions } from './quick-actions'
import { UserMenu } from './user-menu'

/** Icon buttons inside the quick-control tray: round, muted until hovered or open; 44px touch targets on phones. */
const TRAY_BUTTON_CLASS =
  'size-11 md:size-9 rounded-full text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-foreground data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-foreground'

/** `beside`: the panel opens to the right of the sidebar instead of below the header. */
function AppearanceMenu({ beside = false }: { beside?: boolean }) {
  const { t, i18n } = useTranslation()
  const isMobile = useIsMobile()
  const { theme, setTheme, style, setStyle, accent, setAccent, immersive, setImmersive } = useTheme()
  // The header and the sidebar each mount a panel.
  const immersiveId = useId()
  // Reading i18n.language re-renders the picker when the language changes.
  const language = i18n.language ? currentLanguage() : 'en'
  const [open, setOpen] = useState(false)
  const label = t('theme.label')

  const themes: { value: Theme; label: string; icon: ReactNode }[] = [
    { value: 'light', label: t('theme.light'), icon: <Sun className="size-3.5" aria-hidden /> },
    { value: 'dark', label: t('theme.dark'), icon: <Moon className="size-3.5" aria-hidden /> },
    { value: 'system', label: t('theme.system'), icon: <Monitor className="size-3.5" aria-hidden /> },
  ]

  const onStyle = (next: Style) => {
    // A style with another chrome layout moves this button away (tablets and desktops): close the
    // panel rather than leave it hanging off a control that is sliding out.
    if (!isMobile && STYLE_CHROME[next] !== STYLE_CHROME[style]) setOpen(false)
    setStyle(next)
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="icon" aria-label={label} title={label} className={TRAY_BUTTON_CLASS}>
          <Palette strokeWidth={1.75} />
        </Button>
      </PopoverTrigger>
      <PopoverContent
        side={beside ? 'right' : 'bottom'}
        align="end"
        sideOffset={beside ? 12 : 8}
        className="max-h-(--radix-popover-content-available-height) w-auto overflow-y-auto rounded-xl p-0"
      >
        <div className="border-b px-4 py-3 text-sm font-semibold">{label}</div>
        <div className="grid gap-4 p-4">
          <Segmented label={t('theme.label')} value={theme} onChange={setTheme} options={themes} />
          <div className="grid gap-1.5">
            <span className="text-xs font-medium text-muted-foreground">{t('style.label')}</span>
            <StylePicker value={style} accent={accent} immersive={immersive} onChange={onStyle} />
          </div>
          <div className="grid gap-1.5">
            <div className="flex min-h-6 items-center justify-between gap-3">
              <span className="text-xs font-medium text-muted-foreground">{t('accent.label')}</span>
              <ImmersiveSwitch id={immersiveId} checked={immersive} onChange={setImmersive} />
            </div>
            <AccentPicker value={accent} style={style} immersive={immersive} onChange={setAccent} />
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
 * Right side of the app header: a bordered tray of quick controls (Quick actions, the Ctrl/⌘K
 * command palette; appearance: theme, style, accent, immersive mode and language), a divider, then the account
 * button with the user's name and role. Phones keep both controls as 44px targets but drop the
 * tray outline and the divider, and show the account as an avatar.
 */
export function HeaderActions() {
  const isMobile = useIsMobile()

  if (isMobile) {
    return (
      <div className="ml-auto flex shrink-0 items-center">
        <QuickActions className={TRAY_BUTTON_CLASS} />
        <AppearanceMenu />
        <UserMenu variant="avatar" />
      </div>
    )
  }

  return (
    <div className="ml-auto flex shrink-0 items-center gap-2">
      <div className="flex items-center gap-0.5 rounded-full border border-sidebar-border bg-background/60 p-0.5">
        <QuickActions className={TRAY_BUTTON_CLASS} />
        <AppearanceMenu />
      </div>
      <span aria-hidden className="h-5 w-px bg-sidebar-border" />
      <UserMenu variant="header" />
    </div>
  )
}

/**
 * The same controls at the bottom of the sidebar, for styles without an app header (`sidebar`
 * chrome): the account row (avatar, name and role; its menu opens upwards) with Quick actions and
 * the appearance button beside it. On the icon rail they stack, the avatar at the bottom.
 */
export function SidebarActions() {
  return (
    <div className="flex items-center gap-1 group-data-[collapsible=icon]:flex-col-reverse">
      <UserMenu variant="sidebar" className="flex-1 group-data-[collapsible=icon]:flex-none" />
      <div className="flex shrink-0 items-center gap-0.5 group-data-[collapsible=icon]:flex-col">
        <QuickActions className={TRAY_BUTTON_CLASS} />
        <AppearanceMenu beside />
      </div>
    </div>
  )
}
