/**
 * Immersive mode's change from one cover's colours to the next (docs/architecture/contract.md
 * §9.3): a View Transition in which the new colours spread out in a circle from the last press
 * (the next button, a song row, an album card) or, with no press just before it (a song that
 * ended), from the playing song's artwork (`data-reveal-origin`, the largest one on screen).
 * With neither they cross-fade. Without View Transitions, in a background tab or with reduced
 * motion the colours change at once.
 */

/** How long a press stays the origin: the next cover may still be sampled after the click. */
const PRESS_MS = 2500
const REVEAL_MS = 650

type Point = { x: number; y: number }

/** The centre of the largest playing artwork in the viewport, or null. */
function artworkOrigin(): Point | null {
  let best: DOMRect | null = null
  for (const element of document.querySelectorAll('[data-reveal-origin]')) {
    const rect = element.getBoundingClientRect()
    const onScreen = rect.width > 0 && rect.bottom > 0 && rect.right > 0 && rect.top < window.innerHeight && rect.left < window.innerWidth
    if (onScreen && element.checkVisibility({ opacityProperty: true, visibilityProperty: true }) && (!best || rect.width * rect.height > best.width * best.height)) {
      best = rect
    }
  }
  return best && { x: best.left + best.width / 2, y: best.top + best.height / 2 }
}

let press: { x: number; y: number; at: number } | null = null
let listening = false
let running: ViewTransition | null = null

/** Start remembering where the pointer last went down (once; applyTheme calls it). */
export function trackPresses(): void {
  if (listening) return
  listening = true
  window.addEventListener(
    'pointerdown',
    (event) => {
      press = { x: event.clientX, y: event.clientY, at: performance.now() }
    },
    { capture: true, passive: true },
  )
}

/**
 * Run `update` (which repaints the colours) inside a reveal. Returns false, without running it,
 * when the colours should change at once instead.
 */
export function revealColors(update: () => void): boolean {
  if (
    typeof document.startViewTransition !== 'function' ||
    document.visibilityState !== 'visible' ||
    window.matchMedia('(prefers-reduced-motion: reduce)').matches
  ) {
    return false
  }
  const origin: Point | null = press && performance.now() - press.at < PRESS_MS ? press : artworkOrigin()
  press = null
  const root = document.documentElement
  // index.css: `circle` turns off the default cross-fade, `fade` slows it down.
  root.dataset.themeReveal = origin ? 'circle' : 'fade'
  const transition = document.startViewTransition(update)
  running = transition
  if (origin) {
    transition.ready
      .then(() => {
        const radius = Math.hypot(Math.max(origin.x, window.innerWidth - origin.x), Math.max(origin.y, window.innerHeight - origin.y))
        root.animate(
          { clipPath: [`circle(0px at ${origin.x}px ${origin.y}px)`, `circle(${radius}px at ${origin.x}px ${origin.y}px)`] },
          { duration: REVEAL_MS, easing: 'cubic-bezier(0.4, 0, 0.2, 1)', pseudoElement: '::view-transition-new(root)' },
        )
      })
      .catch(() => {
        // skipped (another change started first): the colours are already in place
      })
  }
  transition.finished
    .catch(() => {})
    .finally(() => {
      if (running !== transition) return
      running = null
      delete root.dataset.themeReveal
    })
  return true
}

/** Finish a running reveal at once (a change that should not animate is about to be applied). */
export function skipReveal(): void {
  running?.skipTransition()
}
