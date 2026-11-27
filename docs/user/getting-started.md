# Getting Started

This page assumes Rainy is running. To install it, follow the
[Quick Start](../../README.md#quick-start) or the NAS guides in
[Docker](../operations/docker.md).

## 1. Create the Administrator

Open `http://<host>:7650`. On a new instance, Rainy shows a setup page instead
of the sign-in page. Choose a username and a strong password; this account
becomes the administrator. Setup is available only until the first account
exists.

## 2. Wait for the First Scan

On first start, Rainy adds the folder mounted at `/data` as the first library
and starts a quick scan. Follow its progress under **Admin → Libraries**. Albums
appear on the home page as soon as the scan finishes.

If the library stays empty, see
[Troubleshooting](../operations/troubleshooting.md#the-library-is-empty).

## 3. Explore

- **Home** shows **Made for You** (your daily and weekly mix, plus last month's
  recap in the first week of the month), **Picked for You** songs, recently
  played albums, albums added in the last two weeks, and random (**Discover**)
  albums; see [Listening](listening.md#songs-on-the-home-page).
- **Playlists** and **Library** (on phones) or the sidebar (on tablets and
  desktops) lead to playlists, albums, artists, songs, favorites, your
  listening report, and radio. Filter albums and songs by genre from their
  toolbars.
- **Search** (top right on Home, the search button in the bar, or Ctrl+K / ⌘K)
  finds songs, albums, artists, playlists, and pages; it ignores case, accents,
  and full-width characters.

Tap an album to open it, or the artist name under its cover to open the artist.
Use the "…" menu on any track to play it next, add it to the queue or a
playlist, go to its album or artist, add it to favorites, download it, and (for
managers) edit its tags.

An album's song list shows each file's own quality (such as `FLAC 24/96` or
`MP3 320 kbps`) and a lyrics column that marks songs with lyrics. Once you have
played songs from the album, a bar shows how often you played each one
compared with your most played song there. Phones show the quality under the
title and the lyrics mark beside the "…" menu, without the bar.

## 4. Add People

Under **Admin → Users**, create accounts for everyone who should listen. Grant
the roles:

- **Downloads** to let someone save original files.
- **Library manager** only to people you trust with your music files.
- **Administrator** only to people who should manage the server itself.

With more than one library you can also limit a listener to some of them: turn
off **All libraries** in their user dialog and choose the libraries they may
see. Administrators and library managers always see every library.

## 5. Add More Libraries (Optional)

A library is a music folder inside the container. To add another one, mount it
in Compose (for example `/path/to/more-music:/music2`), recreate the container,
then add `/music2` under **Admin → Libraries**. Removing a library deletes its
database entries only, never files.

## 6. Install the App and Connect Clients

- Install the web app on your phone: see
  [Playback](playback.md#install-the-app). This requires HTTPS.
- Connect Subsonic apps: see [Clients](clients.md).

## Related Docs

- [Library](library.md)
- [Configuration](../operations/configuration.md)
- [Deployment security](../operations/security.md)
