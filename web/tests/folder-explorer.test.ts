// Run with `pnpm test` (node --test). Synthetic paths and tracks only.
import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import type { FileEntry, FolderListing, Track } from '../src/lib/api/types.ts'
import {
  ancestorDirs,
  buildFolderNavigatorRows,
  dirWithin,
  groupFolderFiles,
  parsePreviewLyrics,
  splitDir,
} from '../src/features/manage/lib/folder-explorer.ts'

function fakeTrack(id: string, path: string, duration = 180): Track {
  const parts = path.split('/')
  const filename = parts[parts.length - 1] ?? path
  const dir = parts.slice(0, -1).join('/')
  return {
    id,
    libraryId: 1,
    path,
    dir,
    filename,
    suffix: 'flac',
    size: 1024,
    mtime: 1000,
    title: filename,
    album: 'Synthetic Album',
    artist: 'Synthetic Artist',
    albumArtist: 'Synthetic Artist',
    albumId: 'alb-1',
    artistId: 'art-1',
    albumArtistId: 'art-1',
    trackNumber: 1,
    trackTotal: 2,
    discNumber: 1,
    discTotal: 1,
    discSubtitle: '',
    year: 2025,
    date: '2025',
    originalYear: 2025,
    genre: 'Electronic',
    genres: ['Electronic'],
    composer: '',
    comment: '',
    hasLrc: false,
    hasLyrics: false,
    bpm: 0,
    compilation: false,
    duration,
    bitrate: 900,
    sampleRate: 44100,
    bitDepth: 16,
    channels: 2,
    codec: 'flac',
    hasCover: true,
    rgTrackGain: null,
    rgTrackPeak: null,
    rgAlbumGain: null,
    rgAlbumPeak: null,
    mbzTrackId: '',
    mbzAlbumId: '',
    mbzArtistId: '',
    mbzAlbumArtistId: '',
    sortTitle: '',
    sortAlbum: '',
    sortArtist: '',
    sortAlbumArtist: '',
    missing: false,
    createdAt: 1000,
    updatedAt: 1000,
    starred: false,
    starredAt: null,
    rating: 0,
    playCount: 0,
    playedAt: 0,
    coverArt: '',
    contentType: 'audio/flac',
  }
}

function fakeFile(partial: Partial<FileEntry> & { name: string; path: string }): FileEntry {
  return {
    size: 100,
    mtime: 1000,
    isAudio: false,
    isImage: false,
    trackId: null,
    track: null,
    ...partial,
  }
}

describe('folder explorer helpers', () => {
  it('normalizes directory paths and ancestors', () => {
    assert.deepEqual(splitDir(''), [])
    assert.deepEqual(splitDir('/Artist\\Album/Disc 1/'), ['Artist', 'Album', 'Disc 1'])
    assert.deepEqual(ancestorDirs('Artist/Album/Disc 1'), ['', 'Artist', 'Artist/Album', 'Artist/Album/Disc 1'])
    assert.equal(dirWithin('Artist/Album/Disc 1', 'Artist/Album'), true)
    assert.equal(dirWithin('Artist/Album 2', 'Artist/Album'), false)
    assert.equal(dirWithin('Artist', ''), true)
    assert.equal(dirWithin(null, 'Artist'), false)
  })

  it('builds progressive folder tree rows from cached listings', () => {
    const listings = new Map<string, FolderListing>([
      [
        '',
        {
          libraryId: 1,
          dir: '',
          parent: null,
          writable: true,
          folders: [
            { name: 'Alpha', path: 'Alpha', trackCount: 5, hasSubfolders: true },
            { name: 'Beta', path: 'Beta', trackCount: 2, hasSubfolders: false },
          ],
          files: [],
        },
      ],
      [
        'Alpha',
        {
          libraryId: 1,
          dir: 'Alpha',
          parent: '',
          writable: true,
          folders: [{ name: 'Disc 1', path: 'Alpha/Disc 1', trackCount: 5, hasSubfolders: false }],
          files: [],
        },
      ],
    ])

    const rows = buildFolderNavigatorRows({
      rootName: 'Main Library',
      listings,
      expandedDirs: new Set(['Alpha']),
    })

    assert.deepEqual(
      rows.map((r) => ({ dir: r.dir, depth: r.depth, hasChildren: r.hasChildren, expanded: r.expanded, trackCount: r.trackCount })),
      [
        { dir: '', depth: 0, hasChildren: true, expanded: true, trackCount: 7 },
        { dir: 'Alpha', depth: 0, hasChildren: true, expanded: true, trackCount: 5 },
        { dir: 'Alpha/Disc 1', depth: 1, hasChildren: false, expanded: false, trackCount: 5 },
        { dir: 'Beta', depth: 0, hasChildren: false, expanded: false, trackCount: 2 },
      ],
    )
  })

  it('groups tracks, images, documents, and pairs sidecar .lrc files', () => {
    const trk1 = fakeTrack('t1', 'Album/01 Intro.flac', 90)
    const files: FileEntry[] = [
      fakeFile({ name: '01 Intro.flac', path: 'Album/01 Intro.flac', isAudio: true, trackId: 't1', track: trk1 }),
      fakeFile({ name: '01 Intro.lrc', path: 'Album/01 Intro.lrc' }),
      fakeFile({ name: '01 Intro.zh.lrc', path: 'Album/01 Intro.zh.lrc' }),
      fakeFile({ name: '02 Draft.flac', path: 'Album/02 Draft.flac', isAudio: true }),
      fakeFile({ name: 'cover.jpg', path: 'Album/cover.jpg', isImage: true }),
      fakeFile({ name: 'album.cue', path: 'Album/album.cue' }),
      fakeFile({ name: 'orphan.lrc', path: 'Album/orphan.lrc' }),
      fakeFile({ name: 'archive.zip', path: 'Album/archive.zip' }),
    ]

    const hidden = groupFolderFiles(files, false)
    assert.equal(hidden.playableTracks.length, 1)
    assert.equal(hidden.unindexedAudioCount, 1)
    assert.equal(hidden.attachedLyricsCount, 2)
    assert.equal(hidden.audio[0]?.playNumber, 1)
    assert.equal(hidden.audio[0]?.sidecars.length, 2)
    assert.equal(hidden.audio[1]?.playNumber, 0)
    assert.deepEqual(hidden.images.map((f) => f.name), ['cover.jpg'])
    assert.deepEqual(hidden.documents.map((f) => f.name), ['album.cue', 'orphan.lrc'])
    assert.deepEqual(hidden.others.map((f) => f.name), ['archive.zip'])

    const shown = groupFolderFiles(files, true)
    assert.deepEqual(shown.documents.map((f) => f.name), ['01 Intro.lrc', '01 Intro.zh.lrc', 'album.cue', 'orphan.lrc'])
  })

  it('parses timed LRC lines and leaves plain text null', () => {
    assert.equal(parsePreviewLyrics('Just plain liner notes\nLine two'), null)
    const timed = parsePreviewLyrics('[ti:Demo]\n[00:04.12]First line\n[00:09.50]<00:09.80>Second line')
    assert.deepEqual(timed, [
      { timeLabel: '00:04', text: 'First line' },
      { timeLabel: '00:09', text: 'Second line' },
    ])
  })

  it('expands stacked timestamps and skips every header tag', () => {
    const lrc = [
      '[au:Synthetic Author]',
      '[tool:Synthetic Tool]',
      '[#:comment]',
      '[00:05.00]Verse',
      '[00:12.00][01:30.00]Chorus',
      '[Bridge]',
      '[01:00.50]Middle',
    ].join('\n')
    assert.deepEqual(parsePreviewLyrics(lrc), [
      { timeLabel: '00:05', text: 'Verse' },
      { timeLabel: '00:12', text: 'Chorus' },
      { timeLabel: null, text: '[Bridge]' },
      { timeLabel: '01:00', text: 'Middle' },
      { timeLabel: '01:30', text: 'Chorus' },
    ])
  })
})
