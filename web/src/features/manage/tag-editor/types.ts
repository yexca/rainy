/** Pending (unsaved) cover and lyrics changes of the tag editor. */

export type CoverScope = 'tracks' | 'album'

export type PendingCover =
  | {
      kind: 'set'
      file: File
      /** Object URL for the preview (revoked when replaced). */
      previewUrl: string
      scope: CoverScope
      embed: boolean
      saveToFolder: boolean
    }
  | {
      kind: 'remove'
      scope: CoverScope
      removeFolderImage: boolean
    }

export type LyricsTarget = 'embedded' | 'lrc'

export interface LyricsDraft {
  text: string
  target: LyricsTarget
}

export type EditorTab = 'details' | 'cover' | 'lyrics' | 'raw' | 'file'
