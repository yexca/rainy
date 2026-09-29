import { Toaster } from '@/components/ui/sonner'
import { useIsMobile } from '@/hooks/use-media-query'

/**
 * App-wide toast host (sonner): top-centre below the status bar on phones, bottom-right clear of
 * the player chrome elsewhere (`--player-toast-*`, set per player layout in index.css). Use
 * `toast()` from `sonner` anywhere.
 */
export function AppToaster() {
  const isMobile = useIsMobile()
  return (
    <Toaster
      position={isMobile ? 'top-center' : 'bottom-right'}
      offset={{
        top: 'calc(var(--safe-top) + 12px)',
        right: 'var(--player-toast-right)',
        bottom: 'var(--player-toast-bottom)',
        left: 16,
      }}
      mobileOffset={{ top: 'calc(var(--safe-top) + 8px)', left: 12, right: 12 }}
      visibleToasts={isMobile ? 2 : 4}
    />
  )
}
