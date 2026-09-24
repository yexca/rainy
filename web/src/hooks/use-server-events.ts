/**
 * Server-sent events from `/api/events` (docs/architecture/contract.md §7.5): one shared `EventSource` for the
 * whole app, reconnecting with exponential backoff.
 *
 * - `useServerEvents()` — mount once (the app shell does). Opens the connection while mounted
 *   and invalidates library queries on `library` events (and after reconnecting, since events
 *   may have been missed).
 * - `useServerEvent(type, handler)` — subscribe anywhere, e.g. live scan progress:
 *     useServerEvent('scan', (status) => setScan(status as ScanStatus))
 */
import { useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef } from 'react'

import { api } from '@/lib/api/endpoints'
import type { ServerEvent } from '@/lib/api/types'
import { LIBRARY_QUERY_ROOTS } from '@/lib/query-keys'

export type ServerEventType = ServerEvent['type']
export type ServerEventHandler = (data: unknown) => void

const EVENT_TYPES: readonly ServerEventType[] = ['scan', 'library', 'nowPlaying']
const MIN_DELAY = 1_000
const MAX_DELAY = 30_000

const listeners = new Map<ServerEventType, Set<ServerEventHandler>>()
const reconnectListeners = new Set<() => void>()

let source: EventSource | null = null
let connections = 0
let attempt = 0
let retryTimer: ReturnType<typeof setTimeout> | undefined
let wasOpen = false

function dispatch(type: ServerEventType, raw: string): void {
  let data: unknown = null
  if (raw) {
    try {
      data = JSON.parse(raw)
    } catch {
      data = raw
    }
  }
  for (const handler of listeners.get(type) ?? []) {
    try {
      handler(data)
    } catch (error) {
      console.error(`[events] "${type}" handler failed`, error)
    }
  }
}

function scheduleReconnect(): void {
  if (connections === 0 || retryTimer !== undefined) return
  const base = Math.min(MAX_DELAY, MIN_DELAY * 2 ** attempt)
  const delay = base / 2 + Math.random() * (base / 2)
  attempt += 1
  retryTimer = setTimeout(() => {
    retryTimer = undefined
    open()
  }, delay)
}

function open(): void {
  if (connections === 0 || source || typeof EventSource === 'undefined') return
  const es = new EventSource(api.eventsUrl())
  source = es
  for (const type of EVENT_TYPES) {
    es.addEventListener(type, (event) => dispatch(type, (event as MessageEvent<string>).data))
  }
  es.onopen = () => {
    attempt = 0
    if (wasOpen) for (const fn of reconnectListeners) fn()
    wasOpen = true
  }
  // Take over from the browser's fixed-interval retry to get backoff (and to recover from
  // HTTP errors, after which EventSource gives up entirely).
  es.onerror = () => {
    es.close()
    if (source === es) source = null
    scheduleReconnect()
  }
}

function close(): void {
  clearTimeout(retryTimer)
  retryTimer = undefined
  source?.close()
  source = null
  attempt = 0
  wasOpen = false
}

/** Mobile browsers drop background connections: reconnect right away when visible again. */
function onVisibilityChange(): void {
  if (document.visibilityState !== 'visible' || connections === 0 || source) return
  clearTimeout(retryTimer)
  retryTimer = undefined
  attempt = 0
  open()
}

function connect(): () => void {
  connections += 1
  if (connections === 1) {
    document.addEventListener('visibilitychange', onVisibilityChange)
    open()
  }
  return () => {
    connections -= 1
    if (connections === 0) {
      document.removeEventListener('visibilitychange', onVisibilityChange)
      close()
    }
  }
}

function subscribe(type: ServerEventType, handler: ServerEventHandler): () => void {
  let set = listeners.get(type)
  if (!set) {
    set = new Set()
    listeners.set(type, set)
  }
  set.add(handler)
  return () => {
    set.delete(handler)
  }
}

/** Keep the shared connection open while mounted and refresh library data on changes. */
export function useServerEvents(): void {
  const queryClient = useQueryClient()

  useEffect(() => {
    const invalidateLibrary = () => {
      void queryClient.invalidateQueries({
        predicate: (query) => LIBRARY_QUERY_ROOTS.has(String(query.queryKey[0])),
      })
    }
    const unsubscribe = subscribe('library', invalidateLibrary)
    reconnectListeners.add(invalidateLibrary)
    const disconnect = connect()
    return () => {
      unsubscribe()
      reconnectListeners.delete(invalidateLibrary)
      disconnect()
    }
  }, [queryClient])
}

/**
 * Run `handler` for every server event of `type` while mounted. The handler may change between
 * renders without resubscribing. Requires `useServerEvents()` to be mounted (the shell does it).
 */
export function useServerEvent(type: ServerEventType, handler: ServerEventHandler): void {
  const handlerRef = useRef(handler)
  useEffect(() => {
    handlerRef.current = handler
  })
  useEffect(() => subscribe(type, (data) => handlerRef.current(data)), [type])
}
