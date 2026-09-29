/**
 * Tablet / desktop player layout preference (docs/architecture/contract.md §9.4), persisted per
 * browser. Read by `<PlayerDock/>` and the side panel; edited from the layout menu.
 *
 *   const mode = usePlayerDock((s) => s.mode)   // 'bar' | 'window' | 'compact'
 */
import { create } from 'zustand'
import { createJSONStorage, persist } from 'zustand/middleware'

import { DEFAULT_DOCK_PREFS, sanitizeDockPrefs, type DockMode, type DockPrefs } from './lib/dock'

export type { DockMode } from './lib/dock'

export interface PlayerDockState extends DockPrefs {
  setMode(mode: DockMode): void
  setBarAutoHide(autoHide: boolean): void
}

export const usePlayerDock = create<PlayerDockState>()(
  persist(
    (set) => ({
      ...DEFAULT_DOCK_PREFS,
      setMode: (mode) => set((s) => sanitizeDockPrefs({ ...s, mode })),
      setBarAutoHide: (barAutoHide) => set((s) => sanitizeDockPrefs({ ...s, barAutoHide })),
    }),
    {
      name: 'rainy.player-dock',
      version: 1,
      storage: createJSONStorage(() => localStorage),
      partialize: (s): DockPrefs => ({ mode: s.mode, barAutoHide: s.barAutoHide }),
      merge: (persisted, current) => ({ ...current, ...sanitizeDockPrefs(persisted as Partial<DockPrefs>) }),
    },
  ),
)
