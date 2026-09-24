/**
 * Low-level HTTP client for the native API (`/api`, docs/architecture/contract.md §7).
 *
 * - Same-origin cookie auth (`credentials: 'same-origin'`).
 * - JSON in / JSON out; `FormData`, `Blob`, `URLSearchParams` and strings are sent as-is.
 * - Non-2xx responses throw {@link ApiError} (`{status, code, message}`), network failures throw an
 *   `ApiError` with status 0 / code `network`; aborted requests reject with the native `AbortError`.
 * - A 401 from any non-auth endpoint calls the handler registered with
 *   {@link setUnauthorizedHandler} (the query client uses it to invalidate `['auth']`).
 */
import type { ApiErrorBody } from './types'

export const API_BASE = '/api'

/** Error codes of the native API (§7) plus client-side `network` / `aborted` / `parse`. */
export type ApiErrorCode =
  | 'bad_request'
  | 'unauthorized'
  | 'forbidden'
  | 'not_found'
  | 'conflict'
  | 'readonly'
  | 'rate_limited'
  | 'internal'
  | 'not_implemented'
  | 'unavailable'
  | 'network'
  | 'parse'
  | (string & {})

export class ApiError extends Error {
  /** HTTP status; 0 when the server could not be reached. */
  readonly status: number
  readonly code: ApiErrorCode

  constructor(status: number, code: ApiErrorCode, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }

  /** True for 4xx responses (client errors that should not be retried). */
  get isClientError(): boolean {
    return this.status >= 400 && this.status < 500
  }
}

export function isApiError(error: unknown): error is ApiError {
  return error instanceof ApiError
}

export function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === 'AbortError'
}

// ---------------------------------------------------------------------------------------------
// 401 handling
// ---------------------------------------------------------------------------------------------

type UnauthorizedHandler = (error: ApiError) => void
let unauthorizedHandler: UnauthorizedHandler | null = null

/** Register the global 401 handler (one at a time). Pass `null` to unregister. */
export function setUnauthorizedHandler(handler: UnauthorizedHandler | null): void {
  unauthorizedHandler = handler
}

function notifyUnauthorized(error: ApiError, skip: boolean | undefined): void {
  if (!skip && error.status === 401) unauthorizedHandler?.(error)
}

// ---------------------------------------------------------------------------------------------
// URLs
// ---------------------------------------------------------------------------------------------

export type QueryValue = string | number | boolean | null | undefined | readonly (string | number)[]
export type QueryParams = Readonly<Record<string, QueryValue>>

/**
 * Build a query string (`?a=1&b=x`, or `''`). Skips `undefined`, `null`, `''`, `NaN` and empty
 * arrays; arrays are joined with commas; booleans become `true` / `false`.
 */
export function buildQuery(params?: QueryParams): string {
  if (!params) return ''
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === '') continue
    if (typeof value === 'number' && Number.isNaN(value)) continue
    if (Array.isArray(value)) {
      if (value.length === 0) continue
      search.set(key, value.join(','))
      continue
    }
    search.set(key, String(value))
  }
  const qs = search.toString()
  return qs ? `?${qs}` : ''
}

/** Absolute-path URL for an API route: `apiUrl('/albums', {limit: 5})` → `/api/albums?limit=5`. */
export function apiUrl(path: string, params?: QueryParams): string {
  return `${API_BASE}${path.startsWith('/') ? path : `/${path}`}${buildQuery(params)}`
}

/** Encode one path segment (ids are opaque strings). */
export function seg(value: string | number): string {
  return encodeURIComponent(String(value))
}

// ---------------------------------------------------------------------------------------------
// fetch wrapper
// ---------------------------------------------------------------------------------------------

export type HttpMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'

/** Options every endpoint function accepts. */
export interface CallOptions {
  signal?: AbortSignal
  /** Let the request outlive the page (e.g. saving the queue on `pagehide`). Body ≤ 64 KiB. */
  keepalive?: boolean
}

export interface RequestOptions extends CallOptions {
  method?: HttpMethod
  query?: QueryParams
  /** JSON-serialised unless it is FormData / Blob / URLSearchParams / string. */
  body?: unknown
  headers?: Record<string, string>
  /** Do not trigger the global 401 handler (auth endpoints). */
  skipAuthHandler?: boolean
}

function isRawBody(body: unknown): body is BodyInit {
  return (
    typeof body === 'string' ||
    body instanceof FormData ||
    body instanceof Blob ||
    body instanceof URLSearchParams ||
    body instanceof ArrayBuffer
  )
}

const STATUS_CODES: Record<number, ApiErrorCode> = {
  400: 'bad_request',
  401: 'unauthorized',
  403: 'forbidden',
  404: 'not_found',
  409: 'conflict',
  429: 'rate_limited',
  500: 'internal',
  501: 'not_implemented',
  502: 'unavailable',
  503: 'unavailable',
  504: 'unavailable',
}

function codeForStatus(status: number): ApiErrorCode {
  return STATUS_CODES[status] ?? (status >= 500 ? 'internal' : 'bad_request')
}

function isErrorBody(value: unknown): value is ApiErrorBody {
  if (typeof value !== 'object' || value === null || !('error' in value)) return false
  const err = (value as { error: unknown }).error
  return typeof err === 'object' && err !== null && 'code' in err && 'message' in err
}

/** Build an {@link ApiError} from a raw response body text. */
function errorFromBody(status: number, statusText: string, text: string): ApiError {
  if (text) {
    try {
      const parsed: unknown = JSON.parse(text)
      if (isErrorBody(parsed)) {
        return new ApiError(status, parsed.error.code || codeForStatus(status), parsed.error.message || statusText)
      }
    } catch {
      // not JSON (e.g. a proxy error page) — fall through
    }
  }
  const message = statusText || `Request failed with status ${status}`
  return new ApiError(status, codeForStatus(status), message)
}

function parseSuccess<T>(text: string, contentType: string | null): T {
  if (!text) return undefined as T
  if (contentType && !contentType.includes('json')) return text as T
  try {
    return JSON.parse(text) as T
  } catch {
    throw new ApiError(200, 'parse', 'Invalid JSON response from server')
  }
}

/**
 * Perform an API request. `path` is relative to `/api` (e.g. `/albums/123`).
 * Resolves with the parsed JSON body, or `undefined` for 204 / empty responses.
 */
export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { method = 'GET', query, body, headers, signal, keepalive, skipAuthHandler } = options

  const init: RequestInit = {
    method,
    credentials: 'same-origin',
    headers: { Accept: 'application/json', ...headers },
    signal,
    keepalive,
  }
  if (body !== undefined) {
    if (isRawBody(body)) {
      init.body = body
    } else {
      init.body = JSON.stringify(body)
      ;(init.headers as Record<string, string>)['Content-Type'] = 'application/json'
    }
  }

  let response: Response
  try {
    response = await fetch(apiUrl(path, query), init)
  } catch (error) {
    if (isAbortError(error)) throw error
    throw new ApiError(0, 'network', error instanceof Error ? error.message : 'Network error')
  }

  if (response.status === 204 || response.status === 205) return undefined as T

  let text: string
  try {
    text = await response.text()
  } catch (error) {
    if (isAbortError(error)) throw error
    throw new ApiError(response.status, 'network', 'Connection lost while reading the response')
  }

  if (!response.ok) {
    const error = errorFromBody(response.status, response.statusText, text)
    notifyUnauthorized(error, skipAuthHandler)
    throw error
  }
  return parseSuccess<T>(text, response.headers.get('Content-Type'))
}

// ---------------------------------------------------------------------------------------------
// Upload with progress (XHR — fetch has no upload progress events)
// ---------------------------------------------------------------------------------------------

export interface UploadProgress {
  loaded: number
  total: number
  /** 0..1 (0 while the total is unknown). */
  fraction: number
}

export interface UploadOptions {
  method?: 'POST' | 'PUT'
  query?: QueryParams
  signal?: AbortSignal
  onProgress?: (progress: UploadProgress) => void
}

/** Send `form` as multipart/form-data to `path` (relative to `/api`), reporting upload progress. */
export function upload<T>(path: string, form: FormData, options: UploadOptions = {}): Promise<T> {
  const { method = 'POST', query, signal, onProgress } = options

  return new Promise<T>((resolve, reject) => {
    if (signal?.aborted) {
      reject(new DOMException('The upload was aborted.', 'AbortError'))
      return
    }

    const xhr = new XMLHttpRequest()
    xhr.open(method, apiUrl(path, query)) // same-origin: the session cookie is sent automatically
    xhr.setRequestHeader('Accept', 'application/json')

    const onAbort = () => xhr.abort()
    signal?.addEventListener('abort', onAbort, { once: true })
    const cleanup = () => signal?.removeEventListener('abort', onAbort)

    if (onProgress) {
      xhr.upload.addEventListener('progress', (event) => {
        const total = event.lengthComputable ? event.total : 0
        onProgress({ loaded: event.loaded, total, fraction: total > 0 ? event.loaded / total : 0 })
      })
    }

    xhr.addEventListener('load', () => {
      cleanup()
      const text = xhr.responseText
      if (xhr.status >= 200 && xhr.status < 300) {
        try {
          resolve(parseSuccess<T>(xhr.status === 204 ? '' : text, xhr.getResponseHeader('Content-Type')))
        } catch (error) {
          reject(error)
        }
        return
      }
      const error = errorFromBody(xhr.status, xhr.statusText, text)
      notifyUnauthorized(error, false)
      reject(error)
    })
    xhr.addEventListener('error', () => {
      cleanup()
      reject(new ApiError(0, 'network', 'Network error during upload'))
    })
    xhr.addEventListener('abort', () => {
      cleanup()
      reject(new DOMException('The upload was aborted.', 'AbortError'))
    })

    xhr.send(form)
  })
}
