/**
 * Pure model helpers for Library tools › Folders: directory paths, progressive folder
 * tree rows, file grouping, sidecar `.lrc` pairing, and timed-lyrics preview parsing.
 */
// Relative `.ts` import and no runtime imports, so node --test (tests/folder-explorer.test.ts) can load it.
import type { FileEntry, FolderListing, Track } from '../../../lib/api/types.ts'

const TEXT_PREVIEW_EXTS = new Set(['lrc', 'txt', 'cue', 'log', 'md', 'nfo', 'm3u', 'm3u8'])

export interface FolderNavigatorRow {
  dir: string
  name: string
  depth: number
  isRoot: boolean
  hasChildren: boolean
  expanded: boolean
  loading: boolean
  trackCount: number
}

export interface FolderTrackItem {
  file: FileEntry
  /** 1-based position among playable indexed tracks in this folder (`0` when not indexed). */
  playNumber: number
  /** 0-based index into `playableTracks` (`-1` when not indexed). */
  playIndex: number
  sidecars: FileEntry[]
}

export interface FolderPreviewItem {
  kind: 'image' | 'text'
  name: string
  path: string
  size: number
  mtime: number
  isLyrics: boolean
}

export interface FolderFileSections {
  audio: FolderTrackItem[]
  playableTracks: Track[]
  unindexedAudioCount: number
  attachedLyricsCount: number
  images: FileEntry[]
  documents: FileEntry[]
  others: FileEntry[]
  imagePreviews: FolderPreviewItem[]
  documentPreviews: FolderPreviewItem[]
}

export interface ParsedPreviewLine {
  timeLabel: string | null
  text: string
}

/** Normalizes a library-relative directory into non-empty path segments (`""` → `[]`). */
export function splitDir(dir: string): string[] {
  const trimmed = dir.replace(/\\/g, '/').trim().replace(/^\/+|\/+$/g, '')
  return trimmed ? trimmed.split('/').filter(Boolean) : []
}

/** Joins path segments into a normalized library-relative directory. */
export function joinDir(parts: readonly string[]): string {
  return parts
    .map((p) => p.trim().replace(/^\/+|\/+$/g, ''))
    .filter(Boolean)
    .join('/')
}

/** Every directory from the library root `""` down to `dir` inclusive. */
export function ancestorDirs(dir: string): string[] {
  const parts = splitDir(dir)
  const out = ['']
  for (let i = 0; i < parts.length; i++) {
    out.push(parts.slice(0, i + 1).join('/'))
  }
  return out
}

/** Whether `candidate` is `prefix` or a descendant folder inside `prefix`. */
export function dirWithin(candidate: string | null | undefined, prefix: string): boolean {
  if (candidate === null || candidate === undefined) return false
  const normCandidate = joinDir(splitDir(candidate))
  const normPrefix = joinDir(splitDir(prefix))
  if (normPrefix === '') return true
  return normCandidate === normPrefix || normCandidate.startsWith(`${normPrefix}/`)
}

/** Lowercase file extension without the leading dot (`""` when none). */
export function fileExtension(name: string): string {
  const dot = name.lastIndexOf('.')
  return dot > 0 && dot < name.length - 1 ? name.slice(dot + 1).toLowerCase() : ''
}

/** File name without its final extension. */
export function fileStem(name: string): string {
  const dot = name.lastIndexOf('.')
  return dot > 0 ? name.slice(0, dot) : name
}

export function isPreviewableTextFile(name: string): boolean {
  return TEXT_PREVIEW_EXTS.has(fileExtension(name))
}

export function isLyricsSidecarFile(name: string): boolean {
  return fileExtension(name) === 'lrc'
}

/** Builds visible folder tree rows from progressively loaded `FolderListing` entries. */
export function buildFolderNavigatorRows(opts: {
  rootName: string
  listings: ReadonlyMap<string, FolderListing>
  loadingDirs?: ReadonlySet<string>
  expandedDirs: ReadonlySet<string>
}): FolderNavigatorRow[] {
  const { rootName, listings, loadingDirs = new Set<string>(), expandedDirs } = opts
  const rootListing = listings.get('')
  const rootTrackCount = rootListing
    ? rootListing.folders.reduce((sum, f) => sum + f.trackCount, 0) +
      rootListing.files.reduce((sum, f) => sum + (f.track ? 1 : 0), 0)
    : 0

  const rows: FolderNavigatorRow[] = [
    {
      dir: '',
      name: rootName,
      depth: 0,
      isRoot: true,
      hasChildren: (rootListing?.folders.length ?? 0) > 0,
      expanded: true,
      loading: loadingDirs.has('') && !rootListing,
      trackCount: rootTrackCount,
    },
  ]

  const visitChildren = (parentDir: string, depth: number) => {
    const listing = listings.get(parentDir)
    if (!listing) return
    for (const folder of listing.folders) {
      const childListing = listings.get(folder.path)
      const hasChildren = childListing ? childListing.folders.length > 0 : folder.hasSubfolders
      const wantsExpand = expandedDirs.has(folder.path)
      const expanded = hasChildren && wantsExpand
      const loading = wantsExpand && loadingDirs.has(folder.path) && !childListing
      rows.push({
        dir: folder.path,
        name: folder.name,
        depth,
        isRoot: false,
        hasChildren,
        expanded,
        loading,
        trackCount: folder.trackCount,
      })
      if (expanded && childListing) {
        visitChildren(folder.path, depth + 1)
      }
    }
  }

  // Root's immediate folders align at depth 0 beneath the root header, matching compact file trees.
  visitChildren('', 0)
  return rows
}

function matchAudioForLyrics(lrcName: string, audioByLowerStem: ReadonlyMap<string, string>): string | null {
  const stem = fileStem(lrcName)
  const exact = audioByLowerStem.get(stem.toLowerCase())
  if (exact) return exact
  // Language-suffixed sidecars such as `01 Song.zh.lrc` or `01 Song.ja.lrc`.
  const langDot = stem.lastIndexOf('.')
  if (langDot > 0) {
    const tag = stem.slice(langDot + 1)
    if (/^[a-z]{2,4}(-[a-z0-9]{2,4})?$/i.test(tag)) {
      const base = stem.slice(0, langDot).toLowerCase()
      return audioByLowerStem.get(base) ?? null
    }
  }
  return null
}

export function toPreviewItem(file: FileEntry): FolderPreviewItem | null {
  if (file.isImage) {
    return {
      kind: 'image',
      name: file.name,
      path: file.path,
      size: file.size,
      mtime: file.mtime,
      isLyrics: false,
    }
  }
  if (isPreviewableTextFile(file.name)) {
    return {
      kind: 'text',
      name: file.name,
      path: file.path,
      size: file.size,
      mtime: file.mtime,
      isLyrics: isLyricsSidecarFile(file.name),
    }
  }
  return null
}

/**
 * Splits a folder's files into audio tracks, an image gallery, documents/lyrics,
 * and other files, pairing `.lrc` sidecars with matching audio tracks.
 */
export function groupFolderFiles(files: readonly FileEntry[], showAttachedLyrics: boolean): FolderFileSections {
  const audioFiles = files.filter((f) => f.isAudio)
  const audioByLowerStem = new Map<string, string>()
  for (const f of audioFiles) {
    const key = fileStem(f.name).toLowerCase()
    if (!audioByLowerStem.has(key)) audioByLowerStem.set(key, f.path)
  }

  const sidecarsByAudioPath = new Map<string, FileEntry[]>()
  const attachedPaths = new Set<string>()
  for (const f of files) {
    if (f.isAudio || f.isImage || !isLyricsSidecarFile(f.name)) continue
    const audioPath = matchAudioForLyrics(f.name, audioByLowerStem)
    if (!audioPath) continue
    attachedPaths.add(f.path)
    const list = sidecarsByAudioPath.get(audioPath) ?? []
    list.push(f)
    sidecarsByAudioPath.set(audioPath, list)
  }

  const playableTracks: Track[] = []
  let unindexedAudioCount = 0
  const audio: FolderTrackItem[] = audioFiles.map((file) => {
    const sidecars = sidecarsByAudioPath.get(file.path) ?? []
    if (file.track) {
      const playIndex = playableTracks.length
      playableTracks.push(file.track)
      return { file, playNumber: playIndex + 1, playIndex, sidecars }
    }
    unindexedAudioCount++
    return { file, playNumber: 0, playIndex: -1, sidecars }
  })

  const images: FileEntry[] = []
  const documents: FileEntry[] = []
  const others: FileEntry[] = []

  for (const file of files) {
    if (file.isAudio) continue
    if (file.isImage) {
      images.push(file)
      continue
    }
    if (attachedPaths.has(file.path) && !showAttachedLyrics) {
      continue
    }
    if (isPreviewableTextFile(file.name)) {
      documents.push(file)
    } else {
      others.push(file)
    }
  }

  return {
    audio,
    playableTracks,
    unindexedAudioCount,
    attachedLyricsCount: attachedPaths.size,
    images,
    documents,
    others,
    imagePreviews: images.map((f) => toPreviewItem(f)!),
    documentPreviews: documents.map((f) => toPreviewItem(f)!),
  }
}

const LRC_TAG_RE = /^\[(\d{1,3}):(\d{2})(?:[.:](\d{1,3}))?\](.*)$/
const LRC_META_RE = /^\[(ar|ti|al|by|offset|length|re|ve):.*\]$/i

/** Parses timed `.lrc` content into timestamp + text rows; returns `null` when untimed. */
export function parsePreviewLyrics(content: string): ParsedPreviewLine[] | null {
  const rawLines = content.replace(/\r\n?/g, '\n').split('\n')
  const parsed: ParsedPreviewLine[] = []
  let timedCount = 0

  for (const raw of rawLines) {
    const trimmed = raw.trim()
    if (!trimmed) continue
    if (LRC_META_RE.test(trimmed)) continue
    const match = LRC_TAG_RE.exec(trimmed)
    if (match) {
      timedCount++
      const mins = match[1].padStart(2, '0')
      const secs = match[2]
      const text = match[4].replace(/<\d{1,3}:\d{2}(?:[.:]\d{1,3})?>/g, '').trim()
      parsed.push({ timeLabel: `${mins}:${secs}`, text })
    } else {
      parsed.push({ timeLabel: null, text: trimmed })
    }
  }

  return timedCount >= 2 ? parsed : null
}
