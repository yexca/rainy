# Frontend

The web app lives in `web/`. It is a Vite single-page app built with React 19,
TypeScript, Tailwind CSS v4, and shadcn/ui. `pnpm build` writes `web/dist`,
which `web/embed.go` embeds into the Go binary, so production needs no separate
web server.

The precise layout, routes, and cross-feature contracts are in
[contract §9](contract.md#9-frontend-architecture-web); the API types are in
[contract §8](contract.md#8-typescript-contract-websrclibapitypests).

## Structure

| Path | Responsibility |
| --- | --- |
| `src/lib/api/` | Fetch client, contract types, and one typed function per endpoint |
| `src/components/`, `src/components/ui/` | Shared components and generated shadcn components |
| `src/layouts/` | The signed-in app shell (sidebar, tab bar, player slots) and the auth layout |
| `src/features/auth/` | Sign-in and first-run setup |
| `src/features/library/` | Browsing pages, track lists, cards, and context menus |
| `src/features/player/` | Audio engine, queue store, player UI, lyrics, and PWA glue |
| `src/features/settings/` | Per-user settings |
| `src/features/manage/`, `src/features/admin/` | Library management and administration |
| `src/locales/{en,zh}/` | Translations, one namespace file per feature |

Every page module lives at `src/features/<area>/pages/<name>-page.tsx` and is
lazy-loaded by `src/router.tsx`. Manage routes require a manager or
administrator, admin routes require an administrator, and signed-out users are
redirected to `/login` (or `/setup` before the first account exists).

## State

- Server state uses TanStack Query. A `401` from any non-auth call invalidates
  the auth query, which sends the router to the sign-in page.
- Server-sent events from `/api/events` keep scan progress and library views
  current without polling.
- The player keeps its queue and transport in a Zustand store. Playback
  position lives in a separate high-frequency store so lists do not re-render
  on every time update.
- The queue is saved to `localStorage` immediately and to `/api/queue` after a
  short debounce, so a Subsonic client can resume where the browser stopped.

Track lists keep only loaded rows in memory; unloaded rows have implicit
positions. Their virtualizer keys, and the queue's keys, keep stable function
references during scrolling so measurements are reused. Artist browsing loads
200 items initially and loads other sorts near the end of the visible list.
The name view completes its A–Z index in later batches of up to 1000, with a
500 ms pause between background requests.

## Player and PWA

A single audio element plays the current track while a second one preloads
the next track shortly before the end. Media Session exposes metadata and
transport controls to lock screens, headsets, and car displays. The streaming
quality setting chooses between original files and transcoded streams.

`vite-plugin-pwa` generates the service worker and manifest. Covers are cached
for offline browsing; streams, downloads, and the event stream are never
cached. The behavior contract for the player is in
[Design](../development/design.md#ios-style-player).

## Internationalization

Every user-visible string goes through i18next with both `en` and `zh`
translations. The language follows the saved choice, then the browser
language, then English. Adding a string means adding it to both locales.

## Development Server

`pnpm dev` serves the app on port 5173 and proxies `/api` and `/rest` to the
backend (`http://localhost:7650` by default; override with `RAINY_BACKEND`).
See [Local development](../development/local-dev.md).

## Related Docs

- [Design](../development/design.md)
- [Native API](native-api.md)
- [Testing](../development/testing.md)
