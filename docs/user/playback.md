# Playback

Rainy's web app is a full music player on phones and desktops. It can also be
installed as an app (PWA).

## The Player

- **On phones**, a floating mini player sits above the tab bar. Tap it or swipe
  up to open **Now Playing**, and drag down to close it.
- **On desktop**, the player bar sits at the bottom of the window. Open the
  queue or lyrics in a side panel, or expand to the full Now Playing view.

Now Playing shows large artwork over a background made from the cover's colors,
a scrubber with elapsed and remaining time, transport controls, a star, and a
"…" menu. On iPhone and iPad, an AirPlay button appears when AirPlay devices are
available.

Lock screens, headsets, car displays, and media keys control playback through
the Media Session API.

### Keyboard Shortcuts (Desktop)

| Key | Action |
| --- | --- |
| Space | Play or pause |
| ← / → | Back or forward 5 seconds |
| Shift+← / Shift+→ | Previous or next song |
| M | Mute or unmute |

## Queue

- Play an album, playlist, or selection to replace the queue, or use **Play
  next** and **Add to queue** from any "…" menu.
- In **Playing Next**, drag songs to reorder them, remove them, shuffle, change
  the repeat mode, or clear the queue.
- The queue is saved on the device immediately and on the server shortly
  after, so you can reload the page, or continue in a Subsonic app that
  supports play-queue sync, where you left off.

## Lyrics

Open lyrics from Now Playing or the player bar. Synced lyrics (from `.lrc`
files or timed embedded lyrics) highlight the current line and scroll with the
music; tap a line to jump to it. Plain lyrics scroll normally. Managers can edit
lyrics; see [Management](management.md#lyrics).

## Playback Settings

These are under **Settings → Playback** and are saved on the current device
only.

| Setting | Options |
| --- | --- |
| Streaming quality | Original, or a maximum bitrate. Higher-bitrate files are converted on the server |
| Conversion format | MP3, AAC, or Opus (Opus is replaced by AAC or MP3 where the browser cannot play it) |
| Volume normalization | Off, Song, or Album, using ReplayGain tags. Not available on iPhone and iPad, where the system controls volume |
| Preload next song | Buffers the next song for near-gapless playback. Not available on iPhone and iPad |

Formats the browser cannot play, such as APE or DSD, are converted
automatically even at "Original" quality. Conversion needs ffmpeg, which the
Docker image includes.

Plays count toward play counts and "recently played" after half of a song or
4 minutes, whichever comes first.

## Internet Radio

**Radio** lists stations that an administrator added with a stream URL. Your
browser connects to the station directly, so the station sees your device's
address, not the server's.

## Install the App

Installing requires HTTPS; see [Reverse proxy](../operations/reverse-proxy.md).

- **iPhone and iPad:** open the HTTPS address in Safari, tap **Share**, then
  **Add to Home Screen**.
- **Android:** open the address in Chrome and choose **Install app** from the
  menu.
- **Desktop:** use the install icon in the Chrome or Edge address bar, or
  **Settings → App → Install**.

The installed app opens full screen, keeps covers cached for faster browsing,
and shows an "update available" message when a new version of Rainy is
deployed. Music itself is always streamed, not stored offline.

On iOS, lock-screen playback works in the installed app, but iOS may suspend a
web app that stays in the background for a long time. For long listening
sessions, a native Subsonic app can be more reliable; see [Clients](clients.md).

## Related Docs

- [Clients](clients.md)
- [Design: iOS-style player](../development/design.md#ios-style-player)
- [Troubleshooting](../operations/troubleshooting.md)
