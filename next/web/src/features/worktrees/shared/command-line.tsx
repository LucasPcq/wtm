import { useQuery } from '@tanstack/react-query'
import { Copy } from 'lucide-react'
import { commandLineQuery } from '@/api/queries/command-line'
import type { DomainFollowUp } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { shellLine } from '@/lib/shell'
import { copy } from './copy'

// CommandLine is a follow-up the web cannot run, as a command line to copy into a terminal; one the command line
// cannot spell either says so, with what it meant to do.
export function CommandLine({ followUp }: { followUp: DomainFollowUp }) {
  const line = useQuery(commandLineQuery(followUp))
  if (line.isPending) return <Skeleton className="h-10 w-full" />
  if (line.isError || line.data.argv.length === 0)
    return (
      <p className="rounded-md bg-muted px-3 py-2 text-sm text-muted-foreground">
        No command line for this{line.isError ? ` (${line.error.message})` : ''}: {followUp.message}
      </p>
    )

  const text = shellLine(line.data.argv)
  return (
    <div className="flex items-center gap-2 rounded-md bg-muted py-1 pr-1 pl-3">
      <code className="min-w-0 flex-1 font-mono text-[13px] break-all">{text}</code>
      <Button variant="ghost" size="icon-sm" aria-label={`Copy ${text}`} onClick={() => copy(text)}>
        <Copy />
      </Button>
    </div>
  )
}

// InlineCommand is a follow-up as a short command beside what it is about, the why of copying it in its tooltip.
export function InlineCommand({ followUp }: { followUp: DomainFollowUp }) {
  const line = useQuery(commandLineQuery(followUp))
  if (line.isPending) return <Skeleton className="h-7 w-36" />
  if (line.isError || line.data.argv.length === 0)
    return (
      <span className="text-xs text-muted-foreground">
        No command line for this{line.isError ? ` (${line.error.message})` : ''}: {followUp.message}
      </span>
    )

  const text = shellLine(line.data.argv)
  return (
    <span className="flex shrink-0 items-center gap-0.5 rounded-md bg-muted py-0.5 pr-0.5 pl-2">
      <code className="font-mono text-xs">{text}</code>
      <Tooltip>
        <TooltipTrigger asChild>
          <Button variant="ghost" size="icon-xs" aria-label={`Copy ${text}`} onClick={() => copy(text)}>
            <Copy />
          </Button>
        </TooltipTrigger>
        <TooltipContent>Not in the web interface yet: copy it into a terminal.</TooltipContent>
      </Tooltip>
    </span>
  )
}
