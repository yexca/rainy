import { useCallback, useSyncExternalStore } from 'react'

/** Matches Tailwind's `md` breakpoint (48rem): below it is the phone layout. */
export const MOBILE_QUERY = '(width < 48rem)'
/** Matches Tailwind's `lg` breakpoint (64rem): the full desktop layout with an expanded sidebar. */
export const DESKTOP_QUERY = '(width >= 64rem)'

/** Live result of a CSS media query (`false` where `matchMedia` is unavailable). */
export function useMediaQuery(query: string): boolean {
  const subscribe = useCallback(
    (onChange: () => void) => {
      const mql = window.matchMedia(query)
      mql.addEventListener('change', onChange)
      return () => mql.removeEventListener('change', onChange)
    },
    [query],
  )
  const getSnapshot = useCallback(() => window.matchMedia(query).matches, [query])
  return useSyncExternalStore(subscribe, getSnapshot, () => false)
}

/** Phone layout (< 768px): bottom tab bar + mini player, no sidebar. */
export function useIsMobile(): boolean {
  return useMediaQuery(MOBILE_QUERY)
}

/** Desktop layout (≥ 1024px): expandable sidebar. 768–1023px is the tablet (icon sidebar) layout. */
export function useIsDesktop(): boolean {
  return useMediaQuery(DESKTOP_QUERY)
}
