import { useNavigation } from 'react-router'

import { cn } from '@/lib/utils'

/** Thin accent bar at the top of the viewport while a lazy route module loads. */
export function NavigationProgress() {
  const navigation = useNavigation()
  const busy = navigation.state !== 'idle'
  return (
    <div
      aria-hidden
      className={cn(
        'pointer-events-none fixed inset-x-0 top-0 z-[60] h-0.5 overflow-hidden transition-opacity duration-300',
        busy ? 'opacity-100 delay-150' : 'opacity-0',
      )}
    >
      <div className={cn('h-full w-1/3 rounded-full bg-primary', busy && 'animate-nav-progress')} />
    </div>
  )
}
