import { Link } from '@tanstack/react-router'
import { ChevronDown, ChevronRight } from 'lucide-react'
import type { RunState } from '@/api/types.gen'
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
import type { Pinned } from '@/features/worktrees/running/utils/visible'

// a window too narrow for the list beside the log folds the list into this: the job shown, and every other to pick
export function JobSwitcher({
  repo,
  pinned,
  groups,
  worktree,
  job,
  state,
  className,
}: {
  repo: string
  pinned: Pinned[]
  groups: RunningGroup[]
  worktree: string
  job: string
  state: RunState
  className?: string
}) {
  const { icon: Shown, tone: shownTone } = jobLook[state]
  const item = (at: string, name: string, state: RunState) => {
    const { icon: Icon, tone } = jobLook[state]
    return (
      <DropdownMenuItem key={`${at}/${name}`} asChild>
        <Link
          to="/r/$repo/running"
          params={{ repo }}
          search={{ worktree: at, job: name }}
          aria-current={at === worktree && name === job ? 'page' : undefined}
        >
          <Icon className={toneIcon({ tone })} aria-hidden="true" />
          {name}
        </Link>
      </DropdownMenuItem>
    )
  }

  return (
    <div className={className}>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="outline" aria-label="Switch job">
            <span className="text-muted-foreground">{worktree}</span>
            <ChevronRight className="text-muted-foreground" aria-hidden="true" />
            <Shown className={toneIcon({ tone: shownTone })} aria-hidden="true" />
            {job}
            <ChevronDown aria-hidden="true" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-64">
          {pinned.length > 0 && (
            <DropdownMenuGroup>
              <DropdownMenuLabel className="text-xs text-muted-foreground">Pinned</DropdownMenuLabel>
              {pinned.map(({ group, row }) => item(group.worktree, row.name, row.job.state))}
            </DropdownMenuGroup>
          )}
          {groups.map((group) => (
            <DropdownMenuGroup key={group.key}>
              <DropdownMenuLabel className="text-xs text-muted-foreground">
                {group.shared ? 'Shared services' : group.worktree}
              </DropdownMenuLabel>
              {group.rows.flatMap((row) => ('job' in row ? [item(group.worktree, row.name, row.job.state)] : []))}
            </DropdownMenuGroup>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  )
}
