# Clients

Rainy speaks the Subsonic API with OpenSubsonic extensions, so dozens of
existing music apps can play from it. On phones, Rainy's own installed web app
is often the best choice (see [Playback](playback.md)), and management features
are available only in the web app.

## Connect an App

Add a server in the app with:

| Field | Value |
| --- | --- |
| Server address | `http://<nas-ip>:7650` or `https://music.example.com`. Do **not** add `/rest` |
| Username | Your Rainy username |
| Password | Your Rainy password, or leave it empty when using an API key |

The **Subsonic apps** section of **Settings → Apps** in the web app shows the
server address and your username to copy, and suggests apps for your platform.

Rainy implements Subsonic API 1.16.1. Most apps use token authentication (`t`
and `s` parameters), which Rainy supports. Older apps that offer "plain
password" or "legacy authentication" (including `enc:` hex-encoded passwords)
work too.

## API Keys

Apps that support the OpenSubsonic `apiKeyAuthentication` extension can sign in
with an API key instead of your password:

1. Open **Settings → Apps** and choose **Create API key**.
2. Copy the key immediately; it is shown only once.
3. Paste it into the app.

You can regenerate or revoke the key at any time; apps using the old key stop
working until you enter the new one. Rainy stores only a hash of the key. Each
user has one API key.

## Transcoding

When an app requests a lower bitrate or a specific format, Rainy converts the
stream with ffmpeg. The server's defaults are **Transcoding format** (MP3) and
**Transcoding bitrate** (192 kbps) in **Admin → Settings**.

- If the app requests MP3, Opus, or AAC, that format is used, at the app's
  bitrate limit or else the default bitrate. A file already in that format and
  within the limit is sent as it is.
- If the app only limits the bitrate, lossless files and files above the limit
  are converted to the default format at that limit; other files are sent as
  they are.
- Formats most apps cannot decode (APE, WMA, DSF/DFF, WavPack, Musepack, and
  TTA) are converted to the default format unless the app explicitly asks for
  the original (`format=raw`).
- Opus streams are capped at 256 kbps, MP3 and AAC at 320 kbps. Without ffmpeg,
  the original file is sent.
- Original files support seeking through HTTP range requests.

## What Syncs

- Favorites (stars), ratings, play counts, and playlists are shared between the
  web app and every Subsonic app, per user.
- The play queue syncs between the web app and apps that support
  `savePlayQueue`/`getPlayQueue` (or their OpenSubsonic index-based versions),
  so you can continue on another device.
- Apps that support the OpenSubsonic `songLyrics` extension show synced
  lyrics. The supported extensions are listed in
  [Subsonic API architecture](../architecture/subsonic.md).
- Apps that download the whole library for offline browsing (such as
  Symfonium) are supported.

## Folder View

Rainy has no raw folder view for Subsonic apps. An app's "folders" or "files"
mode shows album artists, then their albums, then songs, based on tags. See
[ADR-0002](../decisions/ADR-0002-simulated-folder-browsing.md).

## Suggested Apps

| Platform | Apps |
| --- | --- |
| iOS and iPadOS | [Amperfy](https://github.com/BLeeEZ/amperfy) (free, open source, CarPlay), play:Sub, substreamer, Narjo |
| Android | [Symfonium](https://symfonium.app/) (paid, Android Auto), [Tempo](https://github.com/CappielloAntonio/tempo), Ultrasonic, DSub |
| Windows, macOS, Linux | [Feishin](https://github.com/jeffvli/feishin), [Supersonic](https://github.com/dweymouth/supersonic) |
| Browser | Rainy's own web app |

Rainy is not affiliated with any of these projects.

## Troubleshooting

- Check `http` versus `https`, and remove `/rest` from the address.
- If the password contains unusual characters, try an API key.
- After 20 failed authentications within 10 minutes, the client address or
  username is locked for 5 minutes. Fix the credentials and wait.
- Outside your home network, use HTTPS: Subsonic apps send credentials on every
  request. See [Deployment security](../operations/security.md#subsonic-credentials).

## Related Docs

- [Subsonic API architecture](../architecture/subsonic.md)
- [Playback](playback.md)
- [Reverse proxy](../operations/reverse-proxy.md)
