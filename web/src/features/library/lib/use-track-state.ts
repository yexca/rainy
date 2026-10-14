import { usePlayer } from '@/features/player/store'

/** Whether `trackId` is the player's current track, and if so whether it is playing. */
export function useTrackState(trackId: string): 'none' | 'playing' | 'paused' {
  return usePlayer((s) => {
    const current = s.index >= 0 ? s.queue[s.index] : undefined
    if (!current || current.id !== trackId) return 'none'
    return s.isPlaying ? 'playing' : 'paused'
  })
}
