import { useCallback, useEffect, useLayoutEffect, useMemo, useState, type ReactNode } from 'react'

import { useMediaQuery } from '@/hooks/use-media-query'
import { ThemeContext, type ThemeContextValue } from '@/hooks/use-theme'
import {
  ACCENT_STORAGE_KEY,
  ALBUM_COLORS_STORAGE_KEY,
  THEME_STORAGE_KEY,
  applyTheme,
  readAccent,
  readAlbumColors,
  readTheme,
  storeAccent,
  storeAlbumColors,
  storeTheme,
  type Accent,
  type ResolvedTheme,
  type Theme,
} from '@/lib/theme'

/**
 * Light / dark / system theme, accent preset and album colours. Persists the choice (localStorage), keeps
 * `<html class="dark" data-accent="…">` and `<meta name="theme-color">` in sync, follows OS
 * changes while on `system`, and syncs across tabs. Read it with `useTheme()`.
 */
export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<Theme>(readTheme)
  const [accent, setAccentState] = useState<Accent>(readAccent)
  const [albumColors, setAlbumColorsState] = useState<boolean>(readAlbumColors)
  const systemDark = useMediaQuery('(prefers-color-scheme: dark)')
  const resolvedTheme: ResolvedTheme = theme === 'system' ? (systemDark ? 'dark' : 'light') : theme

  useLayoutEffect(() => {
    applyTheme(resolvedTheme, accent)
  }, [resolvedTheme, accent])

  // Another tab changed the preference.
  useEffect(() => {
    const onStorage = (event: StorageEvent) => {
      if (event.key === THEME_STORAGE_KEY) setThemeState(readTheme())
      if (event.key === ACCENT_STORAGE_KEY) setAccentState(readAccent())
      if (event.key === ALBUM_COLORS_STORAGE_KEY) setAlbumColorsState(readAlbumColors())
    }
    window.addEventListener('storage', onStorage)
    return () => window.removeEventListener('storage', onStorage)
  }, [])

  const setTheme = useCallback((next: Theme) => {
    storeTheme(next)
    setThemeState(next)
  }, [])

  const setAccent = useCallback((next: Accent) => {
    storeAccent(next)
    setAccentState(next)
  }, [])

  const setAlbumColors = useCallback((next: boolean) => {
    storeAlbumColors(next)
    setAlbumColorsState(next)
  }, [])

  const value = useMemo<ThemeContextValue>(
    () => ({ theme, resolvedTheme, accent, albumColors, setTheme, setAccent, setAlbumColors }),
    [theme, resolvedTheme, accent, albumColors, setTheme, setAccent, setAlbumColors],
  )

  return <ThemeContext value={value}>{children}</ThemeContext>
}
