import type { ReactNode } from 'react'

import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Drawer, DrawerContent, DrawerDescription, DrawerHeader, DrawerTitle } from '@/components/ui/drawer'
import { useIsMobile } from '@/hooks/use-media-query'
import { cn } from '@/lib/utils'

const WIDTHS = {
  sm: 'sm:max-w-md',
  md: 'sm:max-w-lg',
  lg: 'sm:max-w-2xl',
  xl: 'sm:max-w-4xl',
} as const

export interface ResponsiveDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: ReactNode
  description?: ReactNode
  /** Scrollable body. */
  children: ReactNode
  /** Sticky footer (buttons). */
  footer?: ReactNode
  size?: keyof typeof WIDTHS
  /** Keep the dialog open on outside clicks / swipes (e.g. while a request runs). */
  dismissible?: boolean
  /** Always use the centred dialog (e.g. when opened from inside another drawer). */
  dialogOnly?: boolean
  className?: string
  bodyClassName?: string
}

/**
 * Centred dialog on tablets/desktop, bottom sheet (vaul Drawer) on phones. The body scrolls;
 * header and footer stay put.
 */
export function ResponsiveDialog({
  open,
  onOpenChange,
  title,
  description,
  children,
  footer,
  size = 'md',
  dismissible = true,
  dialogOnly,
  className,
  bodyClassName,
}: ResponsiveDialogProps) {
  const isMobile = useIsMobile()

  if (isMobile && !dialogOnly) {
    return (
      <Drawer open={open} onOpenChange={onOpenChange} dismissible={dismissible} repositionInputs={false}>
        <DrawerContent
          className={cn(
            'data-[vaul-drawer-direction=bottom]:max-h-[calc(100dvh-var(--safe-top)-1rem)]',
            className,
          )}
        >
          <DrawerHeader className="text-left">
            <DrawerTitle className="text-[17px]">{title}</DrawerTitle>
            {description ? <DrawerDescription>{description}</DrawerDescription> : null}
          </DrawerHeader>
          <div className={cn('min-h-0 flex-1 overflow-y-auto overscroll-contain px-4 pb-2', bodyClassName)}>{children}</div>
          {footer ? (
            <div className="hairline-t flex flex-col-reverse gap-2 px-4 pt-3 pb-[calc(var(--safe-bottom)+0.75rem)] [&_button]:h-11">
              {footer}
            </div>
          ) : (
            <div className="pb-safe" />
          )}
        </DrawerContent>
      </Drawer>
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className={cn('flex max-h-[min(85dvh,56rem)] flex-col gap-0 p-0', WIDTHS[size], className)}
        onInteractOutside={dismissible ? undefined : (e) => e.preventDefault()}
        onEscapeKeyDown={dismissible ? undefined : (e) => e.preventDefault()}
      >
        <DialogHeader className="px-6 pt-6 pb-4">
          <DialogTitle>{title}</DialogTitle>
          {description ? <DialogDescription>{description}</DialogDescription> : null}
        </DialogHeader>
        <div className={cn('min-h-0 flex-1 overflow-y-auto px-6 pb-4', bodyClassName)}>{children}</div>
        {footer ? <div className="hairline-t flex justify-end gap-2 px-6 py-4">{footer}</div> : null}
      </DialogContent>
    </Dialog>
  )
}
