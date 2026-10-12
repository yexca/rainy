/**
 * Theme, style + accent model shared by the ThemeProvider, the pre-paint script in `index.html`
 * (keep the storage keys in sync) and the settings UI.
 */

import {
  DEFAULT_ACCENT,
  DEFAULT_STYLE,
  STYLE_CHROME,
  STYLE_IMMERSIVE,
  isAccent,
  isStyle,
  themeColor,
  type Accent,
  type ResolvedTheme,
  type Style,
} from '@/lib/theme-styles'

export * from '@/lib/theme-styles'

export const THEMES = ['light', 'dark', 'system'] as const
export type Theme = (typeof THEMES)[number]

export const THEME_STORAGE_KEY = 'rainy.theme'
export const STYLE_STORAGE_KEY = 'rainy.style'
export const ACCENT_STORAGE_KEY = 'rainy.accent'
/** Album pages take on the cover's colours (`'0'` turns it off). */
export const ALBUM_COLORS_STORAGE_KEY = 'rainy.album-colors'
/** Immersive mode: the whole app takes on the current cover's colours (`'1'` on, `'0'` off, unset: the style decides). */
export const IMMERSIVE_STORAGE_KEY = 'rainy.immersive'

/** Tokens that replace the theme on `<html>` (immersive mode; a `CoverTheme` from the library). */
export interface ThemeOverride {
  /** Render dark (`.dark` on `<html>`) whatever the user's theme. */
  dark: boolean
  /** Custom properties set inline on `<html>`. */
  vars: Readonly<Record<string, string>>
  /** Browser chrome colour (`<meta name="theme-color">`). */
  themeColor: string
}

export function isTheme(value: unknown): value is Theme {
  return typeof value === 'string' && (THEMES as readonly string[]).includes(value)
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

export function readStyle(): Style {
  const value = readStorage(STYLE_STORAGE_KEY)
  return isStyle(value) ? value : DEFAULT_STYLE
}

export function readAccent(): Accent {
  const value = readStorage(ACCENT_STORAGE_KEY)
  return isAccent(value) ? value : DEFAULT_ACCENT
}

export function readAlbumColors(): boolean {
  return readStorage(ALBUM_COLORS_STORAGE_KEY) !== '0'
}

/** The saved choice, or what the style starts with. */
export function readImmersive(style: Style = readStyle()): boolean {
  const value = readStorage(IMMERSIVE_STORAGE_KEY)
  return value === '1' || value === '0' ? value === '1' : STYLE_IMMERSIVE[style]
}

export function storeTheme(theme: Theme): void {
  writeStorage(THEME_STORAGE_KEY, theme)
}

export function storeStyle(style: Style): void {
  writeStorage(STYLE_STORAGE_KEY, style)
}

export function storeAccent(accent: Accent): void {
  writeStorage(ACCENT_STORAGE_KEY, accent)
}

export function storeAlbumColors(enabled: boolean): void {
  writeStorage(ALBUM_COLORS_STORAGE_KEY, enabled ? '1' : '0')
}

export function storeImmersive(enabled: boolean): void {
  writeStorage(IMMERSIVE_STORAGE_KEY, enabled ? '1' : '0')
}

/**
 * Run `fn` with CSS transitions disabled so a theme switch doesn't animate every surface. Elements
 * marked `data-chrome-motion` keep theirs, so the app chrome slides to its new place when a style
 * moves it; they must not transition colours.
 */
function withoutTransitions(fn: () => void): void {
  const style = document.createElement('style')
  style.textContent = '*:not([data-chrome-motion]),*::before,*::after{transition:none!important}'
  document.head.appendChild(style)
  fn()
  // Force a style flush before re-enabling transitions.
  void window.getComputedStyle(document.body).opacity
  requestAnimationFrame(() => style.remove())
}

/** The override applied to `<html>` by the last applyTheme() (its inline properties to remove later). */
let applied: ThemeOverride | null = null

/** The theme colour for what `<html>` shows right now (after applyTheme or the pre-paint script). */
export function currentThemeColor(): string {
  if (applied) return applied.themeColor
  const root = document.documentElement
  const style = isStyle(root.dataset.style) ? root.dataset.style : DEFAULT_STYLE
  return themeColor(style, root.classList.contains('dark') ? 'dark' : 'light')
}

/**
 * Apply the resolved theme, style (with its chrome layout) and accent to `<html>` and the browser
 * chrome colour. An `override` (immersive mode) replaces the colour tokens and the light / dark
 * choice.
 */
export function applyTheme(resolved: ResolvedTheme, style: Style, accent: Accent, override: ThemeOverride | null = null): void {
  const root = document.documentElement
  const dark = override ? override.dark : resolved === 'dark'
  const changed =
    root.classList.contains('dark') !== dark ||
    root.dataset.style !== style ||
    root.dataset.accent !== accent ||
    applied !== override
  const apply = () => {
    if (applied) for (const key of Object.keys(applied.vars)) root.style.removeProperty(key)
    if (override) for (const [key, value] of Object.entries(override.vars)) root.style.setProperty(key, value)
    // The pre-paint <style> in index.html paints <html> in the style's colour; follow the cover.
    root.style.backgroundColor = override?.vars['--background'] ?? ''
    applied = override
    root.classList.toggle('dark', dark)
    root.style.colorScheme = dark ? 'dark' : 'light'
    root.dataset.style = style
    root.dataset.chrome = STYLE_CHROME[style]
    root.dataset.accent = accent
    for (const meta of document.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]')) {
      meta.content = override ? override.themeColor : themeColor(style, resolved)
    }
  }
  if (changed) withoutTransitions(apply)
  else apply()
}
