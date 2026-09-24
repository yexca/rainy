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

**Settings → Subsonic apps** in the web app shows the server address and your
username to copy.

Most apps use token authentication (`t` and `s` parameters), which Rainy
supports. Older apps that offer "plain password" or "legacy authentication"
work too.

## API Keys

Apps that support the OpenSubsonic `apiKeyAuthentication` extension can sign in
with an API key instead of your password:

1. Open **Settings → Subsonic apps** and choose **Create API key**.
2. Copy the key immediately; it is shown only once.
3. Paste it into the app.

You can regenerate or revoke the key at any time; apps using the old key stop
working until you enter the new one. Rainy stores only a hash of the key. Each
user has one API key.

## Transcoding

When an app requests a lower bitrate or a specific format, Rainy converts the
stream with ffmpeg:

- If the app requests MP3, Opus, or AAC, that format is used.
- If the app only limits the bitrate, Rainy uses the server's default format
  (MP3 unless changed in **Admin → Settings**), at 192 kbps unless a bitrate is
  given.
- Formats most apps cannot decode (APE, WMA, DSF/DFF, WavPack, Musepack, and
  TTA) are converted unless the app explicitly asks for the original.
- Original files support seeking through HTTP range requests.

## What Syncs

- Favorites (stars), ratings, play counts, and playlists are shared between the
  web app and every Subsonic app, per user.
- The play queue syncs between the web app and apps that support
  `savePlayQueue`/`getPlayQueue`, so you can continue on another device.
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
