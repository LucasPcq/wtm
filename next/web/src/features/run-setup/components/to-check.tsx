import { ChevronDown, TriangleAlert } from 'lucide-react'
import type { ReactNode } from 'react'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'

// what the core says to check in a part, at its top: one line each, its fix beside it, the fix's effect under it
export function ToCheck({ children, count }: { children: ReactNode; count: number }) {
  if (count === 0) return null

  return (
    <Collapsible defaultOpen className="flex flex-col gap-3">
      <CollapsibleTrigger className="group flex items-center gap-2 self-start text-sm text-muted-foreground">
        <ChevronDown className="size-4 transition-transform group-data-[state=closed]:-rotate-90" aria-hidden="true" />
        To check · {count}
      </CollapsibleTrigger>
      <CollapsibleContent className="flex flex-col gap-4">{children}</CollapsibleContent>
    </Collapsible>
  )
}

export function CheckRow({
  subject,
  said,
  action,
  children,
}: {
  subject: string
  said: string
  action?: ReactNode
  children?: ReactNode
}) {
  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-3">
        <TriangleAlert className="size-4 shrink-0 text-amber-500" aria-hidden="true" />
        <span className="font-medium">{subject}</span>
        <span className="flex-1 text-sm text-muted-foreground">{said}</span>
        {action}
      </div>
      {children && <div className="pl-7 text-sm">{children}</div>}
    </div>
  )
}
