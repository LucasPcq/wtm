import type { ReactNode } from 'react'
import { DialogContent } from '@/components/ui/dialog'
import { cn } from '@/lib/utils'

// a run keeps one size while its phases come and go, only its lists scroll; once it ended, the dialog eases to what
// it has left to say
export function RunDialogContent({ running, children }: { running: boolean; children: ReactNode }) {
  return (
    <DialogContent
      className={cn(
        'flex max-h-[90vh] flex-col gap-6 transition-[height] duration-300 ease-out [interpolate-size:allow-keywords] sm:max-w-xl motion-reduce:transition-none',
        running && 'h-[min(36rem,85vh)]',
      )}
      showCloseButton={!running}
    >
      {children}
    </DialogContent>
  )
}
