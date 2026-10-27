import { ChevronDown, GitBranchMinus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { MenuEntry } from '@/features/worktrees/components/menu-entry'
import { openPrune } from '@/features/worktrees/hooks/use-prune-dialog'
import { openRemove } from '@/features/worktrees/hooks/use-remove-dialog'
import { entryOf, menu, notInTheWeb, type Subject } from '@/features/worktrees/utils/actions'

// reparent is not here: it needs a subject
export function HeaderActions({ repo, rows }: { repo: string; rows: Subject[] }) {
  const removable = rows.filter(({ subject, offers }) => 'followUp' in entryOf(subject, offers, 'clean'))

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
          icon={GitBranchMinus}
          label="Prune"
          why={undefined}
          onSelect={() => openPrune(repo, [])}
        />
        {(['fast_forward', 'exec'] as const).map((action) => (
          <MenuEntry
            key={action}
            Item={DropdownMenuItem}
            icon={menu[action].icon}
            label={menu[action].label}
            why={notInTheWeb}
          />
        ))}
        <DropdownMenuSeparator />
        <MenuEntry
          Item={DropdownMenuItem}
          icon={menu.clean.icon}
          label={menu.clean.label}
          why={removable.length > 0 ? undefined : 'No worktree can be removed'}
          destructive
          onSelect={() =>
            openRemove(repo, { worktrees: removable.map(({ subject }) => subject), left: [], prechecked: false })
          }
        />
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
