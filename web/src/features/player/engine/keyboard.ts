/**
 * Global keyboard shortcuts (docs/architecture/contract.md §9.4):
 * Space play/pause · ←/→ seek 5 s · Shift+←/→ previous/next · M mute.
 * Ignored while typing, while a modifier (other than Shift) is held, and when the focused
 * element handles the key itself (buttons, sliders, menus, …).
 */
import { usePlayback, usePlayer } from '../store'

const SEEK_STEP = 5

/** Elements that consume Space / arrow keys themselves. */
const INTERACTIVE_SELECTOR = [
  'input',
  'textarea',
  'select',
  'button',
  'a[href]',
  '[contenteditable=""]',
  '[contenteditable="true"]',
  '[role="slider"]',
  '[role="button"]',
  '[role="menuitem"]',
  '[role="menuitemradio"]',
  '[role="menuitemcheckbox"]',
  '[role="option"]',
  '[role="tab"]',
  '[role="switch"]',
  '[role="checkbox"]',
  '[role="radio"]',
  '[role="combobox"]',
  '[role="listbox"]',
  '[role="textbox"]',
].join(',')

function isTyping(target: EventTarget | null): boolean {
  if (!(target instanceof Element)) return false
  if (target instanceof HTMLElement && target.isContentEditable) return true
  return target.closest('input, textarea, select, [contenteditable=""], [contenteditable="true"], [role="textbox"]') !== null
}

function consumesKey(target: EventTarget | null): boolean {
  return target instanceof Element && target.closest(INTERACTIVE_SELECTOR) !== null
}

/** A modal (dialog, sheet, menu) is open: its own keyboard handling wins. */
function modalOpen(): boolean {
  return document.querySelector('[role="dialog"][data-state="open"], [role="alertdialog"], [role="menu"]') !== null
}

function onKeyDown(event: KeyboardEvent): void {
  if (event.defaultPrevented || event.ctrlKey || event.metaKey || event.altKey || event.isComposing) return
  if (isTyping(event.target)) return
  const player = usePlayer.getState()
  const hasTrack = player.index >= 0

  switch (event.key) {
    case ' ':
    case 'Spacebar': {
      if (event.repeat || consumesKey(event.target) || modalOpen() || !hasTrack) return
      event.preventDefault()
      player.togglePlay()
      return
    }
    case 'ArrowLeft':
    case 'ArrowRight': {
      if (consumesKey(event.target) || modalOpen() || !hasTrack) return
      event.preventDefault()
      const forward = event.key === 'ArrowRight'
      if (event.shiftKey) {
        if (event.repeat) return
        if (forward) player.next()
        else player.prev()
        return
      }
      const current = player.queue[player.index]
      if (current?.isRadio) return
      const playback = usePlayback.getState()
      playback.seek(playback.currentTime + (forward ? SEEK_STEP : -SEEK_STEP))
      return
    }
    case 'm':
    case 'M': {
      if (event.repeat || modalOpen()) return
      event.preventDefault()
      player.toggleMute()
      return
    }
    default:
  }
}

/** Install the shortcuts; returns the uninstaller. */
export function installKeyboardShortcuts(): () => void {
  window.addEventListener('keydown', onKeyDown)
  return () => window.removeEventListener('keydown', onKeyDown)
}
