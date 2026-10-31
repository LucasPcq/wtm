import { Ellipsis } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { ProfilesMenu } from '@/features/worktrees/running/components/profiles-menu'
import { RunningJob } from '@/features/worktrees/running/components/running-job'
import { groupLine, type RunningGroup } from '@/features/worktrees/running/utils/groups'

export function RunGroup({ repo, group, now }: { repo: string; group: RunningGroup; now: number }) {
  const line = groupLine(group)

  return (
    <section aria-label={group.worktree} className="flex flex-col gap-1 py-3">
      <div className="flex items-start gap-3">
        <div className="flex min-w-0 flex-1 flex-col gap-0.5">
          <h2 className="truncate text-sm font-semibold">{group.worktree}</h2>
          {line && <p className="text-xs text-muted-foreground">{line}</p>}
        </div>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon-sm" aria-label={`More for ${group.worktree}`}>
              <Ellipsis />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-64">
            <DropdownMenuLabel className="truncate text-xs text-muted-foreground">
              {[group.worktree, group.profile && `profile ${group.profile}`].filter(Boolean).join(' · ')}
            </DropdownMenuLabel>
            <ProfilesMenu repo={repo} worktree={group.worktree} />
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      <ul className="ml-1.5 flex flex-col border-l pl-4">
        {group.rows.map((row) => (
          <RunningJob key={row.name} row={row} now={now} />
        ))}
      </ul>
    </section>
  )
}
