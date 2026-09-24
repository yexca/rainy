import type { SVGProps } from 'react'

/**
 * Filled transport glyphs (rounded, SF-Symbols-like) — crisper and more "native" than stroked
 * icons filled with currentColor.
 */
type GlyphProps = SVGProps<SVGSVGElement>

function Glyph({ children, ...props }: GlyphProps) {
  return (
    <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden focusable="false" {...props}>
      {children}
    </svg>
  )
}

export function PlayGlyph(props: GlyphProps) {
  return (
    <Glyph {...props}>
      <path d="M7.2 4.9v14.2c0 1 1.1 1.6 1.9 1.1l11.2-7.1c.8-.5.8-1.7 0-2.2L9.1 3.8c-.8-.5-1.9.1-1.9 1.1Z" />
    </Glyph>
  )
}

export function PauseGlyph(props: GlyphProps) {
  return (
    <Glyph {...props}>
      <rect x="5.5" y="4" width="4.6" height="16" rx="1.3" />
      <rect x="13.9" y="4" width="4.6" height="16" rx="1.3" />
    </Glyph>
  )
}

const FORWARD =
  'M1.8 6.6v10.8c0 .9 1 1.4 1.7.9l7.6-5.4c.6-.4.6-1.4 0-1.8L3.5 5.7c-.7-.5-1.7 0-1.7.9Zm10.6 0v10.8c0 .9 1 1.4 1.7.9l7.6-5.4c.6-.4.6-1.4 0-1.8l-7.6-5.4c-.7-.5-1.7 0-1.7.9Z'

export function ForwardGlyph(props: GlyphProps) {
  return (
    <Glyph {...props}>
      <path d={FORWARD} />
    </Glyph>
  )
}

export function BackwardGlyph(props: GlyphProps) {
  return (
    <Glyph {...props}>
      <path d={FORWARD} transform="matrix(-1 0 0 1 24 0)" />
    </Glyph>
  )
}
