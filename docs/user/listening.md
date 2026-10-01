# Listening Report and Scrobbling

Rainy keeps a history of what each user plays and turns it into a listening
report, much like Last.fm. It can also send your plays to Last.fm or
ListenBrainz ("scrobbling").

## What Counts as a Play

A song counts as played once you have listened to half of it or 4 minutes of
it, whichever comes first. Plays from the web app and from Subsonic apps that
report plays to the server (most do) are counted alike, and each play records
which player it came from.

Your report and history are private: other users, including administrators,
cannot see them in the app.

## The Report

Open **Listening** in the sidebar (on phones: **Library → Listening**). Choose
a period in the top-right corner: the last 7, 30, or 90 days, the last 12
months, all time, or a calendar year. Days and hours follow your device's time
zone.

The **Overview** tab shows:

- **Totals**: plays and listening time (with the change against the period
  just before), how many artists, songs, and albums you played and how many of
  them were new to you, the days with music and your longest streak, and your
  plays per day.
- **Over time**: plays or listening hours per hour, day, week, or month,
  depending on the length of the period. Hover or touch a bar to see its value;
  with the keyboard, focus the chart and use the arrow keys.
- **Top artists, albums, and songs**. Tap a song to play the top songs from
  there, or open the "…" menu for more.
- **When you listen**: a weekday-by-hour grid; darker squares mean more plays.
- **Top genres** and **Players** (web, Symfonium, and so on).

The **History** tab lists every play of the period, newest first and grouped
by day.

Songs deleted from the library stay in your history and report under their
old names, dimmed and without links. Listening time is the total length of the
songs you played.

## Scrobbling to Last.fm and ListenBrainz

An administrator first turns the service on in **Admin →
Scrobbling** (see [Configuration](../operations/configuration.md#server-settings)).
That only allows the service: every user, administrators included, then
connects their own account in **Settings → Scrobbling**, and plays go to that
user's account:

- **Last.fm**: select **Connect**, sign in on Last.fm, and allow access. Last.fm
  sends you back to Rainy, which finishes the connection. Starring or unstarring
  a song in Rainy or a Subsonic app also loves or unloves it on Last.fm.
- **ListenBrainz**: copy the user token from your ListenBrainz settings page,
  paste it, and select **Connect**.

From then on, Rainy sends each play and what is playing now. If the service
is unreachable, plays wait on the server and are sent later; the settings show
how many are waiting and the last error. Services refuse plays older than 14
days, so plays that waited that long are dropped. Only plays made while an
account is connected are sent.

The switch next to an account pauses scrobbling (plays made while paused are
not sent later). **Disconnect** forgets the account and its waiting plays; to
revoke Rainy's access completely, also remove it in your Last.fm or
ListenBrainz account settings. When a service stops accepting the connection,
the settings ask you to connect again, and waiting plays are sent afterwards.

If a Subsonic app scrobbles to Last.fm on its own, turn that off in the app,
or the service receives each play twice.

## Related Docs

- [Playback](playback.md)
- [Clients](clients.md)
- [Privacy](../../PRIVACY.md#scrobbling-to-lastfm-and-listenbrainz)
