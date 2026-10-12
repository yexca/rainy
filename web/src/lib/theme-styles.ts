/**
 * Style and accent presets (pure data, no DOM): read by lib/theme.ts, the pickers and tests.
 * Keep in sync with the `[data-style]` / `[data-accent]` / `[data-chrome]` rules in index.css and
 * the pre-paint script in index.html.
 */

export type ResolvedTheme = 'light' | 'dark'

/**
 * Style presets: surfaces, radius, fonts, the original accent (`<html data-style>`), where the app
 * chrome sits (`STYLE_CHROME`) and whether the style starts in immersive mode (`STYLE_IMMERSIVE`).
 */
export const STYLES = ['anthropic', 'openai', 'apple', 'google-md'] as const
export type Style = (typeof STYLES)[number]
export const DEFAULT_STYLE: Style = 'anthropic'

/**
 * `original` keeps the style's own `--primary`; the others override it. In immersive mode
 * `original` instead takes the highlight from the cover (the style's own until there is one).
 */
export const ACCENTS = ['original', 'rain', 'rose', 'violet', 'emerald', 'amber', 'graphite'] as const
export type Accent = (typeof ACCENTS)[number]
export const DEFAULT_ACCENT: Accent = 'original'

/** Immersive mode (the whole app takes its colours from the cover on screen or playing) per style, until the user sets it. */
export const STYLE_IMMERSIVE: Readonly<Record<Style, boolean>> = {
  anthropic: false,
  openai: false,
  apple: true,
  'google-md': false,
}

/**
 * Where the logo, quick controls and account menu sit from 768px up (`<html data-chrome>`; phones
 * always keep the app header): `header` is a bar across the top, `sidebar` drops the bar and puts
 * them in the sidebar, with the account at its bottom.
 */
export const CHROMES = ['header', 'sidebar'] as const
export type Chrome = (typeof CHROMES)[number]

export const STYLE_CHROME: Readonly<Record<Style, Chrome>> = {
  anthropic: 'header',
  openai: 'header',
  apple: 'sidebar',
  'google-md': 'header',
}

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

/** Picker fill for `original` in immersive mode: the highlight comes from the cover, so a wheel of cover-like hues. */
export const IMMERSIVE_SWATCH = 'conic-gradient(from 200deg, #e8607a, #f0a24c, #58c08c, #4c9bea, #9a6cea, #e8607a)'

/** The accent's solid colour under a style. */
export function accentSwatch(accent: Accent, style: Style): string {
  return accent === 'original' ? STYLE_PREVIEWS[style].primary : ACCENT_SWATCHES[accent]
}

/** CSS `background` of the accent's swatch in a picker. */
export function accentFill(accent: Accent, style: Style, immersive = false): string {
  return immersive && accent === 'original' ? IMMERSIVE_SWATCH : accentSwatch(accent, style)
}

/**
 * Immersive mode after switching styles: the old style's starting value becomes the new style's,
 * and a value the user set against the old style's stays.
 */
export function immersiveForStyle(immersive: boolean, from: Style, to: Style): boolean {
  return immersive === STYLE_IMMERSIVE[from] ? STYLE_IMMERSIVE[to] : immersive
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
