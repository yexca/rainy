/**
 * Style and accent presets (pure data, no DOM): read by lib/theme.ts, the pickers and tests.
 * Keep in sync with the `[data-style]` / `[data-accent]` tokens in index.css and the pre-paint
 * script in index.html.
 */

export type ResolvedTheme = 'light' | 'dark'

/** Style presets: surfaces, radius, fonts and the original accent (`<html data-style>`). */
export const STYLES = ['anthropic', 'openai', 'apple', 'google-md'] as const
export type Style = (typeof STYLES)[number]
export const DEFAULT_STYLE: Style = 'anthropic'

/** `original` keeps the style's own `--primary`; the others override it. */
export const ACCENTS = ['original', 'rain', 'rose', 'violet', 'emerald', 'amber', 'graphite'] as const
export type Accent = (typeof ACCENTS)[number]
export const DEFAULT_ACCENT: Accent = 'original'

/** Swatch colours for accent pickers (the light-theme `--primary` of each preset in index.css). */
export const ACCENT_SWATCHES: Readonly<Record<Exclude<Accent, 'original'>, string>> = {
  rain: 'oklch(0.62 0.17 255)',
  rose: 'oklch(0.62 0.22 12)',
  violet: 'oklch(0.57 0.23 293)',
  emerald: 'oklch(0.58 0.14 163)',
  amber: 'oklch(0.7 0.16 62)',
  graphite: 'oklch(0.3 0.008 270)',
}

export interface StylePreview {
  /** Light-theme `--primary` (the `original` accent). */
  primary: string
  /** Light and dark page backgrounds = browser chrome colour (`<meta name="theme-color">`). */
  background: Readonly<Record<ResolvedTheme, string>>
}

export const STYLE_PREVIEWS: Readonly<Record<Style, StylePreview>> = {
  anthropic: { primary: '#ac4e2f', background: { light: '#f7f3ed', dark: '#191614' } },
  openai: { primary: '#0b835b', background: { light: '#fafafa', dark: '#141414' } },
  apple: { primary: '#006adb', background: { light: '#f3f3f7', dark: '#111113' } },
  'google-md': { primary: '#1663df', background: { light: '#f8fafc', dark: '#121416' } },
}

export function accentSwatch(accent: Accent, style: Style): string {
  return accent === 'original' ? STYLE_PREVIEWS[style].primary : ACCENT_SWATCHES[accent]
}

/** Browser chrome colour (`<meta name="theme-color">`) = the page background. */
export function themeColor(style: Style, resolved: ResolvedTheme): string {
  return STYLE_PREVIEWS[style].background[resolved]
}

export function isStyle(value: unknown): value is Style {
  return typeof value === 'string' && (STYLES as readonly string[]).includes(value)
}

export function isAccent(value: unknown): value is Accent {
  return typeof value === 'string' && (ACCENTS as readonly string[]).includes(value)
}
