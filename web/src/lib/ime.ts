/**
 * IME composition for live text fields (search, filters).
 *
 * While a Chinese, Japanese or Korean input method composes, the field holds the reading
 * (`ni'hao`, `にほん`) rather than the text the user means. Live consumers should only see the
 * committed text, so the field keeps the reading as a local draft until composition ends.
 *
 * Browsers order the events differently: Chromium and Firefox fire the last `input` before
 * `compositionend`; Safari fires `compositionend` first and then an `input` that is no longer
 * composing. Both orders commit the final text exactly once per distinct value.
 */

export interface ImeState {
  composing: boolean
  /** What the field shows while composing; `null` shows the committed value. */
  draft: string | null
}

export type ImeEvent =
  | { type: 'start'; value: string }
  | { type: 'input'; value: string; isComposing: boolean }
  | { type: 'end'; value: string }

export interface ImeStep {
  state: ImeState
  /** Text to pass to the consumer, if any. */
  commit?: string
}

export const IME_IDLE: ImeState = { composing: false, draft: null }

export function stepIme(state: ImeState, event: ImeEvent): ImeStep {
  switch (event.type) {
    case 'start':
      return { state: { composing: true, draft: event.value } }
    case 'input':
      if (state.composing || event.isComposing) return { state: { composing: state.composing, draft: event.value } }
      return { state: IME_IDLE, commit: event.value }
    case 'end':
      return { state: IME_IDLE, commit: event.value }
  }
}

/** True for a key event that belongs to an IME (confirming or cancelling a reading). */
export function isImeKeyEvent(event: { isComposing?: boolean; keyCode?: number }): boolean {
  // Safari reports the confirming Enter after `compositionend`, with only keyCode 229 left.
  return !!event.isComposing || event.keyCode === 229
}
