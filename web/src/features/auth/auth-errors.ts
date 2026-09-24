import type { TFunction } from 'i18next'

import { isApiError } from '@/lib/api/client'
import { errorMessage } from '@/lib/errors'

/** Message for a failed login / setup request. */
export function authErrorMessage(error: unknown, t: TFunction, mode: 'login' | 'setup'): string {
  if (isApiError(error)) {
    if (error.status === 429 || error.code === 'rate_limited') return t('auth:errors.rateLimited')
    if (mode === 'login' && (error.status === 401 || error.code === 'unauthorized')) {
      return t('auth:errors.invalidCredentials')
    }
    if (mode === 'setup' && (error.status === 403 || error.status === 409)) {
      return t('auth:errors.alreadyInitialized')
    }
  }
  return errorMessage(error, t)
}
