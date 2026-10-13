import { createContext, useContext } from 'react'

import type { Accent, ResolvedTheme, Style, Theme, ThemeOverride } from '@/lib/theme'

export interface ThemeContextValue {
  /** The user's choice (`system` follows the OS). */
  theme: Theme
  /** What is actually shown. */
  resolvedTheme: ResolvedTheme
  /** Style preset (surfaces, radius, fonts, the `original` accent and the chrome layout). */
  style: Style
  /** The highlight; in immersive mode `original` takes it from the cover and a preset keeps its own. */
  accent: Accent
  /** Immersive mode: the whole app takes its colours from the cover on screen or playing. */
  immersive: boolean
  setTheme: (theme: Theme) => void
  /** Also moves immersive mode to what the new style starts with, unless the user set it otherwise. */
  setStyle: (style: Style) => void
  setAccent: (accent: Accent) => void
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
