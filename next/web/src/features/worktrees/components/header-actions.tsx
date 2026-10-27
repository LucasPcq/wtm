import { ChevronDown, GitBranchMinus, type LucideIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { openRemove } from '@/features/worktrees/hooks/use-remove-dialog'
import { entryOf, menu, notInTheWeb, type Subject } from '@/features/worktrees/utils/actions'
import { cn } from '@/lib/utils'

// the commands that pick their own worktrees, each opening on its picker; reparent needs a subject, so it is not here
export function HeaderActions({ repo, rows }: { repo: string; rows: Subject[] }) {
  const removable = rows.filter(({ subject, offers }) => 'followUp' in entryOf(subject, offers, 'clean'))

  const item = (
    icon: LucideIcon,
    label: string,
    why: string | undefined,
    onSelect?: () => void,
    destructive = false,
  ) => {
    const Icon = icon
    return (
      // aria-disabled, not disabled: the keyboard reaches the reason
      <DropdownMenuItem
        aria-disabled={why !== undefined || undefined}
        variant={destructive ? 'destructive' : 'default'}
        className={cn(why !== undefined && 'opacity-50')}
        onSelect={(event) => {
          if (why !== undefined) return event.preventDefault()
          onSelect?.()
        }}
      >
        <Icon />
        <span className="flex flex-col">
          {label}
          {why && <span className="text-xs text-muted-foreground">{why}</span>}
        </span>
      </DropdownMenuItem>
    )
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline">
          Actions
          <ChevronDown />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-64">
        {item(GitBranchMinus, 'Prune', notInTheWeb)}
        {item(menu.fast_forward.icon, menu.fast_forward.label, notInTheWeb)}
        {item(menu.exec.icon, menu.exec.label, notInTheWeb)}
        <DropdownMenuSeparator />
        {item(
          menu.clean.icon,
          menu.clean.label,
          removable.length > 0 ? undefined : 'No worktree can be removed',
          () => openRemove(repo, { worktrees: removable.map(({ subject }) => subject), left: [], prechecked: false }),
          true,
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
