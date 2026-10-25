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
import { shellLine } from '@/lib/shell'
import { copy } from '../shared/copy'

// ActionsMenu is what may be done next with a worktree that the web does not run yet: each item copies its command
// line, which it shows; the last one, which deletes it, apart.
export function ActionsMenu({ actions }: { actions: DomainFollowUp[] }) {
  const lines = useQueries({ queries: actions.map(commandLineQuery) }).map((line) => line.data?.argv)
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
          const argv = lines[i]
          const line = argv && argv.length > 0 ? shellLine(argv) : undefined
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
                {line ?? (argv ? 'no command line for this' : '…')}
              </code>
            </DropdownMenuItem>,
          ]
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
