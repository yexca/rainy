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
- **Artwork leads.** Covers are square, rounded, lazy-loaded, and fade in.
  Missing art shows the mascot (see [Mascot](#mascot)), or a gradient
  placeholder with a glyph when illustrations are off. Artists are circles.
- **Accent sparingly.** The accent color marks play buttons, active
  navigation, and progress, not decoration.
- **Every string is translated.** Use i18next keys with both `en` and `zh`
  text; never hard-code visible copy.
- **Accessible by default.** Every icon button has an `aria-label`, focus rings
  stay visible, and color contrast meets WCAG AA.

## Theme

- shadcn/ui `new-york` components themed by CSS variables.
- Light and dark themes follow the system unless the user overrides them.
- **Styles.** Four presets set the surfaces, borders, radius, fonts, and their
  own accent for both themes: `anthropic` (default: warm paper tones, a
  terracotta accent, serif page titles), `openai` (neutral greys, a green
  accent, Inter), `apple` (cool greys, a blue accent, the system font, larger
  radius), and `google-md` (blue-tinted surfaces, a Material blue accent,
  Roboto). Styles only bundle what Rainy already ships (Inter); other faces
  are used when the system has them. Keep each style's light and dark blocks
  complete so no token leaks from another style.
- **A style is more than colours.** Each one also picks where the app chrome
  sits (see [Layout](#layout): `apple` has no app header from tablets up) and
  whether it starts in immersive mode (only `apple` does). Entering `apple`
  turns immersive mode on and leaving it turns it off; between the other
  styles it stays as the user set it.
- Accent presets set `--primary`: `original` (default, the style's own
  accent), `rain` (periwinkle blue), `rose`, `violet`, `emerald`, `amber`, and
  `graphite`. A preset wins over every style.
- **Album colours.** Album, playlist, and artist pages take on their
  artwork's colours, like the Apple Music album view: the dominant colour becomes the page background
  (a deep tone in the dark theme or for dark covers, a pale tint otherwise)
  and the most vivid colour becomes `--primary`. The tokens are scoped to the
  page element, so the sidebar, player, and menus keep the user's theme, and
  every text colour keeps WCAG AA contrast. Greyscale covers keep the user's
  accent; pages without artwork keep the plain theme. Users can turn this
  off under Settings → Appearance.
- **Immersive mode** applies the same cover colours to the whole app —
  sidebar, header, player, menus — using the cover page on screen, or else the
  playing song. Its switch sits beside the accent label, and the swatches
  below pick the highlight: the first becomes a colour wheel, meaning the
  highlight follows the cover too (the style's own colour when there is no
  cover or it is greyscale), and the presets stay fixed highlights over the
  cover's surfaces. Without a cover the user's theme returns. It works
  whatever the album colours setting, which only governs the mode when off.
- Each style picks its Latin faces and falls back to system and CJK fonts;
  `h1` titles use the style's heading face. Times and durations use tabular
  numbers.

## Layout

| Width | Navigation | Player |
| --- | --- | --- |
| 1024 px and wider | App header, left sidebar (240 px) below it, collapsible to icons | 80 px bottom player bar |
| 768 to 1023 px | App header, collapsed icon sidebar | Bottom player bar |
| Below 768 px | Glass app header, 49 px bottom tab bar plus the safe area | Floating 56 px mini player above the tab bar |

A full-width app header runs across the top at every width. From tablets up
it uses the sidebar's colours: the sidebar toggle and the logo on the left; on
the right, a round tray of quick controls (Quick actions; appearance with theme,
style, accent and language), a divider, and the account menu (avatar, plus the
name and role on desktops). Settings and About sit in the sidebar footer. Phones
get a glass bar matching the tab bar: the logo on the left, and Quick actions,
the appearance button and the account avatar on the right (44 px targets).
Page top bars stick right below the header.

The `apple` style uses the sidebar chrome instead: from tablets up there is no
app header. The sidebar reaches the top of the window with the toggle and the
logo above its groups, and its bottom-left corner holds the account row
(avatar, name and role; the menu opens upwards) with Quick actions and the
appearance button beside it, in place of Settings and About (both are in the
account menu). On the icon rail the three stack, the avatar at the bottom.
Page top bars stick to the top of the window. Phones keep the glass header.
Switching between a header style and a sidebar style animates: the header
slides up while the account row rises in the corner, and back again. Mark
elements that take part with `data-chrome-motion`; they may transition
position and size but never colours.

Quick actions is a lightning icon, never a search field, so it is not mistaken
for library search; its tooltip names the shortcut, Ctrl+K (⌘K on Apple
devices), which opens it from anywhere. It opens a command palette — a centered
dialog from tablets up, a bottom sheet on phones — that filters playback
controls (while something is queued), library pages and playlists, the manager
and admin pages the user may open, and personal settings. Typed text also offers
a library search, ranked below the matching actions so Enter opens the best
match.

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

### Page Width

Every page uses `<Page>` with `<PageHeader>` first. The title, subtitle, and
section tabs always run full width and start at the page gutter, so they never
move when switching tabs. The body takes one of two widths:

| Body | Width | Used by |
| --- | --- | --- |
| Full width (default) | The whole content area | Collections and grids, track tables, lists, and dashboards: library pages, Tracks › Metadata, Library tools, Admin › Users and Libraries |
| Column (`<PageColumn>`) | Up to 768 px (`max-w-3xl`), 32 px between sections | Forms, settings, and single-task tools: Settings, Admin › Server settings, Tracks › Upload, Online, and Links |

A column is left-aligned under the title, never centered, so its left edge
matches the header and its full-width sibling tabs. Do not add page-level
`max-w-*` or `mx-auto` wrappers; pick one of the two widths. Full-page empty,
error, and status states stay centered, and About keeps its own two-column
reading layout.

## Typography and Density

- Page titles are large and bold. On mobile, the large title collapses into a
  centered navigation title while scrolling (`PageHeader` handles this).
- Section titles are `text-xl font-semibold`; body text is `text-sm`; secondary
  text uses `text-muted-foreground`.
- Desktop list rows are 48 px. Mobile rows are 60 px with 44 px artwork and
  iOS-style hairline separators inset from the artwork.

## Surfaces, Motion, and Icons

- Glass surfaces (tab bar, mini player, top bar on scroll) use a translucent
  background with a strong backdrop blur and a subtle border on larger screens.
  Phones, including landscape phones up to 540 px high, use opaque surfaces to
  keep native scrolling smooth. Backdrop filters never animate.
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
  the listening report), month and year recaps and the home recap card,
  errors (offline, render errors), 404, no access, the
  startup splash, the admin scan panel while a scan runs, beside the sign-in
  card on wide screens, Settings › Mascot, and as a companion in the
  bottom-right corner on tablets and desktops. Route changes keep the small
  spinner. Compact empty states in panels
  and lists keep their icon.
- **Missing artwork.** Songs and albums without a cover show one of three
  square mascot covers (picked by title, so a grid doesn't repeat one picture);
  artists without a photo, playlists without a cover, and radio stations each
  have their own. Covers that are still loading keep the plain gradient, so the
  mascot never flashes in before real art. The lock screen and notifications
  (Media Session) show the same placeholder for songs without a cover.
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

- **Mini player (mobile).** A floating opaque pill above the tab bar with 40 px
  artwork, one-line title and artist, play/pause and next, and a thin progress
  line. Tapping or swiping up opens Now Playing.
- **Now Playing.** A full-screen sheet on phones (a large overlay on desktop)
  that slides up with a spring and closes by dragging down from the grabber.
  The background uses static gradients from the artwork colors on phones;
  larger screens add one blurred cover sized to the player container, drifting
  only during playback and when reduced motion is off. Backgrounds cross-fade
  on cover changes; extracting the palette does not create another layer.
  Text stays light. The artwork shrinks with a spring when paused and grows
  back when playing.
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
  track, and M mutes. Ctrl/⌘+K opens Quick actions.
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
