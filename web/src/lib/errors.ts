import type { TFunction } from 'i18next'

import { isApiError } from '@/lib/api/client'

/** Codes whose server message is specific (validation, conflicts) and should be shown as-is. */
const SPECIFIC_CODES: ReadonlySet<string> = new Set(['bad_request', 'conflict'])

/** Codes with a generic, translated message in `common:errors.<code>`. */
const GENERIC_CODES: ReadonlySet<string> = new Set([
  'network',
  'unauthorized',
  'forbidden',
  'not_found',
  'rate_limited',
  'readonly',
  'internal',
  'unavailable',
  'not_implemented',
])

/**
 * Human, translated message for any thrown value: the server's message for specific
 * validation / conflict errors, translated text for generic HTTP and network failures.
 */
export function errorMessage(error: unknown, t: TFunction): string {
  if (isApiError(error)) {
    if (SPECIFIC_CODES.has(error.code) && error.message) return error.message
    if (GENERIC_CODES.has(error.code)) return t(`common:errors.${error.code}`)
    return error.message || t('common:errors.unknown')
  }
  if (error instanceof Error && error.message) return error.message
  return t('common:errors.unknown')
}

/** True when the server could not be reached at all (offline, backend down, proxy error). */
export function isNetworkError(error: unknown): boolean {
  return isApiError(error) && (error.code === 'network' || error.status === 502 || error.status === 504)
}
