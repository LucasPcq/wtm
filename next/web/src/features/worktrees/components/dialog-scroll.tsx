import type { ReactNode } from 'react'
import { ScrollArea } from '@/components/ui/scroll-area'

export function DialogScroll({ children }: { children: ReactNode }) {
  return (
    // Radix lays the viewport's content out as a table, which never narrows: a long path would push the commands out
    // of sight instead of truncating
    <ScrollArea className="min-h-0 flex-1 [&_[data-slot=scroll-area-viewport]>div]:!block">
      <div className="flex flex-col gap-4 pr-3">{children}</div>
    </ScrollArea>
  )
}
