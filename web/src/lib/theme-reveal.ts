/**
 * Immersive mode's change from one cover's colours to the next (docs/architecture/contract.md
 * §9.3): a View Transition that cross-fades the whole window from the old colours to the new
 * ones (`<html data-theme-reveal>`, timed in index.css). Without View Transitions, in a
 * background tab or with reduced motion the colours change at once.
 */

let running: ViewTransition | null = null

/**
 * Run `update` (which repaints the colours) inside a cross-fade. Returns false, without running
 * it, when the colours should change at once instead.
 */
export function revealColors(update: () => void): boolean {
  if (
    typeof document.startViewTransition !== 'function' ||
    document.visibilityState !== 'visible' ||
    window.matchMedia('(prefers-reduced-motion: reduce)').matches
  ) {
    return false
  }
  const root = document.documentElement
  root.dataset.themeReveal = ''
  const transition = document.startViewTransition(update)
  running = transition
  transition.finished
    .catch(() => {})
    .finally(() => {
      if (running !== transition) return
      running = null
      delete root.dataset.themeReveal
    })
  return true
}

/** Finish a running cross-fade at once (a change that should not animate is about to be applied). */
export function skipReveal(): void {
  running?.skipTransition()
}
