import { useUI } from '@/stores/ui'

/** Whether the listener wants mascot illustrations (Settings › Mascot). */
export function useMascotArt(): boolean {
  return useUI((s) => s.mascotArt)
}
