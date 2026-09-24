/**
 * Theme + accent model shared by the ThemeProvider, the pre-paint script in `index.html`
 * (keep the storage keys in sync) and the settings UI.
 */

export const THEMES = ['light', 'dark', 'system'] as const
export type Theme = (typeof THEMES)[number]
export type ResolvedTheme = 'light' | 'dark'

export const ACCENTS = ['rain', 'rose', 'violet', 'emerald', 'amber', 'graphite'] as const
export type Accent = (typeof ACCENTS)[number]
export const DEFAULT_ACCENT: Accent = 'rain'

export const THEME_STORAGE_KEY = 'rainy.theme'
export const ACCENT_STORAGE_KEY = 'rainy.accent'

/** Swatch colours for accent pickers (the light-theme `--primary` of each preset in index.css). */
export const ACCENT_SWATCHES: Readonly<Record<Accent, string>> = {
  rain: 'oklch(0.62 0.17 255)',
  rose: 'oklch(0.62 0.22 12)',
  violet: 'oklch(0.57 0.23 293)',
  emerald: 'oklch(0.58 0.14 163)',
  amber: 'oklch(0.7 0.16 62)',
  graphite: 'oklch(0.3 0.008 270)',
}

/** Browser chrome colour (`<meta name="theme-color">`) = the page background. */
export const THEME_COLORS: Readonly<Record<ResolvedTheme, string>> = {
  light: '#ffffff',
  dark: '#0a0a0a',
}

export function isTheme(value: unknown): value is Theme {
  return typeof value === 'string' && (THEMES as readonly string[]).includes(value)
}

export function isAccent(value: unknown): value is Accent {
  return typeof value === 'string' && (ACCENTS as readonly string[]).includes(value)
}

function readStorage(key: string): string | null {
  try {
    return localStorage.getItem(key)
  } catch {
    return null
  }
}

function writeStorage(key: string, value: string): void {
  try {
    localStorage.setItem(key, value)
  } catch {
    // storage unavailable (private mode): the choice lasts for this session only
  }
}

export function readTheme(): Theme {
  const value = readStorage(THEME_STORAGE_KEY)
  return isTheme(value) ? value : 'system'
}

export function readAccent(): Accent {
  const value = readStorage(ACCENT_STORAGE_KEY)
  return isAccent(value) ? value : DEFAULT_ACCENT
}

export function storeTheme(theme: Theme): void {
  writeStorage(THEME_STORAGE_KEY, theme)
}

export function storeAccent(accent: Accent): void {
  writeStorage(ACCENT_STORAGE_KEY, accent)
}

/** Run `fn` with CSS transitions disabled so a theme switch doesn't animate every surface. */
function withoutTransitions(fn: () => void): void {
  const style = document.createElement('style')
  style.textContent = '*,*::before,*::after{transition:none!important}'
  document.head.appendChild(style)
  fn()
  // Force a style flush before re-enabling transitions.
  void window.getComputedStyle(document.body).opacity
  requestAnimationFrame(() => style.remove())
}

/** Apply the resolved theme + accent to `<html>` and the browser chrome colour. */
export function applyTheme(resolved: ResolvedTheme, accent: Accent): void {
  const root = document.documentElement
  const changed = root.classList.contains('dark') !== (resolved === 'dark') || root.dataset.accent !== accent
  const apply = () => {
    root.classList.toggle('dark', resolved === 'dark')
    root.style.colorScheme = resolved
    root.dataset.accent = accent
    for (const meta of document.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]')) {
      meta.content = THEME_COLORS[resolved]
    }
  }
  if (changed) withoutTransitions(apply)
  else apply()
}
