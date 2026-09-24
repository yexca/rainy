/**
 * Authentication state (`['auth']` query = `/api/auth/status`) and the login / setup / logout
 * mutations. The router guards and the shell read everything from here.
 */
import { queryOptions, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import { usePlayer } from '@/features/player/store'
import { api, type Credentials } from '@/lib/api/endpoints'
import type { AuthStatus, User } from '@/lib/api/types'
import { authQueryKey } from '@/lib/query-client'

export const authStatusQuery = queryOptions({
  queryKey: authQueryKey,
  queryFn: ({ signal }) => api.auth.status({ signal }),
  staleTime: 5 * 60_000,
  refetchOnReconnect: true,
  // The guards swap whole subtrees on pending/error. If a failed status refetched whenever a
  // new observer mounts, the error screen would mount observers → pending → loader → error → …
  // Retrying is explicit (refetch / reconnect) instead.
  retryOnMount: false,
})

/** Managers may edit tags / upload / delete files (admins always can). */
export function canManage(user: User | null | undefined): boolean {
  return !!user && (user.isAdmin || user.canManage)
}

export interface AuthState {
  status: AuthStatus | undefined
  user: User | null
  /** Whether the first admin account exists. */
  initialized: boolean
  /** Server version string (from the status endpoint). */
  version: string
  /** First load in progress (no data yet). */
  isLoading: boolean
  /** The status could not be loaded and there is no cached value. */
  isError: boolean
  error: Error | null
  refetch: () => void
  isAuthenticated: boolean
  isManager: boolean
  isAdmin: boolean
}

export function useAuth(): AuthState {
  const query = useQuery(authStatusQuery)
  const status = query.data
  const user = status?.user ?? null
  return {
    status,
    user,
    initialized: status?.initialized ?? false,
    version: status?.version ?? '',
    isLoading: query.isPending,
    isError: query.isError && !status,
    error: query.error,
    refetch: () => void query.refetch(),
    isAuthenticated: user !== null,
    isManager: canManage(user),
    isAdmin: user?.isAdmin ?? false,
  }
}

/** The signed-in user, or `null`. */
export function useCurrentUser(): User | null {
  return useQuery({ ...authStatusQuery, select: (s) => s.user }).data ?? null
}

function useSetSignedIn() {
  const queryClient = useQueryClient()
  return (user: User) => {
    queryClient.setQueryData<AuthStatus>(authQueryKey, (old) => ({
      initialized: true,
      version: old?.version ?? '',
      user,
    }))
  }
}

export function useLogin() {
  const setSignedIn = useSetSignedIn()
  return useMutation({
    mutationKey: ['auth', 'login'],
    mutationFn: (credentials: Credentials) => api.auth.login(credentials),
    onSuccess: setSignedIn,
  })
}

/** First-run: create the admin account (only allowed while no user exists). */
export function useSetup() {
  const setSignedIn = useSetSignedIn()
  return useMutation({
    mutationKey: ['auth', 'setup'],
    mutationFn: (credentials: Credentials) => api.auth.setup(credentials),
    onSuccess: setSignedIn,
  })
}

export function useLogout() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationKey: ['auth', 'logout'],
    mutationFn: () => api.auth.logout(),
    // Even if the request fails (offline), forget everything locally.
    onSettled: () => {
      usePlayer.getState().clearQueue()
      usePlayer.getState().setNowPlayingOpen(false)
      queryClient.removeQueries({ predicate: (q) => q.queryKey[0] !== authQueryKey[0] })
      queryClient.setQueryData<AuthStatus>(authQueryKey, (old) => ({
        initialized: old?.initialized ?? true,
        version: old?.version ?? '',
        user: null,
      }))
    },
  })
}
