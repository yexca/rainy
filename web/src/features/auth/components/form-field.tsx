import type { ReactNode } from 'react'

import { Label } from '@/components/ui/label'

export interface FormFieldProps {
  id: string
  label: string
  /** Validation message; also wires `aria-describedby` via `${id}-error`. */
  error?: string
  children: ReactNode
}

/** Label + control + inline validation message. */
export function FormField({ id, label, error, children }: FormFieldProps) {
  return (
    <div className="grid gap-2">
      <Label htmlFor={id} className="text-[13px] font-medium text-foreground/80">
        {label}
      </Label>
      {children}
      {error ? (
        <p id={`${id}-error`} role="alert" className="text-[13px] text-destructive">
          {error}
        </p>
      ) : null}
    </div>
  )
}
