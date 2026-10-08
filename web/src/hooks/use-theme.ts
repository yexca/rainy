import { createContext, useContext } from 'react'

import type { Accent, ResolvedTheme, Style, Theme, ThemeOverride } from '@/lib/theme'

export interface ThemeContextValue {
  /** The user's choice (`system` follows the OS). */
  theme: Theme
  /** What is actually shown. */
  resolvedTheme: ResolvedTheme
  /** Style preset (surfaces, radius, fonts and the `original` accent). */
  style: Style
  accent: Accent
  /** Album pages take on the cover's colours. */
  albumColors: boolean
  /** The user turned immersive mode on (it takes effect only with `albumColors`). */
  immersive: boolean
  setTheme: (theme: Theme) => void
  setStyle: (style: Style) => void
  setAccent: (accent: Accent) => void
  setAlbumColors: (enabled: boolean) => void
  setImmersive: (enabled: boolean) => void
  /** Immersive mode: tokens that replace the theme on `<html>` (`null` restores it). */
  setOverride: (override: ThemeOverride | null) => void
}

export const ThemeContext = createContext<ThemeContextValue | null>(null)

/** Current theme, style + accent and their setters (persisted). Requires `<ThemeProvider>`. */
export function useTheme(): ThemeContextValue {
  const context = useContext(ThemeContext)
  if (!context) throw new Error('useTheme must be used within <ThemeProvider>')
  return context
}
