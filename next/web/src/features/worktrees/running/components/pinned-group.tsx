import { ChevronDown, ChevronRight, Ellipsis, PinOff } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Rail } from '@/features/worktrees/components/rail'
import { RunningJob } from '@/features/worktrees/running/components/running-job'
import { type Pinned, pinOf } from '@/features/worktrees/running/utils/visible'

// the jobs a viewer pinned, on top and out of their groups, each saying its worktree; it folds as a group does
export function PinnedGroup({
  repo,
  pinned,
  now,
  selected,
  folded,
  onFold,
  onUnpinAll,
}: {
  repo: string
  pinned: Pinned[]
  now: number
  selected: string | undefined
  folded: boolean
  onFold: () => void
  onUnpinAll: () => void
}) {
  const Chevron = folded ? ChevronRight : ChevronDown

  return (
    <section aria-label="Pinned" className="flex flex-col gap-1 py-3">
      <div className="flex items-start gap-1">
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={folded ? 'Open Pinned' : 'Fold Pinned'}
          aria-expanded={!folded}
          onClick={onFold}
          className="-ml-2"
        >
          <Chevron />
        </Button>
        <div className="flex min-w-0 flex-1 flex-col gap-0.5 pt-1">
          <h2 className="text-sm font-semibold">Pinned</h2>
          <p className="text-xs text-muted-foreground">stays on top, for you</p>
        </div>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon-sm" aria-label="More for Pinned">
              <Ellipsis />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onSelect={onUnpinAll}>
              <PinOff />
              Unpin all
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      {!folded && (
        <Rail elbow={16} indent={14} className="ml-2">
          {pinned.map(({ group, row }) => (
            <RunningJob
              key={pinOf(group.worktree, row.name)}
              repo={repo}
              worktree={group.worktree}
              row={row}
              now={now}
              selected={selected === pinOf(group.worktree, row.name)}
              pinned
            />
          ))}
        </Rail>
      )}
    </section>
  )
}
