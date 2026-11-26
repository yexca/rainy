# User Guide

These pages describe how to use Rainy once it is running. If a change affects
what users see, which actions are available, or how something is named in the
app, update these pages in the same change.

- [Getting started](getting-started.md): first sign-in, the first scan, users,
  and libraries.
- [Library](library.md): folder layout, tags, scanning, ignored folders, and
  garbled-tag repair.
- [Playback](playback.md): the web player, the PWA, lyrics, the queue, and
  playback settings.
- [Listening report, mixes, and scrobbling](listening.md): your listening
  statistics and monthly recaps, play history, the daily and weekly mix, the
  home page's song picks, and sending plays to Last.fm or ListenBrainz.
- [Management](management.md): tag editing, covers, lyrics, rename and
  organize, upload, trash, the doctor, and history.
- [Clients](clients.md): Subsonic and OpenSubsonic apps and API keys.

The web app is available in English and Simplified Chinese. Change the language
in the **Appearance** panel (the palette button beside your account) or under
**Settings → Appearance**.
Both places also pick the theme (light, dark, or system), one of four styles
(Anthropic; OpenAI; Apple, the default; Google MD), and the accent colour; the
**Original** accent follows the style. A style also sets the layout: on tablets
and desktops Apple and OpenAI have no bar across the top and keep your account
and the appearance button in the bottom-left corner of the sidebar, OpenAI and
Google MD show Search as a field in the sidebar or header, and Google MD labels
the icons of the collapsed sidebar. The **Immersive** switch next to the accent
colour, on by default in Apple, colours the whole app — sidebar, player, and
menus — with the album, playlist, or artist on screen, or with the song playing
on other pages; the cover's colours glow softly behind the sidebar, the header,
and the page, and those pages blur their artwork behind the header. Opening
another album, playlist, or artist keeps the current colours while it loads,
then shifts them smoothly to its cover; you can keep scrolling meanwhile. With
Immersive off every page keeps the style's own look. While it is on, the first
accent swatch takes the highlight from the cover too, and the other swatches
keep a highlight colour of your choice.

**Search** (the search field or button, or Ctrl+K / ⌘K on a Mac) finds songs,
albums, artists, and playlists right away, controls playback, or jumps to a
page.

Rainy's mascot appears on empty, error, and sign-in pages and keeps you company
in the bottom-right corner on tablets and desktops. The **Mascot** section of
**Settings → Appearance** turns the **Illustrations** and the **Corner
companion** off separately.

## Roles

| Role | Can do |
| --- | --- |
| Listener | Browse, search, play, star, rate, and manage their own playlists, queue, profile, password, and API key |
| Downloads | Additionally download original files and album zips, while **Allow downloads** is on in **Admin → Settings** |
| Library manager | Additionally use **Tracks** and **Library tools** under **Manage** |
| Administrator | Additionally use **Admin** (users, libraries and scans, server settings) and add radio stations on the **Radio** page |

Each user has their own favorites, ratings, play counts, listening history and
report, scrobbling accounts, playlists, and play queue. Playlists can be made public so other users can see and play them.

An administrator can limit a listener to some libraries. Such a user sees only
those libraries everywhere, in the web app and in Subsonic apps: songs, albums,
artists, genres, search, recommendations, and the songs of shared playlists
from other libraries do not appear for them. Administrators and library
managers always see every library.

## Related Docs

- [Overview](../overview.md)
- [Docker](../operations/docker.md)
- [Troubleshooting](../operations/troubleshooting.md)
