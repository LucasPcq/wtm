import { Ellipsis, Square } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { MenuEntry } from '@/features/worktrees/components/menu-entry'
import { Rail } from '@/features/worktrees/components/rail'
import { openRunDown } from '@/features/worktrees/hooks/use-run-down-dialog'
import { ProfilesMenu } from '@/features/worktrees/running/components/profiles-menu'
import { RunningJob } from '@/features/worktrees/running/components/running-job'
import { groupLine, type RunningGroup } from '@/features/worktrees/running/utils/groups'
import { menu } from '@/features/worktrees/utils/actions'
import { named } from '@/features/worktrees/utils/run-words'

export function RunGroup({
  repo,
  group,
  now,
  selected,
}: {
  repo: string
  group: RunningGroup
  now: number
  selected: string | undefined
}) {
  const line = groupLine(group)
  const stop = () => openRunDown(repo, { worktrees: [group.worktree] })
  const stops = ['run down', group.holds.length > 0 && `releases its hold on ${named(group.holds)}`]
    .filter(Boolean)
    .join(' · ')

  return (
    <section aria-label={group.worktree} className="flex flex-col gap-1 py-3">
      <div className="flex items-start gap-3">
        <div className="flex min-w-0 flex-1 flex-col gap-0.5">
          <h2 className="truncate text-sm font-semibold">{group.worktree}</h2>
          {line && <p className="text-xs text-muted-foreground">{line}</p>}
        </div>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label={`${menu.run_down.label} of ${group.worktree}`}
              onClick={stop}
            >
              <Square />
            </Button>
          </TooltipTrigger>
          <TooltipContent>{menu.run_down.label}</TooltipContent>
        </Tooltip>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon-sm" aria-label={`More for ${group.worktree}`}>
              <Ellipsis />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-64">
            <DropdownMenuLabel className="truncate text-xs text-muted-foreground">{group.worktree}</DropdownMenuLabel>
            <MenuEntry
              Item={DropdownMenuItem}
              icon={menu.run_down.icon}
              label={menu.run_down.label}
              description={stops}
              why={undefined}
              onSelect={stop}
            />
            <ProfilesMenu repo={repo} worktree={group.worktree} />
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      <Rail elbow={16} indent={14} className="ml-2">
        {group.rows.map((row) => (
          <RunningJob
            key={row.name}
            repo={repo}
            worktree={group.worktree}
            row={row}
            now={now}
            selected={row.name === selected}
          />
        ))}
      </Rail>
    </section>
  )
}
