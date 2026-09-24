import type { LucideIcon } from 'lucide-react'
import { createContext, useContext } from 'react'

/** One entry of an action menu (rendered as dropdown, context menu or mobile action sheet). */
export interface ActionItem {
  key: string
  label: string
  icon: LucideIcon
  onSelect?: () => void
  /** Render as a link (e.g. downloads). */
  href?: string
  /** `download` attribute for `href` items. */
  download?: boolean
  destructive?: boolean
  disabled?: boolean
  /**
   * Run `onSelect` only after the menu has closed — for actions that open another dialog or
   * sheet (avoids focus / pointer-event fights between the two overlays).
   */
  deferred?: boolean
}

/** Groups are separated by dividers; empty groups are skipped. */
export type ActionGroups = ActionItem[][]

export type ActionMenuKind = 'dropdown' | 'context' | 'sheet'

export interface ActionMenuContextValue {
  kind: ActionMenuKind
  /** Close the surrounding menu (sheets must be closed explicitly). */
  close: () => void
}

export const ActionMenuContext = createContext<ActionMenuContextValue>({ kind: 'dropdown', close: () => {} })

export function useActionMenu(): ActionMenuContextValue {
  return useContext(ActionMenuContext)
}
