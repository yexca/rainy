/**
 * Upload queue (module-level zustand store): uploads keep running while the user browses other
 * pages, one request per file (per-file progress via XHR), two at a time.
 */
import { toast } from 'sonner'
import { create } from 'zustand'

import { isAbortError } from '@/lib/api/client'
import { api } from '@/lib/api/endpoints'
import { errorMessage } from '@/lib/errors'
import i18n from '@/lib/i18n'
import { queryClient } from '@/lib/query-client'

import { invalidateLibrary } from '../queries'
import { isUploadable, type PickedFile } from './upload-files'

const CONCURRENCY = 2

export type UploadStatus = 'queued' | 'uploading' | 'done' | 'error' | 'canceled'

export interface UploadTarget {
  libraryId: number
  dir: string
  organize: boolean
}

export interface UploadEntry {
  id: string
  file: File
  path: string
  size: number
  target: UploadTarget
  status: UploadStatus
  loaded: number
  error?: string
  /** Tracks created/updated by this upload (audio files). */
  trackIds: string[]
}

interface UploadQueueState {
  entries: UploadEntry[]
  add: (files: PickedFile[], target: UploadTarget) => void
  cancel: (id: string) => void
  cancelAll: () => void
  retry: (id: string) => void
  retryFailed: () => void
  remove: (id: string) => void
  clearFinished: () => void
}

const controllers = new Map<string, AbortController>()
let active = 0
let seq = 0

function patch(id: string, update: Partial<UploadEntry>): void {
  useUploadQueue.setState((s) => ({ entries: s.entries.map((e) => (e.id === id ? { ...e, ...update } : e)) }))
}

function onBeforeUnload(e: BeforeUnloadEvent): void {
  e.preventDefault()
}

function syncUnloadGuard(): void {
  if (active > 0) window.addEventListener('beforeunload', onBeforeUnload)
  else window.removeEventListener('beforeunload', onBeforeUnload)
}

async function upload(entry: UploadEntry): Promise<void> {
  const controller = new AbortController()
  controllers.set(entry.id, controller)
  patch(entry.id, { status: 'uploading', loaded: 0, error: undefined })
  try {
    const result = await api.manage.upload(
      {
        files: [{ file: entry.file, path: entry.path }],
        libraryId: entry.target.libraryId,
        dir: entry.target.dir || undefined,
        organize: entry.target.organize,
      },
      {
        signal: controller.signal,
        onProgress: (p) => patch(entry.id, { loaded: p.loaded }),
      },
    )
    const errors = result.errors ?? []
    if (errors.length > 0) {
      patch(entry.id, { status: 'error', error: errors.map((e) => e.error).join('; ') })
    } else {
      patch(entry.id, { status: 'done', loaded: entry.size, trackIds: (result.updated ?? []).map((t) => t.id) })
    }
  } catch (error) {
    if (isAbortError(error)) patch(entry.id, { status: 'canceled' })
    else patch(entry.id, { status: 'error', error: errorMessage(error, i18n.t.bind(i18n)) })
  } finally {
    controllers.delete(entry.id)
  }
}

/** Outcome counters of the current batch (reset after it is announced). */
let batch = { done: 0, failed: 0 }

function announce(): void {
  const t = i18n.t.bind(i18n)
  if (batch.failed > 0) toast.error(t('manage:upload.finishedWithErrors', { count: batch.done, failed: batch.failed }))
  else if (batch.done > 0) toast.success(t('manage:upload.finished', { count: batch.done }))
  batch = { done: 0, failed: 0 }
}

function pump(): void {
  while (active < CONCURRENCY) {
    const next = useUploadQueue.getState().entries.find((e) => e.status === 'queued')
    if (!next) break
    active++
    // Mark synchronously so the next loop iteration doesn't pick the same entry.
    patch(next.id, { status: 'uploading' })
    syncUnloadGuard()
    void upload(next).finally(() => {
      active--
      const status = useUploadQueue.getState().entries.find((e) => e.id === next.id)?.status
      if (status === 'done') batch.done++
      else if (status === 'error') batch.failed++
      syncUnloadGuard()
      pump()
      if (active === 0) {
        announce()
        void invalidateLibrary(queryClient)
      }
    })
  }
}

export const useUploadQueue = create<UploadQueueState>()((set, get) => ({
  entries: [],
  add: (files, target) => {
    const added: UploadEntry[] = files.filter((f) => isUploadable(f.path)).map((f) => ({
      id: `u${++seq}`,
      file: f.file,
      path: f.path,
      size: f.file.size,
      target,
      status: 'queued',
      loaded: 0,
      trackIds: [],
    }))
    set((s) => ({ entries: [...s.entries, ...added] }))
    pump()
  },
  cancel: (id) => {
    const controller = controllers.get(id)
    if (controller) controller.abort()
    else patch(id, { status: 'canceled' })
  },
  cancelAll: () => {
    for (const e of get().entries) if (e.status === 'queued') patch(e.id, { status: 'canceled' })
    for (const c of controllers.values()) c.abort()
  },
  retry: (id) => {
    patch(id, { status: 'queued', loaded: 0, error: undefined })
    pump()
  },
  retryFailed: () => {
    for (const e of get().entries) if (e.status === 'error' || e.status === 'canceled') patch(e.id, { status: 'queued', loaded: 0, error: undefined })
    pump()
  },
  remove: (id) => {
    controllers.get(id)?.abort()
    set((s) => ({ entries: s.entries.filter((e) => e.id !== id) }))
  },
  clearFinished: () =>
    set((s) => ({ entries: s.entries.filter((e) => e.status === 'queued' || e.status === 'uploading') })),
}))
