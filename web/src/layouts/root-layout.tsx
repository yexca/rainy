import { Outlet, ScrollRestoration } from 'react-router'

/** Top-level route element: scroll restoration for the whole app (the document scrolls). */
export function RootLayout() {
  return (
    <>
      <ScrollRestoration />
      <Outlet />
    </>
  )
}
