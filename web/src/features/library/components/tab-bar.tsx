import type { ReactNode } from 'react'

import { TabsList, TabsTrigger } from '@/components/ui/tabs'
import { cn } from '@/lib/utils'

export interface TabItem<T extends string> {
  value: T
  label: ReactNode
}

/**
 * Segmented control (iOS style) for page tabs; full width on phones. Render inside `<Tabs>`.
 */
export function SegmentedTabs<T extends string>({ items, className }: { items: readonly TabItem<T>[]; className?: string }) {
  return (
    <TabsList className={cn('mb-5 h-9 w-full rounded-[10px] p-0.5 md:mb-6 md:w-fit', className)}>
      {items.map((item) => (
        <TabsTrigger
          key={item.value}
          value={item.value}
          className="h-full rounded-[8px] px-4 text-[13px] font-semibold data-[state=active]:shadow-sm md:min-w-24"
        >
          {item.label}
        </TabsTrigger>
      ))}
    </TabsList>
  )
}
