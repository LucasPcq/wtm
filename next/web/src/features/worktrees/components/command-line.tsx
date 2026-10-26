import { useQuery } from '@tanstack/react-query'
import { Copy } from 'lucide-react'
import { commandLineQuery } from '@/api/queries/command-line'
import type { DomainFollowUp } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { copy } from '@/features/worktrees/utils/copy'
import { cn } from '@/lib/utils'

// copy is the button alone: the fix in one click from a line that folds the command away
type Look = 'block' | 'inline' | 'copy'

export function CommandLine({ followUp, look = 'block' }: { followUp: DomainFollowUp; look?: Look }) {
  const line = useQuery(commandLineQuery(followUp))
  const inline = look === 'inline'
  const text = line.data

  const copyButton = (
    <Button
      variant="ghost"
      size={inline ? 'icon-xs' : 'icon-sm'}
      disabled={!text}
      aria-label={text ? `Copy ${text}` : 'Copy the command'}
      onClick={() => text && copy(text)}
    >
      <Copy />
    </Button>
  )

  if (look === 'copy') return copyButton
  if (line.isPending) return <Skeleton className={inline ? 'h-7 w-36' : 'h-10 w-full'} />

  if (line.isError || !text) {
    const why = line.isError ? ` (${line.error.message})` : ''
    return (
      <span className={cn('text-muted-foreground', inline ? 'text-xs' : 'block rounded-md bg-muted px-3 py-2 text-sm')}>
        No command line for this{why}: {followUp.message}
      </span>
    )
  }

  if (!inline)
    return (
      <div className="flex items-center gap-2 rounded-md bg-muted py-1 pr-1 pl-3">
        <code className="min-w-0 flex-1 font-mono text-[13px] break-all">{text}</code>
        {copyButton}
      </div>
    )

  return (
    <span className="flex shrink-0 items-center gap-0.5 rounded-md bg-muted py-0.5 pr-0.5 pl-2">
      <code className="font-mono text-xs">{text}</code>
      <Tooltip>
        <TooltipTrigger asChild>{copyButton}</TooltipTrigger>
        <TooltipContent>Not in the web interface yet: copy it into a terminal.</TooltipContent>
      </Tooltip>
    </span>
  )
}
