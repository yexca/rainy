/**
 * Routes (docs/architecture/contract.md §9.1). Every page module lives at
 * `src/features/<area>/pages/<name>-page.tsx` and default-exports its component; they are
 * lazy-loaded so each page is its own chunk.
 */
import type { ComponentType } from 'react'
import { Navigate, createBrowserRouter, type RouteObject } from 'react-router'

import { FullscreenLoader } from '@/components/spinner'
import { NotFoundPage, RouteErrorPage } from '@/components/status-pages'
import { AppShell } from '@/layouts/app-shell'
import { AuthLayout } from '@/layouts/auth-layout'
import { GuestOnly, RequireAdmin, RequireAuth, RequireManager } from '@/layouts/guards'
import { RootLayout } from '@/layouts/root-layout'

type PageModule = { default: ComponentType }

/** `lazy` for a page module with a default export. */
function page(load: () => Promise<PageModule>): RouteObject['lazy'] {
  return async () => ({ Component: (await load()).default })
}

export const routes: RouteObject[] = [
  {
    id: 'root',
    element: <RootLayout />,
    errorElement: <RouteErrorPage />,
    hydrateFallbackElement: <FullscreenLoader />,
    children: [
      // ---- public (no shell)
      {
        element: <GuestOnly />,
        children: [
          {
            element: <AuthLayout />,
            children: [
              { path: 'login', lazy: page(() => import('@/features/auth/pages/login-page')) },
              { path: 'setup', lazy: page(() => import('@/features/auth/pages/setup-page')) },
            ],
          },
        ],
      },

      // ---- signed in
      {
        element: <RequireAuth />,
        children: [
          {
            element: <AppShell />,
            children: [
              {
                // Page errors render inside the shell instead of replacing it.
                errorElement: <RouteErrorPage />,
                children: [
                  { index: true, lazy: page(() => import('@/features/library/pages/home-page')) },
                  { path: 'search', lazy: page(() => import('@/features/library/pages/search-page')) },
                  { path: 'library', lazy: page(() => import('@/features/library/pages/library-page')) },
                  { path: 'albums', lazy: page(() => import('@/features/library/pages/albums-page')) },
                  { path: 'albums/:id', lazy: page(() => import('@/features/library/pages/album-page')) },
                  { path: 'artists', lazy: page(() => import('@/features/library/pages/artists-page')) },
                  { path: 'artists/:id', lazy: page(() => import('@/features/library/pages/artist-page')) },
                  { path: 'songs', lazy: page(() => import('@/features/library/pages/songs-page')) },
                  { path: 'genres', lazy: page(() => import('@/features/library/pages/genres-page')) },
                  { path: 'genres/:name', lazy: page(() => import('@/features/library/pages/genre-page')) },
                  { path: 'favorites', lazy: page(() => import('@/features/library/pages/favorites-page')) },
                  { path: 'playlists', lazy: page(() => import('@/features/library/pages/playlists-page')) },
                  { path: 'playlists/:id', lazy: page(() => import('@/features/library/pages/playlist-page')) },
                  { path: 'radio', lazy: page(() => import('@/features/library/pages/radio-page')) },
                  { path: 'settings', lazy: page(() => import('@/features/settings/pages/settings-page')) },
                  {
                    path: 'manage',
                    element: <RequireManager />,
                    children: [
                      { index: true, lazy: page(() => import('@/features/manage/pages/manage-page')) },
                      { path: 'folders', lazy: page(() => import('@/features/manage/pages/folders-page')) },
                      { path: 'upload', lazy: page(() => import('@/features/manage/pages/upload-page')) },
                      { path: 'online', lazy: page(() => import('@/features/manage/pages/online-page')) },
                      { path: 'doctor', lazy: page(() => import('@/features/manage/pages/doctor-page')) },
                      { path: 'trash', lazy: page(() => import('@/features/manage/pages/trash-page')) },
                      { path: 'history', lazy: page(() => import('@/features/manage/pages/history-page')) },
                    ],
                  },
                  {
                    path: 'admin',
                    element: <RequireAdmin />,
                    children: [
                      { index: true, element: <Navigate to="users" replace /> },
                      { path: 'users', lazy: page(() => import('@/features/admin/pages/users-page')) },
                      { path: 'libraries', lazy: page(() => import('@/features/admin/pages/libraries-page')) },
                      { path: 'settings', lazy: page(() => import('@/features/admin/pages/settings-page')) },
                    ],
                  },
                  { path: '*', element: <NotFoundPage /> },
                ],
              },
            ],
          },
        ],
      },
    ],
  },
]

export const router = createBrowserRouter(routes)
