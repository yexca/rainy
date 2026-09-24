/** Collect files (incl. whole folders) from drag & drop and file inputs for uploading. */

/** Audio extensions the scanner indexes (docs/architecture/contract.md §5.7) — keys without dot. */
export const AUDIO_EXTENSIONS: ReadonlySet<string> = new Set([
  'mp3', 'flac', 'm4a', 'm4b', 'mp4', 'aac', 'alac', 'ogg', 'oga', 'opus', 'wav', 'aif', 'aiff', 'wma', 'ape', 'wv',
  'mpc', 'dsf', 'dff', 'tta', 'spx',
])

export interface PickedFile {
  file: File
  /** Relative path incl. sub-folders for folder uploads (`Album/01 Song.flac`), else the name. */
  path: string
}

export function extension(name: string): string {
  const dot = name.lastIndexOf('.')
  return dot > 0 ? name.slice(dot + 1).toLowerCase() : ''
}

/**
 * The upload endpoint accepts audio files only (covers go through the tag editor / cover dialog,
 * lyrics through the lyrics editor), so everything else is skipped client-side.
 */
export function isUploadable(name: string): boolean {
  return AUDIO_EXTENSIONS.has(extension(name))
}

export function isAudioName(name: string): boolean {
  return AUDIO_EXTENSIONS.has(extension(name))
}

function isHidden(name: string): boolean {
  return name.startsWith('.') || name === 'Thumbs.db' || name === 'desktop.ini' || name.startsWith('@eaDir')
}

function readEntries(reader: FileSystemDirectoryReader): Promise<FileSystemEntry[]> {
  return new Promise((resolve, reject) => reader.readEntries(resolve, reject))
}

function entryFile(entry: FileSystemFileEntry): Promise<File> {
  return new Promise((resolve, reject) => entry.file(resolve, reject))
}

async function walk(entry: FileSystemEntry, out: PickedFile[]): Promise<void> {
  if (isHidden(entry.name)) return
  if (entry.isFile) {
    const file = await entryFile(entry as FileSystemFileEntry)
    out.push({ file, path: entry.fullPath.replace(/^\/+/, '') || file.name })
    return
  }
  if (entry.isDirectory) {
    const reader = (entry as FileSystemDirectoryEntry).createReader()
    // readEntries returns results in batches (≤100 in Chrome) until an empty batch.
    for (;;) {
      const batch = await readEntries(reader)
      if (batch.length === 0) break
      for (const child of batch) await walk(child, out)
    }
  }
}

/** Files from a drop event, descending into dropped folders where the browser supports it. */
export async function filesFromDataTransfer(dt: DataTransfer): Promise<PickedFile[]> {
  const entries = Array.from(dt.items)
    .filter((item) => item.kind === 'file')
    .map((item) => item.webkitGetAsEntry?.() ?? null)
  if (entries.length > 0 && entries.every((e) => e !== null)) {
    const out: PickedFile[] = []
    for (const entry of entries) await walk(entry as FileSystemEntry, out)
    return out
  }
  return Array.from(dt.files).map((file) => ({ file, path: file.name }))
}

/** Files from an `<input type=file>` (folder inputs carry `webkitRelativePath`). */
export function filesFromInput(list: FileList | null): PickedFile[] {
  if (!list) return []
  return Array.from(list)
    .filter((file) => !(file.webkitRelativePath || file.name).split('/').some(isHidden))
    .map((file) => ({ file, path: file.webkitRelativePath || file.name }))
}
