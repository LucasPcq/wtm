import { useQueries } from '@tanstack/react-query'
import { Ellipsis } from 'lucide-react'
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
import { copy } from '@/features/worktrees/shared/copy'

export function ActionsMenu({ actions }: { actions: DomainFollowUp[] }) {
  const lines = useQueries({ queries: actions.map(commandLineQuery) })
  const last = actions.at(-1)

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
        {actions.map((action, i) => {
          const { data: line, isPending } = lines[i] ?? { isPending: true }
          const destructive = action === last && action.command === 'clean'
          return [
            destructive && <DropdownMenuSeparator key={`${action.code}-separator`} />,
            <DropdownMenuItem
              key={action.code}
              variant={destructive ? 'destructive' : 'default'}
              disabled={!line}
              onSelect={() => line && copy(line)}
              className="flex-col items-start gap-0"
            >
              <span>{action.message}</span>
              <code className="font-mono text-xs text-muted-foreground">
                {line ?? (isPending ? '…' : 'no command line for this')}
              </code>
            </DropdownMenuItem>,
          ]
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
