import { MoreHorizontal } from 'lucide-react'
import { Fragment, useCallback, useEffect, useMemo, useRef, useState, type ReactElement, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { ContextMenuItem, ContextMenuSeparator } from '@/components/ui/context-menu'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Drawer, DrawerContent, DrawerDescription, DrawerTitle, DrawerTrigger } from '@/components/ui/drawer'
import { useIsMobile } from '@/hooks/use-media-query'
import { cn } from '@/lib/utils'

import {
  ActionMenuContext,
  useActionMenu,
  type ActionGroups,
  type ActionItem,
  type ActionMenuContextValue,
} from '../lib/actions'

/** Delay before running a deferred action: long enough for the sheet's exit animation. */
const SHEET_DEFER_MS = 280

function run(item: ActionItem, kind: ActionMenuContextValue['kind']): void {
  if (!item.onSelect) return
  if (item.deferred) {
    const fn = item.onSelect
    window.setTimeout(fn, kind === 'sheet' ? SHEET_DEFER_MS : 0)
    return
  }
  item.onSelect()
}

/**
 * Renders action groups for the surrounding menu kind (see {@link ActionMenu} and
 * `ActionMenuContext`): dropdown / context-menu items, or iOS-style action sheet rows.
 */
export function ActionList({ groups }: { groups: ActionGroups }) {
  const { kind, close } = useActionMenu()
  const visible = groups.filter((group) => group.length > 0)

  if (kind === 'sheet') {
    return (
      <div className="flex flex-col gap-3 px-3">
        {visible.map((group, gi) => (
          <ul key={gi} className="overflow-hidden rounded-2xl bg-muted/70 dark:bg-muted/50">
            {group.map((item) => (
              <li key={item.key} className="hairline-inset [--hairline-inset:3.25rem]">
                <SheetRow item={item} onDone={close} />
              </li>
            ))}
          </ul>
        ))}
      </div>
    )
  }

  if (kind === 'context') {
    return visible.map((group, gi) => (
      <Fragment key={gi}>
        {gi > 0 ? <ContextMenuSeparator /> : null}
        {group.map((item) =>
          item.href ? (
            <ContextMenuItem key={item.key} asChild disabled={item.disabled}>
              <a href={item.href} download={item.download ? '' : undefined}>
                <item.icon strokeWidth={1.75} />
                {item.label}
              </a>
            </ContextMenuItem>
          ) : (
            <ContextMenuItem
              key={item.key}
              disabled={item.disabled}
              variant={item.destructive ? 'destructive' : 'default'}
              onSelect={() => run(item, 'context')}
            >
              <item.icon strokeWidth={1.75} />
              {item.label}
            </ContextMenuItem>
          ),
        )}
      </Fragment>
    ))
  }

  return visible.map((group, gi) => (
    <Fragment key={gi}>
      {gi > 0 ? <DropdownMenuSeparator /> : null}
      {group.map((item) =>
        item.href ? (
          <DropdownMenuItem key={item.key} asChild disabled={item.disabled}>
            <a href={item.href} download={item.download ? '' : undefined}>
              <item.icon strokeWidth={1.75} />
              {item.label}
            </a>
          </DropdownMenuItem>
        ) : (
          <DropdownMenuItem
            key={item.key}
            disabled={item.disabled}
            variant={item.destructive ? 'destructive' : 'default'}
            onSelect={() => run(item, 'dropdown')}
          >
            <item.icon strokeWidth={1.75} />
            {item.label}
          </DropdownMenuItem>
        ),
      )}
    </Fragment>
  ))
}

const SHEET_ROW =
  'flex h-[52px] w-full items-center gap-4 px-4 text-left text-[17px] transition-colors outline-none active:bg-foreground/10 focus-visible:bg-foreground/10 disabled:opacity-50'

function SheetRow({ item, onDone }: { item: ActionItem; onDone: () => void }) {
  const content = (
    <>
      <item.icon
        className={cn('size-5 shrink-0', item.destructive ? 'text-destructive' : 'text-primary')}
        strokeWidth={1.75}
        aria-hidden
      />
      <span className={cn('min-w-0 flex-1 truncate', item.destructive && 'text-destructive')}>{item.label}</span>
    </>
  )
  if (item.href) {
    return (
      <a
        href={item.href}
        download={item.download ? '' : undefined}
        className={SHEET_ROW}
        onClick={onDone}
        aria-disabled={item.disabled}
      >
        {content}
      </a>
    )
  }
  return (
    <button
      type="button"
      className={SHEET_ROW}
      disabled={item.disabled}
      onClick={() => {
        onDone()
        run(item, 'sheet')
      }}
    >
      {content}
    </button>
  )
}

export interface ActionMenuProps {
  /** Accessible name of the trigger and the sheet. */
  label: string
  /** Menu body, mounted only while open. Render {@link ActionList} inside. */
  children: ReactNode
  /** Mobile sheet header (artwork + title). */
  sheetHeader?: ReactNode
  /** Custom trigger (rendered `asChild`); defaults to a "…" icon button. */
  trigger?: ReactElement
  align?: 'start' | 'center' | 'end'
  className?: string
  /** Use the sheet on mobile (default) or always the dropdown. */
  responsive?: boolean
  onOpenChange?: (open: boolean) => void
}

/** "…" menu: DropdownMenu on desktop, bottom action sheet (vaul Drawer) on phones. */
export function ActionMenu({
  label,
  children,
  sheetHeader,
  trigger,
  align = 'end',
  className,
  responsive = true,
  onOpenChange,
}: ActionMenuProps) {
  const { t } = useTranslation()
  const isMobile = useIsMobile()
  const [open, setOpen] = useState(false)
  // Latest callback without re-creating the context value on every render.
  const onOpenChangeRef = useRef(onOpenChange)
  useEffect(() => {
    onOpenChangeRef.current = onOpenChange
  })
  const setOpenState = useCallback((next: boolean) => {
    setOpen(next)
    onOpenChangeRef.current?.(next)
  }, [])
  const sheet = responsive && isMobile
  const context = useMemo<ActionMenuContextValue>(
    () => ({ kind: sheet ? 'sheet' : 'dropdown', close: () => setOpenState(false) }),
    [sheet, setOpenState],
  )

  const button = trigger ?? (
    <Button
      variant="ghost"
      size="icon"
      aria-label={label}
      title={sheet ? undefined : label}
      className={cn('size-11 rounded-full text-muted-foreground md:size-8', className)}
      onClick={(event) => event.stopPropagation()}
      onDoubleClick={(event) => event.stopPropagation()}
    >
      <MoreHorizontal className="size-5" strokeWidth={1.75} />
    </Button>
  )

  if (sheet) {
    return (
      <Drawer open={open} onOpenChange={setOpenState}>
        <DrawerTrigger asChild>{button}</DrawerTrigger>
        <DrawerContent className="ui-chrome max-h-[88dvh] rounded-t-2xl border-none bg-background pb-[calc(var(--safe-bottom)+0.75rem)]">
          <DrawerTitle className="sr-only">{label}</DrawerTitle>
          <DrawerDescription className="sr-only">{label}</DrawerDescription>
          <div className="min-h-0 overflow-y-auto overscroll-contain pt-2">
            {sheetHeader ? <div className="px-5 pt-2 pb-4">{sheetHeader}</div> : null}
            <ActionMenuContext.Provider value={context}>{children}</ActionMenuContext.Provider>
            <div className="px-3 pt-3">
              <button
                type="button"
                onClick={() => setOpenState(false)}
                className="h-[52px] w-full rounded-2xl bg-muted/70 text-[17px] font-semibold text-primary transition-colors active:bg-foreground/10 dark:bg-muted/50"
              >
                {t('common:actions.cancel')}
              </button>
            </div>
          </div>
        </DrawerContent>
      </Drawer>
    )
  }

  return (
    <DropdownMenu open={open} onOpenChange={setOpenState}>
      <DropdownMenuTrigger asChild>{button}</DropdownMenuTrigger>
      <DropdownMenuContent align={align} className="min-w-56 rounded-xl" onClick={(event) => event.stopPropagation()}>
        <ActionMenuContext.Provider value={context}>{children}</ActionMenuContext.Provider>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

/** Artwork + two lines of text for the top of an action sheet. */
export function ActionSheetHeader({ art, title, subtitle }: { art?: ReactNode; title: string; subtitle?: string }) {
  return (
    <div className="flex items-center gap-3">
      {art}
      <div className="min-w-0 flex-1">
        <p className="truncate text-[17px] font-semibold">{title}</p>
        {subtitle ? <p className="truncate text-[15px] text-muted-foreground">{subtitle}</p> : null}
      </div>
    </div>
  )
}
