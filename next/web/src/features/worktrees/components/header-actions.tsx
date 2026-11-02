import { ChevronDown } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { MenuEntry } from '@/features/worktrees/components/menu-entry'
import { openExec } from '@/features/worktrees/hooks/use-exec-dialog'
import { openFastForward } from '@/features/worktrees/hooks/use-fast-forward-dialog'
import { openPrune } from '@/features/worktrees/hooks/use-prune-dialog'
import { openRemove } from '@/features/worktrees/hooks/use-remove-dialog'
import { openSync } from '@/features/worktrees/hooks/use-sync-dialog'
import { entryOf, menu, noneWords, pruneAction, type Subject } from '@/features/worktrees/utils/actions'

// reparent is not here: it needs a subject
// needsSync: the read on screen says a worktree needs a sync
export function HeaderActions({ repo, rows, needsSync }: { repo: string; rows: Subject[]; needsSync: boolean }) {
  const removable = rows.filter(({ subject, offers }) => 'followUp' in entryOf(subject, offers, 'clean'))
  const onOrigin = rows.some(({ subject, offers }) => 'followUp' in entryOf(subject, offers, 'fast_forward'))

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline">
          Actions
          <ChevronDown />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-64">
        <MenuEntry
          Item={DropdownMenuItem}
          icon={pruneAction.icon}
          label={pruneAction.label}
          why={undefined}
          onSelect={() => openPrune(repo, [])}
        />
        {/* the picker opens prechecked by the command: the worktrees behind origin */}
        <MenuEntry
          Item={DropdownMenuItem}
          icon={menu.fast_forward.icon}
          label={menu.fast_forward.label}
          why={onOrigin ? undefined : noneWords.fast_forward}
          onSelect={() => openFastForward(repo, { worktrees: [], left: [], picking: true })}
        />
        {/* the tree opens with every stack, the command's own pick checked: the worktrees that need a sync */}
        <MenuEntry
          Item={DropdownMenuItem}
          icon={menu.sync.icon}
          label={menu.sync.label}
          why={needsSync ? undefined : noneWords.sync}
          onSelect={() => openSync(repo, { answers: [], left: [], picking: true, deciding: true })}
        />
        <MenuEntry
          Item={DropdownMenuItem}
          icon={menu.exec.icon}
          label={menu.exec.label}
          why={undefined}
          onSelect={() => openExec(repo, { worktrees: [], left: [] })}
        />
        <DropdownMenuSeparator />
        <MenuEntry
          Item={DropdownMenuItem}
          icon={menu.clean.icon}
          label={menu.clean.label}
          why={removable.length > 0 ? undefined : noneWords.clean}
          destructive
          onSelect={() =>
            openRemove(repo, { worktrees: removable.map(({ subject }) => subject), left: [], prechecked: false })
          }
        />
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
