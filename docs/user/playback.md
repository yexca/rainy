# Playback

Rainy's web app is a full music player on phones and desktops. It can also be
installed as an app (PWA).

## The Player

- **On phones**, a floating mini player sits above the tab bar. Tap it or swipe
  up to open **Now Playing**, and drag down to close it.
- **On tablets and desktops**, the **Player layout** button picks how the
  player sits on the page; the choice is saved in this browser:
  - **Bottom bar** (the default) spans the bottom of the window. Open the queue
    or lyrics in a side panel, or expand to the full-screen player. The tab
    above the bar hides it, and its pin chooses between keeping the bar open
    and hiding it until the pointer reaches the bottom edge.
  - **Floating window** is a small Now Playing window in the bottom-right
    corner.
  - **Floating mini bar** is a compact bar in the bottom-right corner; drag
    across it to seek.
  - **Full-screen player** opens Now Playing over the page and returns to the
    chosen layout when closed.

Now Playing shows large artwork over a background made from the cover's colors,
a scrubber with elapsed and remaining time, transport controls, a star, and a
"…" menu. On iPhone and iPad, an AirPlay button appears when AirPlay devices are
available.

Lock screens, headsets, car displays, and media keys control playback through
the Media Session API.

### Keyboard Shortcuts (Desktop)

| Key | Action |
| --- | --- |
| Ctrl+K (⌘K on a Mac) | Open Quick actions |
| Space | Play or pause |
| ← / → | Back or forward 5 seconds |
| Shift+← / Shift+→ | Previous or next song |
| M | Mute or unmute |

## Queue

- Play an album, playlist, or selection to replace the queue, or use **Play
  next** and **Add to queue** from any "…" menu.
- In **Playing Next**, drag songs to reorder them, remove them, shuffle, change
  the repeat mode, turn infinite mode on or off, or clear the queue.
- Long songs (10 minutes or longer by default) are left out when you play a
  list or add several songs, and a message says how many. The song you tap
  still plays, and a list of only long songs is queued as it is. Turn this
  off or change the length under **Settings → Playback**.
- The queue is saved on the device immediately and on the server shortly
  after, so you can reload the page, or continue in a Subsonic app that
  supports play-queue sync, where you left off.

### Infinite Mode

Turn on infinite mode with the ∞ button in **Playing Next** (or in **Settings →
Playback**) and the music doesn't stop when the queue runs out. When two songs
or fewer are left, Rainy adds about ten songs like the ones you are playing:
songs by the same artists, by artists sharing their genres, and from the same
genres, favoring songs you starred, rated highly, or play often, with at most
two songs per artist when the library allows it. Songs already in the queue
and songs you rated one star are left out, and songs you played in the last
day are picked only when little else fits.

Suggested songs are marked with ∞ in the queue. Songs you add with **Add to
queue** play before them, and turning infinite mode off removes the
suggestions that haven't played yet. Infinite mode waits while repeat is on,
and the choice is saved on the device.

## Auto-Stop Timer

The timer button (in Now Playing, the floating window, and the player bar on
wider windows) stops the music for you. Pick a mode at the top, then a time
from 15 minutes to 2 hours, or type your own number of minutes (up to 1440,
a full day) and choose **Start**:

- **Sleep** stops playback once, that long after you set it. The panel shows
  the time left.
- **Inactivity** stops playback when you haven't used Rainy for that long.
  Moving the pointer, tapping, typing, scrolling, or using the lock screen and
  media keys to skip or seek starts the count again. It stays on until you
  turn it off and is remembered on the device.

The button is highlighted while a timer is on; open it and choose **Turn off**
to cancel. When a timer stops the music, a message says so until you close it
or start playback again. Only what you do in Rainy counts as activity, so with
Inactivity on, music stops while you work in other apps.

## Lyrics

Now Playing opens on the lyrics; the lyrics button switches back to the artwork.
The player bar opens them in a side panel. Synced lyrics (from `.lrc`
files or timed embedded lyrics) highlight the current line and scroll with the
music; tap a line to jump to it. Plain lyrics scroll normally.

Bilingual lyrics show the translation in smaller text under each original
line. Rainy recognizes three layouts: lines that share a timestamp (original
first), lines that hold the original, a space, and the Chinese translation, and
lines that end with the translation in brackets (`待ってよ「等一下」`).
Japanese and Korean lines use matching Japanese or Korean fonts. The translation
button next to the lyrics button hides or shows translations; the choice is
remembered in this browser and is also under **Settings → Playback**. Managers can edit
lyrics; see [Management](management.md#lyrics).

### Picture-in-Picture Lyrics

The picture-in-picture button (next to the lyrics button in the player bar,
Now Playing, and the floating window) opens a small window that stays on top
of other apps and windows. It shows the song, previous / play-pause / next,
the line being sung with its translation, and the next line dimmed. Press the
button again or close the window to stop.

- In Chrome and Edge on a computer, the window is a full mini player.
- In Safari (Mac, iPhone, and iPad) the lyrics play as a small video; its
  play and pause buttons control the music. On iPhone and iPad the lyrics may
  stop moving while Safari is in the background.
- Browsers without picture-in-picture (such as Firefox) don't show the
  button. Lyrics without timing can't follow the song, so the window says so.

## Playback Settings

These are under **Settings → Playback** and are saved on the current device
only.

| Setting | Options |
| --- | --- |
| Streaming quality | Original, or at most 320, 192, or 128 kbps. Lossless files and files above the limit are converted on the server |
| Conversion format | MP3, AAC, or Opus (Opus is replaced by AAC or MP3 where the browser cannot play it) |
| Volume normalization | Off, Song, or Album, using ReplayGain tags. Not available on iPhone and iPad, where the system controls volume |
| Preload next song | Buffers the next song for near-gapless playback. Not available on iPhone and iPad |
| Skip long songs | On by default. Leaves songs of the chosen length (1–600 minutes, 10 by default) or longer out of the queue and out of infinite mode's suggestions |

Formats the browser cannot play, such as APE or DSD, are converted
automatically even at **Original** quality (at 320 kbps). Conversion needs
ffmpeg, which the Docker image includes; without it the original file is sent.

When a song counts as played is described in
[What Counts as a Play](listening.md#what-counts-as-a-play).

## Internet Radio

**Radio** lists stations that an administrator added on that page with a
stream URL (MP3, AAC, Ogg, or HLS). Your
browser connects to the station directly, so the station sees your device's
address, not the server's.

## Install the App

Installing requires HTTPS; see [Reverse proxy](../operations/reverse-proxy.md).

- **iPhone and iPad:** open the HTTPS address in Safari, tap **Share**, then
  **Add to Home Screen**.
- **Android:** open the address in Chrome and choose **Install app** from the
  menu.
- **Desktop:** use the install icon in the Chrome or Edge address bar, or
  **Install** under **Settings → Apps**.

The installed app opens full screen, opens even without a connection, keeps
covers cached for faster browsing, and offers to reload when a new version of
Rainy is deployed. Music itself is always streamed, not stored offline.

On iOS, lock-screen playback works in the installed app, but iOS may suspend a
web app that stays in the background for a long time. For long listening
sessions, a native Subsonic app can be more reliable; see [Clients](clients.md).

## Related Docs

- [Clients](clients.md)
- [Design: iOS-style player](../development/design.md#ios-style-player)
- [Troubleshooting](../operations/troubleshooting.md)
