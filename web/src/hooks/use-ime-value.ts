import { useRef, useState, type CompositionEvent } from 'react'

import { IME_IDLE, stepIme, type ImeEvent, type ImeState } from '@/lib/ime'

/**
 * Wraps a controlled live text value so IME readings stay in the field and `onChange` only
 * receives committed text. Spread `handlers` on the `<input>` and feed its changes to `change`.
 */
export function useImeValue(value: string, onChange: (value: string) => void) {
  // The ref is authoritative: composition and input events can arrive within one render.
  const state = useRef<ImeState>(IME_IDLE)
  const [draft, setDraft] = useState<string | null>(null)

  const dispatch = (event: ImeEvent) => {
    const step = stepIme(state.current, event)
    state.current = step.state
    setDraft(step.state.draft)
    if (step.commit !== undefined && step.commit !== value) onChange(step.commit)
  }

  return {
    value: draft ?? value,
    /** True while an input method holds an uncommitted reading. */
    isComposing: () => state.current.composing,
    /** Call from the input's change handler. */
    change: (next: string, nativeEvent?: Event) =>
      dispatch({ type: 'input', value: next, isComposing: !!(nativeEvent as InputEvent | undefined)?.isComposing }),
    handlers: {
      onCompositionStart: (event: CompositionEvent<HTMLInputElement>) =>
        dispatch({ type: 'start', value: event.currentTarget.value }),
      onCompositionEnd: (event: CompositionEvent<HTMLInputElement>) =>
        dispatch({ type: 'end', value: event.currentTarget.value }),
    },
  }
}
