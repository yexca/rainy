/**
 * Immersive mode's change from one cover's colours to the next (docs/architecture/contract.md
 * §9.3): the colour tokens on `<html>` glide to their new values. Nothing is captured or laid
 * over the page, so it keeps scrolling, loading and taking presses while the colours change.
 *
 * The tokens an override sets are registered as inherited `<color>` properties
 * (`CSS.registerProperty`) so `<html>` can transition them; meanwhile every other element holds
 * its own transitions, so a control's `transition-colors` does not trail the tokens. Elements
 * marked `data-chrome-motion` keep theirs (they never transition colours). Without registered
 * properties, in a background tab or with reduced motion the colours change at once.
 */

/** How long the tokens take to reach the next cover's colours. */
export const MORPH_MS = 700
const EASING = 'cubic-bezier(0.4, 0, 0.2, 1)'
const HOLD_CSS = 'html *:not([data-chrome-motion]),html *::before,html *::after{transition:none!important}'

const registered = new Set<string>()
let hold: HTMLStyleElement | null = null
let timer: ReturnType<typeof setTimeout> | undefined

/**
 * Register `names` as inherited `<color>` custom properties, once each. Returns false when the
 * browser cannot register properties.
 */
export function registerColorTokens(names: Iterable<string>): boolean {
  if (typeof CSS === 'undefined' || typeof CSS.registerProperty !== 'function') return false
  for (const name of names) {
    if (registered.has(name)) continue
    registered.add(name)
    try {
      CSS.registerProperty({ name, syntax: '<color>', inherits: true, initialValue: 'transparent' })
    } catch {
      // Already registered (a hot reload): it keeps working as a colour.
    }
  }
  return true
}

/**
 * Run `paint`, which sets the next colours of the tokens `names` on `<html>`, as a glide from the
 * current ones. Returns false, without running it, when the colours should change at once.
 */
export function morphColors(names: readonly string[], paint: () => void): boolean {
  if (
    document.visibilityState !== 'visible' ||
    window.matchMedia('(prefers-reduced-motion: reduce)').matches ||
    !registerColorTokens(names)
  ) {
    return false
  }
  const root = document.documentElement
  if (!hold) {
    hold = document.createElement('style')
    hold.textContent = HOLD_CSS
    document.head.appendChild(hold)
  }
  // A glide that is still running continues from the colours it has reached.
  root.style.transition = names.map((name) => `${name} ${MORPH_MS}ms ${EASING}`).join(', ')
  paint()
  clearTimeout(timer)
  timer = setTimeout(stopMorph, MORPH_MS + 50)
  return true
}

/** End a running glide: the tokens take their current values at once. */
export function stopMorph(): void {
  clearTimeout(timer)
  timer = undefined
  document.documentElement.style.removeProperty('transition')
  hold?.remove()
  hold = null
}
