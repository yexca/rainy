import type { ReactNode } from 'react'

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Drawer, DrawerContent, DrawerDescription, DrawerFooter, DrawerHeader, DrawerTitle } from '@/components/ui/drawer'
import { useIsMobile } from '@/hooks/use-media-query'
import { cn } from '@/lib/utils'

export interface ResponsiveDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: ReactNode
  description?: ReactNode
  /** Hide the description visually (still announced). */
  hideDescription?: boolean
  children: ReactNode
  footer?: ReactNode
  className?: string
}

/** Centered dialog on desktop, bottom sheet (vaul Drawer) on phones. */
export function ResponsiveDialog({
  open,
  onOpenChange,
  title,
  description,
  hideDescription,
  children,
  footer,
  className,
}: ResponsiveDialogProps) {
  const isMobile = useIsMobile()

  if (isMobile) {
    return (
      <Drawer open={open} onOpenChange={onOpenChange} repositionInputs={false}>
        <DrawerContent className={cn('max-h-[92dvh] rounded-t-2xl border-none', className)}>
          <DrawerHeader className="pb-2 text-left">
            <DrawerTitle className="text-lg">{title}</DrawerTitle>
            <DrawerDescription className={cn(hideDescription || !description ? 'sr-only' : undefined)}>
              {description ?? title}
            </DrawerDescription>
          </DrawerHeader>
          <div className="min-h-0 flex-1 overflow-y-auto overscroll-contain px-4">{children}</div>
          {footer ? <DrawerFooter className="pb-[calc(var(--safe-bottom)+1rem)]">{footer}</DrawerFooter> : <div className="h-[calc(var(--safe-bottom)+1rem)]" />}
        </DrawerContent>
      </Drawer>
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className={cn('rounded-2xl sm:max-w-md', className)}>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription className={cn(hideDescription || !description ? 'sr-only' : undefined)}>
            {description ?? title}
          </DialogDescription>
        </DialogHeader>
        {children}
        {footer ? <DialogFooter>{footer}</DialogFooter> : null}
      </DialogContent>
    </Dialog>
  )
}
