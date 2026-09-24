/** Recent search terms, kept per browser in localStorage (most recent first, max 10). */
import { useSyncExternalStore } from 'react'

const STORAGE_KEY = 'rainy.recentSearches'
const MAX = 10
const EMPTY: readonly string[] = []

const listeners = new Set<() => void>()
let cache: readonly string[] | null = null

function read(): readonly string[] {
  if (cache) return cache
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    const parsed: unknown = raw ? JSON.parse(raw) : []
    cache = Array.isArray(parsed) ? parsed.filter((v): v is string => typeof v === 'string').slice(0, MAX) : EMPTY
  } catch {
    cache = EMPTY
  }
  return cache
}

function write(next: readonly string[]): void {
  cache = next
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(next))
  } catch {
    // storage unavailable (private mode): keep the in-memory list for this session
  }
  for (const fn of listeners) fn()
}

function subscribe(fn: () => void): () => void {
  listeners.add(fn)
  const onStorage = (event: StorageEvent) => {
    if (event.key !== STORAGE_KEY) return
    cache = null
    fn()
  }
  window.addEventListener('storage', onStorage)
  return () => {
    listeners.delete(fn)
    window.removeEventListener('storage', onStorage)
  }
}

/** Remember a term (case-insensitive de-duplication, moved to the front). */
export function addRecentSearch(term: string): void {
  const value = term.trim()
  if (!value) return
  const lower = value.toLowerCase()
  write([value, ...read().filter((v) => v.toLowerCase() !== lower)].slice(0, MAX))
}

export function removeRecentSearch(term: string): void {
  write(read().filter((v) => v !== term))
}

export function clearRecentSearches(): void {
  write(EMPTY)
}

export function useRecentSearches(): readonly string[] {
  return useSyncExternalStore(subscribe, read, () => EMPTY)
}
