import { ChevronRight } from 'lucide-react'
import type { ReactNode } from 'react'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { cn } from '@/lib/utils'

// what most people never read, one deliberate click away: the hooks and folders of a plan, the steps of a run
export function Details({
  label,
  summary,
  quiet = false,
  children,
}: {
  label: string
  summary?: string | undefined
  quiet?: boolean
  children: ReactNode
}) {
  return (
    <Collapsible className="flex flex-col gap-3">
      <CollapsibleTrigger className="group flex w-fit items-center gap-1.5 text-sm">
        <ChevronRight className="size-4 text-muted-foreground transition-transform group-data-[state=open]:rotate-90 motion-reduce:transition-none" />
        <span className={cn(quiet ? 'text-muted-foreground' : 'font-medium')}>{label}</span>
        {summary && <span className="text-muted-foreground">· {summary}</span>}
      </CollapsibleTrigger>
      <CollapsibleContent className="flex flex-col gap-4 pl-[22px]">{children}</CollapsibleContent>
    </Collapsible>
  )
}
