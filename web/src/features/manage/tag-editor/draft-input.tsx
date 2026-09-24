import { useState, type ComponentProps } from 'react'

import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'

type Common = {
  value: string
  /** Called on every keystroke with the raw text. */
  onValueChange: (value: string) => void
}

/**
 * Text input that keeps the user's raw text while focused. Fields normalise what they store
 * (trimming, splitting on `;`, digits only), so a plain controlled input would eat trailing
 * spaces mid-typing; the draft shows exactly what was typed until blur.
 */
export function DraftInput({ value, onValueChange, onFocus, onBlur, ...props }: Common & Omit<ComponentProps<typeof Input>, 'value' | 'onChange'>) {
  const [draft, setDraft] = useState<string | null>(null)
  return (
    <Input
      {...props}
      value={draft ?? value}
      onFocus={(e) => {
        setDraft(value)
        onFocus?.(e)
      }}
      onBlur={(e) => {
        setDraft(null)
        onBlur?.(e)
      }}
      onChange={(e) => {
        setDraft(e.target.value)
        onValueChange(e.target.value)
      }}
    />
  )
}

export function DraftTextarea({
  value,
  onValueChange,
  onFocus,
  onBlur,
  ...props
}: Common & Omit<ComponentProps<typeof Textarea>, 'value' | 'onChange'>) {
  const [draft, setDraft] = useState<string | null>(null)
  return (
    <Textarea
      {...props}
      value={draft ?? value}
      onFocus={(e) => {
        setDraft(value)
        onFocus?.(e)
      }}
      onBlur={(e) => {
        setDraft(null)
        onBlur?.(e)
      }}
      onChange={(e) => {
        setDraft(e.target.value)
        onValueChange(e.target.value)
      }}
    />
  )
}
