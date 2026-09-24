import { Languages, Monitor, Moon, Sun } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Outlet } from 'react-router'

import { AppToaster } from '@/components/app-toaster'
import { Button } from '@/components/ui/button'
import { useAuth } from '@/hooks/use-auth'
import { useTheme } from '@/hooks/use-theme'
import { LANGUAGES, currentLanguage, setLanguage } from '@/lib/i18n'
import type { Theme } from '@/lib/theme'

const NEXT_THEME: Record<Theme, Theme> = { system: 'light', light: 'dark', dark: 'system' }

/** Soft, slowly drifting colour fields tinted by the accent (radial gradients — no blur filters). */
function Backdrop() {
  return (
    <div aria-hidden className="pointer-events-none absolute inset-0 -z-10 overflow-hidden">
      <div className="absolute -top-[20%] -left-[15%] size-[42rem] max-w-none animate-blob rounded-full bg-[radial-gradient(closest-side,color-mix(in_oklab,var(--primary)_55%,transparent),transparent)] opacity-70 dark:opacity-45" />
      <div className="absolute -right-[20%] -bottom-[25%] size-[46rem] max-w-none animate-blob rounded-full bg-[radial-gradient(closest-side,oklch(0.72_0.16_305/0.45),transparent)] opacity-70 [animation-delay:-8s] dark:opacity-40" />
      <div className="absolute top-[10%] right-[5%] size-[30rem] max-w-none animate-blob rounded-full bg-[radial-gradient(closest-side,oklch(0.82_0.11_200/0.45),transparent)] opacity-60 [animation-delay:-15s] dark:opacity-30" />
      <div className="absolute inset-0 bg-[linear-gradient(to_bottom,transparent,var(--background)_92%)] opacity-60" />
    </div>
  )
}

function Toolbar() {
  const { t } = useTranslation()
  const { theme, setTheme } = useTheme()
  const language = currentLanguage()
  const other = LANGUAGES.find((l) => l.code !== language) ?? LANGUAGES[0]
  const ThemeIcon = theme === 'dark' ? Moon : theme === 'light' ? Sun : Monitor

  return (
    <div className="absolute top-0 right-0 flex items-center gap-1 p-3 pt-[calc(var(--safe-top)+0.75rem)] pr-[calc(var(--safe-right)+0.75rem)]">
      <Button
        variant="ghost"
        size="sm"
        className="h-11 rounded-full text-muted-foreground md:h-9"
        onClick={() => void setLanguage(other.code)}
        aria-label={`${t('language.label')}: ${other.label}`}
        lang={other.htmlLang}
      >
        <Languages aria-hidden />
        {other.label}
      </Button>
      <Button
        variant="ghost"
        size="icon"
        className="size-11 rounded-full text-muted-foreground md:size-9"
        onClick={() => setTheme(NEXT_THEME[theme])}
        aria-label={`${t('theme.label')}: ${t(`theme.${theme}`)}`}
        title={t(`theme.${theme}`)}
      >
        <ThemeIcon aria-hidden />
      </Button>
    </div>
  )
}

/** Full-screen layout for /login and /setup (no app shell). */
export function AuthLayout() {
  const { t } = useTranslation('auth')
  const { version } = useAuth()

  return (
    <div className="relative isolate flex min-h-dvh flex-col overflow-hidden bg-background">
      <Backdrop />
      <Toolbar />
      <main className="flex flex-1 items-center justify-center px-4 pt-[calc(var(--safe-top)+4rem)] pb-8">
        <Outlet />
      </main>
      <footer className="pb-[calc(var(--safe-bottom)+1rem)] text-center text-xs text-muted-foreground">
        {version ? `${t('footer.version', { version })} · ` : null}
        {t('footer.tagline')}
      </footer>
      <AppToaster />
    </div>
  )
}
