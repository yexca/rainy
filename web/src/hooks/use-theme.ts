import { createContext, useContext } from 'react'

import type { Accent, ResolvedTheme, Theme } from '@/lib/theme'

export interface ThemeContextValue {
  /** The user's choice (`system` follows the OS). */
  theme: Theme
  /** What is actually shown. */
  resolvedTheme: ResolvedTheme
  accent: Accent
  setTheme: (theme: Theme) => void
  setAccent: (accent: Accent) => void
}

export const ThemeContext = createContext<ThemeContextValue | null>(null)

/** Current theme + accent and their setters (persisted). Requires `<ThemeProvider>`. */
export function useTheme(): ThemeContextValue {
  const context = useContext(ThemeContext)
  if (!context) throw new Error('useTheme must be used within <ThemeProvider>')
  return context
}
