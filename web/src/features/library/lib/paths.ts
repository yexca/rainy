/** Route builders for library pages (docs/architecture/contract.md §9.1). */

export const albumPath = (id: string) => `/albums/${encodeURIComponent(id)}`
export const artistPath = (id: string) => `/artists/${encodeURIComponent(id)}`
export const playlistPath = (id: string) => `/playlists/${encodeURIComponent(id)}`
/** Genre names may contain `/`, `&`, spaces… */
export const genrePath = (name: string) => `/genres/${encodeURIComponent(name)}`
