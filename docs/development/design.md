# Design

Rainy's interface aims for calm, modern, Apple Music-inspired minimalism: lots
of whitespace, large artwork, and restrained color. The exact tokens and sizes
are part of the contract in
[§9.3](../architecture/contract.md#93-design-system) and
[§9.4](../architecture/contract.md#94-ios-style-player-player-agent); this page
explains the rules every UI change must keep.

## Principles

- **Mobile first.** Design at 375 px width first, then scale up. Respect iOS
  safe areas and keep every touch target at least 44 px.
- **Artwork leads.** Covers are square, rounded, lazy-loaded, and fade in. A
  gradient placeholder with a music-note icon replaces missing art. Artists are
  circles.
- **Accent sparingly.** The accent color marks play buttons, active
  navigation, and progress, not decoration.
- **Every string is translated.** Use i18next keys with both `en` and `zh`
  text; never hard-code visible copy.
- **Accessible by default.** Every icon button has an `aria-label`, focus rings
  stay visible, and color contrast meets WCAG AA.

## Theme

- shadcn/ui `new-york` style, `neutral` base color, CSS variables, and a
  `0.75rem` radius.
- Light and dark themes follow the system unless the user overrides them.
- Accent presets set `--primary`: `rain` (default periwinkle blue), `rose`,
  `violet`, `emerald`, `amber`, and `graphite`.
- **Album colours.** Album, playlist, and artist pages take on their
  artwork's colours, like the Apple Music album view: the dominant colour becomes the page background
  (a deep tone in the dark theme or for dark covers, a pale tint otherwise)
  and the most vivid colour becomes `--primary`. The tokens are scoped to the
  page element, so the sidebar, player, and menus keep the user's theme, and
  every text colour keeps WCAG AA contrast. Greyscale covers keep the user's
  accent; pages without artwork keep the plain theme. Users can turn this
  off under Settings → Appearance.
- The font stack starts with Inter Variable and falls back to system and CJK
  fonts. Times and durations use tabular numbers.

## Layout

| Width | Navigation | Player |
| --- | --- | --- |
| 1024 px and wider | App header, left sidebar (240 px) below it, collapsible to icons | 80 px bottom player bar |
| 768 to 1023 px | App header, collapsed icon sidebar | Bottom player bar |
| Below 768 px | 49 px bottom tab bar plus the safe area | Floating 56 px mini player above the tab bar |

From tablets up, a full-width app header runs across the top in the sidebar's
colours: the sidebar toggle and the logo on the left, the account menu
(avatar, plus the name on desktops) on the right. Page top bars stick right
below it. Phones have no app header; their account menu is the avatar in the
Home and Library nav bars.

Navigation is tiered so everyday listening stays uncluttered. Managers get
three sections, each a single sidebar item: **Tracks** (metadata, upload,
online), **Library tools** (folders, doctor, trash, history), and, for admins,
**Admin** (users, libraries, server settings). A section's pages are
horizontal tabs at the top of each of its pages, so the sidebar never grows a
second level. On phones the Manage tab opens Tracks, with links to Library
tools and Admin beside its tabs.

The CSS variables `--tabbar-h`, `--miniplayer-h`, `--playerbar-h`, and
`--safe-top`/`--safe-bottom` describe the player chrome. Pages use the
`.page-pad` utility so content never hides behind it.

## Typography and Density

- Page titles are large and bold. On mobile, the large title collapses into a
  centered navigation title while scrolling (`PageHeader` handles this).
- Section titles are `text-xl font-semibold`; body text is `text-sm`; secondary
  text uses `text-muted-foreground`.
- Desktop list rows are 48 px. Mobile rows are 60 px with 44 px artwork and
  iOS-style hairline separators inset from the artwork.

## Surfaces, Motion, and Icons

- Glass surfaces (tab bar, mini player, top bar on scroll) use a translucent
  background with a strong backdrop blur and a subtle border.
- Motion uses `motion/react` springs and honours the user's reduced-motion
  preference. Mobile presses scale down slightly for feedback.
- Icons come from lucide-react at `size-4` or `size-5` with a 1.75 stroke;
  transport controls use filled glyphs.
- On touch devices, disable the tap highlight and body overscroll.

## Mascot

Rainy's mascot is a girl with long silver-blue hair, blue cat-ear headphones
with raindrop ear cups, a white sailor dress, and a clear umbrella that drips
music notes. She is also the app icon: listening to music, eyes closed, on the
rain-blue tile.

- **Where she appears.** Full-size empty states (empty library pages and
  history, no search results, an empty trash, a clean doctor report, no plays in
  the listening report), errors (offline, render errors), 404, no access, the
  startup splash, the admin scan panel while a scan runs, beside the sign-in
  card on wide screens, Settings › Mascot, and as a companion in the
  bottom-right corner on tablets and desktops. Route changes keep the small
  spinner. Compact empty states in panels
  and lists keep their icon.
- **Corner companion.** She sits on top of the player chrome (above the bottom
  bar or mini bar, beside the floating window) and follows playback: listening
  while music plays, waving when paused, asleep after two minutes. Clicking her
  shows a short line; the × hides her. She never appears on phones, where the
  tab bar and mini player already fill the bottom edge.
- **Off switches.** Settings › Mascot turns the illustrations and the
  companion off separately; with illustrations off, empty and error states fall
  back to their icons.
- **Art rules.** Illustrations are decorative transparent WebP files
  (`alt=""`), sized by height, with a white sticker outline so they work in
  light and dark themes. New poses keep the same character sheet and palette;
  upper-body poses for the corner end in a flat bottom edge.

## iOS-Style Player

The player is Rainy's signature surface. Changes must keep this behavior:

- **Mini player (mobile).** A floating glass pill above the tab bar with 40 px
  artwork, one-line title and artist, play/pause and next, and a thin progress
  line. Tapping or swiping up opens Now Playing.
- **Now Playing.** A full-screen sheet on phones (a large overlay on desktop)
  that slides up with a spring and closes by dragging down from the grabber.
  The background is built from blurred, saturated artwork colors with light
  text. The artwork shrinks with a spring when paused and grows back when
  playing.
- **Controls.** The title, a tappable artist link, a star button, and a "…"
  menu sit above a scrubber that thickens while dragging, with elapsed and
  remaining time beneath. Transport controls are large filled glyphs with
  press feedback. The volume slider is hidden on iOS, where the page cannot set
  volume. An AirPlay button appears when Safari supports it.
- **Lyrics.** Synced lyrics highlight the active line, dim the others,
  auto-scroll to keep the active line centered, and seek when a line is tapped.
  Plain lyrics scroll normally. Bilingual lyrics show the translation beneath
  each line at about two thirds of its size and slightly dimmer, and every line
  carries a `lang` hint so Japanese and Korean use their own CJK glyphs. A
  translation toggle sits beside the lyrics toggle while lyrics are open.
- **Queue.** "Playing Next" supports drag-to-reorder, shuffle, repeat, and
  clear.
- **Desktop player bar.** Artwork, title and artist links, and a star on the
  left; transport and scrubber in the center; lyrics, queue, volume, the layout
  menu, and expand on the right. Queue and lyrics open in a 360 px side panel.
- **Player layouts (tablet and desktop).** The layout menu switches between the
  bottom bar, a floating window, and a floating mini bar, both in the
  bottom-right corner; full screen covers any of them. The bottom bar can
  auto-hide like NetEase Cloud Music: it slides away, leaving a small handle
  that brings it back on hover and pins it on click. The floating window is a
  compact Now Playing (artwork backdrop, transport, volume, lyrics and queue
  inside it). The mini bar shows artwork, title, play/pause, and next, opens
  the window on tap, and scrubs when dragged sideways. Layout-dependent spacing
  uses `--player-reserve` and `--player-clearance`, never `--playerbar-h`.
- **Keyboard.** Space plays or pauses, ←/→ seek 5 seconds, Shift+←/→ change
  track, and M mutes.
- **Continuity.** The next track preloads shortly before the end; Media Session
  exposes artwork and transport controls; plays are scrobbled after half the
  track or 4 minutes; playback errors show a toast and skip ahead.

## PWA

The manifest names the app "Rainy", uses `standalone` display, and provides
192 px, 512 px, maskable, and 180 px Apple touch icons, all drawn from the
mascot icon art in `web/public/icon-source*.png` (`pnpm generate-pwa-assets`). iOS meta tags enable
full-screen, black-translucent status bar, and `viewport-fit=cover` layouts.
An "update available" toast offers new versions instead of reloading silently.

## Related Docs

- [Frontend architecture](../architecture/frontend.md)
- [Testing: browser checks](testing.md#browser-checks)
- [Playback guide](../user/playback.md)
