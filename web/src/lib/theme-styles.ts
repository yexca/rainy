/**
 * Style and accent presets (pure data, no DOM): read by lib/theme.ts, the pickers and tests.
 * Keep in sync with the `[data-style]` / `[data-accent]` / `[data-chrome]` rules in index.css and
 * the pre-paint script in index.html.
 */

export type ResolvedTheme = 'light' | 'dark'

/**
 * Style presets: surfaces, radius, fonts, the original accent (`<html data-style>`), the layout
 * of the app chrome (`STYLE_CHROME`, `STYLE_QUICK_ACTIONS`, `STYLE_RAIL`) and whether the style
 * starts in immersive mode (`STYLE_IMMERSIVE`).
 */
export const STYLES = ['anthropic', 'openai', 'apple', 'google-md'] as const
export type Style = (typeof STYLES)[number]
export const DEFAULT_STYLE: Style = 'apple'

/**
 * Highlight colours. `original` is the style's own `--primary`; each style offers only the hues
 * its design source publishes (`STYLE_ACCENTS`), and a hue it lacks shows as `original`. In
 * immersive mode `original` instead takes the highlight from the cover (the style's own until
 * there is one).
 */
export const ACCENTS = ['original', 'blue', 'green', 'purple', 'pink', 'orange'] as const
export type Accent = (typeof ACCENTS)[number]
export const DEFAULT_ACCENT: Accent = 'original'

/** Accent values stored before the per-style palettes, read as their closest hue. */
export const RETIRED_ACCENTS: Readonly<Record<string, Accent>> = {
  rain: 'blue',
  rose: 'pink',
  violet: 'purple',
  emerald: 'green',
  amber: 'orange',
  graphite: 'original',
}

/** A highlight in one theme: `--primary` and the text on it (`--primary-foreground`). */
export interface AccentColors {
  primary: string
  foreground: string
}

export type AccentSwatch = Readonly<Record<ResolvedTheme, AccentColors>>

const WHITE = '#ffffff'

/**
 * Each style's highlights, light and dark, as the `[data-style][data-accent]` rules in index.css
 * set them (the swatches show the current theme's value). Sources, read 2026-11-27:
 * - apple: Human Interface Guidelines system colours (June 2025 values), white text.
 * - google-md: Material 3 tones of Google palette seeds (material-color-utilities 0.3.0):
 *   primary tone 40 / on-primary 100 light, tone 80 / 20 dark.
 * - openai: Apps SDK (`@openai/apps-sdk-ui` 0.2.2): the gray primary solid with inverse text,
 *   and the info, discovery and warning solids with white text in both themes.
 * - anthropic: the brand Orange, Blue and Green with the brand Dark text in dark; in light the
 *   brand hue and chroma (OKLCH) at the lightness that reaches 4.6:1 on the page, with the brand
 *   Light text (the brand colours are too light for text on a pale page).
 */
export const STYLE_ACCENTS: Readonly<Record<Style, Readonly<{ original: AccentSwatch } & Partial<Record<Accent, AccentSwatch>>>>> = {
  anthropic: {
    original: { light: { primary: '#b15333', foreground: '#faf9f5' }, dark: { primary: '#d97757', foreground: '#141413' } },
    blue: { light: { primary: '#4272a0', foreground: '#faf9f5' }, dark: { primary: '#6a9bcc', foreground: '#141413' } },
    green: { light: { primary: '#627547', foreground: '#faf9f5' }, dark: { primary: '#788c5d', foreground: '#141413' } },
  },
  openai: {
    original: { light: { primary: '#181818', foreground: WHITE }, dark: { primary: '#f3f3f3', foreground: '#0d0d0d' } },
    blue: { light: { primary: '#0285ff', foreground: WHITE }, dark: { primary: '#0285ff', foreground: WHITE } },
    purple: { light: { primary: '#924ff7', foreground: WHITE }, dark: { primary: '#924ff7', foreground: WHITE } },
    orange: { light: { primary: '#e25507', foreground: WHITE }, dark: { primary: '#e25507', foreground: WHITE } },
  },
  apple: {
    original: { light: { primary: '#0088ff', foreground: WHITE }, dark: { primary: '#0091ff', foreground: WHITE } },
    purple: { light: { primary: '#cb30e0', foreground: WHITE }, dark: { primary: '#db34f2', foreground: WHITE } },
    pink: { light: { primary: '#ff2d55', foreground: WHITE }, dark: { primary: '#ff375f', foreground: WHITE } },
    orange: { light: { primary: '#ff8d28', foreground: WHITE }, dark: { primary: '#ff9230', foreground: WHITE } },
  },
  'google-md': {
    original: { light: { primary: '#005bc0', foreground: WHITE }, dark: { primary: '#adc7ff', foreground: '#002e68' } },
    green: { light: { primary: '#006e2a', foreground: WHITE }, dark: { primary: '#73dc82', foreground: '#003912' } },
    purple: { light: { primary: '#8622d9', foreground: WHITE }, dark: { primary: '#deb7ff', foreground: '#4a0080' } },
    orange: { light: { primary: '#994700', foreground: WHITE }, dark: { primary: '#ffb68a', foreground: '#522300' } },
  },
}

/** The accents a style offers, `original` first. */
export function styleAccents(style: Style): Accent[] {
  return ACCENTS.filter((accent) => accent in STYLE_ACCENTS[style])
}

/** What the style shows for the chosen accent: `original` for a hue it does not offer (the choice itself is kept). */
export function resolveAccent(accent: Accent, style: Style): Accent {
  return accent in STYLE_ACCENTS[style] ? accent : 'original'
}

/** A stored accent value: current, retired (mapped to its hue) or unknown (`original`). */
export function parseAccent(value: unknown): Accent {
  if (isAccent(value)) return value
  return typeof value === 'string' && Object.hasOwn(RETIRED_ACCENTS, value) ? RETIRED_ACCENTS[value] : DEFAULT_ACCENT
}

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
  openai: 'sidebar',
  apple: 'sidebar',
  'google-md': 'header',
}

/**
 * How Quick actions shows in the chrome from 768px up: the lightning `icon` among the quick
 * controls, or a search-like `field` — centred in the app header, or under the logo in the
 * sidebar. Both open the same palette; phones always get the icon.
 */
export type QuickActionsEntry = 'icon' | 'field'

export const STYLE_QUICK_ACTIONS: Readonly<Record<Style, QuickActionsEntry>> = {
  anthropic: 'icon',
  openai: 'field',
  apple: 'icon',
  'google-md': 'field',
}

/**
 * The collapsed sidebar: `icons` only, or a wider navigation rail with a `labels` line under each
 * icon and a pill behind the active one (Material).
 */
export type Rail = 'icons' | 'labels'

export const STYLE_RAIL: Readonly<Record<Style, Rail>> = {
  anthropic: 'icons',
  openai: 'icons',
  apple: 'icons',
  'google-md': 'labels',
}

export interface StylePreview {
  /** Light and dark page backgrounds = browser chrome colour (`<meta name="theme-color">`). */
  background: Readonly<Record<ResolvedTheme, string>>
}

export const STYLE_PREVIEWS: Readonly<Record<Style, StylePreview>> = {
  anthropic: { background: { light: '#faf9f5', dark: '#141413' } },
  openai: { background: { light: '#ffffff', dark: '#212121' } },
  apple: { background: { light: '#f2f2f7', dark: '#111113' } },
  'google-md': { background: { light: '#f9f9ff', dark: '#10131a' } },
}

/** Picker fill for `original` in immersive mode: the highlight comes from the cover, so a wheel of cover-like hues. */
export const IMMERSIVE_SWATCH = 'conic-gradient(from 200deg, #e8607a, #f0a24c, #58c08c, #4c9bea, #9a6cea, #e8607a)'

/** The accent's colours under a style (its `original` for a hue the style does not offer). */
export function accentSwatch(accent: Accent, style: Style): AccentSwatch {
  return STYLE_ACCENTS[style][resolveAccent(accent, style)] ?? STYLE_ACCENTS[style].original
}

/** Whether a picker draws the accent as the cover colour wheel (`original` in immersive mode). */
export function showsCoverWheel(accent: Accent, style: Style, immersive: boolean): boolean {
  return immersive && resolveAccent(accent, style) === 'original'
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
