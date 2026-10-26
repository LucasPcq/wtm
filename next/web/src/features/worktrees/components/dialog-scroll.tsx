import type { ReactNode } from 'react'
import { ScrollArea } from '@/components/ui/scroll-area'

export function DialogScroll({ children }: { children: ReactNode }) {
  return (
    // Radix lays the viewport's content out as a table, which never narrows: a long path would push the commands out
    // of sight instead of truncating. The viewport's 100% height means nothing in a dialog that has only a max height,
    // so the area is a column the viewport fills. It spans the dialog's padding, so its bar sits in the gutter and the
    // content keeps the header's edges.
    <ScrollArea className="-mx-6 flex min-h-0 flex-1 flex-col [&_[data-slot=scroll-area-viewport]>div]:!block [&>[data-slot=scroll-area-viewport]]:min-h-0 [&>[data-slot=scroll-area-viewport]]:flex-1">
      <div className="flex flex-col gap-6 px-6">{children}</div>
    </ScrollArea>
  )
}
