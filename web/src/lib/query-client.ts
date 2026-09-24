import { QueryClient } from '@tanstack/react-query'

import { isApiError, setUnauthorizedHandler } from '@/lib/api/client'
import { queryKeys } from '@/lib/query-keys'

/** `['auth']` — the auth status query (see `useAuth`). */
export const authQueryKey = queryKeys.auth

const MAX_RETRIES = 1

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      refetchOnWindowFocus: false,
      // Client errors (4xx) are final; anything else gets one more attempt.
      retry: (failureCount, error) => {
        if (isApiError(error) && error.isClientError) return false
        return failureCount < MAX_RETRIES
      },
    },
    mutations: {
      retry: false,
    },
  },
})

// A 401 from any (non-auth) endpoint means the session is gone: refresh the auth status so the
// router guard sends the user to /login.
setUnauthorizedHandler(() => {
  void queryClient.invalidateQueries({ queryKey: authQueryKey })
})
