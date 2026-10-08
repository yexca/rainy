import { useCallback, useEffect, useLayoutEffect, useMemo, useState, type ReactNode } from 'react'

import { useMediaQuery } from '@/hooks/use-media-query'
import { ThemeContext, type ThemeContextValue } from '@/hooks/use-theme'
import {
  ACCENT_STORAGE_KEY,
  ALBUM_COLORS_STORAGE_KEY,
  IMMERSIVE_STORAGE_KEY,
  STYLE_STORAGE_KEY,
  THEME_STORAGE_KEY,
  applyTheme,
  readAccent,
  readAlbumColors,
  readImmersive,
  readStyle,
  readTheme,
  storeAccent,
  storeAlbumColors,
  storeImmersive,
  storeStyle,
  storeTheme,
  type Accent,
  type ResolvedTheme,
  type Style,
  type Theme,
  type ThemeOverride,
} from '@/lib/theme'

/**
 * Light / dark / system theme, style preset, accent preset, album colours and immersive mode. Persists the
 * choice (localStorage), keeps `<html class="dark" data-style="…" data-accent="…">` and
 * `<meta name="theme-color">` in sync, follows OS changes while on `system`, and syncs across tabs. In
 * immersive mode `<ImmersiveTheme/>` hands the cover's tokens over through `setOverride`; they replace
 * the theme on `<html>`. Read it with `useTheme()`.
 */
export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<Theme>(readTheme)
  const [style, setStyleState] = useState<Style>(readStyle)
  const [accent, setAccentState] = useState<Accent>(readAccent)
  const [albumColors, setAlbumColorsState] = useState<boolean>(readAlbumColors)
  const [immersive, setImmersiveState] = useState<boolean>(readImmersive)
  const [override, setOverride] = useState<ThemeOverride | null>(null)
  const systemDark = useMediaQuery('(prefers-color-scheme: dark)')
  const resolvedTheme: ResolvedTheme = theme === 'system' ? (systemDark ? 'dark' : 'light') : theme

  useLayoutEffect(() => {
    applyTheme(resolvedTheme, style, accent, override)
  }, [resolvedTheme, style, accent, override])

  // Another tab changed the preference.
  useEffect(() => {
    const onStorage = (event: StorageEvent) => {
      if (event.key === THEME_STORAGE_KEY) setThemeState(readTheme())
      if (event.key === STYLE_STORAGE_KEY) setStyleState(readStyle())
      if (event.key === ACCENT_STORAGE_KEY) setAccentState(readAccent())
      if (event.key === ALBUM_COLORS_STORAGE_KEY) setAlbumColorsState(readAlbumColors())
      if (event.key === IMMERSIVE_STORAGE_KEY) setImmersiveState(readImmersive())
    }
    window.addEventListener('storage', onStorage)
    return () => window.removeEventListener('storage', onStorage)
  }, [])

  const setTheme = useCallback((next: Theme) => {
    storeTheme(next)
    setThemeState(next)
  }, [])

  const setStyle = useCallback((next: Style) => {
    storeStyle(next)
    setStyleState(next)
  }, [])

  const setAccent = useCallback((next: Accent) => {
    storeAccent(next)
    setAccentState(next)
  }, [])

  const setAlbumColors = useCallback((next: boolean) => {
    storeAlbumColors(next)
    setAlbumColorsState(next)
  }, [])

  const setImmersive = useCallback((next: boolean) => {
    storeImmersive(next)
    setImmersiveState(next)
  }, [])

  const value = useMemo<ThemeContextValue>(
    () => ({
      theme,
      resolvedTheme,
      style,
      accent,
      albumColors,
      immersive,
      setTheme,
      setStyle,
      setAccent,
      setAlbumColors,
      setImmersive,
      setOverride,
    }),
    [theme, resolvedTheme, style, accent, albumColors, immersive, setTheme, setStyle, setAccent, setAlbumColors, setImmersive],
  )

  return <ThemeContext value={value}>{children}</ThemeContext>
}
