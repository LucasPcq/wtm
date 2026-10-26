import { useQueries } from '@tanstack/react-query'
import { Ellipsis, Trash2 } from 'lucide-react'
import { commandLineQuery } from '@/api/queries/command-line'
import type { DomainFollowUp } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { openRemove } from '@/features/worktrees/hooks/use-remove-dialog'
import { copy } from '@/features/worktrees/utils/copy'

export function ActionsMenu({ repo, actions }: { repo: string; actions: DomainFollowUp[] }) {
  // clean runs here, through the catalogue: the others are still copied to the terminal
  const removal = actions.find((action) => action.command === 'clean')
  const copied = actions.filter((action) => action !== removal)
  const removed = removal?.answers?.find((answer) => answer.path === 'worktree')?.values ?? []
  const lines = useQueries({ queries: copied.map(commandLineQuery) })

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon-sm" aria-label="More actions">
          <Ellipsis />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-80">
        <DropdownMenuLabel className="font-normal text-muted-foreground">
          Not in the web interface yet: click to copy the command.
        </DropdownMenuLabel>
        {copied.map((action, i) => {
          const { data: line, isPending } = lines[i] ?? { isPending: true }
          return (
            <DropdownMenuItem
              key={action.code}
              disabled={!line}
              onSelect={() => line && copy(line)}
              className="flex-col items-start gap-0"
            >
              <span>{action.message}</span>
              <code className="font-mono text-xs text-muted-foreground">
                {line ?? (isPending ? '…' : 'no command line for this')}
              </code>
            </DropdownMenuItem>
          )
        })}
        {removed.length > 0 && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem variant="destructive" onSelect={() => openRemove(repo, removed)}>
              <Trash2 />
              Remove worktree…
            </DropdownMenuItem>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
