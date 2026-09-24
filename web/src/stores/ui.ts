/**
 * Global UI state (docs/architecture/contract.md §9.2): cross-feature dialog openers and shell preferences.
 *
 *   const openTagEditor = useUI((s) => s.openTagEditor)
 *   openTagEditor(tracks.map((t) => t.id))
 *
 * The shell mounts the hosts that react to it: `<TagEditorHost/>` and `<AddToPlaylistHost/>`.
 */
import { create } from 'zustand'
import { createJSONStorage, persist } from 'zustand/middleware'

export interface TrackSelectionDialog {
  open: boolean
  trackIds: string[]
}

export interface UIState {
  tagEditor: TrackSelectionDialog
  openTagEditor: (trackIds: string[]) => void
  closeTagEditor: () => void

  addToPlaylist: TrackSelectionDialog
  openAddToPlaylist: (trackIds: string[]) => void
  closeAddToPlaylist: () => void

  /** Desktop sidebar expanded (persisted). Tablets always show the icon rail. */
  sidebarOpen: boolean
  setSidebarOpen: (open: boolean) => void
}

const closed: TrackSelectionDialog = { open: false, trackIds: [] }

export const useUI = create<UIState>()(
  persist(
    (set) => ({
      tagEditor: closed,
      openTagEditor: (trackIds) => set({ tagEditor: { open: trackIds.length > 0, trackIds: [...trackIds] } }),
      // Keep the ids while the close animation runs.
      closeTagEditor: () => set((s) => ({ tagEditor: { ...s.tagEditor, open: false } })),

      addToPlaylist: closed,
      openAddToPlaylist: (trackIds) =>
        set({ addToPlaylist: { open: trackIds.length > 0, trackIds: [...trackIds] } }),
      closeAddToPlaylist: () => set((s) => ({ addToPlaylist: { ...s.addToPlaylist, open: false } })),

      sidebarOpen: true,
      setSidebarOpen: (sidebarOpen) => set({ sidebarOpen }),
    }),
    {
      name: 'rainy.ui',
      version: 1,
      storage: createJSONStorage(() => localStorage),
      partialize: (s) => ({ sidebarOpen: s.sidebarOpen }),
    },
  ),
)
