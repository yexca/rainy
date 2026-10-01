/**
 * Infinite mode (docs/architecture/contract.md §9.2): while it is on and repeat is off, asks
 * `/api/recommend/mix` for songs like the ones playing whenever two or fewer are left, and
 * appends them as `autoAdded` entries. A request that failed or found nothing is not repeated
 * until the queue changes.
 */
import { api } from '@/lib/api/endpoints'

import { mixExclude, mixSeeds, needsRefill, refillKey, REFILL_SIZE } from '../lib/infinite'
import { entryKey, usePlayer, type PlayerState } from '../store'

export class InfiniteRefill {
  private lastKey = ''
  private inFlight: AbortController | null = null
  private unsubscribe: (() => void) | null = null

  start(): void {
    this.unsubscribe = usePlayer.subscribe((state) => this.check(state))
    this.check(usePlayer.getState())
  }

  dispose(): void {
    this.unsubscribe?.()
    this.inFlight?.abort()
  }

  private check(state: PlayerState): void {
    if (!needsRefill(state)) {
      // Turned off or the queue was replaced: drop a request that no longer applies.
      if (!state.infinite) this.inFlight?.abort()
      return
    }
    const key = refillKey(state.queue, state.index)
    if (this.inFlight || key === this.lastKey) return
    this.lastKey = key
    void this.refill(state)
  }

  private async refill(state: PlayerState): Promise<void> {
    const ctrl = new AbortController()
    this.inFlight = ctrl
    try {
      const tracks = await api.recommend.mix(
        { seeds: mixSeeds(state.queue, state.index), exclude: mixExclude(state.queue), limit: REFILL_SIZE },
        { signal: ctrl.signal },
      )
      const now = usePlayer.getState()
      // Skip when the queue was replaced meanwhile (its first entry is another entry).
      const same = now.queue.length > 0 && entryKey(now.queue[0]) === entryKey(state.queue[0])
      if (!ctrl.signal.aborted && same) now.appendAuto(tracks)
    } catch {
      // Offline or the server refused: try again when the queue changes.
      if (!ctrl.signal.aborted && usePlayer.getState().awaitingMore) usePlayer.setState({ awaitingMore: false })
    } finally {
      if (this.inFlight === ctrl) this.inFlight = null
    }
    // The queue may have run low again while the request was out.
    if (!ctrl.signal.aborted) this.check(usePlayer.getState())
  }
}
