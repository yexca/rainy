import { Toaster } from '@/components/ui/sonner'
import { useIsMobile } from '@/hooks/use-media-query'

/**
 * App-wide toast host (sonner): top-centre below the status bar on phones, bottom-right above
 * the player bar elsewhere. Use `toast()` from `sonner` anywhere.
 */
export function AppToaster() {
  const isMobile = useIsMobile()
  return (
    <Toaster
      position={isMobile ? 'top-center' : 'bottom-right'}
      offset={{
        top: 'calc(var(--safe-top) + 12px)',
        right: 16,
        bottom: 'calc(var(--playerbar-h) + var(--safe-bottom) + 16px)',
        left: 16,
      }}
      mobileOffset={{ top: 'calc(var(--safe-top) + 8px)', left: 12, right: 12 }}
      visibleToasts={isMobile ? 2 : 4}
    />
  )
}
