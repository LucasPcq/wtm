import { ChevronRight } from 'lucide-react'
import type { ReactNode } from 'react'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { cn } from '@/lib/utils'

export function Details({
  label,
  summary,
  quiet = false,
  flush = false,
  children,
}: {
  label: string
  summary?: string | undefined
  quiet?: boolean
  flush?: boolean
  children: ReactNode
}) {
  return (
    <Collapsible className="flex flex-col gap-3">
      <CollapsibleTrigger className="group flex w-fit items-center gap-1.5 text-sm">
        <ChevronRight className="size-4 text-muted-foreground transition-transform group-data-[state=open]:rotate-90 motion-reduce:transition-none" />
        <span className={cn(quiet ? 'text-muted-foreground' : 'font-medium')}>{label}</span>
        {summary && <span className="text-muted-foreground">· {summary}</span>}
      </CollapsibleTrigger>
      <CollapsibleContent className={cn('flex flex-col gap-4', flush ? 'pt-1' : 'pl-[22px]')}>
        {children}
      </CollapsibleContent>
    </Collapsible>
  )
}
