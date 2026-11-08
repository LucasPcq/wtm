import { useQuery } from '@tanstack/react-query'
import { ChevronDown, ChevronRight, Ellipsis, Square } from 'lucide-react'
import { questionsQuery } from '@/api/queries/commands'
import type { RunpsJob } from '@/api/types.gen'
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
import { QuietRow } from '@/features/worktrees/running/components/quiet-row'
import { RunningJob } from '@/features/worktrees/running/components/running-job'
import { StartJobEntry } from '@/features/worktrees/running/components/start-job-entry'
import type { RunningGroup } from '@/features/worktrees/running/utils/groups'
import { groupLine } from '@/features/worktrees/running/utils/list-words'
import { menu } from '@/features/worktrees/utils/actions'
import { named, runsSomething } from '@/features/worktrees/utils/run-words'

// a group folds on its header; a worktree's group closes on the quiet row of its ended jobs, shown on demand
export function RunGroup({
  repo,
  group,
  now,
  selected,
  folded,
  onFold,
  endedShown,
  onQuiet,
}: {
  repo: string
  group: RunningGroup
  now: number
  selected: string | undefined
  folded: boolean
  onFold: () => void
  endedShown: RunpsJob[]
  onQuiet: () => void
}) {
  const line = groupLine(group, folded)
  const title = group.shared ? 'Shared services' : group.worktree
  const Chevron = folded ? ChevronRight : ChevronDown
  const stop = () => openRunDown(repo, { worktrees: [group.worktree] })
  // what a stop of it reaches is run down's to say: its holds (main's own among them), and none of a shared service main
  // runs for the others alone
  const form = useQuery(questionsQuery(repo, 'run-down'))
  const field = form.data?.fields.find((one) => one.path === 'worktree')
  const option = (field?.kind === 'multi_select' ? field.options : []).find((one) => one.value === group.worktree)
  const why =
    option && !runsSomething(option)
      ? `Runs ${named(group.rows.map((row) => row.name))} for the other worktrees only`
      : undefined

  return (
    <section aria-label={title} className="flex flex-col gap-1 py-3">
      <div className="flex items-start gap-1">
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={folded ? `Open ${title}` : `Fold ${title}`}
          aria-expanded={!folded}
          onClick={onFold}
          className="-ml-2"
        >
          <Chevron />
        </Button>
        <div className="flex min-w-0 flex-1 flex-col gap-0.5 pt-1">
          <h2 className="truncate text-sm font-semibold">{title}</h2>
          {line && <p className="text-xs text-muted-foreground">{line}</p>}
        </div>
        {!group.shared && (
          <>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  aria-label={`${menu.run_down.label} of ${group.worktree}`}
                  aria-disabled={why !== undefined}
                  className="aria-disabled:cursor-not-allowed aria-disabled:opacity-50"
                  onClick={() => why === undefined && stop()}
                >
                  <Square />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{why ?? menu.run_down.label}</TooltipContent>
            </Tooltip>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon-sm" aria-label={`More for ${group.worktree}`}>
                  <Ellipsis />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-64">
                <DropdownMenuLabel className="truncate text-xs text-muted-foreground">
                  {group.worktree}
                </DropdownMenuLabel>
                <StopEntry holds={option?.holds ?? []} why={why} onSelect={stop} />
                <ProfilesMenu repo={repo} worktree={group.worktree} />
                <StartJobEntry repo={repo} worktree={group.worktree} />
              </DropdownMenuContent>
            </DropdownMenu>
          </>
        )}
      </div>
      {!folded && (
        <Rail elbow={16} indent={14} className="ml-2">
          {[...group.rows, ...endedShown.map((job) => ({ name: job.name, job }))].map((row) => (
            <RunningJob
              key={row.name}
              repo={repo}
              worktree={group.worktree}
              row={row}
              now={now}
              selected={row.name === selected}
            />
          ))}
          {group.ended.length > 0 && <QuietRow ended={group.ended} shown={endedShown.length > 0} onToggle={onQuiet} />}
        </Rail>
      )}
    </section>
  )
}

function StopEntry({ holds, why, onSelect }: { holds: string[]; why: string | undefined; onSelect: () => void }) {
  const description = ['run down', holds.length > 0 && `releases its hold on ${named(holds)}`]
    .filter(Boolean)
    .join(' · ')

  return (
    <MenuEntry
      Item={DropdownMenuItem}
      icon={menu.run_down.icon}
      label={menu.run_down.label}
      description={description}
      why={why}
      onSelect={onSelect}
    />
  )
}
