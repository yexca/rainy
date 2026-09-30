/** Helpers for download jobs: links from YouTube / bilibili and online music (`/api/manage/downloads`). */
// Relative `.ts` import and no runtime imports, so node --test (tests/downloads.test.ts) can load it.
import type { DownloadJob, DownloadSite } from '../../../lib/api/types.ts'

/** Scheme added to a pasted link without one ("youtu.be/…"). */
const DEFAULT_SCHEME = 'https:'

/** Hosts the server accepts, by site (mirrors `internal/ytdlp/sites.go`). */
const SITE_HOSTS: Record<string, DownloadSite> = {
  'youtube.com': 'youtube',
  'www.youtube.com': 'youtube',
  'm.youtube.com': 'youtube',
  'music.youtube.com': 'youtube',
  'youtu.be': 'youtube',
  'bilibili.com': 'bilibili',
  'www.bilibili.com': 'bilibili',
  'm.bilibili.com': 'bilibili',
  'space.bilibili.com': 'bilibili',
  'b23.tv': 'bilibili',
}

/**
 * The site of the first link in pasted text (share text such as "【Title】 https://b23.tv/x" works),
 * or `null`. Only a hint for the UI: the server validates the link again.
 */
export function detectSite(text: string): DownloadSite | null {
  const trimmed = text.trim()
  const match = /https?:\/\/[^\s"'<>，。、【】（）《》「」]+/i.exec(trimmed)
  let raw = match?.[0]
  if (!raw) {
    if (!trimmed || /\s/.test(trimmed) || trimmed.includes('://')) return null
    raw = `${DEFAULT_SCHEME}//${trimmed}`
  }
  try {
    const url = new URL(raw)
    if (url.username || url.password) return null
    if (url.port && url.port !== '443' && url.port !== '80') return null
    return SITE_HOSTS[url.hostname.toLowerCase().replace(/\.$/, '')] ?? null
  } catch {
    return null
  }
}

export function isActiveJob(job: Pick<DownloadJob, 'status'>): boolean {
  return job.status === 'queued' || job.status === 'running' || job.status === 'importing'
}

/** Whole seconds as `m:ss` / `h:mm:ss` (`''` when unknown). */
export function formatEta(seconds: number): string {
  if (!(seconds >= 0) || !Number.isFinite(seconds)) return ''
  const s = Math.round(seconds)
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const ss = String(s % 60).padStart(2, '0')
  return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${ss}` : `${m}:${ss}`
}

/** A job's display name ("title — artists" for online music). */
export function jobName(job: Pick<DownloadJob, 'title' | 'url' | 'online'>): string {
  if (job.online) {
    const artists = job.online.song.artists.join(' / ')
    return artists ? `${job.online.song.title} — ${artists}` : job.online.song.title
  }
  return job.title || job.url
}
