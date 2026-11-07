# Unreleased

Changes through v0.1.3 are summarized in [v0.1.3](v0.1.3.md).

<details>
<summary><strong>Playback And The Web Player</strong></summary>

- In immersive mode, the next song's colours fade in across the whole window
  in a slower, softer cross-fade instead of spreading out from a tap, click,
  or the artwork. Taps and clicks still work while the colours fade.
- On tablets and desktops the bottom bar is now a pill floating over the
  content, laid out like the Apple Music web player. Click the artwork for
  the full-screen player or the title for the floating window; click the
  progress line to jump to that point, and hover it to preview the time. The
  bar still hides and pins as before. The floating mini bar is gone: the
  layout menu offers full screen, floating window, and bottom bar. The corner
  mascot sits on the bar near its right end.

</details>

<details>
<summary><strong>Library Management</strong></summary>

- Link and online music downloads can wait in the new **Tracks → Review** tab
  instead of going straight into the library. Correct their tags, folder, and
  organizing there, see possible duplicates (an existing file, an earlier
  download from the same source, similar tracks), then import or discard them.
  **Review downloads before importing** in Admin → Settings sets the default;
  the Links and Online tabs can change it per session.
- At startup Rainy removes staged uploads and online download work
  directories that a crash left in `<data>/tmp`.

</details>
