/** Location state set by the auth guard when it sends a signed-out user to /login. */
export interface LoginRedirectState {
  from?: string
}

/** Where to go after signing in: the page that required auth, or Home. Same-origin paths only. */
export function redirectTarget(state: unknown): string {
  const from = (state as LoginRedirectState | null)?.from
  if (typeof from !== 'string' || !from.startsWith('/') || from.startsWith('//')) return '/'
  if (from.startsWith('/login') || from.startsWith('/setup')) return '/'
  return from
}
