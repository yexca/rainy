import { useTranslation } from 'react-i18next'
import { Navigate, Outlet, useLocation } from 'react-router'

import { ErrorState } from '@/components/error-state'
import { Logo } from '@/components/logo'
import { FullscreenLoader } from '@/components/spinner'
import { ForbiddenPage } from '@/components/status-pages'
import { useAuth } from '@/hooks/use-auth'
import { useMascotArt } from '@/hooks/use-mascot-art'
import { redirectTarget, type LoginRedirectState } from '@/lib/navigation'

/** The auth status could not be loaded (backend down / offline). The logo stands in when illustrations are off. */
function ServerUnavailable({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  const { t } = useTranslation()
  const showArt = useMascotArt()
  return (
    <div className="grid min-h-dvh place-items-center px-6 pt-safe pb-safe">
      <div className="flex flex-col items-center">
        {showArt ? null : <Logo size={48} />}
        <ErrorState error={error} title={t('errors.serverUnreachableTitle')} onRetry={onRetry} className="py-8" />
      </div>
    </div>
  )
}

/** Signed-in users only. Sends others to /setup (first run) or /login (remembering the page). */
export function RequireAuth() {
  const { isLoading, isError, error, refetch, initialized, user } = useAuth()
  const location = useLocation()

  if (isLoading) return <FullscreenLoader />
  if (isError) return <ServerUnavailable error={error} onRetry={refetch} />
  if (!initialized) return <Navigate to="/setup" replace />
  if (!user) {
    const state: LoginRedirectState = { from: `${location.pathname}${location.search}${location.hash}` }
    return <Navigate to="/login" replace state={state} />
  }
  return <Outlet />
}

/**
 * /login and /setup: signed-in users go back to where they came from; /login ↔ /setup follow
 * the server's `initialized` flag. When the server is unreachable the page still renders (it
 * shows its own "can't reach the server" notice).
 */
export function GuestOnly() {
  const { isLoading, isError, initialized, user } = useAuth()
  const location = useLocation()

  if (isLoading) return <FullscreenLoader />
  if (!isError) {
    if (user) return <Navigate to={redirectTarget(location.state)} replace />
    const onSetup = location.pathname.startsWith('/setup')
    if (!initialized && !onSetup) return <Navigate to="/setup" replace />
    if (initialized && onSetup) return <Navigate to="/login" replace />
  }
  return <Outlet />
}

/** Library managers (canManage) and admins. */
export function RequireManager() {
  const { isManager } = useAuth()
  return isManager ? <Outlet /> : <ForbiddenPage />
}

/** Admins only. */
export function RequireAdmin() {
  const { isAdmin } = useAuth()
  return isAdmin ? <Outlet /> : <ForbiddenPage />
}
