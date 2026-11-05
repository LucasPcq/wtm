import { Link } from '@tanstack/react-router'
import { ChevronDown, ChevronRight } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { toneIcon } from '@/features/worktrees/components/glyph'
import type { RunningGroup } from '@/features/worktrees/running/utils/groups'
import { jobLook } from '@/features/worktrees/running/utils/job-look'
import { cn } from '@/lib/utils'

// a window too narrow for the list beside the log folds the list into this: the job shown, and every other to pick
export function JobSwitcher({
  repo,
  groups,
  worktree,
  job,
  className,
}: {
  repo: string
  groups: RunningGroup[]
  worktree: string
  job: string
  className?: string
}) {
  return (
    <div className={cn('px-6 pt-4', className)}>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="outline" aria-label="Switch job">
            <span className="text-muted-foreground">{worktree}</span>
            <ChevronRight className="text-muted-foreground" aria-hidden="true" />
            {job}
            <ChevronDown aria-hidden="true" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-64">
          {groups.map((group) => (
            <DropdownMenuGroup key={group.worktree}>
              <DropdownMenuLabel className="text-xs text-muted-foreground">{group.worktree}</DropdownMenuLabel>
              {group.rows.flatMap((row) => {
                if (!('job' in row)) return []
                const { icon: Icon, tone } = jobLook[row.job.state ?? 'stopped']
                return [
                  <DropdownMenuItem key={row.name} asChild>
                    <Link
                      to="/r/$repo/running"
                      params={{ repo }}
                      search={{ worktree: group.worktree, job: row.name }}
                      aria-current={group.worktree === worktree && row.name === job ? 'page' : undefined}
                    >
                      <Icon className={toneIcon({ tone })} aria-hidden="true" />
                      {row.name}
                    </Link>
                  </DropdownMenuItem>,
                ]
              })}
            </DropdownMenuGroup>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  )
}
