import type { ReactNode } from 'react'
import { DialogContent } from '@/components/ui/dialog'
import { cn } from '@/lib/utils'

// a run keeps one size while its phases come and go; once it ended, the dialog eases to its content
export function RunDialogContent({ running, children }: { running: boolean; children: ReactNode }) {
  return (
    <DialogContent
      className={cn(
        'flex max-h-[90vh] flex-col gap-6 p-6 transition-[height] duration-300 ease-out [interpolate-size:allow-keywords] sm:max-w-xl motion-reduce:transition-none',
        // the frame pads 24 where shadcn pads 16: its footer and close button follow; its title is 4 above its line
        '[&_[data-slot=dialog-header]]:gap-1 [&_[data-slot=dialog-footer]]:-mx-6 [&_[data-slot=dialog-footer]]:-mb-6 [&_[data-slot=dialog-footer]]:px-6 [&_[data-slot=dialog-footer]]:py-4 [&>button.absolute]:top-4 [&>button.absolute]:right-4',
        running && 'h-[min(36rem,85vh)]',
      )}
      showCloseButton={!running}
    >
      {children}
    </DialogContent>
  )
}
